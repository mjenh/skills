# tapo

Go client for Tapo P100 smart plugs.

[![Go Reference](https://pkg.go.dev/badge/github.com/mjenh/skills/tapo.svg)](https://pkg.go.dev/github.com/mjenh/skills/tapo)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## Features

- Turn on, turn off, and toggle the plug
- Query device info with automatic base64 decoding of nickname and SSID
- KLAP and legacy transport with automatic negotiation (KLAP first)
- Environment-variable-based or explicit configuration
- Goroutine-safe — share a single `*Plug` across goroutines
- Zero external dependencies (stdlib only)

## Installation

```
go get github.com/mjenh/skills/tapo
```

Requires **Go 1.25** or later.

## Quickstart

### Explicit credentials

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/mjenh/skills/tapo"
)

func main() {
	ctx := context.Background()

	plug, err := tapo.NewPlug(ctx, "192.168.1.42", "you@example.com", "your-password")
	if err != nil {
		log.Fatal(err)
	}

	if err := plug.TurnOn(ctx); err != nil {
		log.Fatal(err)
	}

	info, err := plug.DeviceInfo(ctx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Device: %s (on=%t)\n", info.Nickname, info.DeviceOn)
}
```

### Environment variables

```go
plug, err := tapo.NewPlugFromEnv(ctx)
if err != nil {
	log.Fatal(err)
}
```

## Environment Variables

| Variable | Required | Description |
|---|---|---|
| `TAPO_HOST` | Yes (or `TAPO_IP`) | IP address of the Tapo device |
| `TAPO_EMAIL` | Yes | TP-Link account email |
| `TAPO_PASSWORD` | Yes | TP-Link account password |

`TAPO_IP` is accepted as an alias for `TAPO_HOST`. If both are set, `TAPO_HOST` takes precedence.

## API Reference

### Constructors

| Function | Description |
|---|---|
| `NewPlug(ctx, host, email, password, opts ...Option) (*Plug, error)` | Create a Plug with explicit credentials |
| `NewPlugFromEnv(ctx, opts ...Option) (*Plug, error)` | Create a Plug from environment variables |

### Methods

| Method | Description |
|---|---|
| `(*Plug).TurnOn(ctx) error` | Turn the plug on |
| `(*Plug).TurnOff(ctx) error` | Turn the plug off |
| `(*Plug).Toggle(ctx) error` | Toggle the current power state |
| `(*Plug).DeviceInfo(ctx) (*DeviceInfo, error)` | Query device information |

### DeviceInfo Fields

| Field | Type | Description |
|---|---|---|
| `DeviceOn` | `bool` | Current power state |
| `Model` | `string` | Device model (e.g. "P100") |
| `Nickname` | `string` | User-assigned name (decoded from base64) |
| `DeviceID` | `string` | Unique device identifier |
| `FirmwareVersion` | `string` | Firmware version string |
| `HardwareVersion` | `string` | Hardware version string |
| `IPAddress` | `string` | Device IP address |
| `MAC` | `string` | MAC address |
| `SSID` | `string` | Connected Wi-Fi SSID (decoded from base64) |

## Options

### WithTimeout

Override the default 10-second timeout for all operations.

```go
plug, err := tapo.NewPlug(ctx, host, email, password,
	tapo.WithTimeout(30 * time.Second),
)
```

### WithTransport

Force a specific transport protocol instead of auto-negotiation.

```go
plug, err := tapo.NewPlug(ctx, host, email, password,
	tapo.WithTransport("klap"),   // or "legacy"
)
```

The default behaviour is to auto-negotiate, trying KLAP first and falling back to the legacy protocol.

## Error Handling

The package exposes four sentinel errors:

| Sentinel | Meaning |
|---|---|
| `ErrAuth` | Authentication failed (wrong email or password) |
| `ErrTimeout` | Operation timed out |
| `ErrUnsupportedModel` | Device model is not P100 (result is still valid) |
| `ErrHandshake` | Transport handshake failed |

Use `errors.Is` to inspect errors:

```go
info, err := plug.DeviceInfo(ctx)
switch {
case errors.Is(err, tapo.ErrAuth):
	log.Fatal("bad credentials")
case errors.Is(err, tapo.ErrTimeout):
	log.Fatal("device unreachable")
case errors.Is(err, tapo.ErrHandshake):
	log.Fatal("handshake failed")
case errors.Is(err, tapo.ErrUnsupportedModel):
	// Warning only — info is still populated.
	log.Printf("unsupported model: %s", info.Model)
default:
	log.Fatal(err)
}
```

`ErrUnsupportedModel` is a warning: the `DeviceInfo` result is still valid, but the device is not a certified P100.

## Non-P100 Models

Commands are not blocked on non-P100 models. If `DeviceInfo` detects a model other than P100 it wraps `ErrUnsupportedModel` as a warning alongside a valid result. Version 1 is certified only for the P100.

## Concurrency

`*Plug` is goroutine-safe. You can share a single instance across multiple goroutines. Session management is serialized internally.

```go
var wg sync.WaitGroup
for i := 0; i < 3; i++ {
	wg.Add(1)
	go func() {
		defer wg.Done()
		info, err := plug.DeviceInfo(ctx)
		if err != nil {
			log.Println(err)
			return
		}
		fmt.Printf("on=%t\n", info.DeviceOn)
	}()
}
wg.Wait()
```

## Support Matrix

| Device | Firmware | Transport | Status |
|---|---|---|---|
| Tapo P100 | v1.4.4 Build 20240514 Rel 35017 | KLAP | Certified (v1.0.0) |

The legacy transport is available as a fallback for older firmware. Community reports of other firmware versions are welcome.

## Testing

Run unit tests:

```
go test ./...
```

Race detector:

```
go test -race ./...
```

Integration tests (when available) require a real Tapo P100 on the local network. Set the environment variables and run:

```
export TAPO_HOST=192.168.1.42
export TAPO_EMAIL=you@example.com
export TAPO_PASSWORD=your-password

go test -tags=integration ./...
```

The plug will be toggled during the test run. Note: integration tests use the `integration` build tag so they are excluded from `go test ./...` by default.

## CLI

```bash
go run ./cmd/tapo on
go run ./cmd/tapo info
```

Requires `TAPO_HOST`, `TAPO_EMAIL`, and `TAPO_PASSWORD` in the environment.

## Google ADK

```go
import tapoadk "github.com/mjenh/skills/tapo/adk"

tool, err := tapoadk.NewFromEnv()
```

## MCP / agent integration

See [`skill/SKILL.md`](skill/SKILL.md) and [`skill/tool.schema.json`](skill/tool.schema.json).

## Layout

```
tapo/             package tapo — pure Go, no ADK/MCP imports
adk/              Google ADK FunctionTool adapter
cmd/tapo/         CLI for manual testing
skill/            Agent/MCP integration docs and tool schema
internal/         KLAP and legacy transport implementations
```

## License

MIT. See [LICENSE](LICENSE).

## Disclaimer

This project is not affiliated with or endorsed by TP-Link. It uses a reverse-engineered local network protocol. All communication is LAN-only — no cloud services are contacted. There are zero external dependencies. Credentials are never logged or persisted.
