package tapo

import "github.com/mjenh/skills/tapo/internal/transport"

// Sentinel errors re-exported from internal/transport so that both the root
// package and transport implementations share the same values.
var (
	// ErrAuth is returned when authentication with the Tapo device fails
	// due to invalid credentials.
	ErrAuth = transport.ErrAuth

	// ErrHandshake is returned when the transport handshake with the device
	// fails for non-credential reasons (network, protocol mismatch).
	ErrHandshake = transport.ErrHandshake

	// ErrTimeout is returned when an operation exceeds its deadline.
	ErrTimeout = transport.ErrTimeout

	// ErrUnsupportedModel is returned as a warning when the connected device
	// reports a model other than P100. The operation result is still valid.
	ErrUnsupportedModel = transport.ErrUnsupportedModel
)
