package tapo

import "time"

// config holds configuration applied via functional options.
type config struct {
	timeout       time.Duration
	transport     interface{} // internal transport.Transport; typed as interface{} to avoid exporting internal types (AD-1)
	transportName string      // "klap", "legacy", or "" (auto-negotiate)
	transportSet  bool        // true when WithTransport was called (distinguishes default from explicit "")
}

// defaultConfig returns the configuration with default values.
func defaultConfig() config {
	return config{
		timeout: 10 * time.Second,
	}
}

// Option configures a Plug client.
type Option func(*config)

// WithTimeout sets the per-request timeout for all commands issued by the Plug.
// The default is 10 seconds. The timeout is applied as a context deadline
// on each individual command (Login, Send), not as an overall client timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *config) {
		c.timeout = d
	}
}

// WithTransport forces a specific transport protocol, bypassing automatic
// negotiation. Valid values are "klap" and "legacy". Any other value
// causes NewPlug to return an error.
func WithTransport(name string) Option {
	return func(c *config) {
		c.transportName = name
		c.transportSet = true
	}
}

// withTransport injects a specific transport implementation, bypassing
// the default KLAP transport. Unexported because the transport.Transport
// interface is internal (AD-1). Used by tests and by NegotiatingTransport
// in Story 2.2.
func withTransport(t interface{}) Option {
	return func(c *config) {
		c.transport = t
	}
}
