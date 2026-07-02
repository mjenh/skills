package klap

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/mjenh/skills/tapo/internal/crypto"
	"github.com/mjenh/skills/tapo/internal/transport"
)

// Compile-time interface check.
var _ transport.Transport = (*Transport)(nil)

// Transport implements the KLAP protocol for communicating with Tapo devices.
type Transport struct {
	host   string
	client *http.Client

	mu       sync.Mutex
	cookie   *http.Cookie
	key      []byte
	iv       []byte // 12-byte IV base
	sig      []byte // 32-byte signature base
	seq      uint32
	loggedIn bool
}

// New creates a new KLAP transport for the given host.
// If client is nil, a default http.Client is used.
func New(host string, client *http.Client) *Transport {
	if client == nil {
		client = &http.Client{}
	}
	return &Transport{
		host:   host,
		client: client,
	}
}

// Login performs the KLAP handshake to authenticate with the device.
func (t *Transport) Login(ctx context.Context, email, password string) error {
	authHash := crypto.KLAPAuthHash(email, password)

	localSeed := make([]byte, 16)
	if _, err := rand.Read(localSeed); err != nil {
		return fmt.Errorf("klap: failed to generate local seed: %w", err)
	}

	// Handshake1
	remoteSeed, serverHash, cookie, err := t.handshake1(ctx, localSeed)
	if err != nil {
		return err
	}

	// Verify server hash: SHA256(localSeed + remoteSeed + authHash)
	h := sha256.New()
	h.Write(localSeed)
	h.Write(remoteSeed)
	h.Write(authHash)
	expectedHash := h.Sum(nil)

	if len(serverHash) != len(expectedHash) {
		return fmt.Errorf("klap: invalid credentials: %w", transport.ErrAuth)
	}
	for i := range expectedHash {
		if serverHash[i] != expectedHash[i] {
			return fmt.Errorf("klap: invalid credentials: %w", transport.ErrAuth)
		}
	}

	// Handshake2: send SHA256(remoteSeed + localSeed + authHash)
	h2 := sha256.New()
	h2.Write(remoteSeed)
	h2.Write(localSeed)
	h2.Write(authHash)
	clientHash := h2.Sum(nil)

	if err := t.handshake2(ctx, clientHash, cookie); err != nil {
		return err
	}

	// Key derivation
	key, ivBase, seq, sig := crypto.KLAPDeriveKeyIVSeqSig(localSeed, remoteSeed, authHash)

	// Store session under mutex
	t.mu.Lock()
	t.cookie = cookie
	t.key = key
	t.iv = ivBase
	t.sig = sig
	t.seq = seq
	t.loggedIn = true
	t.mu.Unlock()

	return nil
}

// handshake1 performs the first step of the KLAP handshake.
func (t *Transport) handshake1(ctx context.Context, localSeed []byte) (remoteSeed, serverHash []byte, cookie *http.Cookie, err error) {
	url := fmt.Sprintf("http://%s/app/handshake1", t.host)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(localSeed))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("klap: handshake1 request creation failed: %w", err)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("klap: handshake1 request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil, nil, fmt.Errorf("klap: handshake1 failed (status %d): %w", resp.StatusCode, transport.ErrHandshake)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("klap: handshake1 read body failed: %w", err)
	}

	if len(body) != 48 {
		return nil, nil, nil, fmt.Errorf("klap: handshake1 unexpected body length %d: %w", len(body), transport.ErrHandshake)
	}

	remoteSeed = body[:16]
	serverHash = body[16:48]

	// Extract TP_SESSIONID cookie
	for _, c := range resp.Cookies() {
		if c.Name == "TP_SESSIONID" {
			cookie = c
			break
		}
	}
	if cookie == nil {
		return nil, nil, nil, fmt.Errorf("klap: handshake1 missing TP_SESSIONID cookie: %w", transport.ErrHandshake)
	}

	return remoteSeed, serverHash, cookie, nil
}

