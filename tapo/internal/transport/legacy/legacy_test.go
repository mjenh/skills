package legacy

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mjenh/skills/tapo/internal/crypto"
	"github.com/mjenh/skills/tapo/internal/transport"
)

// mockDevice simulates a Tapo legacy device for testing.
type mockDevice struct {
	aesKey []byte
	aesIV  []byte
	token  string

	handshakeDone bool
	loginDone     bool

	// Controls for error injection.
	handshakeError    bool
	loginErrorCode    int
	commandErrorCode  int
}

func newMockDevice() *mockDevice {
	return &mockDevice{
		aesKey: []byte("0123456789abcdef"),
		aesIV:  []byte("abcdef0123456789"),
		token:  "mock-token-12345",
	}
}

func (m *mockDevice) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()

		var req struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		switch req.Method {
		case "handshake":
			m.handleHandshake(w, body)
		case "securePassthrough":
			m.handleSecurePassthrough(w, body, r)
		default:
			http.Error(w, "unknown method", http.StatusBadRequest)
		}
	})
}

func (m *mockDevice) handleHandshake(w http.ResponseWriter, body []byte) {
	if m.handshakeError {
		resp := handshakeResponse{ErrorCode: -1}
		json.NewEncoder(w).Encode(resp)
		return
	}

	var req handshakeRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "bad handshake request", http.StatusBadRequest)
		return
	}

	// Parse PEM-encoded public key from client.
	pemKey := req.Params.Key
	pemKey = strings.Replace(pemKey, "-----BEGIN PUBLIC KEY-----\n", "", 1)
	pemKey = strings.Replace(pemKey, "\n-----END PUBLIC KEY-----\n", "", 1)

	pubDER, err := base64.StdEncoding.DecodeString(pemKey)
	if err != nil {
		http.Error(w, "bad base64 in public key", http.StatusBadRequest)
		return
	}

	pubKey, err := x509.ParsePKCS1PublicKey(pubDER)
	if err != nil {
		http.Error(w, "bad public key", http.StatusBadRequest)
		return
	}

	// Combine AES key + IV as the key material (32 bytes).
	keyMaterial := append([]byte(nil), m.aesKey...)
	keyMaterial = append(keyMaterial, m.aesIV...)

	// Encrypt key material with client's public key.
	encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, pubKey, keyMaterial)
	if err != nil {
		http.Error(w, "rsa encrypt failed", http.StatusInternalServerError)
		return
	}

	resp := handshakeResponse{}
	resp.Result.Key = base64.StdEncoding.EncodeToString(encrypted)

	m.handshakeDone = true
	json.NewEncoder(w).Encode(resp)
}

func (m *mockDevice) handleSecurePassthrough(w http.ResponseWriter, body []byte, _ *http.Request) {
	var req securePassthroughRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "bad securePassthrough request", http.StatusBadRequest)
		return
	}

	// Decrypt inner request.
	decoded, err := base64.StdEncoding.DecodeString(req.Params.Request)
	if err != nil {
		http.Error(w, "bad base64 in request", http.StatusBadRequest)
		return
	}

	plaintext, err := crypto.Decrypt(m.aesKey, m.aesIV, decoded)
	if err != nil {
		http.Error(w, "decrypt failed", http.StatusBadRequest)
		return
	}

	var innerReq struct {
		Method string `json:"method"`
	}
	if err := json.Unmarshal(plaintext, &innerReq); err != nil {
		http.Error(w, "bad inner request", http.StatusBadRequest)
		return
	}

	var innerResp []byte

	switch innerReq.Method {
	case "login_device":
		innerResp = m.handleLogin(plaintext)
	case "get_device_info":
		innerResp = m.handleGetDeviceInfo()
	default:
		innerResp = m.handleGenericCommand(innerReq.Method)
	}

	// Encrypt inner response.
	ciphertext, err := crypto.Encrypt(m.aesKey, m.aesIV, innerResp)
	if err != nil {
		http.Error(w, "encrypt failed", http.StatusInternalServerError)
		return
	}

	resp := securePassthroughResponse{}
	resp.Result.Response = base64.StdEncoding.EncodeToString(ciphertext)
	json.NewEncoder(w).Encode(resp)
}

func (m *mockDevice) handleLogin(plaintext []byte) []byte {
	if m.loginErrorCode != 0 {
		resp, _ := json.Marshal(deviceResponse{ErrorCode: m.loginErrorCode})
		return resp
	}

	var lr struct {
		Params struct {
			Username string `json:"username"`
			Password string `json:"password"`
		} `json:"params"`
	}
	json.Unmarshal(plaintext, &lr)

	// Verify username is base64(SHA1(email)) format — just check it's valid base64
	// decoding to 20 bytes.
	decoded, err := base64.StdEncoding.DecodeString(lr.Params.Username)
	if err != nil || len(decoded) != 20 {
		resp, _ := json.Marshal(deviceResponse{ErrorCode: -1501})
		return resp
	}

	m.loginDone = true

	tokenResult, _ := json.Marshal(map[string]string{"token": m.token})
	resp, _ := json.Marshal(deviceResponse{
		ErrorCode: 0,
		Result:    json.RawMessage(tokenResult),
	})
	return resp
}

