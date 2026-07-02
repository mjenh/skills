package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockTransportNeg is a mock transport for negotiation tests.
type mockTransportNeg struct {
	loginErr   error
	sendResp   json.RawMessage
	sendErr    error
	loginCalls int
	sendCalls  int
}

func (m *mockTransportNeg) Login(_ context.Context, _, _ string) error {
	m.loginCalls++
	return m.loginErr
}

func (m *mockTransportNeg) Send(_ context.Context, _ string, _ json.RawMessage) (json.RawMessage, error) {
	m.sendCalls++
	return m.sendResp, m.sendErr
}

// noopFactory returns a factory that always returns the given mock.
func mockFactory(m *mockTransportNeg) NewTransportFunc {
	return func(_ string, _ *http.Client) Transport { return m }
}

func TestNegotiateKLAPSuccess(t *testing.T) {
	klapMock := &mockTransportNeg{sendResp: json.RawMessage(`{"ok":true}`)}
	legacyMock := &mockTransportNeg{}

	n := NewNegotiating("192.168.1.1", &http.Client{}, mockFactory(klapMock), mockFactory(legacyMock))

	err := n.Login(context.Background(), "user@example.com", "pass")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if klapMock.loginCalls != 1 {
		t.Errorf("expected KLAP loginCalls == 1, got %d", klapMock.loginCalls)
	}
	if legacyMock.loginCalls != 0 {
		t.Errorf("expected legacy loginCalls == 0, got %d", legacyMock.loginCalls)
	}

	// Send should delegate to KLAP.
	resp, err := n.Send(context.Background(), "get_device_info", nil)
	if err != nil {
		t.Fatalf("Send: unexpected error: %v", err)
	}
	if string(resp) != `{"ok":true}` {
		t.Errorf("expected KLAP response, got %s", resp)
	}
	if klapMock.sendCalls != 1 {
		t.Errorf("expected KLAP sendCalls == 1, got %d", klapMock.sendCalls)
	}
}

func TestNegotiateFallbackToLegacy(t *testing.T) {
	klapMock := &mockTransportNeg{loginErr: fmt.Errorf("klap: handshake1 failed: %w", ErrHandshake)}
	legacyMock := &mockTransportNeg{sendResp: json.RawMessage(`{"legacy":true}`)}

	n := NewNegotiating("192.168.1.1", &http.Client{}, mockFactory(klapMock), mockFactory(legacyMock))

	err := n.Login(context.Background(), "user@example.com", "pass")
	if err != nil {
		t.Fatalf("expected nil error after fallback, got %v", err)
	}

	if klapMock.loginCalls != 1 {
		t.Errorf("expected KLAP loginCalls == 1, got %d", klapMock.loginCalls)
	}
	if legacyMock.loginCalls != 1 {
		t.Errorf("expected legacy loginCalls == 1, got %d", legacyMock.loginCalls)
	}

	// Send should delegate to legacy.
	resp, err := n.Send(context.Background(), "get_device_info", nil)
	if err != nil {
		t.Fatalf("Send: unexpected error: %v", err)
	}
	if string(resp) != `{"legacy":true}` {
		t.Errorf("expected legacy response, got %s", resp)
	}
	if legacyMock.sendCalls != 1 {
		t.Errorf("expected legacy sendCalls == 1, got %d", legacyMock.sendCalls)
	}
}

func TestNegotiateNoFallbackOnErrAuth(t *testing.T) {
	klapMock := &mockTransportNeg{loginErr: fmt.Errorf("klap: invalid credentials: %w", ErrAuth)}
	legacyMock := &mockTransportNeg{}

	n := NewNegotiating("192.168.1.1", &http.Client{}, mockFactory(klapMock), mockFactory(legacyMock))

	err := n.Login(context.Background(), "user@example.com", "pass")
	if !errors.Is(err, ErrAuth) {
		t.Fatalf("expected ErrAuth, got %v", err)
	}

	if legacyMock.loginCalls != 0 {
		t.Errorf("expected legacy loginCalls == 0, got %d", legacyMock.loginCalls)
	}
}

