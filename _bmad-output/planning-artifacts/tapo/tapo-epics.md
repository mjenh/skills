---
stepsCompleted: [step-01-validate-prerequisites, step-02-design-epics, step-03-create-stories, step-04-final-validation]
inputDocuments:
  - tapo-prd.md
  - tapo-spec.md
  - tapo-architecture.md
  - addendum.md
  - glossary.md
---

# tapo - Epic Breakdown

## Overview

This document provides the complete epic and story breakdown for tapo, decomposing the requirements from the PRD, Spec, and Architecture into implementable stories.

## Requirements Inventory

### Functional Requirements

- FR-1: Construct Plug client given host, email, password, and optional config (CAP-1)
- FR-2: Authenticate with Plug; distinct errors for creds/network/protocol; auto re-auth on expiry (CAP-2)
- FR-3: Construct client from env vars TAPO_HOST/TAPO_IP, TAPO_EMAIL, TAPO_PASSWORD (CAP-3)
- FR-4: Turn Plug on (CAP-4)
- FR-5: Turn Plug off (CAP-5)
- FR-6: Toggle Plug to opposite state in one call (CAP-6)
- FR-7: Retrieve DeviceInfo with minimum field set; decode base64 fields (CAP-7)
- FR-8: Warn on non-P100 model via ErrUnsupportedModel; don't block control (CAP-8)
- FR-9: Auto-negotiate transport — KLAP first, legacy fallback, cache winner (CAP-9)
- FR-10: Goroutine-safe client — pass race detector, serialize session establishment (CAP-10)

### NonFunctional Requirements

- NFR-1: All errors inspectable via errors.Is/errors.As; four sentinels (ErrAuth, ErrTimeout, ErrUnsupportedModel, ErrHandshake)
- NFR-2: Default per-request timeout 10s, overridable via options
- NFR-3: Auto re-login on session expiry at most once per command
- NFR-4: Credentials never persisted to disk or logged at any level
- NFR-5: No telemetry or phone-home
- NFR-6: Local LAN communication only
- NFR-7: Typical on/off or info call within 2s on healthy LAN
- NFR-8: Go 1.24+ minimum
- NFR-9: Test against Linux and macOS; Windows best-effort
- NFR-10: context.Context on all exported methods

### Additional Requirements

- AR-1: Layered package structure with internal/ boundary (AD-1)
- AR-2: internal/transport + internal/crypto package split (AD-2)
- AR-3: Thin Transport interface — Login + Send only (AD-3)
- AR-4: Transport-owned session state (AD-4)
- AR-5: sync.Mutex concurrency at both layers (AD-5)
- AR-6: Sentinel error vars with fmt.Errorf %w wrapping (AD-6)
- AR-7: NegotiatingTransport composite for lazy protocol selection (AD-7)
- AR-8: DeviceInfo decoding in root package (AD-8)
- AR-9: Zero external dependencies — stdlib only

### UX Design Requirements

N/A — Library project, no UI.

### FR Coverage Map

| FR | Epic | Description |
|---|---|---|
| FR-1 | Epic 1 | Construct Plug client |
| FR-2 | Epic 1 | Authenticate with Plug |
| FR-3 | Epic 1 | Env-based construction |
| FR-4 | Epic 1 | Turn on |
| FR-5 | Epic 1 | Turn off |
| FR-6 | Epic 1 | Toggle |
| FR-7 | Epic 1 | DeviceInfo |
| FR-8 | Epic 1 | Unsupported model warning |
| FR-9 | Epic 2 | Transport negotiation |
| FR-10 | Epic 1 | Goroutine safety |

## Epic List

### Epic 1: Core P100 Control Library
A developer can `go get` this module, connect to a P100 on KLAP firmware, turn it on/off/toggle, read DeviceInfo, and use it safely from concurrent goroutines — with env-based config and predictable error handling.
**FRs covered:** FR-1, FR-2, FR-3, FR-4, FR-5, FR-6, FR-7, FR-8, FR-10