func (m *mockDevice) handleGetDeviceInfo() []byte {
	if m.commandErrorCode != 0 {
		resp, _ := json.Marshal(deviceResponse{ErrorCode: m.commandErrorCode})
		return resp
	}

	info, _ := json.Marshal(map[string]interface{}{
		"device_id": "test-device-123",
		"model":     "P100",
		"nickname":  "dGVzdC1wbHVn", // base64 of "test-plug"
	})
	resp, _ := json.Marshal(deviceResponse{
		ErrorCode: 0,
		Result:    json.RawMessage(info),
	})
	return resp
}

func (m *mockDevice) handleGenericCommand(method string) []byte {
	if m.commandErrorCode != 0 {
		resp, _ := json.Marshal(deviceResponse{ErrorCode: m.commandErrorCode})
		return resp
	}

	result, _ := json.Marshal(map[string]string{"method": method})
	resp, _ := json.Marshal(deviceResponse{
		ErrorCode: 0,
		Result:    json.RawMessage(result),
	})
	return resp
}

// newTestTransport creates a Transport pointing at the given test server.
func newTestTransport(serverURL string) *Transport {
	host := strings.TrimPrefix(serverURL, "http://")
	return New(host, 5*time.Second)
}

func TestHandshakeSuccess(t *testing.T) {
	mock := newMockDevice()
	server := httptest.NewServer(mock.handler())
	defer server.Close()

	tr := newTestTransport(server.URL)

	err := tr.handshake(context.Background())
	if err != nil {
		t.Fatalf("handshake failed: %v", err)
	}

	if len(tr.aesKey) != 16 {
		t.Errorf("aesKey length %d, want 16", len(tr.aesKey))
	}
	if len(tr.aesIV) != 16 {
		t.Errorf("aesIV length %d, want 16", len(tr.aesIV))
	}

	// Verify the keys match what the mock device sent.
	if string(tr.aesKey) != string(mock.aesKey) {
		t.Errorf("aesKey mismatch: got %x, want %x", tr.aesKey, mock.aesKey)
	}
	if string(tr.aesIV) != string(mock.aesIV) {
		t.Errorf("aesIV mismatch: got %x, want %x", tr.aesIV, mock.aesIV)
	}

	if !mock.handshakeDone {
		t.Error("mock device did not register handshake")
	}
}

func TestHandshakeFailure(t *testing.T) {
	mock := newMockDevice()
	mock.handshakeError = true
	server := httptest.NewServer(mock.handler())
	defer server.Close()

	tr := newTestTransport(server.URL)

	err := tr.handshake(context.Background())
	if err == nil {
		t.Fatal("expected handshake error, got nil")
	}
	if !errors.Is(err, transport.ErrHandshake) {
		t.Errorf("expected error wrapping ErrHandshake, got: %v", err)
	}
}

func TestHandshakeContextCancelled(t *testing.T) {
	mock := newMockDevice()
	server := httptest.NewServer(mock.handler())
	defer server.Close()

	tr := newTestTransport(server.URL)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	err := tr.handshake(ctx)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
	// The error should wrap ErrHandshake (since our handshake wraps all errors)
	// and the underlying cause should be a context error.
	if !errors.Is(err, transport.ErrHandshake) {
		t.Errorf("expected error wrapping ErrHandshake, got: %v", err)
	}
}