func TestNegotiateNoFallbackOnNetworkError(t *testing.T) {
	klapMock := &mockTransportNeg{loginErr: errors.New("connection refused")}
	legacyMock := &mockTransportNeg{}

	n := NewNegotiating("192.168.1.1", &http.Client{}, mockFactory(klapMock), mockFactory(legacyMock))

	err := n.Login(context.Background(), "user@example.com", "pass")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if legacyMock.loginCalls != 0 {
		t.Errorf("expected legacy loginCalls == 0, got %d", legacyMock.loginCalls)
	}
}

func TestNegotiateNoFallbackOnContextCancelled(t *testing.T) {
	klapMock := &mockTransportNeg{loginErr: context.Canceled}
	legacyMock := &mockTransportNeg{}

	n := NewNegotiating("192.168.1.1", &http.Client{}, mockFactory(klapMock), mockFactory(legacyMock))

	err := n.Login(context.Background(), "user@example.com", "pass")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	if legacyMock.loginCalls != 0 {
		t.Errorf("expected legacy loginCalls == 0, got %d", legacyMock.loginCalls)
	}
}

func TestNegotiateSendWithoutLogin(t *testing.T) {
	klapMock := &mockTransportNeg{}
	legacyMock := &mockTransportNeg{}

	n := NewNegotiating("192.168.1.1", &http.Client{}, mockFactory(klapMock), mockFactory(legacyMock))

	_, err := n.Send(context.Background(), "get_device_info", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "negotiate: login has not been called" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestNegotiateSendDelegates(t *testing.T) {
	klapMock := &mockTransportNeg{sendResp: json.RawMessage(`{"result":"ok"}`)}
	legacyMock := &mockTransportNeg{}

	n := NewNegotiating("192.168.1.1", &http.Client{}, mockFactory(klapMock), mockFactory(legacyMock))

	if err := n.Login(context.Background(), "user@example.com", "pass"); err != nil {
		t.Fatalf("Login: %v", err)
	}

	// First Send.
	resp, err := n.Send(context.Background(), "get_device_info", nil)
	if err != nil {
		t.Fatalf("Send 1: %v", err)
	}
	if string(resp) != `{"result":"ok"}` {
		t.Errorf("Send 1: unexpected response: %s", resp)
	}

	// Second Send.
	resp, err = n.Send(context.Background(), "set_device_info", json.RawMessage(`{"device_on":true}`))
	if err != nil {
		t.Fatalf("Send 2: %v", err)
	}

	if klapMock.sendCalls != 2 {
		t.Errorf("expected 2 sendCalls, got %d", klapMock.sendCalls)
	}
}

func TestNegotiateLoginIdempotent(t *testing.T) {
	klapMock := &mockTransportNeg{}
	legacyMock := &mockTransportNeg{}

	n := NewNegotiating("192.168.1.1", &http.Client{}, mockFactory(klapMock), mockFactory(legacyMock))

	// First login.
	if err := n.Login(context.Background(), "user@example.com", "pass"); err != nil {
		t.Fatalf("Login 1: %v", err)
	}

	// Second login should be a no-op.
	if err := n.Login(context.Background(), "user@example.com", "pass"); err != nil {
		t.Fatalf("Login 2: %v", err)
	}

	if klapMock.loginCalls != 1 {
		t.Errorf("expected loginCalls == 1 (idempotent), got %d", klapMock.loginCalls)
	}
}

func TestNegotiateBothFail(t *testing.T) {
	klapMock := &mockTransportNeg{loginErr: fmt.Errorf("klap: handshake failed: %w", ErrHandshake)}
	legacyMock := &mockTransportNeg{loginErr: fmt.Errorf("legacy: invalid credentials: %w", ErrAuth)}

	n := NewNegotiating("192.168.1.1", &http.Client{}, mockFactory(klapMock), mockFactory(legacyMock))

	err := n.Login(context.Background(), "user@example.com", "pass")
	if err == nil {
		t.Fatal("expected error when both fail, got nil")
	}

	// The wrapped error should contain the legacy error (via %w).
	if !errors.Is(err, ErrAuth) {
		t.Errorf("expected ErrAuth from legacy, got %v", err)
	}

	// The error message should also mention the KLAP failure for diagnostics.
	if !strings.Contains(err.Error(), "klap handshake failure") {
		t.Errorf("expected error to mention klap failure, got %v", err)
	}

	if klapMock.loginCalls != 1 {
		t.Errorf("expected KLAP loginCalls == 1, got %d", klapMock.loginCalls)
	}
	if legacyMock.loginCalls != 1 {
		t.Errorf("expected legacy loginCalls == 1, got %d", legacyMock.loginCalls)
	}
}

// TestNegotiateConcurrentLoginSingleFlight verifies that concurrent Login
// calls are single-flighted: only one negotiation runs.
func TestNegotiateConcurrentLoginSingleFlight(t *testing.T) {
	// Use a factory that counts how many times it is called.
	var factoryCalls atomic.Int64
	klapFactory := func(_ string, _ *http.Client) Transport {
		factoryCalls.Add(1)
		return &mockTransportNeg{} // loginErr nil → Login succeeds
	}

	legacyMock := &mockTransportNeg{}
	n := NewNegotiating("192.168.1.1", &http.Client{}, klapFactory, mockFactory(legacyMock))

	const goroutines = 5
	var wg sync.WaitGroup
	errs := make([]error, goroutines)
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			errs[idx] = n.Login(context.Background(), "user@example.com", "pass")
		}(i)
	}

	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: unexpected error: %v", i, err)
		}
	}

	// Only one KLAP factory call should have been made (single-flighted).
	if fc := factoryCalls.Load(); fc != 1 {
		t.Errorf("expected 1 KLAP factory call (single-flighted), got %d", fc)
	}

	// Legacy should never have been called (KLAP succeeded).
	if legacyMock.loginCalls != 0 {
		t.Errorf("expected legacy loginCalls == 0, got %d", legacyMock.loginCalls)
	}
}