### Epic 2: Legacy Transport & Protocol Negotiation
The library auto-detects the right protocol and works with any P100 regardless of firmware version, including older units that only support the legacy protocol.
**FRs covered:** FR-9

### Epic 3: Documentation & Release
A new contributor can follow the README quickstart and control a real P100 within 15 minutes. The module is tagged v1.0.0 with a verified support matrix.
**FRs covered:** Cross-cutting (SM-1, SM-2, SM-4)

---

## Epic 1: Core P100 Control Library

A developer can `go get` this module, connect to a P100 on KLAP firmware, turn it on/off/toggle, read DeviceInfo, and use it safely from concurrent goroutines — with env-based config and predictable error handling.

### Story 1.1: Module Scaffold & Shared Crypto Helpers

As a library developer,
I want the Go module initialized with the architectural directory structure and shared crypto primitives,
So that transport implementations can be built on a tested foundation.

**Acceptance Criteria:**

**Given** an empty repository
**When** the module scaffold is created
**Then** `go.mod` declares module `github.com/mjenh/tapo` with Go 1.24+ minimum
**And** the directory structure matches AD-1/AD-2: root package files, `internal/transport/`, `internal/crypto/`
**And** `internal/crypto/crypto.go` provides AES-CBC encrypt and decrypt functions using `crypto/aes` and `crypto/cipher` stdlib
**And** `internal/crypto/auth.go` provides the KLAP auth hash function: `SHA256(SHA1(email) + SHA1(password))` using stdlib `crypto/sha256` and `crypto/sha1`
**And** `internal/crypto/auth.go` provides KLAP key and IV derivation from handshake seeds
**And** all crypto functions have unit tests with known test vectors
**And** zero external dependencies exist in `go.mod`

### Story 1.2: KLAP Transport

As a library developer,
I want a KLAP transport implementation behind the Transport interface,
So that the library can communicate with current-firmware P100 plugs.

**Acceptance Criteria:**

**Given** the crypto helpers from Story 1.1
**When** the KLAP transport is implemented
**Then** `internal/transport/transport.go` defines the `Transport` interface with `Login(ctx context.Context, email, password string) error` and `Send(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error)` per AD-3
**And** `internal/transport/klap/klap.go` implements `Transport` with KLAP two-stage seed handshake establishing `TP_SESSIONID` cookie
**And** `Login` derives the auth hash from credentials, performs the handshake with the device HTTP endpoint, and stores session state (cookie, AES keys, sequence counter) internally per AD-4
**And** `Send` encrypts the payload with the derived AES key/IV, includes the sequence counter in the request URL, and decrypts the response
**And** `Login` returns a wrapped `ErrAuth` on invalid credentials and a wrapped `ErrHandshake` on handshake failure
**And** all operations honor the provided `context.Context` for cancellation and deadlines
**And** the transport holds its own `sync.Mutex` guarding session state per AD-5
**And** unit tests verify handshake, encrypt/decrypt round-trip, and error paths using a mock HTTP server
**And** credentials are never logged

### Story 1.3: Plug Client Construction & DeviceInfo

As a Go developer,
I want to construct a Plug client and retrieve device information,
So that I can verify connectivity and read my plug's state programmatically.

**Acceptance Criteria:**

**Given** the KLAP transport from Story 1.2
**When** a developer calls `NewPlug(ctx, host, email, password)`
**Then** the function returns a `*Plug` and `nil` error when host and credentials are non-empty
**And** the function returns a descriptive error when host is empty or credentials are missing
**And** `NewPlug` performs no network I/O per AD-7 (lazy negotiation)
**And** `NewPlugFromEnv(ctx)` reads `TAPO_HOST` (preferred) / `TAPO_IP` (alias), `TAPO_EMAIL`, `TAPO_PASSWORD` from environment
**And** `NewPlugFromEnv` returns an error listing which required variables are absent when any are missing
**And** both constructors accept variadic `Option` arguments (`WithTimeout` sets per-request timeout, default 10s)
**And** calling `plug.DeviceInfo(ctx)` triggers KLAP login on first call, sends `get_device_info`, and returns a `*DeviceInfo` struct
**And** `DeviceInfo` contains at minimum: `DeviceOn` (bool), `Model`, `Nickname`, `DeviceID`, `FirmwareVersion`, `HardwareVersion`, `IPAddress`, `MAC`
**And** base64-encoded fields (`Nickname`, `SSID`) are decoded to plain UTF-8 before return per AD-8
**And** when `Model != "P100"`, the returned error wraps `ErrUnsupportedModel` (detectable via `errors.Is`) but `DeviceInfo` is still populated per FR-8
**And** `errors.go` exports four sentinels: `ErrAuth`, `ErrTimeout`, `ErrUnsupportedModel`, `ErrHandshake`

