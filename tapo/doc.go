// Package tapo provides a Go client for controlling Tapo P100 smart plugs
// over the local network.
//
// Construct a client with [NewPlug] or [NewPlugFromEnv], then call [Plug.DeviceInfo],
// [Plug.TurnOn], [Plug.TurnOff], or [Plug.Toggle] to interact with the device.
// The client authenticates lazily on the first command and is safe for
// concurrent use.
//
// Two transport protocols are supported: KLAP (default) and the older legacy
// protocol. By default the client auto-negotiates, trying KLAP first and
// falling back to legacy. Use [WithTransport] to force a specific protocol.
//
// # Quick start
//
// Create a plug with explicit credentials and turn it on:
//
//	ctx := context.Background()
//	plug, err := tapo.NewPlug(ctx, "192.168.1.42", "you@example.com", "your-password")
//	if err != nil {
//		log.Fatal(err)
//	}
//	if err := plug.TurnOn(ctx); err != nil {
//		log.Fatal(err)
//	}
//
// Or use environment variables (TAPO_HOST, TAPO_EMAIL, TAPO_PASSWORD):
//
//	plug, err := tapo.NewPlugFromEnv(ctx)
//
// # Error handling
//
// All errors are inspectable with [errors.Is] using the package sentinels:
// [ErrAuth], [ErrTimeout], [ErrUnsupportedModel], and [ErrHandshake].
// [ErrUnsupportedModel] is a warning — the result is still valid when the
// device is not a P100.
//
// See the project README for the full support matrix and integration test
// instructions.
package tapo
