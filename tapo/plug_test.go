package tapo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mjenh/skills/tapo/internal/transport"
	"github.com/mjenh/skills/tapo/internal/transport/klap"
	"github.com/mjenh/skills/tapo/internal/transport/legacy"
)

// mockTransport implements transport.Transport for testing.
// All field access is synchronized via mu for goroutine safety (Story 1.5).
// When loginFunc or sendFunc hooks are set, they are called instead of
// the default behavior; hooks handle their own synchronization.
type mockTransport struct {
	mu sync.Mutex

	loginCalled bool
	loginCount  int
	loginErr    error
	sendResult  json.RawMessage
	sendErr     error
	sendMethod  string

	// Enhanced fields for Story 1.4 tests.
	sendPayload json.RawMessage           // last payload passed to Send
	sendCalls   []mockSendCall            // ordered record of all Send calls
	sendResults map[string]mockSendResult // per-method results (method -> result)

	// Story 1.5: optional hooks for custom behavior in concurrent tests.
	loginFunc func(ctx context.Context, email, password string) error
	sendFunc  func(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error)
}

type mockSendCall struct {
	Method  string
	Payload json.RawMessage
}

type mockSendResult struct {
	Result json.RawMessage
	Err    error
}

func (m *mockTransport) Login(ctx context.Context, email, password string) error {
	if m.loginFunc != nil {
		return m.loginFunc(ctx, email, password)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.loginCalled = true
	m.loginCount++
	return m.loginErr
}

func (m *mockTransport) Send(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
	if m.sendFunc != nil {
		return m.sendFunc(ctx, method, payload)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendMethod = method
	m.sendPayload = payload
	m.sendCalls = append(m.sendCalls, mockSendCall{Method: method, Payload: payload})

	// If per-method results are configured, use them.
	if m.sendResults != nil {
		if r, ok := m.sendResults[method]; ok {
			return r.Result, r.Err
		}
	}

	return m.sendResult, m.sendErr
}

// --- Task 8: NewPlug tests ---

func TestNewPlug_ValidInputs(t *testing.T) {
	plug, err := NewPlug(context.Background(), "192.168.1.1", "test@example.com", "password")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if plug == nil {
		t.Fatal("expected non-nil plug")
	}
}

func TestNewPlug_EmptyHost(t *testing.T) {
	plug, err := NewPlug(context.Background(), "", "test@example.com", "password")
	if err == nil {
		t.Fatal("expected error for empty host")
	}
	if !strings.Contains(err.Error(), "host") {
		t.Errorf("error should mention 'host', got: %v", err)
	}
	if plug != nil {
		t.Error("expected nil plug")
	}
}

func TestNewPlug_EmptyEmail(t *testing.T) {
	plug, err := NewPlug(context.Background(), "192.168.1.1", "", "password")
	if err == nil {
		t.Fatal("expected error for empty email")
	}
	if !strings.Contains(err.Error(), "email") {
		t.Errorf("error should mention 'email', got: %v", err)
	}
	if plug != nil {
		t.Error("expected nil plug")
	}
}

func TestNewPlug_EmptyPassword(t *testing.T) {
	plug, err := NewPlug(context.Background(), "192.168.1.1", "test@example.com", "")
	if err == nil {
		t.Fatal("expected error for empty password")
	}
	if !strings.Contains(err.Error(), "password") {
		t.Errorf("error should mention 'password', got: %v", err)
	}
	if plug != nil {
		t.Error("expected nil plug")
	}
}

func TestNewPlug_NoNetworkIO(t *testing.T) {
	mock := &mockTransport{}
	plug, err := NewPlug(context.Background(), "192.168.1.1", "test@example.com", "password", withTransport(mock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plug == nil {
		t.Fatal("expected non-nil plug")
	}
	if mock.loginCalled {
		t.Error("Login should not have been called during construction")
	}
	if mock.sendMethod != "" {
		t.Error("Send should not have been called during construction")
	}
}

func TestNewPlug_WithTimeoutOption(t *testing.T) {
	mock := &mockTransport{}
	plug, err := NewPlug(context.Background(), "192.168.1.1", "test@example.com", "password",
		withTransport(mock), WithTimeout(5*time.Second))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plug.cfg.timeout != 5*time.Second {
		t.Errorf("expected timeout 5s, got %v", plug.cfg.timeout)
	}
}

func TestNewPlug_DefaultTimeout(t *testing.T) {
	mock := &mockTransport{}
	plug, err := NewPlug(context.Background(), "192.168.1.1", "test@example.com", "password",
		withTransport(mock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plug.cfg.timeout != 10*time.Second {
		t.Errorf("expected default timeout 10s, got %v", plug.cfg.timeout)
	}
}

// --- Task 9: NewPlugFromEnv tests ---

func TestNewPlugFromEnv_WithTAPO_HOST(t *testing.T) {
	t.Setenv("TAPO_HOST", "192.168.1.1")
	t.Setenv("TAPO_EMAIL", "test@example.com")
	t.Setenv("TAPO_PASSWORD", "password")

	mock := &mockTransport{}
	plug, err := NewPlugFromEnv(context.Background(), withTransport(mock))
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if plug == nil {
		t.Fatal("expected non-nil plug")
	}
	if plug.host != "192.168.1.1" {
		t.Errorf("expected host 192.168.1.1, got %s", plug.host)
	}
}

func TestNewPlugFromEnv_FallbackToTAPO_IP(t *testing.T) {
	// TAPO_HOST is explicitly empty, fall back to TAPO_IP
	t.Setenv("TAPO_HOST", "")
	t.Setenv("TAPO_IP", "10.0.0.1")
	t.Setenv("TAPO_EMAIL", "test@example.com")
	t.Setenv("TAPO_PASSWORD", "password")

	mock := &mockTransport{}
	plug, err := NewPlugFromEnv(context.Background(), withTransport(mock))
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if plug == nil {
		t.Fatal("expected non-nil plug")
	}
	if plug.host != "10.0.0.1" {
		t.Errorf("expected host 10.0.0.1, got %s", plug.host)
	}
}

func TestNewPlugFromEnv_HOST_TakesPrecedence(t *testing.T) {
	t.Setenv("TAPO_HOST", "192.168.1.1")
	t.Setenv("TAPO_IP", "10.0.0.1")
	t.Setenv("TAPO_EMAIL", "test@example.com")
	t.Setenv("TAPO_PASSWORD", "password")

	mock := &mockTransport{}
	plug, err := NewPlugFromEnv(context.Background(), withTransport(mock))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plug.host != "192.168.1.1" {
		t.Errorf("expected TAPO_HOST to take precedence, got host %s", plug.host)
	}
}

func TestNewPlugFromEnv_MissingHost(t *testing.T) {
	// Neither TAPO_HOST nor TAPO_IP set
	t.Setenv("TAPO_HOST", "")
	t.Setenv("TAPO_IP", "")
	t.Setenv("TAPO_EMAIL", "test@example.com")
	t.Setenv("TAPO_PASSWORD", "password")

	plug, err := NewPlugFromEnv(context.Background())
	if err == nil {
		t.Fatal("expected error for missing host")
	}
	if !strings.Contains(err.Error(), "TAPO_HOST") {
		t.Errorf("error should mention TAPO_HOST, got: %v", err)
	}
	if plug != nil {
		t.Error("expected nil plug")
	}
}

func TestNewPlugFromEnv_MissingEmail(t *testing.T) {
	t.Setenv("TAPO_HOST", "192.168.1.1")
	t.Setenv("TAPO_EMAIL", "")
	t.Setenv("TAPO_PASSWORD", "password")

	plug, err := NewPlugFromEnv(context.Background())
	if err == nil {
		t.Fatal("expected error for missing email")
	}
	if !strings.Contains(err.Error(), "TAPO_EMAIL") {
		t.Errorf("error should mention TAPO_EMAIL, got: %v", err)
	}
	if plug != nil {
		t.Error("expected nil plug")
	}
}

func TestNewPlugFromEnv_MissingPassword(t *testing.T) {
	t.Setenv("TAPO_HOST", "192.168.1.1")
	t.Setenv("TAPO_EMAIL", "test@example.com")
	t.Setenv("TAPO_PASSWORD", "")

	plug, err := NewPlugFromEnv(context.Background())
	if err == nil {
		t.Fatal("expected error for missing password")
	}
	if !strings.Contains(err.Error(), "TAPO_PASSWORD") {
		t.Errorf("error should mention TAPO_PASSWORD, got: %v", err)
	}
	if plug != nil {
		t.Error("expected nil plug")
	}
}

func TestNewPlugFromEnv_AllMissing(t *testing.T) {
	// Explicitly clear all env vars
	t.Setenv("TAPO_HOST", "")
	t.Setenv("TAPO_IP", "")
	t.Setenv("TAPO_EMAIL", "")
	t.Setenv("TAPO_PASSWORD", "")

	plug, err := NewPlugFromEnv(context.Background())
	if err == nil {
		t.Fatal("expected error for all missing vars")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "TAPO_HOST") {
		t.Errorf("error should mention TAPO_HOST, got: %v", err)
	}
	if !strings.Contains(errMsg, "TAPO_EMAIL") {
		t.Errorf("error should mention TAPO_EMAIL, got: %v", err)
	}
	if !strings.Contains(errMsg, "TAPO_PASSWORD") {
		t.Errorf("error should mention TAPO_PASSWORD, got: %v", err)
	}
	if plug != nil {
		t.Error("expected nil plug")
	}
}

func TestNewPlugFromEnv_AcceptsOptions(t *testing.T) {
	t.Setenv("TAPO_HOST", "192.168.1.1")
	t.Setenv("TAPO_EMAIL", "test@example.com")
	t.Setenv("TAPO_PASSWORD", "password")

	mock := &mockTransport{}
	plug, err := NewPlugFromEnv(context.Background(), withTransport(mock), WithTimeout(3*time.Second))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plug.cfg.timeout != 3*time.Second {
		t.Errorf("expected timeout 3s, got %v", plug.cfg.timeout)
	}
}

// --- Task 10: DeviceInfo tests ---

func validDeviceInfoJSON() json.RawMessage {
	return json.RawMessage(`{
		"device_on": true,
		"model": "P100",
		"nickname": "TXkgUGx1Zw==",
		"device_id": "abc123",
		"fw_ver": "1.4.4 Build 20240514 Rel 35017",
		"hw_ver": "1.0",
		"ip": "192.168.1.42",
		"mac": "AA:BB:CC:DD:EE:FF",
		"ssid": "TXlXaUZp"
	}`)
}

func newTestPlug(mock *mockTransport) *Plug {
	plug, _ := NewPlug(context.Background(), "192.168.1.1", "test@example.com", "password",
		withTransport(mock))
	return plug
}

func TestDeviceInfo_TriggersLogin(t *testing.T) {
	mock := &mockTransport{
		sendResult: validDeviceInfoJSON(),
	}
	plug := newTestPlug(mock)

	_, _ = plug.DeviceInfo(context.Background())
	if !mock.loginCalled {
		t.Error("DeviceInfo should trigger login on first call")
	}
}

func TestDeviceInfo_SendsGetDeviceInfo(t *testing.T) {
	mock := &mockTransport{
		sendResult: validDeviceInfoJSON(),
	}
	plug := newTestPlug(mock)

	_, _ = plug.DeviceInfo(context.Background())
	if mock.sendMethod != "get_device_info" {
		t.Errorf("expected send method 'get_device_info', got %q", mock.sendMethod)
	}
}

func TestDeviceInfo_PopulatedStruct(t *testing.T) {
	mock := &mockTransport{
		sendResult: validDeviceInfoJSON(),
	}
	plug := newTestPlug(mock)

	info, err := plug.DeviceInfo(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info == nil {
		t.Fatal("expected non-nil DeviceInfo")
	}
	if !info.DeviceOn {
		t.Error("expected DeviceOn to be true")
	}
	if info.Model != "P100" {
		t.Errorf("expected Model P100, got %s", info.Model)
	}
	if info.Nickname != "My Plug" {
		t.Errorf("expected Nickname 'My Plug', got %q", info.Nickname)
	}
	if info.DeviceID != "abc123" {
		t.Errorf("expected DeviceID abc123, got %s", info.DeviceID)
	}
	if info.FirmwareVersion != "1.4.4 Build 20240514 Rel 35017" {
		t.Errorf("expected FirmwareVersion '1.4.4 Build 20240514 Rel 35017', got %s", info.FirmwareVersion)
	}
	if info.HardwareVersion != "1.0" {
		t.Errorf("expected HardwareVersion 1.0, got %s", info.HardwareVersion)
	}
	if info.IPAddress != "192.168.1.42" {
		t.Errorf("expected IPAddress 192.168.1.42, got %s", info.IPAddress)
	}
	if info.MAC != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("expected MAC AA:BB:CC:DD:EE:FF, got %s", info.MAC)
	}
	if info.SSID != "MyWiFi" {
		t.Errorf("expected SSID 'MyWiFi', got %q", info.SSID)
	}
}

func TestDeviceInfo_DecodesBase64Nickname(t *testing.T) {
	mock := &mockTransport{
		sendResult: json.RawMessage(`{"model":"P100","nickname":"TXkgUGx1Zw=="}`),
	}
	plug := newTestPlug(mock)

	info, err := plug.DeviceInfo(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Nickname != "My Plug" {
		t.Errorf("expected decoded nickname 'My Plug', got %q", info.Nickname)
	}
}

func TestDeviceInfo_DecodesBase64SSID(t *testing.T) {
	mock := &mockTransport{
		sendResult: json.RawMessage(`{"model":"P100","ssid":"TXlXaUZp"}`),
	}
	plug := newTestPlug(mock)

	info, err := plug.DeviceInfo(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.SSID != "MyWiFi" {
		t.Errorf("expected decoded SSID 'MyWiFi', got %q", info.SSID)
	}
}

func TestDeviceInfo_PreservesRawOnInvalidBase64(t *testing.T) {
	mock := &mockTransport{
		sendResult: json.RawMessage(`{"model":"P100","nickname":"Not-Valid-Base64!!!"}`),
	}
	plug := newTestPlug(mock)

	info, err := plug.DeviceInfo(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Nickname != "Not-Valid-Base64!!!" {
		t.Errorf("expected raw nickname preserved, got %q", info.Nickname)
	}
}

func TestDeviceInfo_PreservesRawOnNonUTF8Base64(t *testing.T) {
	// "\xff\xfe" is valid base64 (encoded as "//4=") but not valid UTF-8
	mock := &mockTransport{
		sendResult: json.RawMessage(`{"model":"P100","nickname":"//4="}`),
	}
	plug := newTestPlug(mock)

	info, err := plug.DeviceInfo(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should preserve raw value since decoded bytes are not valid UTF-8
	if info.Nickname != "//4=" {
		t.Errorf("expected raw nickname preserved for non-UTF-8 base64, got %q", info.Nickname)
	}
}

func TestDeviceInfo_ErrUnsupportedModelForNonP100(t *testing.T) {
	mock := &mockTransport{
		sendResult: json.RawMessage(`{"model":"P110","device_on":true,"nickname":"TXkgUGx1Zw=="}`),
	}
	plug := newTestPlug(mock)

	info, err := plug.DeviceInfo(context.Background())
	if !errors.Is(err, ErrUnsupportedModel) {
		t.Errorf("expected ErrUnsupportedModel, got %v", err)
	}
	if info == nil {
		t.Fatal("expected non-nil DeviceInfo even with unsupported model")
	}
	if info.Model != "P110" {
		t.Errorf("expected Model P110, got %s", info.Model)
	}
}

func TestDeviceInfo_NilErrorForP100(t *testing.T) {
	mock := &mockTransport{
		sendResult: json.RawMessage(`{"model":"P100"}`),
	}
	plug := newTestPlug(mock)

	_, err := plug.DeviceInfo(context.Background())
	if err != nil {
		t.Errorf("expected nil error for P100 model, got %v", err)
	}
}

func TestDeviceInfo_PropagatesLoginError(t *testing.T) {
	mock := &mockTransport{
		loginErr: fmt.Errorf("mock: %w", ErrAuth),
	}
	plug := newTestPlug(mock)

	info, err := plug.DeviceInfo(context.Background())
	if !errors.Is(err, ErrAuth) {
		t.Errorf("expected ErrAuth, got %v", err)
	}
	if info != nil {
		t.Error("expected nil DeviceInfo on login error")
	}
}

func TestDeviceInfo_PropagatesSendError(t *testing.T) {
	mock := &mockTransport{
		sendErr: fmt.Errorf("mock: network error"),
	}
	plug := newTestPlug(mock)

	info, err := plug.DeviceInfo(context.Background())
	if err == nil {
		t.Error("expected error on send failure")
	}
	if info != nil {
		t.Error("expected nil DeviceInfo on send error")
	}
}

func TestDeviceInfo_SkipsLoginOnSubsequentCalls(t *testing.T) {
	mock := &mockTransport{
		sendResult: json.RawMessage(`{"model":"P100"}`),
	}
	plug := newTestPlug(mock)

	_, _ = plug.DeviceInfo(context.Background())
	_, _ = plug.DeviceInfo(context.Background())

	if mock.loginCount != 1 {
		t.Errorf("expected Login called exactly once, got %d", mock.loginCount)
	}
}

func TestDeviceInfo_CachesModel(t *testing.T) {
	mock := &mockTransport{
		sendResult: json.RawMessage(`{"model":"P100","device_on":true}`),
	}
	plug := newTestPlug(mock)

	_, _ = plug.DeviceInfo(context.Background())
	plug.mu.Lock()
	model := plug.model
	plug.mu.Unlock()
	if model != "P100" {
		t.Errorf("expected cached model P100, got %q", model)
	}
}

// --- Story 1.4: TurnOn / TurnOff tests ---

func TestTurnOn_SendsCorrectCommand(t *testing.T) {
	mock := &mockTransport{
		sendResults: map[string]mockSendResult{
			"set_device_info": {Result: json.RawMessage(`{}`), Err: nil},
		},
	}
	plug := newTestPlug(mock)
	// Pre-cache model as P100 to avoid ErrUnsupportedModel.
	plug.mu.Lock()
	plug.model = "P100"
	plug.mu.Unlock()

	err := plug.TurnOn(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if mock.sendMethod != "set_device_info" {
		t.Errorf("expected send method 'set_device_info', got %q", mock.sendMethod)
	}
	if !strings.Contains(string(mock.sendPayload), `"device_on":true`) {
		t.Errorf("expected payload with device_on:true, got %s", mock.sendPayload)
	}
}

func TestTurnOff_SendsCorrectCommand(t *testing.T) {
	mock := &mockTransport{
		sendResults: map[string]mockSendResult{
			"set_device_info": {Result: json.RawMessage(`{}`), Err: nil},
		},
	}
	plug := newTestPlug(mock)
	plug.mu.Lock()
	plug.model = "P100"
	plug.mu.Unlock()

	err := plug.TurnOff(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if mock.sendMethod != "set_device_info" {
		t.Errorf("expected send method 'set_device_info', got %q", mock.sendMethod)
	}
	if !strings.Contains(string(mock.sendPayload), `"device_on":false`) {
		t.Errorf("expected payload with device_on:false, got %s", mock.sendPayload)
	}
}

func TestTurnOn_PropagatesTransportError(t *testing.T) {
	mock := &mockTransport{
		sendResults: map[string]mockSendResult{
			"set_device_info": {Err: fmt.Errorf("mock: %w", ErrAuth)},
		},
	}
	plug := newTestPlug(mock)

	err := plug.TurnOn(context.Background())
	if !errors.Is(err, ErrAuth) {
		t.Errorf("expected ErrAuth, got %v", err)
	}
}

func TestTurnOff_PropagatesTransportError(t *testing.T) {
	mock := &mockTransport{
		sendResults: map[string]mockSendResult{
			"set_device_info": {Err: fmt.Errorf("mock: %w", ErrTimeout)},
		},
	}
	plug := newTestPlug(mock)

	err := plug.TurnOff(context.Background())
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("expected ErrTimeout, got %v", err)
	}
}

func TestTurnOn_PropagatesLoginError(t *testing.T) {
	mock := &mockTransport{
		loginErr: fmt.Errorf("mock: %w", ErrAuth),
	}
	plug := newTestPlug(mock)

	err := plug.TurnOn(context.Background())
	if !errors.Is(err, ErrAuth) {
		t.Errorf("expected ErrAuth, got %v", err)
	}
}

func TestTurnOn_ErrorSymmetryWithTurnOff(t *testing.T) {
	for _, sentinel := range []error{ErrAuth, ErrTimeout, ErrHandshake} {
		mockOn := &mockTransport{
			sendResults: map[string]mockSendResult{
				"set_device_info": {Err: fmt.Errorf("mock: %w", sentinel)},
			},
		}
		plugOn := newTestPlug(mockOn)
		errOn := plugOn.TurnOn(context.Background())

		mockOff := &mockTransport{
			sendResults: map[string]mockSendResult{
				"set_device_info": {Err: fmt.Errorf("mock: %w", sentinel)},
			},
		}
		plugOff := newTestPlug(mockOff)
		errOff := plugOff.TurnOff(context.Background())

		if !errors.Is(errOn, sentinel) {
			t.Errorf("TurnOn: expected %v, got %v", sentinel, errOn)
		}
		if !errors.Is(errOff, sentinel) {
			t.Errorf("TurnOff: expected %v, got %v", sentinel, errOff)
		}
	}
}

// --- Story 1.4: Toggle tests ---

func TestToggle_FromOnToOff(t *testing.T) {
	mock := &mockTransport{
		sendResults: map[string]mockSendResult{
			"get_device_info": {Result: json.RawMessage(`{"model":"P100","device_on":true}`), Err: nil},
			"set_device_info": {Result: json.RawMessage(`{}`), Err: nil},
		},
	}
	plug := newTestPlug(mock)

	err := plug.Toggle(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	// Should have two calls: get_device_info then set_device_info.
	if len(mock.sendCalls) != 2 {
		t.Fatalf("expected 2 send calls, got %d", len(mock.sendCalls))
	}
	if mock.sendCalls[0].Method != "get_device_info" {
		t.Errorf("expected first call to be get_device_info, got %q", mock.sendCalls[0].Method)
	}
	if mock.sendCalls[1].Method != "set_device_info" {
		t.Errorf("expected second call to be set_device_info, got %q", mock.sendCalls[1].Method)
	}
	// Was on, so should turn off.
	if !strings.Contains(string(mock.sendCalls[1].Payload), `"device_on":false`) {
		t.Errorf("expected set_device_info payload with device_on:false, got %s", mock.sendCalls[1].Payload)
	}
}

func TestToggle_FromOffToOn(t *testing.T) {
	mock := &mockTransport{
		sendResults: map[string]mockSendResult{
			"get_device_info": {Result: json.RawMessage(`{"model":"P100","device_on":false}`), Err: nil},
			"set_device_info": {Result: json.RawMessage(`{}`), Err: nil},
		},
	}
	plug := newTestPlug(mock)

	err := plug.Toggle(context.Background())
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(mock.sendCalls) != 2 {
		t.Fatalf("expected 2 send calls, got %d", len(mock.sendCalls))
	}
	// Was off, so should turn on.
	if !strings.Contains(string(mock.sendCalls[1].Payload), `"device_on":true`) {
		t.Errorf("expected set_device_info payload with device_on:true, got %s", mock.sendCalls[1].Payload)
	}
}

func TestToggle_FailsWhenDeviceInfoFails(t *testing.T) {
	mock := &mockTransport{
		sendErr: fmt.Errorf("mock: network unreachable"),
	}
	plug := newTestPlug(mock)

	err := plug.Toggle(context.Background())
	if err == nil {
		t.Fatal("expected error when DeviceInfo fails")
	}
	// Should only have one call (get_device_info) -- no set_device_info.
	if len(mock.sendCalls) != 1 {
		t.Errorf("expected 1 send call (get_device_info only), got %d", len(mock.sendCalls))
	}
	if mock.sendCalls[0].Method != "get_device_info" {
		t.Errorf("expected only call to be get_device_info, got %q", mock.sendCalls[0].Method)
	}
}

func TestToggle_DoesNotGuessStateOnError(t *testing.T) {
	mock := &mockTransport{
		sendResults: map[string]mockSendResult{
			"get_device_info": {Err: fmt.Errorf("mock: %w", ErrTimeout)},
		},
	}
	plug := newTestPlug(mock)

	err := plug.Toggle(context.Background())
	if !errors.Is(err, ErrTimeout) {
		t.Errorf("expected ErrTimeout, got %v", err)
	}
	// set_device_info should never have been called.
	for _, call := range mock.sendCalls {
		if call.Method == "set_device_info" {
			t.Error("set_device_info should not have been called when DeviceInfo fails")
		}
	}
}

func TestToggle_PropagatesSetDeviceOnError(t *testing.T) {
	mock := &mockTransport{
		sendResults: map[string]mockSendResult{
			"get_device_info": {Result: json.RawMessage(`{"model":"P100","device_on":true}`), Err: nil},
			"set_device_info": {Err: fmt.Errorf("mock: %w", ErrAuth)},
		},
	}
	plug := newTestPlug(mock)

	err := plug.Toggle(context.Background())
	if !errors.Is(err, ErrAuth) {
		t.Errorf("expected ErrAuth from set_device_info failure, got %v", err)
	}
	// Both calls should have been made.
	if len(mock.sendCalls) != 2 {
		t.Fatalf("expected 2 send calls, got %d", len(mock.sendCalls))
	}
	if mock.sendCalls[0].Method != "get_device_info" {
		t.Errorf("expected first call get_device_info, got %q", mock.sendCalls[0].Method)
	}
	if mock.sendCalls[1].Method != "set_device_info" {
		t.Errorf("expected second call set_device_info, got %q", mock.sendCalls[1].Method)
	}
}

// --- Story 1.4: ErrUnsupportedModel warning tests ---

func TestTurnOn_NonP100_ReturnsErrUnsupportedModel(t *testing.T) {
	mock := &mockTransport{
		sendResults: map[string]mockSendResult{
			"set_device_info": {Result: json.RawMessage(`{}`), Err: nil},
		},
	}
	plug := newTestPlug(mock)
	// Pre-cache model as P110 (non-P100).
	plug.mu.Lock()
	plug.model = "P110"
	plug.mu.Unlock()

	err := plug.TurnOn(context.Background())
	if !errors.Is(err, ErrUnsupportedModel) {
		t.Errorf("expected ErrUnsupportedModel, got %v", err)
	}
	// Command should still have been sent.
	if mock.sendMethod != "set_device_info" {
		t.Errorf("expected set_device_info to be sent, got %q", mock.sendMethod)
	}
}

func TestTurnOff_NonP100_ReturnsErrUnsupportedModel(t *testing.T) {
	mock := &mockTransport{
		sendResults: map[string]mockSendResult{
			"set_device_info": {Result: json.RawMessage(`{}`), Err: nil},
		},
	}
	plug := newTestPlug(mock)
	plug.mu.Lock()
	plug.model = "P110"
	plug.mu.Unlock()

	err := plug.TurnOff(context.Background())
	if !errors.Is(err, ErrUnsupportedModel) {
		t.Errorf("expected ErrUnsupportedModel, got %v", err)
	}
}

func TestToggle_NonP100_ReturnsErrUnsupportedModel(t *testing.T) {
	mock := &mockTransport{
		sendResults: map[string]mockSendResult{
			"get_device_info": {Result: json.RawMessage(`{"model":"P110","device_on":true}`), Err: nil},
			"set_device_info": {Result: json.RawMessage(`{}`), Err: nil},
		},
	}
	plug := newTestPlug(mock)

	err := plug.Toggle(context.Background())
	if !errors.Is(err, ErrUnsupportedModel) {
		t.Errorf("expected ErrUnsupportedModel, got %v", err)
	}
	// set_device_info should have been called with device_on:false (toggling from on).
	if len(mock.sendCalls) < 2 {
		t.Fatalf("expected at least 2 send calls, got %d", len(mock.sendCalls))
	}
	if !strings.Contains(string(mock.sendCalls[1].Payload), `"device_on":false`) {
		t.Errorf("expected set_device_info with device_on:false, got %s", mock.sendCalls[1].Payload)
	}
}

func TestTurnOn_P100_ReturnsNilError(t *testing.T) {
	mock := &mockTransport{
		sendResults: map[string]mockSendResult{
			"set_device_info": {Result: json.RawMessage(`{}`), Err: nil},
		},
	}
	plug := newTestPlug(mock)
	plug.mu.Lock()
	plug.model = "P100"
	plug.mu.Unlock()

	err := plug.TurnOn(context.Background())
	if err != nil {
		t.Errorf("expected nil error for P100, got %v", err)
	}
}

func TestTurnOn_EmptyModel_ReturnsNilError(t *testing.T) {
	mock := &mockTransport{
		sendResults: map[string]mockSendResult{
			"set_device_info": {Result: json.RawMessage(`{}`), Err: nil},
		},
	}
	plug := newTestPlug(mock)
	// model is empty (not yet cached) -- should not warn.

	err := plug.TurnOn(context.Background())
	if err != nil {
		t.Errorf("expected nil error when model is unknown, got %v", err)
	}
}

// --- Story 1.5: Goroutine safety & session resilience tests ---

// TestConcurrentAccess_NoRace spawns multiple goroutines calling TurnOn, TurnOff,
// Toggle, and DeviceInfo concurrently on a single Plug. The test must pass under
// go test -race with zero data race reports.
func TestConcurrentAccess_NoRace(t *testing.T) {
	mock := &mockTransport{
		sendFunc: func(_ context.Context, method string, _ json.RawMessage) (json.RawMessage, error) {
			if method == "get_device_info" {
				return json.RawMessage(`{"model":"P100","device_on":true}`), nil
			}
			return json.RawMessage(`{}`), nil
		},
	}
	plug := newTestPlug(mock)

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			ctx := context.Background()
			switch n % 4 {
			case 0:
				_ = plug.TurnOn(ctx)
			case 1:
				_ = plug.TurnOff(ctx)
			case 2:
				_ = plug.Toggle(ctx)
			case 3:
				_, _ = plug.DeviceInfo(ctx)
			}
		}(i)
	}
	wg.Wait()
}

// TestSessionExpiry_SingleReAuth verifies that when multiple goroutines
// simultaneously encounter session expiry (error 9999), exactly one goroutine
// performs re-auth while the others wait and then all retry successfully.
func TestSessionExpiry_SingleReAuth(t *testing.T) {
	var loginCount atomic.Int64

	// Track whether login has been called — after login, Send succeeds.
	var reauthed atomic.Bool

	mock := &mockTransport{
		loginFunc: func(_ context.Context, _, _ string) error {
			loginCount.Add(1)
			reauthed.Store(true)
			return nil
		},
		sendFunc: func(_ context.Context, method string, _ json.RawMessage) (json.RawMessage, error) {
			if !reauthed.Load() {
				return nil, fmt.Errorf("mock: %w", transport.ErrSessionExpired)
			}
			if method == "get_device_info" {
				return json.RawMessage(`{"model":"P100","device_on":true}`), nil
			}
			return json.RawMessage(`{}`), nil
		},
	}
	plug := newTestPlug(mock)
	// Mark as already logged in so the initial login() is skipped.
	plug.mu.Lock()
	plug.loggedIn = true
	plug.mu.Unlock()

	const goroutines = 5
	var wg sync.WaitGroup
	errs := make([]error, goroutines)
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			errs[n] = plug.TurnOn(context.Background())
		}(i)
	}
	wg.Wait()

	// All goroutines should succeed.
	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: expected nil error, got %v", i, err)
		}
	}

	// Login should have been called exactly once (the initial login + one re-auth).
	// The initial login() in send() sees loggedIn=true and skips, so loginFunc
	// is only called from the re-auth path.
	if lc := loginCount.Load(); lc != 1 {
		t.Errorf("expected Login called exactly 1 time for re-auth, got %d", lc)
	}
}

// TestSessionExpiry_ReAuthFailure verifies that when Send returns
// ErrSessionExpired and the subsequent Login also fails, the returned error
// wraps both the original session-expiry context and the re-auth failure,
// inspectable via errors.Is.
func TestSessionExpiry_ReAuthFailure(t *testing.T) {
	loginErr := fmt.Errorf("mock login: %w", ErrAuth)

	mock := &mockTransport{
		loginFunc: func(_ context.Context, _, _ string) error {
			return loginErr
		},
		sendFunc: func(_ context.Context, _ string, _ json.RawMessage) (json.RawMessage, error) {
			return nil, fmt.Errorf("mock: %w", transport.ErrSessionExpired)
		},
	}
	plug := newTestPlug(mock)
	plug.mu.Lock()
	plug.loggedIn = true
	plug.mu.Unlock()

	err := plug.TurnOn(context.Background())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// The error should wrap ErrSessionExpired (from the original Send failure).
	if !errors.Is(err, transport.ErrSessionExpired) {
		t.Errorf("expected error to wrap ErrSessionExpired, got: %v", err)
	}

	// The error should also wrap ErrAuth (from the failed Login).
	if !errors.Is(err, ErrAuth) {
		t.Errorf("expected error to wrap ErrAuth, got: %v", err)
	}

	// The error message should mention re-auth.
	if !strings.Contains(err.Error(), "re-auth") {
		t.Errorf("expected error to mention 're-auth', got: %v", err)
	}
}

// TestSessionExpiry_RetrySucceeds verifies the happy path: a single goroutine
// hits session expiry, re-auths successfully, and the retry succeeds.
// Asserts exactly 2 Send calls total (original + retry).
func TestSessionExpiry_RetrySucceeds(t *testing.T) {
	var sendCount atomic.Int64

	mock := &mockTransport{
		loginFunc: func(_ context.Context, _, _ string) error {
			return nil
		},
		sendFunc: func(_ context.Context, method string, _ json.RawMessage) (json.RawMessage, error) {
			n := sendCount.Add(1)
			if n == 1 {
				// First call: session expired.
				return nil, fmt.Errorf("mock: %w", transport.ErrSessionExpired)
			}
			// Second call (retry after re-auth): succeed.
			return json.RawMessage(`{}`), nil
		},
	}
	plug := newTestPlug(mock)
	plug.mu.Lock()
	plug.loggedIn = true
	plug.model = "P100"
	plug.mu.Unlock()

	err := plug.TurnOn(context.Background())
	if err != nil {
		t.Fatalf("expected nil error after retry, got %v", err)
	}

	if sc := sendCount.Load(); sc != 2 {
		t.Errorf("expected exactly 2 Send calls (original + retry), got %d", sc)
	}
}

// TestSessionExpiry_RetryAlsoFails verifies that when re-auth succeeds but the
// retry Send also fails, the retry error is returned (not the original).
func TestSessionExpiry_RetryAlsoFails(t *testing.T) {
	var sendCount atomic.Int64
	retryErr := fmt.Errorf("mock: retry network error")

	mock := &mockTransport{
		loginFunc: func(_ context.Context, _, _ string) error {
			return nil
		},
		sendFunc: func(_ context.Context, method string, _ json.RawMessage) (json.RawMessage, error) {
			n := sendCount.Add(1)
			if n == 1 {
				return nil, fmt.Errorf("mock: %w", transport.ErrSessionExpired)
			}
			// Retry also fails with a different error.
			return nil, retryErr
		},
	}
	plug := newTestPlug(mock)
	plug.mu.Lock()
	plug.loggedIn = true
	plug.model = "P100"
	plug.mu.Unlock()

	err := plug.TurnOn(context.Background())
	if err == nil {
		t.Fatal("expected error from retry, got nil")
	}

	// The returned error should be from the retry, not the original session expiry.
	if !strings.Contains(err.Error(), "retry network error") {
		t.Errorf("expected retry error, got: %v", err)
	}

	// It should NOT wrap ErrSessionExpired (that was the original, not the retry).
	if errors.Is(err, transport.ErrSessionExpired) {
		t.Errorf("expected error to NOT wrap ErrSessionExpired (should be retry error), got: %v", err)
	}

	if sc := sendCount.Load(); sc != 2 {
		t.Errorf("expected exactly 2 Send calls, got %d", sc)
	}
}

// --- Story 2.2: WithTransport tests ---

func TestNewPlugDefaultTransport(t *testing.T) {
	plug, err := NewPlug(context.Background(), "192.168.1.1", "test@example.com", "password")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := plug.tp.(*transport.NegotiatingTransport); !ok {
		t.Errorf("expected *transport.NegotiatingTransport, got %T", plug.tp)
	}
}

func TestWithTransportKLAP(t *testing.T) {
	plug, err := NewPlug(context.Background(), "192.168.1.1", "test@example.com", "password",
		WithTransport("klap"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := plug.tp.(*klap.Transport); !ok {
		t.Errorf("expected *klap.Transport, got %T", plug.tp)
	}
}

func TestWithTransportLegacy(t *testing.T) {
	plug, err := NewPlug(context.Background(), "192.168.1.1", "test@example.com", "password",
		WithTransport("legacy"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := plug.tp.(*legacy.Transport); !ok {
		t.Errorf("expected *legacy.Transport, got %T", plug.tp)
	}
}

func TestWithTransportInvalid(t *testing.T) {
	plug, err := NewPlug(context.Background(), "192.168.1.1", "test@example.com", "password",
		WithTransport("invalid"))
	if err == nil {
		t.Fatal("expected error for invalid transport name")
	}
	if !strings.Contains(err.Error(), "unknown transport") {
		t.Errorf("expected error to mention 'unknown transport', got: %v", err)
	}
	if !strings.Contains(err.Error(), "invalid") {
		t.Errorf("expected error to contain the invalid name, got: %v", err)
	}
	if plug != nil {
		t.Error("expected nil plug")
	}
}

func TestWithTransportEmptyString(t *testing.T) {
	plug, err := NewPlug(context.Background(), "192.168.1.1", "test@example.com", "password",
		WithTransport(""))
	if err == nil {
		t.Fatal("expected error for empty transport name")
	}
	if !strings.Contains(err.Error(), "unknown transport") {
		t.Errorf("expected error to mention 'unknown transport', got: %v", err)
	}
	if plug != nil {
		t.Error("expected nil plug")
	}
}

// --- Additional tests: parseDeviceInfo error paths, WithTransport, NewPlug validation ---

// TestParseDeviceInfoInvalidJSON verifies that DeviceInfo surfaces a parse
// error (via parseDeviceInfo) when the transport returns malformed JSON.
// parseDeviceInfo is unexported, so this exercises it through the exported
// DeviceInfo method with a mock transport returning invalid JSON.
func TestParseDeviceInfoInvalidJSON(t *testing.T) {
	mock := &mockTransport{
		sendResult: json.RawMessage(`{not valid json`),
	}
	plug := newTestPlug(mock)

	info, err := plug.DeviceInfo(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to parse device info") {
		t.Errorf("expected error to mention 'failed to parse device info', got: %v", err)
	}
	if info != nil {
		t.Error("expected nil DeviceInfo on JSON parse error")
	}
}

// TestParseDeviceInfoEmptyObject verifies that an empty JSON object `{}`
// yields a populated (zero-value) DeviceInfo with an empty Model, and that
// the returned error wraps ErrUnsupportedModel since "" != "P100".
func TestParseDeviceInfoEmptyObject(t *testing.T) {
	mock := &mockTransport{
		sendResult: json.RawMessage(`{}`),
	}
	plug := newTestPlug(mock)

	info, err := plug.DeviceInfo(context.Background())
	if !errors.Is(err, ErrUnsupportedModel) {
		t.Errorf("expected ErrUnsupportedModel, got %v", err)
	}
	if info == nil {
		t.Fatal("expected non-nil DeviceInfo even for empty object")
	}
	if info.Model != "" {
		t.Errorf("expected empty Model, got %q", info.Model)
	}
}

