package transport_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/mjenh/skills/tapo/internal/transport"
)

// mockTransport is a minimal implementation that satisfies the Transport interface.
type mockTransport struct {
	loginCalled bool
	sendCalled  bool
}

func (m *mockTransport) Login(_ context.Context, _, _ string) error {
	m.loginCalled = true
	return nil
}

func (m *mockTransport) Send(_ context.Context, _ string, _ json.RawMessage) (json.RawMessage, error) {
	m.sendCalled = true
	return json.RawMessage(`{}`), nil
}

func TestMockTransportSatisfiesInterface(t *testing.T) {
	var tr transport.Transport = &mockTransport{}

	if err := tr.Login(context.Background(), "user@example.com", "pass"); err != nil {
		t.Fatalf("Login: %v", err)
	}

	result, err := tr.Send(context.Background(), "get_device_info", nil)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	if string(result) != "{}" {
		t.Errorf("unexpected result: %s", result)
	}

	mock := tr.(*mockTransport)
	if !mock.loginCalled {
		t.Error("Login was not called")
	}
	if !mock.sendCalled {
		t.Error("Send was not called")
	}
}
