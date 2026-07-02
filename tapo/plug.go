package tapo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/mjenh/skills/tapo/internal/transport"
	"github.com/mjenh/skills/tapo/internal/transport/klap"
	"github.com/mjenh/skills/tapo/internal/transport/legacy"
)

// Plug is a client for a single Tapo smart plug. Create one with NewPlug or
// NewPlugFromEnv. All public methods are safe for concurrent use from
// multiple goroutines.
type Plug struct {
	host     string
	email    string
	password string
	cfg      config
	tp       transport.Transport

	mu       sync.Mutex // guards loggedIn, model, and authGen reads
	loggedIn bool
	model    string  // cached device model from DeviceInfo
	authGen  uint64  // incremented on each successful re-auth

	authMu sync.Mutex // gates re-authentication decision (AD-5, separate from mu)
}

// NewPlug creates a new Plug client for the device at the given host.
// host is the IP address (and optional port) of the Tapo device on the LAN.
// email and password are the Tapo account credentials used by the mobile app.
//
// NewPlug validates that host, email, and password are non-empty but performs
// no network I/O (AD-7). Authentication occurs lazily on the first command.
//
// Use functional options to override defaults:
//
//	plug, err := tapo.NewPlug(ctx, "192.168.1.42", email, pass, tapo.WithTimeout(5*time.Second))
func NewPlug(_ context.Context, host, email, password string, opts ...Option) (*Plug, error) {
	if host == "" {
		return nil, fmt.Errorf("tapo: host must not be empty")
	}
	if email == "" {
		return nil, fmt.Errorf("tapo: email must not be empty")
	}
	if password == "" {
		return nil, fmt.Errorf("tapo: password must not be empty")
	}

	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}

	var tp transport.Transport
	if cfg.transport != nil {
		var ok bool
		tp, ok = cfg.transport.(transport.Transport)
		if !ok {
			return nil, fmt.Errorf("tapo: invalid transport option")
		}
	} else {
		httpClient := &http.Client{Timeout: cfg.timeout}
		switch {
		case cfg.transportName == "" && !cfg.transportSet:
			tp = transport.NewNegotiating(host, httpClient,
				func(h string, c *http.Client) transport.Transport {
					return klap.New(h, c)
				},
				func(h string, c *http.Client) transport.Transport {
					return legacy.New(h, c.Timeout)
				},
			)
		case cfg.transportName == "klap":
			tp = klap.New(host, httpClient)
		case cfg.transportName == "legacy":
			tp = legacy.New(host, cfg.timeout)
		default:
			return nil, fmt.Errorf("tapo: unknown transport %q (valid: \"klap\", \"legacy\")", cfg.transportName)
		}
	}

	return &Plug{
		host:     host,
		email:    email,
		password: password,
		cfg:      cfg,
		tp:       tp,
	}, nil
}

// NewPlugFromEnv creates a new Plug client using environment variables.
//
// Required variables:
//   - TAPO_HOST (preferred) or TAPO_IP (alias): device IP address
//   - TAPO_EMAIL: Tapo account email
//   - TAPO_PASSWORD: Tapo account password
//
// Returns an error listing all missing variables.
func NewPlugFromEnv(ctx context.Context, opts ...Option) (*Plug, error) {
	host := os.Getenv("TAPO_HOST")
	if host == "" {
		host = os.Getenv("TAPO_IP")
	}
	email := os.Getenv("TAPO_EMAIL")
	password := os.Getenv("TAPO_PASSWORD")

	var missing []string
	if host == "" {
		missing = append(missing, "TAPO_HOST")
	}
	if email == "" {
		missing = append(missing, "TAPO_EMAIL")
	}
	if password == "" {
		missing = append(missing, "TAPO_PASSWORD")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("tapo: missing required environment variables: %s", strings.Join(missing, ", "))
	}

	return NewPlug(ctx, host, email, password, opts...)
}

// login ensures the transport is authenticated. Called before every command.
// Serializes access to the loggedIn flag under mutex (AD-5); the mutex is
// never held during I/O. Concurrent first-calls may issue duplicate Login
// requests — this is harmless (idempotent) and avoids holding a mutex during
// network I/O on the initial connection. Session-expiry re-auth is serialized
// separately via authMu in execute().
func (p *Plug) login(ctx context.Context) error {
	p.mu.Lock()
	if p.loggedIn {
		p.mu.Unlock()
		return nil
	}
	p.mu.Unlock()

	if err := p.tp.Login(ctx, p.email, p.password); err != nil {
		return err
	}

	p.mu.Lock()
	p.loggedIn = true
	p.mu.Unlock()

	return nil
}

// applyTimeout wraps the given context with the configured per-request timeout.
func (p *Plug) applyTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if p.cfg.timeout > 0 {
		return context.WithTimeout(ctx, p.cfg.timeout)
	}
	return ctx, func() {}
}

