// Package legacy implements the Tapo legacy transport protocol using RSA
// handshake and AES-CBC securePassthrough.
package legacy

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"sync"
	"time"

	"github.com/mjenh/skills/tapo/internal/crypto"
	"github.com/mjenh/skills/tapo/internal/transport"
)

// Compile-time interface check.
var _ transport.Transport = (*Transport)(nil)

// Wire-format structs.

type handshakeRequest struct {
	Method string `json:"method"`
	Params struct {
		Key string `json:"key"`
	} `json:"params"`
}

type handshakeResponse struct {
	ErrorCode int `json:"error_code"`
	Result    struct {
		Key string `json:"key"`
	} `json:"result"`
}

type securePassthroughRequest struct {
	Method string `json:"method"`
	Params struct {
		Request string `json:"request"`
	} `json:"params"`
}

type securePassthroughResponse struct {
	ErrorCode int `json:"error_code"`
	Result    struct {
		Response string `json:"response"`
	} `json:"result"`
}

type loginRequest struct {
	Method          string `json:"method"`
	Params          struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"params"`
	RequestTimeMils int64 `json:"requestTimeMils"`
}

type deviceResponse struct {
	ErrorCode int              `json:"error_code"`
	Result    json.RawMessage  `json:"result"`
}

// Transport implements the legacy Tapo protocol.
type Transport struct {
	host   string
	client *http.Client
	mu     sync.Mutex
	aesKey []byte
	aesIV  []byte
	token  string
}

// New creates a new legacy Transport for the given host and timeout.
func New(host string, timeout time.Duration) *Transport {
	// cookiejar.New with nil options never returns an error.
	jar, _ := cookiejar.New(nil)
	return &Transport{
		host: host,
		client: &http.Client{
			Timeout: timeout,
			Jar:     jar,
		},
	}
}

// handshake performs the RSA key exchange with the device.
// It uses RSA-1024 because the Tapo legacy protocol requires it — the device
// only accepts 1024-bit client public keys. The key is ephemeral (generated
// per-login) and wraps only a 32-byte AES key/IV, so the bounded risk is
// acceptable. Go 1.24 enforces a 1024-bit minimum; future Go versions may
// raise the floor.
func (t *Transport) handshake(ctx context.Context) error {
	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return fmt.Errorf("legacy: rsa key generation: %w", transport.ErrHandshake)
	}

	pubDER := x509.MarshalPKCS1PublicKey(&privateKey.PublicKey)
	pubB64 := base64.StdEncoding.EncodeToString(pubDER)
	pemKey := "-----BEGIN PUBLIC KEY-----\n" + pubB64 + "\n-----END PUBLIC KEY-----\n"

	req := handshakeRequest{Method: "handshake"}
	req.Params.Key = pemKey

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("legacy: marshal handshake request: %w", transport.ErrHandshake)
	}

	url := fmt.Sprintf("http://%s/app", t.host)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("legacy: create handshake request: %w", transport.ErrHandshake)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("legacy: handshake request: %w", transport.ErrHandshake)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("legacy: read handshake response: %w", transport.ErrHandshake)
	}

	var hsResp handshakeResponse
	if err := json.Unmarshal(respBody, &hsResp); err != nil {
		return fmt.Errorf("legacy: unmarshal handshake response: %w", transport.ErrHandshake)
	}

	if hsResp.ErrorCode != 0 {
		return fmt.Errorf("legacy: handshake error_code %d: %w", hsResp.ErrorCode, transport.ErrHandshake)
	}

	encryptedKey, err := base64.StdEncoding.DecodeString(hsResp.Result.Key)
	if err != nil {
		return fmt.Errorf("legacy: decode handshake key: %w", transport.ErrHandshake)
	}

	keyMaterial, err := rsa.DecryptPKCS1v15(rand.Reader, privateKey, encryptedKey)
	if err != nil {
		return fmt.Errorf("legacy: decrypt handshake key: %w", transport.ErrHandshake)
	}

	if len(keyMaterial) != 32 {
		return fmt.Errorf("legacy: unexpected key material length %d: %w", len(keyMaterial), transport.ErrHandshake)
	}

	// Persist key material under mutex.
	t.mu.Lock()
	t.aesKey = keyMaterial[:16]
	t.aesIV = keyMaterial[16:]
	t.mu.Unlock()

	return nil
}

// securePassthrough encrypts an inner JSON request, sends it via the
// securePassthrough method, and returns the decrypted inner response.
// Session state is snapshotted under a short lock; network I/O is unlocked.
func (t *Transport) securePassthrough(ctx context.Context, innerJSON []byte) (json.RawMessage, error) {
	// Snapshot session state under short lock.
	t.mu.Lock()
	aesKey := make([]byte, len(t.aesKey))
	copy(aesKey, t.aesKey)
	aesIV := make([]byte, len(t.aesIV))
	copy(aesIV, t.aesIV)
	token := t.token
	t.mu.Unlock()

	ciphertext, err := crypto.Encrypt(aesKey, aesIV, innerJSON)
	if err != nil {
		return nil, fmt.Errorf("legacy: encrypt request: %w", err)
	}

	encoded := base64.StdEncoding.EncodeToString(ciphertext)

	req := securePassthroughRequest{Method: "securePassthrough"}
	req.Params.Request = encoded

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("legacy: marshal securePassthrough request: %w", err)
	}

	url := fmt.Sprintf("http://%s/app", t.host)
	if token != "" {
		url += "?token=" + token
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("legacy: create securePassthrough request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("legacy: securePassthrough request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("legacy: read securePassthrough response: %w", err)
	}

	var spResp securePassthroughResponse
	if err := json.Unmarshal(respBody, &spResp); err != nil {
		return nil, fmt.Errorf("legacy: unmarshal securePassthrough response: %w", err)
	}

	if spResp.ErrorCode != 0 {
		return nil, fmt.Errorf("legacy: securePassthrough error_code %d", spResp.ErrorCode)
	}

	decoded, err := base64.StdEncoding.DecodeString(spResp.Result.Response)
	if err != nil {
		return nil, fmt.Errorf("legacy: decode securePassthrough response: %w", err)
	}

	plaintext, err := crypto.Decrypt(aesKey, aesIV, decoded)
	if err != nil {
		return nil, fmt.Errorf("legacy: decrypt securePassthrough response: %w", err)
	}

	return json.RawMessage(plaintext), nil
}

// Login performs the legacy handshake and login_device sequence.
// The mutex is held only while persisting session state, not during network
// I/O, matching the pattern established by the KLAP transport.
func (t *Transport) Login(ctx context.Context, email, password string) error {
	if err := t.handshake(ctx); err != nil {
		return err
	}

	lr := loginRequest{
		Method:          "login_device",
		RequestTimeMils: time.Now().UnixMilli(),
	}
	lr.Params.Username = crypto.LegacyLoginHash(email)
	lr.Params.Password = base64.StdEncoding.EncodeToString([]byte(password))

	innerJSON, err := json.Marshal(lr)
	if err != nil {
		return fmt.Errorf("legacy: marshal login request: %w", err)
	}

	result, err := t.securePassthrough(ctx, innerJSON)
	if err != nil {
		return fmt.Errorf("legacy: login request: %w", err)
	}

	var devResp deviceResponse
	if err := json.Unmarshal(result, &devResp); err != nil {
		return fmt.Errorf("legacy: unmarshal login response: %w", err)
	}

	if devResp.ErrorCode == -1501 {
		return fmt.Errorf("legacy: invalid credentials: %w", transport.ErrAuth)
	}

	if devResp.ErrorCode != 0 {
		return fmt.Errorf("legacy: login error_code %d", devResp.ErrorCode)
	}

	// Extract token from result.
	var tokenResult struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(devResp.Result, &tokenResult); err != nil {
		return fmt.Errorf("legacy: unmarshal login token: %w", err)
	}

	// Persist session state under mutex.
	t.mu.Lock()
	t.token = tokenResult.Token
	t.mu.Unlock()

	return nil
}

// Send sends a command to the device via securePassthrough.
// The mutex is held only to snapshot/check session state, not during I/O.
func (t *Transport) Send(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
	t.mu.Lock()
	if t.token == "" {
		t.mu.Unlock()
		return nil, fmt.Errorf("legacy: not logged in: %w", transport.ErrAuth)
	}
	t.mu.Unlock()

	var innerJSON []byte
	var err error

	if len(payload) == 0 {
		innerJSON, err = json.Marshal(map[string]string{"method": method})
	} else {
		innerJSON, err = json.Marshal(map[string]interface{}{
			"method": method,
			"params": json.RawMessage(payload),
		})
	}
	if err != nil {
		return nil, fmt.Errorf("legacy: marshal command: %w", err)
	}

	result, err := t.securePassthrough(ctx, innerJSON)
	if err != nil {
		return nil, fmt.Errorf("legacy: send command: %w", err)
	}

	var devResp deviceResponse
	if err := json.Unmarshal(result, &devResp); err != nil {
		return nil, fmt.Errorf("legacy: unmarshal command response: %w", err)
	}

	if devResp.ErrorCode == 9999 {
		return nil, fmt.Errorf("legacy: session expired: %w", transport.ErrSessionExpired)
	}

	if devResp.ErrorCode != 0 {
		return nil, fmt.Errorf("legacy: command error_code %d", devResp.ErrorCode)
	}

	return devResp.Result, nil
}
