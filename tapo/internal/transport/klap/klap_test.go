package klap_test

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/mjenh/skills/tapo/internal/crypto"
	"github.com/mjenh/skills/tapo/internal/transport"
	"github.com/mjenh/skills/tapo/internal/transport/klap"
)

const (
	testEmail    = "test@example.com"
	testPassword = "testpassword"
)

// Compile-time interface check.
var _ transport.Transport = (*klap.Transport)(nil)

// mockServer creates an httptest.Server that implements the KLAP handshake and request endpoints.
// It uses a fixed remoteSeed and derives keys from the provided email/password.
func mockServer(t *testing.T, email, password string, opts ...mockOption) *httptest.Server {
	t.Helper()

	cfg := &mockConfig{}
	for _, o := range opts {
		o(cfg)
	}

	authHash := crypto.KLAPAuthHash(email, password)
	remoteSeed := []byte("remote-seed-1234") // fixed 16-byte remote seed

	mux := http.NewServeMux()

	mux.HandleFunc("POST /app/handshake1", func(w http.ResponseWriter, r *http.Request) {
		localSeed, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("handshake1: read body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if cfg.handshake1Status != 0 {
			w.WriteHeader(cfg.handshake1Status)
			return
		}

		// Compute serverHash = SHA256(localSeed + remoteSeed + authHash)
		var hashInput []byte
		if cfg.badServerHash {
			hashInput = append(hashInput, []byte("wrong")...)
		} else {
			hashInput = append(hashInput, localSeed...)
		}
		hashInput = append(hashInput, remoteSeed...)
		hashInput = append(hashInput, authHash...)
		serverHash := sha256.Sum256(hashInput)

		http.SetCookie(w, &http.Cookie{
			Name:  "TP_SESSIONID",
			Value: "test-session-id",
		})

		body := make([]byte, 0, 48)
		body = append(body, remoteSeed...)
		body = append(body, serverHash[:]...)
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	})

	mux.HandleFunc("POST /app/handshake2", func(w http.ResponseWriter, r *http.Request) {
		if cfg.handshake2Status != 0 {
			w.WriteHeader(cfg.handshake2Status)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("POST /app/request", func(w http.ResponseWriter, r *http.Request) {
		seqStr := r.URL.Query().Get("seq")
		seq64, err := strconv.ParseInt(seqStr, 10, 32)
		if err != nil {
			t.Errorf("request: parse seq: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		seq := uint32(int32(seq64))

		// We need localSeed to derive key/iv, but we don't have it here.
		// The test must provide the key derivation info via cfg.
		if cfg.keyDeriveFn == nil {
			t.Error("request: no key derivation function provided")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		key, ivBase := cfg.keyDeriveFn()

		fullIV := make([]byte, 16)
		copy(fullIV, ivBase)
		binary.BigEndian.PutUint32(fullIV[12:], seq)

		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("request: read body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Strip 32-byte signature prefix from request body
		if len(body) < 32 {
			t.Errorf("request: body too short (%d bytes)", len(body))
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		encryptedReq := body[32:]

		// Decrypt the request
		decrypted, err := crypto.Decrypt(key, fullIV, encryptedReq)
		if err != nil {
			t.Errorf("request: decrypt: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Parse the command
		var cmd struct {
			Method string          `json:"method"`
			Params json.RawMessage `json:"params,omitempty"`
		}
		if err := json.Unmarshal(decrypted, &cmd); err != nil {
			t.Errorf("request: unmarshal command: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Record the sequence number for verification
		if cfg.seqRecorder != nil {
			cfg.seqRecorder(seq)
		}

		// Build response
		response := map[string]any{
			"result": map[string]any{
				"method":  cmd.Method,
				"success": true,
			},
		}
		responseJSON, _ := json.Marshal(response)

		// Encrypt response with same fullIV
		encrypted, err := crypto.Encrypt(key, fullIV, responseJSON)
		if err != nil {
			t.Errorf("request: encrypt response: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Response = 32-byte dummy signature + encrypted data
		sig := make([]byte, 32)
		w.WriteHeader(http.StatusOK)
		w.Write(sig)
		w.Write(encrypted)
	})

	return httptest.NewServer(mux)
}

type mockConfig struct {
	badServerHash    bool
	handshake1Status int
	handshake2Status int
	keyDeriveFn      func() (key, iv []byte)
	seqRecorder      func(seq uint32)
}

type mockOption func(*mockConfig)

func withBadServerHash() mockOption {
	return func(c *mockConfig) { c.badServerHash = true }
}

func withHandshake2Status(status int) mockOption {
	return func(c *mockConfig) { c.handshake2Status = status }
}

func withKeyDeriveFn(fn func() (key, iv []byte)) mockOption {
	return func(c *mockConfig) { c.keyDeriveFn = fn }
}

func withSeqRecorder(fn func(seq uint32)) mockOption {
	return func(c *mockConfig) { c.seqRecorder = fn }
}

// loginAndCapture performs Login and returns the derived key material
// by intercepting the handshake1 to capture localSeed.
func loginAndCapture(t *testing.T, email, password string) (*httptest.Server, *klap.Transport, func() ([]byte, []byte)) {
	t.Helper()

	authHash := crypto.KLAPAuthHash(email, password)
	remoteSeed := []byte("remote-seed-1234")

	var keyOnce func() ([]byte, []byte)

	// We wrap the server to capture the localSeed from handshake1.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app/handshake1":
			localSeed, _ := io.ReadAll(r.Body)

			key, iv, _, _ := crypto.KLAPDeriveKeyIVSeqSig(localSeed, remoteSeed, authHash)
			keyOnce = func() ([]byte, []byte) { return key, iv }

			h := sha256.New()
			h.Write(localSeed)
			h.Write(remoteSeed)
			h.Write(authHash)
			serverHash := h.Sum(nil)

			http.SetCookie(w, &http.Cookie{
				Name:  "TP_SESSIONID",
				Value: "test-session-id",
			})

			body := make([]byte, 0, 48)
			body = append(body, remoteSeed...)
			body = append(body, serverHash...)
			w.WriteHeader(http.StatusOK)
			w.Write(body)

		case "/app/handshake2":
			w.WriteHeader(http.StatusOK)

		case "/app/request":
			seqStr := r.URL.Query().Get("seq")
			seq64, _ := strconv.ParseInt(seqStr, 10, 32)
			seq := uint32(int32(seq64))

			key, ivBase := keyOnce()
			fullIV := make([]byte, 16)
			copy(fullIV, ivBase)
			binary.BigEndian.PutUint32(fullIV[12:], seq)

			body, _ := io.ReadAll(r.Body)
			// Strip 32-byte signature prefix
			if len(body) < 32 {
				t.Errorf("request: body too short (%d bytes)", len(body))
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			encryptedReq := body[32:]

			decrypted, err := crypto.Decrypt(key, fullIV, encryptedReq)
			if err != nil {
				t.Errorf("request: decrypt: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			var cmd struct {
				Method string          `json:"method"`
				Params json.RawMessage `json:"params,omitempty"`
			}
			json.Unmarshal(decrypted, &cmd)

			response := map[string]any{
				"result": map[string]any{
					"method":  cmd.Method,
					"success": true,
				},
			}
			responseJSON, _ := json.Marshal(response)

			encrypted, _ := crypto.Encrypt(key, fullIV, responseJSON)
			// Response = 32-byte dummy signature + encrypted data
			sig := make([]byte, 32)
			w.WriteHeader(http.StatusOK)
			w.Write(sig)
			w.Write(encrypted)
		}
	}))

	host := strings.TrimPrefix(srv.URL, "http://")
	tr := klap.New(host, srv.Client())

	return srv, tr, func() ([]byte, []byte) {
		if keyOnce != nil {
			return keyOnce()
		}
		return nil, nil
	}
}

func TestSuccessfulHandshakeAndSend(t *testing.T) {
	srv, tr, _ := loginAndCapture(t, testEmail, testPassword)
	defer srv.Close()

	err := tr.Login(context.Background(), testEmail, testPassword)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	payload := json.RawMessage(`{"device_on":true}`)
	result, err := tr.Send(context.Background(), "set_device_info", payload)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if parsed["method"] != "set_device_info" {
		t.Errorf("expected method set_device_info, got %v", parsed["method"])
	}
	if parsed["success"] != true {
		t.Errorf("expected success true, got %v", parsed["success"])
	}
}

func TestSendWithNilPayload(t *testing.T) {
	srv, tr, _ := loginAndCapture(t, testEmail, testPassword)
	defer srv.Close()

	err := tr.Login(context.Background(), testEmail, testPassword)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	result, err := tr.Send(context.Background(), "get_device_info", nil)
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}

	if parsed["method"] != "get_device_info" {
		t.Errorf("expected method get_device_info, got %v", parsed["method"])
	}
}

func TestErrAuthOnBadCredentials(t *testing.T) {
	authHash := crypto.KLAPAuthHash(testEmail, testPassword)
	remoteSeed := []byte("remote-seed-1234")

	srv := mockServer(t, testEmail, testPassword, withBadServerHash(), withKeyDeriveFn(func() ([]byte, []byte) {
		key, iv, _, _ := crypto.KLAPDeriveKeyIVSeqSig([]byte("0123456789abcdef"), remoteSeed, authHash)
		return key, iv
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	tr := klap.New(host, srv.Client())

	err := tr.Login(context.Background(), testEmail, testPassword)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, transport.ErrAuth) {
		t.Errorf("expected ErrAuth, got: %v", err)
	}
}

func TestErrHandshakeOnHandshake2Failure(t *testing.T) {
	authHash := crypto.KLAPAuthHash(testEmail, testPassword)
	remoteSeed := []byte("remote-seed-1234")

	srv := mockServer(t, testEmail, testPassword, withHandshake2Status(http.StatusForbidden), withKeyDeriveFn(func() ([]byte, []byte) {
		key, iv, _, _ := crypto.KLAPDeriveKeyIVSeqSig([]byte("0123456789abcdef"), remoteSeed, authHash)
		return key, iv
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	tr := klap.New(host, srv.Client())

	err := tr.Login(context.Background(), testEmail, testPassword)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, transport.ErrHandshake) {
		t.Errorf("expected ErrHandshake, got: %v", err)
	}
}

func TestContextCancellationAbortsLogin(t *testing.T) {
	srv, tr, _ := loginAndCapture(t, testEmail, testPassword)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := tr.Login(ctx, testEmail, testPassword)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}

func TestContextCancellationAbortsSend(t *testing.T) {
	srv, tr, _ := loginAndCapture(t, testEmail, testPassword)
	defer srv.Close()

	err := tr.Login(context.Background(), testEmail, testPassword)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err = tr.Send(ctx, "get_device_info", nil)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}

func TestSequenceCounterIncrements(t *testing.T) {
	srv, tr, _ := loginAndCapture(t, testEmail, testPassword)
	defer srv.Close()

	err := tr.Login(context.Background(), testEmail, testPassword)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	// Send multiple requests and verify sequence increments
	for i := 0; i < 3; i++ {
		result, err := tr.Send(context.Background(), "get_device_info", nil)
		if err != nil {
			t.Fatalf("Send %d failed: %v", i, err)
		}
		if result == nil {
			t.Fatalf("Send %d returned nil result", i)
		}
	}
}

func TestSequenceCounterIncrementsWithCapture(t *testing.T) {
	authHash := crypto.KLAPAuthHash(testEmail, testPassword)
	remoteSeed := []byte("remote-seed-1234")

	var capturedSeqs []uint32
	var derivedKey, derivedIV []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app/handshake1":
			localSeed, _ := io.ReadAll(r.Body)

			key, iv, _, _ := crypto.KLAPDeriveKeyIVSeqSig(localSeed, remoteSeed, authHash)
			derivedKey = key
			derivedIV = iv

			h := sha256.New()
			h.Write(localSeed)
			h.Write(remoteSeed)
			h.Write(authHash)
			serverHash := h.Sum(nil)

			http.SetCookie(w, &http.Cookie{
				Name:  "TP_SESSIONID",
				Value: "test-session-id",
			})

			body := make([]byte, 0, 48)
			body = append(body, remoteSeed...)
			body = append(body, serverHash...)
			w.WriteHeader(http.StatusOK)
			w.Write(body)

		case "/app/handshake2":
			w.WriteHeader(http.StatusOK)

		case "/app/request":
			seqStr := r.URL.Query().Get("seq")
			seq64, _ := strconv.ParseInt(seqStr, 10, 32)
			seq := uint32(int32(seq64))
			capturedSeqs = append(capturedSeqs, seq)

			fullIV := make([]byte, 16)
			copy(fullIV, derivedIV)
			binary.BigEndian.PutUint32(fullIV[12:], seq)

			body, _ := io.ReadAll(r.Body)
			// Strip 32-byte signature prefix
			if len(body) < 32 {
				t.Errorf("request: body too short (%d bytes)", len(body))
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			encryptedReq := body[32:]

			decrypted, err := crypto.Decrypt(derivedKey, fullIV, encryptedReq)
			if err != nil {
				t.Errorf("decrypt: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			var cmd struct {
				Method string `json:"method"`
			}
			json.Unmarshal(decrypted, &cmd)

			response := map[string]any{
				"result": map[string]any{
					"method":  cmd.Method,
					"success": true,
				},
			}
			responseJSON, _ := json.Marshal(response)
			encrypted, _ := crypto.Encrypt(derivedKey, fullIV, responseJSON)
			// Response = 32-byte dummy signature + encrypted data
			dummySig := make([]byte, 32)
			w.WriteHeader(http.StatusOK)
			w.Write(dummySig)
			w.Write(encrypted)
		}
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	tr := klap.New(host, srv.Client())

	err := tr.Login(context.Background(), testEmail, testPassword)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	for i := 0; i < 3; i++ {
		_, err := tr.Send(context.Background(), "get_device_info", nil)
		if err != nil {
			t.Fatalf("Send %d failed: %v", i, err)
		}
	}

	if len(capturedSeqs) != 3 {
		t.Fatalf("expected 3 captured seqs, got %d", len(capturedSeqs))
	}

	// Verify sequences are strictly incrementing
	for i := 1; i < len(capturedSeqs); i++ {
		if capturedSeqs[i] != capturedSeqs[i-1]+1 {
			t.Errorf("seq[%d]=%d is not seq[%d]=%d + 1", i, capturedSeqs[i], i-1, capturedSeqs[i-1])
		}
	}
}

func TestCredentialsNotInErrors(t *testing.T) {
	// Use a server that returns bad hash to trigger auth error
	authHash := crypto.KLAPAuthHash(testEmail, testPassword)
	remoteSeed := []byte("remote-seed-1234")

	srv := mockServer(t, testEmail, testPassword, withBadServerHash(), withKeyDeriveFn(func() ([]byte, []byte) {
		key, iv, _, _ := crypto.KLAPDeriveKeyIVSeqSig([]byte("0123456789abcdef"), remoteSeed, authHash)
		return key, iv
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	tr := klap.New(host, srv.Client())

	err := tr.Login(context.Background(), testEmail, testPassword)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	errMsg := err.Error()
	if strings.Contains(errMsg, testEmail) {
		t.Errorf("error message contains email: %s", errMsg)
	}
	if strings.Contains(errMsg, testPassword) {
		t.Errorf("error message contains password: %s", errMsg)
	}
}

func TestSendWithoutLogin(t *testing.T) {
	tr := klap.New("127.0.0.1:9999", nil)

	_, err := tr.Send(context.Background(), "get_device_info", nil)
	if err == nil {
		t.Fatal("expected error when sending without login")
	}

	if !errors.Is(err, transport.ErrAuth) {
		t.Errorf("expected ErrAuth, got: %v", err)
	}
}

func TestNewWithNilClient(t *testing.T) {
	tr := klap.New("192.168.1.100", nil)
	if tr == nil {
		t.Fatal("New returned nil")
	}
}

func TestHandshake1NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	tr := klap.New(host, srv.Client())

	err := tr.Login(context.Background(), testEmail, testPassword)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !errors.Is(err, transport.ErrHandshake) {
		t.Errorf("expected ErrHandshake, got: %v", err)
	}
}

// Verify that error messages from handshake failures don't leak credentials.
func TestHandshakeErrorsDoNotContainCredentials(t *testing.T) {
	email := "secret_user@company.com"
	password := "super_secret_password_123"

	tests := []struct {
		name    string
		handler http.HandlerFunc
	}{
		{
			name: "handshake1 non-200",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusForbidden)
			},
		},
		{
			name: "handshake1 bad body",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("short"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(tt.handler)
			defer srv.Close()

			host := strings.TrimPrefix(srv.URL, "http://")
			tr := klap.New(host, srv.Client())

			err := tr.Login(context.Background(), email, password)
			if err == nil {
				t.Fatal("expected error")
			}

			errMsg := err.Error()
			if strings.Contains(errMsg, email) {
				t.Errorf("error contains email: %s", errMsg)
			}
			if strings.Contains(errMsg, password) {
				t.Errorf("error contains password: %s", errMsg)
			}
		})
	}
}

func TestSendNon200Response(t *testing.T) {
	srv, tr, _ := loginAndCapture(t, testEmail, testPassword)

	err := tr.Login(context.Background(), testEmail, testPassword)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	// Replace the server handler to return non-200 for /app/request
	srv.Close()
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	// Create a new transport pointing to the broken server, logged in
	// We can't reuse the old one since its host changed, so test the error path directly
	host := strings.TrimPrefix(srv.URL, "http://")
	tr2 := klap.New(host, srv.Client())

	// Login against a working server first, then Send against the broken one
	// Instead, we test by creating a server that works for login but fails on /app/request
	srv.Close()

	authHash := crypto.KLAPAuthHash(testEmail, testPassword)
	remoteSeed := []byte("remote-seed-1234")

	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app/handshake1":
			localSeed, _ := io.ReadAll(r.Body)

			h := sha256.New()
			h.Write(localSeed)
			h.Write(remoteSeed)
			h.Write(authHash)
			serverHash := h.Sum(nil)

			http.SetCookie(w, &http.Cookie{Name: "TP_SESSIONID", Value: "test"})

			body := make([]byte, 0, 48)
			body = append(body, remoteSeed...)
			body = append(body, serverHash...)
			w.WriteHeader(http.StatusOK)
			w.Write(body)

		case "/app/handshake2":
			w.WriteHeader(http.StatusOK)

		case "/app/request":
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv2.Close()

	host2 := strings.TrimPrefix(srv2.URL, "http://")
	tr2 = klap.New(host2, srv2.Client())

	if err := tr2.Login(context.Background(), testEmail, testPassword); err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	_, err = tr2.Send(context.Background(), "get_device_info", nil)
	if err == nil {
		t.Fatal("expected error from non-200 response")
	}
	if !errors.Is(err, transport.ErrHandshake) {
		t.Errorf("expected ErrHandshake, got: %v", err)
	}
}

func TestMissingSessionCookie(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return valid handshake1 body but no TP_SESSIONID cookie
		remoteSeed := []byte("remote-seed-1234")
		authHash := crypto.KLAPAuthHash(testEmail, testPassword)
		localSeed, _ := io.ReadAll(r.Body)

		h := sha256.New()
		h.Write(localSeed)
		h.Write(remoteSeed)
		h.Write(authHash)
		serverHash := h.Sum(nil)

		// Set a different cookie name
		http.SetCookie(w, &http.Cookie{Name: "OTHER_COOKIE", Value: "test"})

		body := make([]byte, 0, 48)
		body = append(body, remoteSeed...)
		body = append(body, serverHash...)
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	tr := klap.New(host, srv.Client())

	err := tr.Login(context.Background(), testEmail, testPassword)
	if err == nil {
		t.Fatal("expected error for missing TP_SESSIONID cookie")
	}
	if !errors.Is(err, transport.ErrHandshake) {
		t.Errorf("expected ErrHandshake, got: %v", err)
	}
}

func TestConcurrentSend(t *testing.T) {
	srv, tr, _ := loginAndCapture(t, testEmail, testPassword)
	defer srv.Close()

	err := tr.Login(context.Background(), testEmail, testPassword)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	const goroutines = 10
	errs := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			_, err := tr.Send(context.Background(), "get_device_info", nil)
			errs <- err
		}()
	}

	for i := 0; i < goroutines; i++ {
		if err := <-errs; err != nil {
			t.Errorf("concurrent Send failed: %v", err)
		}
	}
}

// klapErrorCodeServer creates an httptest.Server that performs a normal KLAP
// handshake, but on /app/request encrypts and returns the given raw JSON
// response body (e.g. `{"error_code":9999}`) instead of the usual success
// envelope. It mirrors the handshake logic in loginAndCapture.
func klapErrorCodeServer(t *testing.T, email, password string, responseJSON []byte) *httptest.Server {
	t.Helper()

	authHash := crypto.KLAPAuthHash(email, password)
	remoteSeed := []byte("remote-seed-1234")

	var derivedKey, derivedIV []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app/handshake1":
			localSeed, _ := io.ReadAll(r.Body)

			key, iv, _, _ := crypto.KLAPDeriveKeyIVSeqSig(localSeed, remoteSeed, authHash)
			derivedKey = key
			derivedIV = iv

			h := sha256.New()
			h.Write(localSeed)
			h.Write(remoteSeed)
			h.Write(authHash)
			serverHash := h.Sum(nil)

			http.SetCookie(w, &http.Cookie{
				Name:  "TP_SESSIONID",
				Value: "test-session-id",
			})

			body := make([]byte, 0, 48)
			body = append(body, remoteSeed...)
			body = append(body, serverHash...)
			w.WriteHeader(http.StatusOK)
			w.Write(body)

		case "/app/handshake2":
			w.WriteHeader(http.StatusOK)

		case "/app/request":
			seqStr := r.URL.Query().Get("seq")
			seq64, _ := strconv.ParseInt(seqStr, 10, 32)
			seq := uint32(int32(seq64))

			fullIV := make([]byte, 16)
			copy(fullIV, derivedIV)
			binary.BigEndian.PutUint32(fullIV[12:], seq)

			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("request: read body: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			// Strip 32-byte signature prefix from request body.
			if len(body) < 32 {
				t.Errorf("request: body too short (%d bytes)", len(body))
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			encryptedReq := body[32:]

			// Decrypt just to make sure the request is well-formed; result unused.
			if _, err := crypto.Decrypt(derivedKey, fullIV, encryptedReq); err != nil {
				t.Errorf("request: decrypt: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			encrypted, err := crypto.Encrypt(derivedKey, fullIV, responseJSON)
			if err != nil {
				t.Errorf("request: encrypt response: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			// Response body = signature (32 bytes) || encrypted data.
			sig := make([]byte, 32)
			w.WriteHeader(http.StatusOK)
			w.Write(sig)
			w.Write(encrypted)
		}
	}))

	return srv
}

func TestSendSessionExpired(t *testing.T) {
	srv := klapErrorCodeServer(t, testEmail, testPassword, []byte(`{"error_code":9999}`))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	tr := klap.New(host, srv.Client())

	if err := tr.Login(context.Background(), testEmail, testPassword); err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	_, err := tr.Send(context.Background(), "get_device_info", nil)
	if err == nil {
		t.Fatal("expected error for error_code 9999, got nil")
	}
	if !errors.Is(err, transport.ErrSessionExpired) {
		t.Errorf("expected ErrSessionExpired, got: %v", err)
	}
}

func TestSendResponseTooShort(t *testing.T) {
	authHash := crypto.KLAPAuthHash(testEmail, testPassword)
	remoteSeed := []byte("remote-seed-1234")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/app/handshake1":
			localSeed, _ := io.ReadAll(r.Body)

			h := sha256.New()
			h.Write(localSeed)
			h.Write(remoteSeed)
			h.Write(authHash)
			serverHash := h.Sum(nil)

			http.SetCookie(w, &http.Cookie{
				Name:  "TP_SESSIONID",
				Value: "test-session-id",
			})

			body := make([]byte, 0, 48)
			body = append(body, remoteSeed...)
			body = append(body, serverHash...)
			w.WriteHeader(http.StatusOK)
			w.Write(body)

		case "/app/handshake2":
			w.WriteHeader(http.StatusOK)

		case "/app/request":
			// Return a body shorter than the required 32-byte signature prefix.
			w.WriteHeader(http.StatusOK)
			w.Write(make([]byte, 16))
		}
	}))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	tr := klap.New(host, srv.Client())

	if err := tr.Login(context.Background(), testEmail, testPassword); err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	_, err := tr.Send(context.Background(), "get_device_info", nil)
	if err == nil {
		t.Fatal("expected error for short response body, got nil")
	}
	if !strings.Contains(err.Error(), "response too short") {
		t.Errorf("expected 'response too short' error, got: %v", err)
	}
}

func TestSendNonZeroErrorCode(t *testing.T) {
	// A non-zero, non-9999 error_code is not treated specially by Send:
	// the current behavior returns the result (and no error) regardless,
	// since only error_code 9999 (session expired) is checked explicitly.
	srv := klapErrorCodeServer(t, testEmail, testPassword, []byte(`{"error_code":1,"result":{"ok":false}}`))
	defer srv.Close()

	host := strings.TrimPrefix(srv.URL, "http://")
	tr := klap.New(host, srv.Client())

	if err := tr.Login(context.Background(), testEmail, testPassword); err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	result, err := tr.Send(context.Background(), "get_device_info", nil)
	if err != nil {
		t.Fatalf("expected no error for non-9999 error_code, got: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(result, &parsed); err != nil {
		t.Fatalf("failed to parse result: %v", err)
	}
	if parsed["ok"] != false {
		t.Errorf("expected result.ok=false, got %v", parsed["ok"])
	}
}
