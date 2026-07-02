package transport

import (
	"context"
	"encoding/json"
	"errors"
)

// Transport defines the interface for communicating with a Tapo device.
type Transport interface {
	Login(ctx context.Context, email, password string) error
	Send(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error)
}

// Sentinel errors shared across the root package and transport implementations.
// The root tapo package re-exports these so callers use tapo.ErrAuth, etc.
var (
	ErrAuth             = errors.New("tapo: authentication failed")
	ErrHandshake        = errors.New("tapo: handshake failed")
	ErrTimeout          = errors.New("tapo: operation timed out")
	ErrUnsupportedModel = errors.New("tapo: unsupported device model")

	// ErrSessionExpired is returned by Transport.Send when the device
	// responds with error code 9999, indicating the session has expired.
	// This is an internal sentinel — it is NOT re-exported by the root
	// tapo package. The Plug layer uses it to trigger automatic re-auth.
	ErrSessionExpired = errors.New("tapo: session expired")
)