### Story 1.4: Plug Power Control

As a Go developer,
I want to turn my plug on, off, or toggle its state,
So that I can control devices programmatically in scripts and automation.

**Acceptance Criteria:**

**Given** a constructed Plug client from Story 1.3
**When** a developer calls `plug.TurnOn(ctx)`
**Then** the client sends `set_device_info` with `{"device_on": true}` via the transport
**And** the call returns `nil` error on success
**And** the relay state is verifiable as on via subsequent `DeviceInfo` call

**Given** a constructed Plug client
**When** a developer calls `plug.TurnOff(ctx)`
**Then** the client sends `set_device_info` with `{"device_on": false}` via the transport
**And** error behavior is symmetric with `TurnOn`

**Given** a constructed Plug client with the plug currently on
**When** a developer calls `plug.Toggle(ctx)`
**Then** the client reads current state via `get_device_info`, then sets the inverse via `set_device_info`
**And** the plug state changes from on to off (or off to on)

**Given** a constructed Plug client where the device is unreachable
**When** `Toggle` cannot read the current state
**Then** it returns an error without guessing the state

**Given** a non-P100 device that accepts the command
**When** `TurnOn`, `TurnOff`, or `Toggle` is called
**Then** the command succeeds and the returned error wraps `ErrUnsupportedModel` (detectable via `errors.Is`) per FR-8

### Story 1.5: Goroutine Safety & Session Resilience

As a Go developer running concurrent automation,
I want thread-safe client usage and automatic session recovery,
So that I can use the library in long-running services without data races or manual re-auth.

**Acceptance Criteria:**

**Given** a single Plug client shared across multiple goroutines
**When** concurrent `TurnOn`, `TurnOff`, `Toggle`, and `DeviceInfo` calls are made from separate goroutines
**Then** no data races are triggered when tested under `go test -race`
**And** the Plug-level `sync.Mutex` serializes re-authentication so only one goroutine re-logins at a time per AD-5

**Given** an authenticated Plug client with an expired session
**When** a command returns error code `9999` (or transport-equivalent session expiry)
**Then** the client automatically re-authenticates once before surfacing the failure to the caller per NFR-3
**And** if re-authentication also fails, the original command's error is returned with context

**Given** three goroutines simultaneously hitting session expiry
**When** all three detect error `9999` concurrently
**Then** exactly one goroutine performs re-auth while the other two block on the mutex
**And** after re-auth completes, all three retry with the fresh session

---

## Epic 2: Legacy Transport & Protocol Negotiation

The library auto-detects the right protocol and works with any P100 regardless of firmware version, including older units that only support the legacy protocol.

### Story 2.1: Legacy Transport Implementation

As a library developer,
I want a legacy transport implementation,
So that P100 plugs on older firmware are supported.

**Acceptance Criteria:**

