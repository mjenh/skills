---
title: Tapo Go Client Library
status: final
created: 2026-07-01
updated: 2026-07-02
license: MIT
module: github.com/mjenh/tapo
go_version: "1.24"
---

# PRD: Tapo Go Client Library

## 0. Document Purpose

This PRD defines v1 requirements for **tapo**, a greenfield Go client library for controlling Tapo smart plugs over the local network. It is written for the library maintainer, downstream contributors, and architects who will implement the module and derive epics/stories from it.

The document uses a Glossary-anchored vocabulary, features grouped with globally numbered Functional Requirements (FRs), and an Assumptions Index (§11) for remaining inferred details. Technical transport and protocol decisions that inform but do not belong in requirement statements live in `addendum.md`.

**Reference implementations (informative, not dependencies):** [fabiankachlock/tapo-api](https://github.com/fabiankachlock/tapo-api) (KLAP, typed device factories) and [tess1o/tapo-go](https://github.com/tess1o/tapo-go) (`context.Context`, retry options). The legacy [rk295/tapo-go](https://pkg.go.dev/github.com/rk295/tapo-go) library is explicitly out of scope as a design baseline due to staleness and lack of KLAP support.

---

## 1. Vision

Developers automating home or small-office environments need a reliable, idiomatic Go library to control Tapo smart plugs without routing commands through vendor cloud APIs. Cloud-dependent control adds latency, creates outage sensitivity, and complicates privacy-sensitive deployments. A well-designed local client lets Go services, CLIs, and automation backends integrate Tapo hardware alongside other systems using standard Go patterns.

**tapo** v1 is a focused, open-source Go module that connects to a Tapo P100 smart plug on the local network, authenticates with the same Tapo account credentials used by the mobile app, switches the plug on or off, and returns structured device information. The library prioritizes clarity of public API, correctness across current P100 firmware (including KLAP transport), and a foundation that future device types can extend without breaking v1 consumers.

Success means another Go developer can `go get` the module, point it at a P100 on their LAN, and control it in a few lines of code—with errors, timeouts, and session handling behaving predictably enough to trust in production automation.

---

## 2. Target User

### 2.1 Jobs To Be Done

- **Control a plug from Go code** — Turn a P100 on or off as part of a script, cron job, or microservice without shelling out to curl or a Python helper.
- **Read plug state programmatically** — Know whether the plug is on, what the device reports (nickname, model, firmware), and use that in conditional logic.
- **Integrate with local automation** — Bridge Tapo hardware into broader Go-based home automation, energy workflows, or custom dashboards that prefer LAN control.
- **Ship dependable open-source tooling** — Publish a module others can import with a stable v1 API, clear docs, and MIT licensing.

### 2.2 Non-Users (v1)

- End users who want a mobile app or GUI (this is a developer library, not a consumer product).
- Operators of P110/P115 energy-monitoring plugs, power strips, bulbs, hubs, or cameras (deferred to later versions).
- Integrators who require official TP-Link/Tapo API support or cloud OAuth (local LAN protocol only in v1).

### 2.3 Key User Journeys

**UJ-1. Alex wires a P100 into a Go automation script.** Alex, a backend developer building a home-energy cron job, has a P100 on the LAN and Tapo app credentials. They `go get github.com/mjenh/tapo`, construct a Plug client with host IP and credentials, call `TurnOn`, then `DeviceInfo` to confirm `DeviceOn == true`. The whole flow completes in under two seconds on a healthy network. **Edge case:** if the plug is unreachable, they get a wrapped network error with context, not a panic.

**UJ-2. Sam publishes a small CLI wrapper for teammates.** Sam imports tapo into a command-line tool that reads `TAPO_HOST`, `TAPO_EMAIL`, and `TAPO_PASSWORD` from the environment, logs in once per invocation, and toggles a plug. Sam expects env-based configuration helpers and documented error types so the CLI can print actionable messages.

---

## 3. Glossary

- **Tapo account** — Email and password pair registered with the Tapo mobile app; used for local device authentication. Not a separate API key.
- **Device** — Any Tapo hardware endpoint addressable on the LAN. v1 supports only the Plug subtype.
- **Plug** — A single-outlet Tapo smart plug. v1 target model: **P100**.
- **P100** — Tapo Wi-Fi smart plug without energy monitoring. Supports on/off control and device info via local API commands `get_device_info` and `set_device_info`.
- **Host** — IP address (optional port) of the Plug on the local network, e.g. `192.168.1.42` or `192.168.1.42:80`.
- **Client** — A configured library instance bound to one Host and Tapo account credentials, responsible for session lifecycle and command dispatch.
- **Session** — Authenticated state between Client and Plug after successful login, including cookies/tokens required for subsequent commands.
- **KLAP** — Newer Tapo local transport protocol (seed handshake, derived AES keys, sequence counter). Required by many current firmware builds.
- **Legacy protocol** — Older Tapo local transport (RSA handshake + AES `securePassthrough` wrapper). Still present on some devices/firmware.
- **DeviceInfo** — Structured snapshot returned by `get_device_info` (on/off state, model, nickname, firmware versions, network metadata, etc.).
- **ErrUnsupportedModel** — Documented sentinel error indicating the connected device reports a model other than P100. Surfaces as a **warning** (inspectable via `errors.Is`); it does not by itself mean a control command failed.
- **Module** — The Go package published at `github.com/mjenh/tapo`.

---

## 4. Features

### 4.1 Client Construction and Authentication

**Description:** A developer creates a Client for a specific Plug Host using Tapo account credentials, establishes a Session, and reuses that Session for subsequent commands. Construction accepts `context.Context` for cancellation and deadline propagation (pattern from tess1o/tapo-go). Login occurs on first command if not already authenticated. The Client transparently re-authenticates when the Session expires (error code `9999` or transport-equivalent). Realizes UJ-1, UJ-2.

**Functional Requirements:**

#### FR-1: Construct Plug client

A developer can create a Plug Client given Host, Tapo account email, and Tapo account password. Realizes UJ-1.

**Consequences (testable):**
- Client construction returns an error if Host is empty or credentials are missing.
- Client accepts optional configuration (timeout, retry policy) without breaking the minimal constructor signature.
- All network operations honor the caller's `context.Context` cancellation and deadline.

#### FR-2: Authenticate with Plug

The Client establishes a Session with the Plug using Tapo account credentials before executing control or info commands. Realizes UJ-1, UJ-2.

**Consequences (testable):**
- Failed authentication returns a distinct, documented error (invalid credentials vs. network failure vs. protocol mismatch).
- Successful authentication allows at least one subsequent command without re-entering credentials manually.
- Session expiry during use triggers automatic re-authentication once before surfacing failure to the caller.

#### FR-3: Configure client from environment

A developer can construct a Client from standard environment variables for Host and Tapo account credentials. Realizes UJ-2.

**Consequences (testable):**
- Supported variables: `TAPO_HOST` (preferred), `TAPO_IP` (alias), `TAPO_EMAIL`, `TAPO_PASSWORD`.
- Missing required variables produce an error listing which variables are absent.
- Environment helper is optional sugar; explicit constructor remains the primary API.

#### FR-9: Negotiate transport protocol

The Client selects the local transport automatically: attempt KLAP first; on a distinguishable protocol/handshake failure, fall back to the legacy protocol. Realizes UJ-1.

**Consequences (testable):**
- A P100 on firmware v1.4.4 Build 20240514 Rel 35017 connects successfully via KLAP without caller configuration.
- Legacy-only P100 units still connect after KLAP failure and legacy retry.
- Transport selection is recorded internally so subsequent commands on the same Client reuse the working transport.
- Callers may override auto-negotiation via Client options for debugging or forced legacy `[ASSUMPTION]`.

**Feature-specific NFRs:**
- Credentials must not be logged by the library at default log levels.

---

### 4.2 Plug Power Control

**Description:** A developer switches a P100 on or off through the Client. Commands map to the Plug's `set_device_info` / `device_on` semantics. Realizes UJ-1.

**Functional Requirements:**

#### FR-4: Turn Plug on

A developer can turn the Plug on via the Client. Realizes UJ-1.

**Consequences (testable):**
- Successful call results in the Plug's relay state being on (verified by subsequent DeviceInfo or command success response).
- Call returns an error if the Client is not authenticated, Host is unreachable, or the device rejects the command.
- Non-P100 model does not block execution; when the device accepts the command, the call completes and the caller can detect `ErrUnsupportedModel` via `errors.Is` (FR-8).

#### FR-5: Turn Plug off

A developer can turn the Plug off via the Client. Realizes UJ-1.

**Consequences (testable):**
- Successful call results in the Plug's relay state being off.
- Symmetric error behavior with FR-4, including unsupported-model warning per FR-8.

#### FR-6: Toggle Plug state

A developer can toggle the Plug to the opposite of its current on/off state in one call. Included in v1.0.0. Realizes UJ-1.

**Consequences (testable):**
- Toggle reads current state then sets inverted state, or uses an atomic device command if available.
- If state cannot be read, returns an error without guessing.
- Non-P100 model warning behavior matches FR-4 (warn, do not block).

**Out of Scope:**
- Scheduling, away mode, or cloud scenes (Non-Goal).

---

### 4.3 Plug Device Information

**Description:** A developer retrieves structured DeviceInfo from the Plug reflecting current state and hardware metadata. Realizes UJ-1, UJ-2.

**Functional Requirements:**

#### FR-7: Retrieve DeviceInfo

A developer can fetch DeviceInfo for the configured Plug. Realizes UJ-1.

**Consequences (testable):**
- Response includes at minimum: `DeviceOn` (bool), `Model`, `Nickname`, `DeviceID`, `FirmwareVersion`, `HardwareVersion`, `IPAddress`, `MAC`.
- Base64-encoded string fields from the device (e.g. Nickname, SSID) are decoded to plain UTF-8 strings before return.
- Call succeeds against a genuine P100 on firmware v1.4.4 Build 20240514 Rel 35017 using KLAP, and against legacy-firmware P100 units via fallback.

#### FR-8: Warn on unsupported device model

DeviceInfo includes the device-reported `Model` string so callers can branch on hardware variant without the library silently misidentifying devices. When the model is not P100, the library warns but does not hard-fail solely for model mismatch. Realizes UJ-1.

**Consequences (testable):**
- `DeviceInfo` always populates the response when the device returns data; the returned error allows `errors.Is(err, ErrUnsupportedModel)` when `Model != "P100"`.
- `TurnOn`, `TurnOff`, and `Toggle` attempt the command regardless of model; unsupported model alone MUST NOT cause a failed return if the device accepts the command.
- When a control command succeeds on a non-P100 device, the caller can detect `ErrUnsupportedModel` via `errors.Is` on the returned error (warning semantics — operation outcome and warning are both observable).
- Hard failures remain limited to authentication, network, protocol, and device-rejected commands.

**Out of Scope:**
- Energy usage, current power draw, or runtime statistics (P110+ only; Non-Goal for v1).

---

### 4.4 Concurrency

**Description:** The Client is safe for use from multiple goroutines in long-running services and concurrent automation jobs. Realizes UJ-1.

**Functional Requirements:**

#### FR-10: Goroutine-safe Client

Multiple goroutines may invoke methods on the same Plug Client concurrently without data races or corrupted Session state. Realizes UJ-1.

**Consequences (testable):**
- Concurrent `TurnOn`, `TurnOff`, `Toggle`, and `DeviceInfo` calls from separate goroutines do not trigger the Go race detector when tested under `-race`.
- Session establishment and re-authentication are serialized internally (e.g. mutex or singleflight) so only one login handshake runs at a time.
- Concurrent commands either queue safely or fail with a documented error; they must not produce undefined relay state.

---

## 5. Non-Goals (Explicit)

- **Cloud API integration** — No TP-Link cloud REST, OAuth, or remote access outside the LAN.
- **Non-P100 device types** — No P110, P115, P105, power strips, bulbs, hubs, sensors, or cameras in v1.
- **Device discovery** — No UDP/mDNS scan to find Plugs; caller supplies Host.
- **Energy monitoring** — No `get_energy_usage`, `get_current_power`, or related APIs.
- **Scheduling / scenes / away mode** — No implementation of Tapo app automation features.
- **Official vendor endorsement** — Not an official TP-Link product; reverse-engineered local protocol.
- **GUI, mobile app, or Home Assistant integration** — Library only; integrations are downstream.
- **Fork or compatibility layer for rk295/tapo-go** — Greenfield API; no guarantee of drop-in replacement.

---

## 6. MVP Scope

### 6.1 In Scope

- Go module `github.com/mjenh/tapo` (Go 1.24+)
- P100 Plug: authenticate, turn on, turn off, toggle (FR-6), get DeviceInfo
- KLAP transport with automatic legacy fallback (FR-9)
- Goroutine-safe Client (FR-10)
- `context.Context` on public methods
- Environment-based configuration helper (FR-3)
- MIT license
- README with quickstart, env vars, error-handling notes, and verified support matrix
- Unit tests with mocked transport; integration test instructions for real hardware `[ASSUMPTION]`

### 6.2 Out of Scope for MVP

| Item | Reason |
|------|--------|
| P110/P115 energy APIs | Different feature set; v2+ |
| Multi-device orchestration | Single Plug per Client in v1 |
| Connection pooling / shared session across hosts | YAGNI for v1 |
| Device discovery | Deferred; caller provides Host |
| Prometheus exporter | Downstream project |
| CLI binary in-module | Consumers build their own; example cmd optional post-v1 |

### 6.3 Verified Support Matrix (v1.0.0)

| Device | Firmware | Transport | Status |
|--------|----------|-----------|--------|
| Tapo P100 | v1.4.4 Build 20240514 Rel 35017 | KLAP (primary) | **Certified for v1.0.0** |

Legacy-protocol P100 units are supported via FR-9 fallback but are not individually certified unless added to this matrix in a patch release.

---

## 7. Success Metrics

**Primary**

- **SM-1:** A new contributor completes the README quickstart against a real P100 within 15 minutes. Validates FR-1, FR-2, FR-4, FR-7.
- **SM-2:** v1.0.0 tagged with semver; zero breaking API changes after tag. Validates API contract stability intent.

**Secondary**

- **SM-3:** ≥80% unit test coverage on protocol and crypto helpers (excluding integration-only paths). Validates maintainability for OSS contributors.
- **SM-4:** README support matrix lists P100 firmware v1.4.4 Build 20240514 Rel 35017 as verified. Validates FR-7, §6.3.

**Counter-metrics (do not optimize)**

- **SM-C1:** Raw GitHub star count — not a proxy for API quality or correctness; do not defer bug fixes for marketing.

---

## 8. Cross-Cutting Non-Functional Requirements

### 8.1 Reliability and Errors

- All exported errors are inspectable with `errors.Is` / `errors.As`; sentinel errors for: authentication failure, network timeout, unsupported model, protocol handshake failure.
- Default per-request timeout: 10 seconds; overridable via Client options.
- Automatic re-login on session expiry at most once per command.

### 8.2 Security and Privacy

- Credentials supplied by caller; never persisted to disk by the library.
- No telemetry or phone-home behavior.
- Local network communication only; document that users must trust their LAN.
- Do not log secrets at any log level.

### 8.3 Performance

- Typical on/off or info call completes within 2 seconds on a healthy LAN (target, not SLA).
- Goroutine-safe Client per FR-10.

### 8.4 Observability

- No structured logging requirement in v1; optional debug mode via Client option `[ASSUMPTION]` that redacts credentials.

### 8.5 Compatibility

- Requires Go 1.24+ as stated in `go.mod`.
- Test against Linux and macOS; Windows best-effort `[ASSUMPTION]`.

---

## 9. Public API Surface and Versioning

### 9.1 v1 Public API

Root package: `github.com/mjenh/tapo`.

| Symbol | Role |
|--------|------|
| `NewPlug(ctx, host, email, password, opts ...Option) (*Plug, error)` | Construct authenticated-capable client |
| `NewPlugFromEnv(ctx, opts ...Option) (*Plug, error)` | Env-based construction |
| `(*Plug) TurnOn(ctx) error` | FR-4 |
| `(*Plug) TurnOff(ctx) error` | FR-5 |
| `(*Plug) Toggle(ctx) error` | FR-6 |
| `(*Plug) DeviceInfo(ctx) (*DeviceInfo, error)` | FR-7 |
| `DeviceInfo` struct | Stable JSON-tagged fields per FR-7 minimum set |
| `Option` functional options | Timeout, retry, transport override, debug |
| `ErrUnsupportedModel` | FR-8 warning sentinel (non-P100 model) |
| Documented sentinel errors | FR-2, FR-8, NFR 8.1 |

Breaking change policy: after `v1.0.0`, follow semver; breaking API changes only in major versions. Pre-1.0 tags may break without migration guide.

### 9.2 Dependency Policy

- Minimize external dependencies; prefer stdlib for HTTP, crypto, JSON.
- Any non-stdlib dependency requires justification in README.

---

## 10. Resolved Decisions

| # | Question | Decision |
|---|----------|----------|
| 1 | Transport selection | KLAP first, automatic legacy fallback (FR-9) |
| 2 | Module path | `github.com/mjenh/tapo` |
| 3 | Toggle API | Included in v1.0.0 (FR-6) |
| 4 | Concurrent Client use | Goroutine-safe guaranteed (FR-10) |
| 5 | Minimum Go version | Go 1.24 |
| 6 | Verified hardware | P100 firmware v1.4.4 Build 20240514 Rel 35017 (§6.3) |
| 7 | Unsupported model | Warn on DeviceInfo and control; do not block control (FR-8) |

---

## 11. Assumptions Index

Remaining inferred details pending implementation or docs:

- **Transport override option** — Optional Client option to force legacy/KLAP for debugging.
- **Integration tests** — Documented manual procedure; not required in CI without hardware.
- **Windows** — Best-effort support; Linux and macOS are primary test targets.
- **Debug logging** — Optional Client option; credentials always redacted.

---

## 12. Deferred Items

| Item | Owner | Revisit when |
|------|-------|--------------|
| Constant-time credential comparison | Maintainer | Side-channel risk raised in security review |
| Additional P100 firmware rows in support matrix | Maintainer | Community reports or hardware on hand |