// handshake2 performs the second step of the KLAP handshake.
func (t *Transport) handshake2(ctx context.Context, clientHash []byte, cookie *http.Cookie) error {
	url := fmt.Sprintf("http://%s/app/handshake2", t.host)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(clientHash))
	if err != nil {
		return fmt.Errorf("klap: handshake2 request creation failed: %w", err)
	}
	req.AddCookie(cookie)

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("klap: handshake2 request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("klap: handshake2 failed (status %d): %w", resp.StatusCode, transport.ErrHandshake)
	}

	return nil
}

// Send sends a command to the device and returns the result.
func (t *Transport) Send(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
	// Read session state under mutex
	t.mu.Lock()
	if !t.loggedIn {
		t.mu.Unlock()
		return nil, fmt.Errorf("klap: send %s: not logged in: %w", method, transport.ErrAuth)
	}
	t.seq++ // increment BEFORE use — derived seq is the handshake seq
	key := make([]byte, len(t.key))
	copy(key, t.key)
	ivBase := make([]byte, len(t.iv))
	copy(ivBase, t.iv)
	sigBase := make([]byte, len(t.sig))
	copy(sigBase, t.sig)
	seq := t.seq
	c := *t.cookie
	cookie := &c
	t.mu.Unlock()

	// Build command JSON
	var commandJSON []byte
	var err error
	if len(payload) == 0 {
		commandJSON, err = json.Marshal(map[string]string{"method": method})
	} else {
		commandJSON, err = json.Marshal(struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}{
			Method: method,
			Params: payload,
		})
	}
	if err != nil {
		return nil, fmt.Errorf("klap: failed to marshal command: %w", err)
	}

	// Construct full 16-byte IV: ivBase (12 bytes) + bigEndian(seq) (4 bytes)
	fullIV := make([]byte, 16)
	copy(fullIV, ivBase)
	binary.BigEndian.PutUint32(fullIV[12:], seq)

	// Encrypt command
	encrypted, err := crypto.Encrypt(key, fullIV, commandJSON)
	if err != nil {
		return nil, fmt.Errorf("klap: encrypt failed: %w", err)
	}

	// Compute signature: SHA256(sig + seq_bytes + encrypted)
	seqBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(seqBytes, seq)

	sigHash := sha256.New()
	sigHash.Write(sigBase)
	sigHash.Write(seqBytes)
	sigHash.Write(encrypted)
	signature := sigHash.Sum(nil)

	// Request body = signature (32 bytes) || encrypted data
	body := make([]byte, 0, len(signature)+len(encrypted))
	body = append(body, signature...)
	body = append(body, encrypted...)

	// POST request
	url := fmt.Sprintf("http://%s/app/request?seq=%d", t.host, int32(seq))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("klap: request creation failed: %w", err)
	}
	req.AddCookie(cookie)

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("klap: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("klap: send %s failed (status %d): %w", method, resp.StatusCode, transport.ErrHandshake)
	}

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("klap: send %s: read response failed: %w", method, err)
	}

	// Response body = signature (32 bytes) || encrypted data
	if len(respBody) < 32 {
		return nil, fmt.Errorf("klap: send %s: response too short (%d bytes)", method, len(respBody))
	}
	respEncrypted := respBody[32:] // strip signature prefix

	decrypted, err := crypto.Decrypt(key, fullIV, respEncrypted)
	if err != nil {
		return nil, fmt.Errorf("klap: send %s: decrypt response failed: %w", method, err)
	}

	// Parse decrypted JSON envelope — check error_code before extracting result.
	var envelope struct {
		ErrorCode int             `json:"error_code"`
		Result    json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(decrypted, &envelope); err != nil {
		return nil, fmt.Errorf("klap: send %s: failed to parse response: %w", method, err)
	}

	if envelope.ErrorCode == 9999 {
		return nil, fmt.Errorf("klap: send %s: %w", method, transport.ErrSessionExpired)
	}

	return envelope.Result, nil
}