**Given** the Transport interface and crypto helpers from Epic 1
**When** the legacy transport is implemented
**Then** `internal/transport/legacy/legacy.go` implements the `Transport` interface
**And** `Login` performs RSA handshake with the device (device returns AES key material), then calls `login_device` with `base64(SHA1(email))` username encoding
**And** `Login` stores the session token internally per AD-4
**And** `Send` wraps commands in `securePassthrough` with AES-CBC encryption and appends the session token to requests
**And** `Login` returns wrapped `ErrAuth` on invalid credentials and wrapped `ErrHandshake` on RSA handshake failure
**And** `internal/crypto/auth.go` is extended with SHA1+base64 login encoding for legacy protocol (if not already present)
**And** the transport holds its own `sync.Mutex` guarding session state per AD-5
**And** all operations honor the provided `context.Context`
**And** unit tests verify handshake, encrypt/decrypt, and error paths using a mock HTTP server
**And** credentials are never logged

### Story 2.2: NegotiatingTransport & Transport Override

As a Go developer,
I want automatic protocol detection and the option to force a specific protocol,
So that I don't need to know which firmware my plug runs, but can override for debugging.

**Acceptance Criteria:**

**Given** both KLAP and legacy transports from Stories 1.2 and 2.1
**When** the NegotiatingTransport is implemented
**Then** `internal/transport/negotiate.go` defines a `NegotiatingTransport` that implements the `Transport` interface per AD-7
**And** on the first `Login` call, it attempts KLAP; on a distinguishable protocol/handshake failure, it retries with legacy
**And** the winning concrete transport is cached and used for all subsequent `Send` calls
**And** `Plug` holds one `Transport` reference and calls `Login` once — it does not know about KLAP or legacy directly

**Given** a developer constructing a Plug with `WithTransport("klap")` or `WithTransport("legacy")`
**When** the option is applied
**Then** `NegotiatingTransport` is bypassed and the specified concrete transport is injected directly
**And** negotiation does not occur

**Given** a P100 on KLAP firmware (v1.4.4 Build 20240514 Rel 35017)
**When** `NegotiatingTransport.Login` is called
**Then** it connects via KLAP on the first attempt without falling back

**Given** a P100 on legacy-only firmware
**When** `NegotiatingTransport.Login` is called
**Then** KLAP fails with a distinguishable error, legacy succeeds, and the legacy transport is cached

---

## Epic 3: Documentation & Release

A new contributor can follow the README quickstart and control a real P100 within 15 minutes. The module is tagged v1.0.0 with a verified support matrix.

### Story 3.1: README, Quickstart & Support Matrix

As a Go developer discovering this library,
I want clear documentation with a working quickstart example,
So that I can adopt the library quickly and confidently.

**Acceptance Criteria:**

**Given** the completed library from Epics 1 and 2
**When** the README is written
**Then** it includes a quickstart code example showing: construct client, turn on, get DeviceInfo, check `DeviceOn`
**And** it documents environment variables: `TAPO_HOST` / `TAPO_IP`, `TAPO_EMAIL`, `TAPO_PASSWORD`
**And** it documents error handling: how to use `errors.Is` with the four sentinels
**And** it documents functional options: `WithTimeout`, `WithTransport`
**And** it includes a verified support matrix with P100 firmware v1.4.4 Build 20240514 Rel 35017 as certified
**And** it states the MIT license
**And** it documents that non-P100 models produce `ErrUnsupportedModel` warnings but are not blocked
**And** it notes integration test instructions for real hardware

### Story 3.2: Test Coverage & v1.0.0 Release

As a library maintainer,
I want comprehensive unit tests and a tagged release,
So that contributors can trust the library and consumers can pin a stable version.

**Acceptance Criteria:**

**Given** the completed library and documentation
**When** the test suite is finalized
**Then** unit test coverage is ≥80% on protocol and crypto helpers (excluding integration-only paths) per SM-3
**And** tests use mocked HTTP servers for transport testing (no real hardware required)
**And** tests include race-condition validation via `go test -race` for concurrent Plug usage
**And** integration test instructions are documented for manual execution against real hardware
**And** `go vet` and `go test ./...` pass cleanly on Linux and macOS

**Given** all tests pass and documentation is complete
**When** the release is prepared
**Then** the module is tagged `v1.0.0` following semver
**And** `go.sum` is committed and the module is publishable via `go get github.com/mjenh/tapo@v1.0.0`
