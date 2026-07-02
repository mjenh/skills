---
name: tapo
description: >-
  Control Tapo P100 smart plugs on the local network. Use when building MCP
  tools or agents that need to turn plugs on/off, toggle power, or read device
  state, or when integrating github.com/mjenh/skills/tapo into an agent runtime.
---

# Tapo Module

Pure Go library for controlling Tapo P100 smart plugs over the LAN.

## Go import

```go
import "github.com/mjenh/skills/tapo"
```

## Environment

| Variable | Required | Description |
|----------|----------|-------------|
| `TAPO_HOST` | Yes (or `TAPO_IP`) | IP address of the Tapo device |
| `TAPO_EMAIL` | Yes | TP-Link Tapo account email |
| `TAPO_PASSWORD` | Yes | TP-Link Tapo account password |

See `../.env.example` for a template.

## Public API

```go
plug, err := tapo.NewPlug(ctx, "192.168.1.42", email, password)
plug, err := tapo.NewPlugFromEnv(ctx)

err := plug.TurnOn(ctx)
err := plug.TurnOff(ctx)
err := plug.Toggle(ctx)
info, err := plug.DeviceInfo(ctx)
```

## MCP integration

Expose tool `tapo_plug` using the schema in `tool.schema.json`:

1. Parse `{ "action": "on" | "off" | "toggle" | "info" }` from the tool call.
2. Call the matching `Plug` method.
3. Return JSON with action result and optional `device` payload for `info`.

## Google ADK integration

Import the adapter subpackage instead of wiring ADK in your app:

```go
import tapoadk "github.com/mjenh/skills/tapo/adk"

tool, err := tapoadk.NewFromEnv()
```

## Errors

| Error | Meaning |
|-------|---------|
| `tapo.ErrAuth` | Invalid Tapo account credentials |
| `tapo.ErrTimeout` | Device unreachable or slow to respond |
| `tapo.ErrHandshake` | Transport negotiation or handshake failed |
| `tapo.ErrUnsupportedModel` | Device is not a certified P100 (warning; result may still be valid) |

Use `errors.Is` to inspect errors.
