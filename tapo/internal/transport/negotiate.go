package transport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
)

// Compile-time interface check.
var _ Transport = (*NegotiatingTransport)(nil)

// NewTransportFunc creates a Transport from a host and HTTP client.
type NewTransportFunc func(host string, client *http.Client) Transport

// NegotiatingTransport tries KLAP first, falling back to legacy on handshake
// failure. Once a protocol succeeds, it is cached for subsequent calls.
// Concurrent Login calls are single-flighted: the first caller performs
// negotiation while subsequent callers wait for the result.
type NegotiatingTransport struct {
	host   string
	client *http.Client

	mu          sync.Mutex
	cached      Transport
	negotiating bool       // true while a Login is in progress
	negotiated  chan struct{} // closed when the in-progress Login completes
	negErr      error      // result of the in-progress Login (valid after negotiated is closed)

	// Factory functions for creating transports. Both must be non-nil.
	newKLAP   NewTransportFunc
	newLegacy NewTransportFunc
}

// NewNegotiating creates a NegotiatingTransport with the given factory
// functions for creating KLAP and legacy transports. The factories are
// provided by the caller to avoid circular imports between the transport
// package and its sub-packages. Both factories must be non-nil.
func NewNegotiating(host string, client *http.Client, klapFactory, legacyFactory NewTransportFunc) *NegotiatingTransport {
	if klapFactory == nil {
		panic("transport: NewNegotiating called with nil klapFactory")
	}
	if legacyFactory == nil {
		panic("transport: NewNegotiating called with nil legacyFactory")
	}
	if client == nil {
		client = &http.Client{}
	}
	return &NegotiatingTransport{
		host:      host,
		client:    client,
		newKLAP:   klapFactory,
		newLegacy: legacyFactory,
	}
}

// Login negotiates the transport protocol. It tries KLAP first; if KLAP
// returns ErrHandshake, it falls back to legacy. Any other error (ErrAuth,
// network errors, context errors) aborts immediately without fallback.
// Concurrent calls are single-flighted: the first caller performs negotiation
// while subsequent callers wait for the result. The mutex is never held
// during network I/O.
func (n *NegotiatingTransport) Login(ctx context.Context, email, password string) error {
	n.mu.Lock()

	// Fast path: already negotiated.
	if n.cached != nil {
		n.mu.Unlock()
		return nil
	}

	// Another goroutine is negotiating — wait for its result.
	if n.negotiating {
		ch := n.negotiated
		n.mu.Unlock()
		select {
		case <-ch:
			// Negotiation finished; check result.
			n.mu.Lock()
			err := n.negErr
			n.mu.Unlock()
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// We are the negotiator.
	n.negotiating = true
	n.negotiated = make(chan struct{})
	n.mu.Unlock()

	err := n.negotiate(ctx, email, password)

	// Publish result and wake waiters.
	n.mu.Lock()
	n.negErr = err
	n.negotiating = false
	close(n.negotiated)
	n.mu.Unlock()

	return err
}

// negotiate performs the actual KLAP-first, legacy-fallback handshake.
// Called exactly once per successful negotiation cycle.
func (n *NegotiatingTransport) negotiate(ctx context.Context, email, password string) error {
	// Try KLAP first.
	klapTP := n.newKLAP(n.host, n.client)
	klapErr := klapTP.Login(ctx, email, password)
	if klapErr == nil {
		n.mu.Lock()
		n.cached = klapTP
		n.mu.Unlock()
		return nil
	}

	// Only fall back on ErrHandshake.
	if !errors.Is(klapErr, ErrHandshake) {
		return klapErr
	}

	// Try legacy.
	legacyTP := n.newLegacy(n.host, n.client)
	legacyErr := legacyTP.Login(ctx, email, password)
	if legacyErr == nil {
		n.mu.Lock()
		n.cached = legacyTP
		n.mu.Unlock()
		return nil
	}

	// Both failed — wrap both errors for diagnostics.
	return fmt.Errorf("negotiate: legacy login failed (%w) after klap handshake failure (%v)", legacyErr, klapErr)
}

// Send delegates to the cached transport. Returns an error if Login has not
// been called or negotiation has not completed.
func (n *NegotiatingTransport) Send(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
	n.mu.Lock()
	tp := n.cached
	n.mu.Unlock()

	if tp == nil {
		return nil, fmt.Errorf("negotiate: login has not been called")
	}

	return tp.Send(ctx, method, payload)
}
