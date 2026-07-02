# Changelog

All notable changes to [github.com/mjenh/skills/tapo](https://github.com/mjenh/skills/tapo) are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0](https://github.com/mjenh/skills/releases/tag/tapo/v1.0.0) - 2026-07-02

### Added

- Pure Go `tapo` package with `Plug`, `NewPlug`, `NewPlugFromEnv`, and functional options
- Power control: `TurnOn`, `TurnOff`, `Toggle`
- `DeviceInfo` returning structured device metadata with base64 field decoding
- KLAP and legacy transport with automatic negotiation (KLAP first)
- Typed sentinel errors: `ErrAuth`, `ErrTimeout`, `ErrUnsupportedModel`, `ErrHandshake`
- Client options: `WithTimeout`, `WithTransport`
- Goroutine-safe `*Plug` with lazy authentication and session re-auth
- Unit tests across root package and `internal/` transports
- Monorepo integration under `github.com/mjenh/skills/tapo`
- `tapo/adk` subpackage with Google ADK `tapo_plug` FunctionTool
- `cmd/tapo` CLI for on/off/toggle/info commands
- Co-located agent/MCP integration docs in `skill/SKILL.md` and `skill/tool.schema.json`
- `.env.example` documenting `TAPO_HOST`, `TAPO_EMAIL`, and `TAPO_PASSWORD`

### Requirements

- Go 1.25.0 or later
- Tapo P100 on the local network with TP-Link account credentials
