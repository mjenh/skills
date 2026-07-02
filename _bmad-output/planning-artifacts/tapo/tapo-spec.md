---
id: SPEC-tapo
companions:
  - glossary.md
  - addendum.md
  - tapo-architecture.md
sources:
  - tapo-prd.md
---

> **Canonical contract.** This SPEC and the files in `companions:` are the complete, preservation-validated contract for what to build, test, and validate. Source documents listed in frontmatter are for traceability only — consult them only if you need narrative rationale or prose color this contract intentionally omits.

# Tapo Go Client Library

## Why

Go developers automating home or small-office environments lack a reliable, idiomatic library for controlling Tapo smart plugs over the local network. Cloud-dependent control adds latency, creates outage sensitivity, and complicates privacy-sensitive deployments. This is a pain to solve: anyone writing Go automation that touches Tapo hardware today must shell out to Python helpers, use stale libraries without KLAP support, or reverse-engineer the protocol themselves. The existing Go options (rk295/tapo-go — stale, legacy-only, unavailable source) do not meet current firmware requirements. **tapo** v1 fills this gap as a focused, MIT-licensed, open-source module for P100 smart plugs.

## Capabilities

- **CAP-1: Construct Plug client**
  - **intent:** Developer creates a Plug client bound to one host and Tapo account credentials, with optional configuration for timeout, retry, and transport.
  - **success:** Construction fails with a descriptive error when host is empty or credentials are missing. All network operations honor the caller's `context.Context` cancellation and deadline.

- **CAP-2: Authenticate with Plug**
  - **intent:** Client establishes an authenticated session with the Plug before executing commands, and transparently re-authenticates on session expiry.
  - **success:** Failed authentication returns a distinct error discriminating invalid credentials, network failure, and protocol mismatch. Session expiry (error code `9999` or transport-equivalent) triggers exactly one automatic re-authentication before surfacing failure to the caller.

- **CAP-3: Env-based client construction**
  - **intent:** Developer constructs a client from standard environment variables instead of passing credentials explicitly.
  - **success:** Reads `TAPO_HOST` (preferred) / `TAPO_IP` (alias), `TAPO_EMAIL`, `TAPO_PASSWORD`. Missing required variables produce an error listing which are absent. Env helper is optional sugar; explicit constructor remains primary API.

- **CAP-4: Turn Plug on**
  - **intent:** Developer turns the Plug's relay on via the client.
  - **success:** Successful call sets relay state to on (verifiable via subsequent DeviceInfo). Returns error on auth failure, unreachable host, or device rejection. Non-P100 model does not block execution if the device accepts the command (see CAP-8).

- **CAP-5: Turn Plug off**
  - **intent:** Developer turns the Plug's relay off via the client.
  - **success:** Successful call sets relay state to off. Error and unsupported-model behavior symmetric with CAP-4.

- **CAP-6: Toggle Plug state**
  - **intent:** Developer toggles the Plug to the opposite of its current on/off state in one call.
  - **success:** Reads current state then sets the inverse. Returns error without guessing if current state cannot be read. Unsupported-model warning behavior matches CAP-4.

- **CAP-7: Retrieve DeviceInfo**
  - **intent:** Developer fetches a structured snapshot of the Plug's current state and hardware metadata.
  - **success:** Response includes at minimum: `DeviceOn` (bool), `Model`, `Nickname`, `DeviceID`, `FirmwareVersion`, `HardwareVersion`, `IPAddress`, `MAC`. Base64-encoded string fields (e.g. Nickname, SSID) decoded to plain UTF-8 before return. Succeeds against P100 firmware v1.4.4 Build 20240514 Rel 35017 via KLAP, and against legacy-firmware units via fallback.

- **CAP-8: Warn on unsupported device model**
  - **intent:** Library signals when the connected device is not a P100 without preventing operations the device accepts.
  - **success:** `ErrUnsupportedModel` sentinel is detectable via `errors.Is` on any command against a non-P100 device. Control commands (TurnOn, TurnOff, Toggle) attempt execution regardless of model; unsupported model alone does not cause failure if the device accepts the command. Both operation outcome and warning are observable on the returned error.

- **CAP-9: Negotiate transport protocol**
  - **intent:** Client auto-selects the correct local transport (KLAP or legacy) without caller configuration.
  - **success:** P100 on firmware v1.4.4 Build 20240514 Rel 35017 connects via KLAP without caller intervention. Legacy-only units connect after KLAP failure and legacy retry. Working transport is recorded and reused on the same client. Callers may override auto-negotiation via Client options for debugging.

- **CAP-10: Goroutine-safe client**
  - **intent:** Multiple goroutines invoke methods on the same client concurrently without data races or session corruption.
  - **success:** Concurrent TurnOn, TurnOff, Toggle, and DeviceInfo calls from separate goroutines do not trigger the Go race detector under `-race`. Session establishment and re-authentication are serialized internally so only one login handshake runs at a time.

## Constraints

- P100 smart plug only; all other Tapo device types (P110, P115, P105, power strips, bulbs, hubs, sensors, cameras) are out of scope for v1.
- Go 1.24+ required as stated in `go.mod` (1.22 is EOL; 1.24 enforces RSA-1024 minimum key size required by legacy transport).
- Local LAN communication only; no cloud, internet, or TP-Link remote API dependency.
- MIT license.
- Minimal external dependencies; prefer stdlib for HTTP, crypto, JSON. Any non-stdlib dependency requires justification in README.
- Credentials supplied by caller; never persisted to disk by the library. Never logged at any log level.
- No telemetry or phone-home behavior.
- All exported methods accept `context.Context` for cancellation and deadline propagation.
- Default per-request timeout: 10 seconds; overridable via Client options.
- All exported errors inspectable with `errors.Is` / `errors.As`; sentinel errors for: authentication failure, network timeout, unsupported model, protocol handshake failure.
- Semver compliance after `v1.0.0` tag; breaking API changes only in major versions.

## Non-goals

- Cloud API integration (TP-Link cloud REST, OAuth, remote access outside the LAN).
- Non-P100 device types in v1.
- Device discovery (UDP/mDNS scanning); caller supplies host.
- Energy monitoring (`get_energy_usage`, `get_current_power`).
- Scheduling, scenes, away mode.
- GUI, mobile app, or Home Assistant integration.
- Fork or compatibility layer for rk295/tapo-go.
- CLI binary shipped in-module.
- Official TP-Link/Tapo vendor endorsement.

## Success signal

A new contributor clones the repo, runs `go get github.com/mjenh/tapo`, follows the README quickstart, and controls a real P100 on their LAN within 15 minutes — constructing a client, toggling power, and reading DeviceInfo with predictable error handling. The module reaches a `v1.0.0` semver tag with zero breaking API changes after tagging.

## Assumptions

- Transport override option is available via Client options for debugging or forced legacy mode.
- Integration tests are documented for manual execution against real hardware; not required in CI without hardware present.
- Windows is supported best-effort; Linux and macOS are the primary test targets.
- Debug logging is an optional Client option; credentials are always redacted.

## Open Questions

<!-- None identified. All PRD assumptions are carried forward above. -->