func TestLoginSuccess(t *testing.T) {
	mock := newMockDevice()
	server := httptest.NewServer(mock.handler())
	defer server.Close()

	tr := newTestTransport(server.URL)

	err := tr.Login(context.Background(), "test@example.com", "testpassword")
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	if tr.token != mock.token {
		t.Errorf("token mismatch: got %q, want %q", tr.token, mock.token)
	}

	if !mock.handshakeDone {
		t.Error("handshake not performed")
	}
	if !mock.loginDone {
		t.Error("login not completed on device")
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	mock := newMockDevice()
	mock.loginErrorCode = -1501
	server := httptest.NewServer(mock.handler())
	defer server.Close()

	tr := newTestTransport(server.URL)

	err := tr.Login(context.Background(), "bad@example.com", "wrongpassword")
	if err == nil {
		t.Fatal("expected auth error, got nil")
	}
	if !errors.Is(err, transport.ErrAuth) {
		t.Errorf("expected error wrapping ErrAuth, got: %v", err)
	}
}

func TestLoginHandshakeFailure(t *testing.T) {
	mock := newMockDevice()
	mock.handshakeError = true
	server := httptest.NewServer(mock.handler())
	defer server.Close()

	tr := newTestTransport(server.URL)

	err := tr.Login(context.Background(), "test@example.com", "testpassword")
	if err == nil {
		t.Fatal("expected handshake error, got nil")
	}
	if !errors.Is(err, transport.ErrHandshake) {
		t.Errorf("expected error wrapping ErrHandshake, got: %v", err)
	}
}

func TestLoginCredentialsNotInErrors(t *testing.T) {
	mock := newMockDevice()
	mock.loginErrorCode = -1501
	server := httptest.NewServer(mock.handler())
	defer server.Close()

	tr := newTestTransport(server.URL)

	email := "sensitive@example.com"
	password := "supersecretpassword"

	err := tr.Login(context.Background(), email, password)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	errMsg := err.Error()
	if strings.Contains(errMsg, email) {
		t.Errorf("error message contains email: %q", errMsg)
	}
	if strings.Contains(errMsg, password) {
		t.Errorf("error message contains password: %q", errMsg)
	}
}

func TestSendSuccess(t *testing.T) {
	mock := newMockDevice()
	server := httptest.NewServer(mock.handler())
	defer server.Close()

	tr := newTestTransport(server.URL)

	// Login first.
	if err := tr.Login(context.Background(), "test@example.com", "testpassword"); err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	// Send get_device_info.
	result, err := tr.Send(context.Background(), "get_device_info", nil)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify the response contains expected fields.
	var info map[string]interface{}
	if err := json.Unmarshal(result, &info); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if info["device_id"] != "test-device-123" {
		t.Errorf("unexpected device_id: %v", info["device_id"])
	}
	if info["model"] != "P100" {
		t.Errorf("unexpected model: %v", info["model"])
	}
}

func TestSendSessionToken(t *testing.T) {
	mock := newMockDevice()

	var capturedURL string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bodyBytes, _ := io.ReadAll(r.Body)
		r.Body.Close()

		var req struct {
			Method string `json:"method"`
		}
		json.Unmarshal(bodyBytes, &req)

		// Only capture URL for securePassthrough calls after login (which have token).
		if req.Method == "securePassthrough" && r.URL.Query().Get("token") != "" {
			capturedURL = r.URL.String()
		}

		// Re-create body reader for the mock handler.
		r.Body = io.NopCloser(strings.NewReader(string(bodyBytes)))
		r.ContentLength = int64(len(bodyBytes))

		mock.handler().ServeHTTP(w, r)
	}))
	defer server.Close()

	tr := newTestTransport(server.URL)

	if err := tr.Login(context.Background(), "test@example.com", "testpassword"); err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	_, err := tr.Send(context.Background(), "get_device_info", nil)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	expectedToken := mock.token
	if !strings.Contains(capturedURL, "token="+expectedToken) {
		t.Errorf("token not in request URL: got %q, expected token=%s", capturedURL, expectedToken)
	}
}

func TestSendWithoutLogin(t *testing.T) {
	tr := New("127.0.0.1:9999", 5*time.Second)

	_, err := tr.Send(context.Background(), "get_device_info", nil)
	if err == nil {
		t.Fatal("expected error when sending without login, got nil")
	}

	if !errors.Is(err, transport.ErrAuth) {
		t.Errorf("expected error wrapping ErrAuth, got: %v", err)
	}
}

func TestSendSessionExpiry(t *testing.T) {
	mock := newMockDevice()
	server := httptest.NewServer(mock.handler())
	defer server.Close()

	tr := newTestTransport(server.URL)

	// Login first.
	if err := tr.Login(context.Background(), "test@example.com", "testpassword"); err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	// Set the mock to return error code 9999 for the next command.
	mock.commandErrorCode = 9999

	_, err := tr.Send(context.Background(), "get_device_info", nil)
	if err == nil {
		t.Fatal("expected session expired error, got nil")
	}

	if !errors.Is(err, transport.ErrSessionExpired) {
		t.Errorf("expected error wrapping ErrSessionExpired, got: %v", err)
	}
}