// TestNegotiateConcurrentLoginWaiterContextCancel verifies that a waiter
// whose context is cancelled returns ctx.Err() without blocking the negotiator.
func TestNegotiateConcurrentLoginWaiterContextCancel(t *testing.T) {
	negotiating := make(chan struct{})
	proceed := make(chan struct{})

	slowFactory := func(_ string, _ *http.Client) Transport {
		return &slowLoginTransport{
			onLogin: func() error {
				close(negotiating) // signal we're in Login
				<-proceed          // block until test says go
				return nil
			},
		}
	}
	legacyMock := &mockTransportNeg{}

	n := NewNegotiating("192.168.1.1", &http.Client{}, slowFactory, mockFactory(legacyMock))

	// Start the negotiator goroutine.
	var negotiatorErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		negotiatorErr = n.Login(context.Background(), "user@example.com", "pass")
	}()

	// Wait for the negotiator to enter Login.
	<-negotiating

	// Start a waiter with a short-lived context.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	waiterErr := n.Login(ctx, "user@example.com", "pass")

	if !errors.Is(waiterErr, context.DeadlineExceeded) {
		t.Errorf("waiter: expected context.DeadlineExceeded, got %v", waiterErr)
	}

	// Let the negotiator finish.
	close(proceed)
	wg.Wait()

	if negotiatorErr != nil {
		t.Errorf("negotiator: unexpected error: %v", negotiatorErr)
	}
}

// slowLoginTransport is a transport whose Login calls a hook function.
type slowLoginTransport struct {
	onLogin func() error
}

func (s *slowLoginTransport) Login(_ context.Context, _, _ string) error {
	return s.onLogin()
}

func (s *slowLoginTransport) Send(_ context.Context, _ string, _ json.RawMessage) (json.RawMessage, error) {
	return json.RawMessage(`{}`), nil
}

// TestNewNegotiatingPanicsOnNilFactory verifies that nil factories panic.
func TestNewNegotiatingPanicsOnNilFactory(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic for nil klapFactory, got none")
		}
	}()
	NewNegotiating("host", &http.Client{}, nil, mockFactory(&mockTransportNeg{}))
}

func TestNewNegotiatingPanicsOnNilLegacyFactory(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic for nil legacyFactory, got none")
		}
	}()
	NewNegotiating("host", &http.Client{}, mockFactory(&mockTransportNeg{}), nil)
}