// send applies the configured timeout, performs lazy login, and delegates to
// the transport. All command methods should use send rather than calling
// tp.Send directly.
func (p *Plug) send(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
	ctx, cancel := p.applyTimeout(ctx)
	defer cancel()

	if err := p.login(ctx); err != nil {
		return nil, err
	}

	return p.tp.Send(ctx, method, payload)
}

// execute wraps send with automatic re-auth on session expiry (NFR-3).
// If send returns ErrSessionExpired, execute acquires authMu, checks whether
// another goroutine already re-authenticated (via authGen), performs Login if
// needed, then retries send exactly once. Re-auth happens at most once per
// execute() call. Note that a composite method like Toggle issues two
// execute() calls (DeviceInfo + setDeviceOn), each independently eligible
// for one re-auth.
func (p *Plug) execute(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
	// Snapshot the current auth generation before sending.
	p.mu.Lock()
	gen := p.authGen
	p.mu.Unlock()

	result, err := p.send(ctx, method, payload)
	if err == nil || !errors.Is(err, transport.ErrSessionExpired) {
		return result, err
	}

	// Session expired — attempt re-auth under authMu.
	origErr := err
	p.authMu.Lock()

	// Check if another goroutine already re-authed while we waited.
	p.mu.Lock()
	currentGen := p.authGen
	p.mu.Unlock()

	if currentGen == gen {
		// No one else re-authed — we must do it.
		loginErr := p.tp.Login(ctx, p.email, p.password)
		if loginErr != nil {
			p.authMu.Unlock()
			return nil, fmt.Errorf("tapo: re-auth failed (%w) after session expiry: %w", loginErr, origErr)
		}
		p.mu.Lock()
		p.authGen++
		p.loggedIn = true
		p.mu.Unlock()
	}

	p.authMu.Unlock()

	// Retry exactly once — no further re-auth on this path.
	return p.send(ctx, method, payload)
}

// DeviceInfo retrieves the current state and metadata of the plug.
// On the first call, it triggers KLAP authentication (lazy login per AD-7).
//
// When the device reports a model other than P100, the returned error wraps
// ErrUnsupportedModel but the DeviceInfo is still fully populated (FR-8).
func (p *Plug) DeviceInfo(ctx context.Context) (*DeviceInfo, error) {
	result, err := p.execute(ctx, "get_device_info", nil)
	if err != nil {
		return nil, err
	}

	info, parseErr := parseDeviceInfo(result)
	if info != nil {
		p.mu.Lock()
		p.model = info.Model
		p.mu.Unlock()
	}

	return info, parseErr
}

// setDeviceInfoParams is the payload for the set_device_info command.
type setDeviceInfoParams struct {
	DeviceOn bool `json:"device_on"`
}

// setDeviceOn sends a set_device_info command to turn the device on or off.
// If the cached model is non-empty and not "P100", the returned error wraps
// ErrUnsupportedModel as a warning (the command still succeeded).
func (p *Plug) setDeviceOn(ctx context.Context, on bool) error {
	payload, err := json.Marshal(setDeviceInfoParams{DeviceOn: on})
	if err != nil {
		return fmt.Errorf("tapo: set_device_info: %w", err)
	}

	if _, err := p.execute(ctx, "set_device_info", payload); err != nil {
		return fmt.Errorf("tapo: set_device_info: %w", err)
	}

	p.mu.Lock()
	model := p.model
	p.mu.Unlock()

	if model != "" && model != "P100" {
		return fmt.Errorf("tapo: set_device_info: device model %q may not be fully supported: %w", model, ErrUnsupportedModel)
	}
	return nil
}

// TurnOn turns the plug on.
func (p *Plug) TurnOn(ctx context.Context) error {
	return p.setDeviceOn(ctx, true)
}

// TurnOff turns the plug off.
func (p *Plug) TurnOff(ctx context.Context) error {
	return p.setDeviceOn(ctx, false)
}

// Toggle reads the current device state and sets the inverse. If the device
// is currently on, it is turned off, and vice versa.
//
// If DeviceInfo returns ErrUnsupportedModel, the toggle still proceeds because
// the DeviceInfo result is populated. Any other DeviceInfo error aborts the
// toggle without guessing the state.
func (p *Plug) Toggle(ctx context.Context) error {
	info, err := p.DeviceInfo(ctx)
	if err != nil && !errors.Is(err, ErrUnsupportedModel) {
		return fmt.Errorf("tapo: toggle: %w", err)
	}
	// Defensive: info is guaranteed non-nil here per parseDeviceInfo's
	// contract (it returns a populated struct even for non-P100 models),
	// but guard against future changes that might break that invariant.
	if info == nil {
		return fmt.Errorf("tapo: toggle: device info unavailable")
	}
	return p.setDeviceOn(ctx, !info.DeviceOn)
}