func TestSendEncryptDecryptRoundTrip(t *testing.T) {
	mock := newMockDevice()
	server := httptest.NewServer(mock.handler())
	defer server.Close()

	tr := newTestTransport(server.URL)

	// Login to establish encryption keys.
	if err := tr.Login(context.Background(), "test@example.com", "testpassword"); err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	// Send a command with payload.
	payload, _ := json.Marshal(map[string]bool{"device_on": true})
	result, err := tr.Send(context.Background(), "set_device_info", json.RawMessage(payload))
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	// Verify the response is valid JSON (survived the encrypt/decrypt pipeline).
	var parsed map[string]interface{}
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("result is not valid JSON after round-trip: %v", err)
	}

	// The mock returns the method in the response for generic commands.
	if method, ok := parsed["method"]; ok {
		if method != "set_device_info" {
			t.Errorf("unexpected method in response: %v", method)
		}
	}

	// Verify that plaintext data survives: do another round-trip.
	result2, err := tr.Send(context.Background(), "get_device_info", nil)
	if err != nil {
		t.Fatalf("second Send failed: %v", err)
	}

	var info map[string]interface{}
	if err := json.Unmarshal(result2, &info); err != nil {
		t.Fatalf("second result is not valid JSON: %v", err)
	}

	// The device_id should survive the encryption round-trip intact.
	if info["device_id"] != "test-device-123" {
		t.Errorf("data corrupted in round-trip: device_id = %v", info["device_id"])
	}


}

// TestSendNonZeroNon9999ErrorCode verifies that Send returns an error (not
// wrapped as ErrSessionExpired) when the device responds with a non-zero
// error_code that is not 9999.
func TestSendNonZeroNon9999ErrorCode(t *testing.T) {
	mock := newMockDevice()
	server := httptest.NewServer(mock.handler())
	defer server.Close()

	tr := newTestTransport(server.URL)

	// Login first.
	if err := tr.Login(context.Background(), "test@example.com", "testpassword"); err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	// Set the mock to return a non-zero, non-9999 error code for the next command.
	mock.commandErrorCode = -1003

	_, err := tr.Send(context.Background(), "get_device_info", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if errors.Is(err, transport.ErrSessionExpired) {
		t.Errorf("did not expect ErrSessionExpired for error_code -1003, got: %v", err)
	}
	if errors.Is(err, transport.ErrAuth) {
		t.Errorf("did not expect ErrAuth for error_code -1003, got: %v", err)
	}
	if !strings.Contains(err.Error(), "-1003") {
		t.Errorf("expected error to mention error_code -1003, got: %v", err)
	}
}

// TestSecurePassthroughOuterError verifies that securePassthrough returns an
// error when the outer securePassthrough response itself carries a non-zero
// error_code (no encryption involved at this layer).
func TestSecurePassthroughOuterError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()

		var req struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		switch req.Method {
		case "handshake":
			newMockDevice().handleHandshake(w, body)
		case "securePassthrough":
			// Outer error_code is non-zero; no result/encryption needed.
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"error_code":-1,"result":{}}`)
		default:
			http.Error(w, "unknown method", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	tr := newTestTransport(server.URL)

	if err := tr.handshake(context.Background()); err != nil {
		t.Fatalf("handshake failed: %v", err)
	}

	innerJSON, _ := json.Marshal(map[string]string{"method": "get_device_info"})
	_, err := tr.securePassthrough(context.Background(), innerJSON)
	if err == nil {
		t.Fatal("expected securePassthrough error, got nil")
	}
	if !strings.Contains(err.Error(), "-1") {
		t.Errorf("expected error to mention error_code -1, got: %v", err)
	}
}

// TestHandshakeInvalidKeyLength verifies that handshake returns an error when
// the decrypted key material is not the expected 32 bytes (16-byte AES key +
// 16-byte IV).
func TestHandshakeInvalidKeyLength(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()

		var req handshakeRequest
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, "bad handshake request", http.StatusBadRequest)
			return
		}

		pemKey := req.Params.Key
		pemKey = strings.Replace(pemKey, "-----BEGIN PUBLIC KEY-----\n", "", 1)
		pemKey = strings.Replace(pemKey, "\n-----END PUBLIC KEY-----\n", "", 1)

		pubDER, err := base64.StdEncoding.DecodeString(pemKey)
		if err != nil {
			http.Error(w, "bad base64 in public key", http.StatusBadRequest)
			return
		}

		pubKey, err := x509.ParsePKCS1PublicKey(pubDER)
		if err != nil {
			http.Error(w, "bad public key", http.StatusBadRequest)
			return
		}

		// Return only 16 bytes of key material instead of the required 32.
		shortKeyMaterial := []byte("0123456789abcdef")

		encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, pubKey, shortKeyMaterial)
		if err != nil {
			http.Error(w, "rsa encrypt failed", http.StatusInternalServerError)
			return
		}

		resp := handshakeResponse{}
		resp.Result.Key = base64.StdEncoding.EncodeToString(encrypted)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	tr := newTestTransport(server.URL)

	err := tr.handshake(context.Background())
	if err == nil {
		t.Fatal("expected handshake error for invalid key length, got nil")
	}
	if !errors.Is(err, transport.ErrHandshake) {
		t.Errorf("expected error wrapping ErrHandshake, got: %v", err)
	}
	if !strings.Contains(err.Error(), "16") {
		t.Errorf("expected error to mention key material length 16, got: %v", err)
	}
}
