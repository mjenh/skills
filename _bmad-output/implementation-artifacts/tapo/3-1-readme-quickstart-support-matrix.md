---
baseline_commit: 1c1ab983064a54c4f0cc34ef17f113417e26c292
---

# Story 3.1: README, Quickstart & Support Matrix

Status: done

## Story

As a Go developer discovering this library,
I want clear documentation with a working quickstart example,
So that I can adopt the library quickly and confidently.

## Acceptance Criteria

1. **Given** the completed library from Epics 1 and 2, **When** the README is written, **Then** it includes a quickstart code example showing: construct client, turn on, get DeviceInfo, check `DeviceOn`.

2. **Given** the README quickstart section, **When** a developer reads it, **Then** it documents environment variables: `TAPO_HOST` / `TAPO_IP`, `TAPO_EMAIL`, `TAPO_PASSWORD`.

3. **Given** the README error handling section, **When** a developer reads it, **Then** it documents how to use `errors.Is` with the four sentinel errors: `ErrAuth`, `ErrTimeout`, `ErrUnsupportedModel`, `ErrHandshake`.

4. **Given** the README options section, **When** a developer reads it, **Then** it documents functional options: `WithTimeout`, `WithTransport`.

5. **Given** the README support matrix section, **When** a developer reads it, **Then** it includes a verified support matrix with P100 firmware v1.4.4 Build 20240514 Rel 35017 as certified for v1.0.0.

6. **Given** the README, **When** a developer reads the footer/license section, **Then** it states the MIT license.

7. **Given** the README unsupported models section, **When** a developer reads it, **Then** it documents that non-P100 models produce `ErrUnsupportedModel` warnings but are not blocked from control operations.

8. **Given** the README testing section, **When** a developer reads it, **Then** it notes integration test instructions for running tests against real hardware.

## Tasks / Subtasks

### Task 1: Create README.md [AC-1, AC-2, AC-3, AC-4, AC-5, AC-6, AC-7, AC-8]

File: `README.md` (repository root)

- [x] Add project title and one-line description: Go client library for controlling Tapo P100 smart plugs over the local network
- [x] Add badges placeholder (Go Reference, license, build status if CI exists)
- [x] Add "Features" section listing key capabilities:
  - Turn on, turn off, toggle
  - DeviceInfo retrieval with base64 field decoding
  - KLAP transport with automatic legacy fallback
  - Environment-based or explicit configuration
  - Goroutine-safe client
  - Zero external dependencies (stdlib only)

#### Subtask 1.1: Installation Section

- [x] Document `go get github.com/mjenh/tapo`
- [x] State Go 1.24+ requirement

#### Subtask 1.2: Quickstart Section [AC-1]

- [x] Write a complete, runnable Go code example demonstrating:
  ```go
  package main

  import (
      "context"
      "fmt"
      "log"

      "github.com/mjenh/tapo"
  )

  func main() {
      ctx := context.Background()

      // Construct client with explicit credentials
      plug, err := tapo.NewPlug(ctx, "192.168.1.42", "you@example.com", "your-password")
      if err != nil {
          log.Fatal(err)
      }

      // Turn the plug on
      if err := plug.TurnOn(ctx); err != nil {
          log.Fatal(err)
      }

      // Read device info
      info, err := plug.DeviceInfo(ctx)
      if err != nil {
          log.Fatal(err)
      }

      fmt.Printf("Device: %s (on=%t)\n", info.Nickname, info.DeviceOn)
  }
  ```
- [x] Add a second example showing `NewPlugFromEnv` usage:
  ```go
  plug, err := tapo.NewPlugFromEnv(ctx)
  ```

#### Subtask 1.3: Environment Variables Section [AC-2]

- [x] Document each environment variable in a table:
  | Variable | Required | Description |
  |---|---|---|
  | `TAPO_HOST` | Yes (or `TAPO_IP`) | IP address of the Tapo plug (e.g. `192.168.1.42`) |
  | `TAPO_IP` | Alias | Alias for `TAPO_HOST`; `TAPO_HOST` takes precedence if both are set |
  | `TAPO_EMAIL` | Yes | Tapo account email address |
  | `TAPO_PASSWORD` | Yes | Tapo account password |
- [x] Note that `NewPlugFromEnv` returns an error listing which required variables are missing

#### Subtask 1.4: API Reference Section

- [x] Document the full public API surface in a table:
  | Function / Method | Description |
  |---|---|
  | `NewPlug(ctx, host, email, password, opts ...Option) (*Plug, error)` | Construct a client with explicit credentials |
  | `NewPlugFromEnv(ctx, opts ...Option) (*Plug, error)` | Construct a client from environment variables |
  | `(*Plug) TurnOn(ctx) error` | Turn the plug on |
  | `(*Plug) TurnOff(ctx) error` | Turn the plug off |
  | `(*Plug) Toggle(ctx) error` | Toggle the plug to the opposite state |
  | `(*Plug) DeviceInfo(ctx) (*DeviceInfo, error)` | Retrieve device information |
- [x] Document `DeviceInfo` struct fields:
  | Field | Type | Description |
  |---|---|---|
  | `DeviceOn` | `bool` | Current relay state |
  | `Model` | `string` | Device model (e.g. `"P100"`) |
  | `Nickname` | `string` | User-assigned nickname (decoded from base64) |
  | `DeviceID` | `string` | Unique device identifier |
  | `FirmwareVersion` | `string` | Firmware version string |
  | `HardwareVersion` | `string` | Hardware version string |
  | `IPAddress` | `string` | Device IP address |
  | `MAC` | `string` | Device MAC address |

#### Subtask 1.5: Options Section [AC-4]

- [x] Document `WithTimeout` with example:
  ```go
  plug, err := tapo.NewPlug(ctx, host, email, password,
      tapo.WithTimeout(5 * time.Second),
  )
  ```
  - Note: default timeout is 10 seconds per request
- [x] Document `WithTransport` with example:
  ```go
  plug, err := tapo.NewPlug(ctx, host, email, password,
      tapo.WithTransport("klap"),    // Force KLAP protocol
  )
  ```
  - Note: accepted values are `"klap"` and `"legacy"`
  - Note: by default, the library auto-negotiates (KLAP first, legacy fallback)

#### Subtask 1.6: Error Handling Section [AC-3]

- [x] Document the four sentinel errors in a table:
  | Sentinel | Meaning |
  |---|---|
  | `tapo.ErrAuth` | Authentication failed (invalid email or password) |
  | `tapo.ErrTimeout` | Request timed out (device unreachable or slow) |
  | `tapo.ErrUnsupportedModel` | Device model is not P100 (warning; operation may still succeed) |
  | `tapo.ErrHandshake` | Transport handshake failed (protocol mismatch or device error) |
- [x] Provide code example using `errors.Is`:
  ```go
  info, err := plug.DeviceInfo(ctx)
  if err != nil {
      switch {
      case errors.Is(err, tapo.ErrAuth):
          log.Fatal("Invalid credentials")
      case errors.Is(err, tapo.ErrTimeout):
          log.Fatal("Device unreachable")
      case errors.Is(err, tapo.ErrHandshake):
          log.Fatal("Protocol handshake failed")
      case errors.Is(err, tapo.ErrUnsupportedModel):
          // Warning only -- info is still populated
          log.Printf("Warning: %v", err)
      default:
          log.Fatal(err)
      }
  }
  ```
- [x] Explain that `ErrUnsupportedModel` is a warning: when a non-P100 device is detected, the error wraps `ErrUnsupportedModel` but the operation result (`DeviceInfo` or control command) is still valid if the device accepted the command

#### Subtask 1.7: Non-P100 Models Section [AC-7]

- [x] Document that non-P100 devices (e.g. P110, P115, P105) are not blocked:
  - The library attempts all commands regardless of device model
  - When `DeviceInfo` reports a model other than `"P100"`, the returned error wraps `ErrUnsupportedModel`
  - Control commands (`TurnOn`, `TurnOff`, `Toggle`) also wrap `ErrUnsupportedModel` when the device reports a non-P100 model, but the command is still executed
  - Callers detect this via `errors.Is(err, tapo.ErrUnsupportedModel)`
  - v1 is tested and certified only for P100; other models may work but are not guaranteed

#### Subtask 1.8: Support Matrix Section [AC-5]

- [x] Add the verified support matrix table:
  | Device | Firmware | Transport | Status |
  |---|---|---|---|
  | Tapo P100 | v1.4.4 Build 20240514 Rel 35017 | KLAP (primary) | Certified for v1.0.0 |
- [x] Note: legacy-protocol P100 units are supported via automatic fallback but are not individually certified unless added to this matrix in a patch release
- [x] Note: community reports of additional firmware versions are welcome via GitHub issues

#### Subtask 1.9: Testing Section [AC-8]

- [x] Document unit test execution:
  ```sh
  go test ./...
  ```
- [x] Document race detector testing:
  ```sh
  go test -race ./...
  ```
- [x] Document integration test instructions for real hardware:
  - Set environment variables for a real P100 on the LAN:
    ```sh
    export TAPO_HOST=192.168.1.42
    export TAPO_EMAIL=you@example.com
    export TAPO_PASSWORD=your-password
    ```
  - Run integration tests (if tagged):
    ```sh
    go test -tags=integration ./...
    ```
  - Note: integration tests require a real P100 on the local network and are not run in CI
  - Note: the plug will be turned on and off during testing

#### Subtask 1.10: Concurrency Section

- [x] Document that the `Plug` client is goroutine-safe:
  - A single `*Plug` instance can be shared across multiple goroutines
  - Session management and re-authentication are serialized internally
  - No external synchronization is required by the caller
- [x] Provide brief example:
  ```go
  plug, _ := tapo.NewPlug(ctx, host, email, password)

  var wg sync.WaitGroup
  for i := 0; i < 10; i++ {
      wg.Add(1)
      go func() {
          defer wg.Done()
          info, err := plug.DeviceInfo(ctx)
          // safe to call concurrently
      }()
  }
  wg.Wait()
  ```

#### Subtask 1.11: License Section [AC-6]

- [x] State: "MIT License. See [LICENSE](LICENSE) for details."

#### Subtask 1.12: Contributing / Links Section

- [x] Note that this is not an official TP-Link product; it uses a reverse-engineered local protocol
- [x] Note local LAN communication only; no cloud dependency
- [x] Note zero external dependencies

### Task 2: Create LICENSE File [AC-6]

File: `LICENSE` (repository root)

- [x] Create MIT license file with:
  - Year: 2026
  - Copyright holder: as per repository owner
  - Full MIT license text

### Task 3: Create or Update doc.go [AC-1]

File: `doc.go` (repository root)

- [x] Ensure `doc.go` contains a comprehensive package-level doc comment:
  ```go
  // Package tapo provides a Go client for controlling Tapo P100 smart plugs
  // over the local network.
  //
  // The client supports KLAP and legacy transport protocols with automatic
  // negotiation. It is safe for concurrent use from multiple goroutines.
  //
  // Quick start:
  //
  //     plug, err := tapo.NewPlug(ctx, "192.168.1.42", "email", "password")
  //     if err != nil {
  //         log.Fatal(err)
  //     }
  //     if err := plug.TurnOn(ctx); err != nil {
  //         log.Fatal(err)
  //     }
  //
  // Environment-based construction:
  //
  //     plug, err := tapo.NewPlugFromEnv(ctx)
  //
  // See the README for full documentation and the support matrix.
  package tapo
  ```
- [x] Verify `go doc` output renders cleanly

### Task 4: Create Example Test File

File: `example_test.go` (repository root)

- [x] Create an `Example` function that mirrors the README quickstart, so `go test` validates the example compiles:
  ```go
  package tapo_test

  import (
      "context"
      "fmt"
      "log"

      "github.com/mjenh/tapo"
  )

  func Example() {
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
- [x] Note: this example will not run against a real device in CI but validates that the API compiles and the documented signatures are correct

## Dev Notes

### README Structure

The README must cover the following sections in order. Each section maps to one or more acceptance criteria:

1. **Title + description** -- project name, one-line purpose, badges
2. **Features** -- bullet list of capabilities
3. **Installation** -- `go get` command, Go version requirement
4. **Quickstart** -- runnable code example (AC-1)
5. **Environment Variables** -- table of env vars for `NewPlugFromEnv` (AC-2)
6. **API Reference** -- full public API surface table + DeviceInfo fields
7. **Options** -- `WithTimeout`, `WithTransport` with examples (AC-4)
8. **Error Handling** -- sentinel errors table + `errors.Is` example (AC-3)
9. **Non-P100 Models** -- `ErrUnsupportedModel` warning semantics (AC-7)
10. **Concurrency** -- goroutine safety documentation
11. **Support Matrix** -- verified firmware table (AC-5)
12. **Testing** -- unit tests, race detector, integration test instructions (AC-8)
13. **License** -- MIT (AC-6)
14. **Disclaimer** -- not official TP-Link, reverse-engineered protocol, LAN-only

### SM-1 Target

The primary success metric for this story is SM-1: "A new contributor completes the README quickstart against a real P100 within 15 minutes." The quickstart must be:
- Self-contained (no external links required to get started)
- Copy-pasteable (the code example should work as-is with only host/credentials changed)
- Progressive (start with the simplest example, then show env-based and options variants)

### Public API Surface to Document

From PRD section 9.1, the complete v1 public API:

| Symbol | Source |
|---|---|
| `NewPlug(ctx, host, email, password, opts ...Option) (*Plug, error)` | FR-1 |
| `NewPlugFromEnv(ctx, opts ...Option) (*Plug, error)` | FR-3 |
| `(*Plug) TurnOn(ctx) error` | FR-4 |
| `(*Plug) TurnOff(ctx) error` | FR-5 |
| `(*Plug) Toggle(ctx) error` | FR-6 |
| `(*Plug) DeviceInfo(ctx) (*DeviceInfo, error)` | FR-7 |
| `DeviceInfo` struct | FR-7 |
| `Option` type + `WithTimeout` + `WithTransport` | FR-1, FR-9 |
| `ErrAuth`, `ErrTimeout`, `ErrUnsupportedModel`, `ErrHandshake` | NFR-1, AD-6 |

### Error Handling Semantics

The README must clearly explain the `ErrUnsupportedModel` dual-return pattern:
- When a non-P100 device is detected, the error wraps `ErrUnsupportedModel`
- BUT the operation result is still returned (`DeviceInfo` is populated, control commands are executed)
- This is a **warning**, not a hard failure
- Callers must check `errors.Is(err, tapo.ErrUnsupportedModel)` to distinguish warnings from real errors

### Transport Protocol Documentation

The README should document:
- Default behavior: auto-negotiation (KLAP first, legacy fallback)
- KLAP is the primary protocol for current firmware
- Legacy protocol is supported for older P100 firmware
- `WithTransport("klap")` or `WithTransport("legacy")` to force a specific protocol
- No caller action needed for protocol selection in normal usage

### Integration Testing Notes

Integration tests against real hardware:
- Require a physical Tapo P100 on the same LAN
- Use environment variables for configuration
- Will physically toggle the plug (turn on/off)
- Not suitable for CI pipelines without hardware
- Serve as the SM-1 validation mechanism

### Dependencies and Constraints

- Zero external dependencies -- the README should explicitly state this
- Go 1.24+ minimum -- document in Installation section
- MIT license -- LICENSE file must exist at repository root
- Local LAN only -- no cloud, no internet dependency
- Credentials never logged or persisted by the library

## Project Structure Notes

Files created or modified in this story:

```
github.com/mjenh/tapo/
  README.md              # Full project documentation (NEW)
  LICENSE                # MIT license file (NEW)
  doc.go                 # Updated package doc comment (MODIFIED -- placeholder from Story 1.1)
  example_test.go        # Compilable example matching README quickstart (NEW)
```

All files are at the repository root. No changes to `internal/` packages. No changes to existing source code behavior.

## References

- **PRD:** `_bmad-output/planning-artifacts/tapo/tapo-prd.md` (section 6.3 Support Matrix, section 7 Success Metrics SM-1/SM-4, section 9.1 Public API Surface)
- **Spec:** `_bmad-output/planning-artifacts/tapo/tapo-spec.md` (Capabilities CAP-1 through CAP-10, Constraints, Success signal)
- **Architecture:** `_bmad-output/planning-artifacts/tapo/tapo-architecture.md` (AD-6 sentinel errors, AD-7 lazy negotiation, Structural Seed, Consistency Conventions)
- **Epics:** `_bmad-output/planning-artifacts/tapo/tapo-epics.md` (Epic 3, Story 3.1)
- **Addendum:** `_bmad-output/planning-artifacts/tapo/addendum.md` (DeviceInfo field mapping, protocol overview, v1.0.0 certified firmware)
- **Glossary:** `_bmad-output/planning-artifacts/tapo/glossary.md`
- **Story 1.1:** `_bmad-output/implementation-artifacts/tapo/1-1-module-scaffold-shared-crypto-helpers.md` (placeholder files created)

## File List

- `README.md` (NEW) — Full project documentation with all sections
- `LICENSE` (NEW) — MIT license, 2026
- `doc.go` (MODIFIED) — Expanded package doc with quickstart, transport notes, error handling
- `example_test.go` (NEW) — Compilable example matching README quickstart

## Change Log

- 2026-07-01: Story 3.1 implementation — created README.md, LICENSE, example_test.go; updated doc.go

## Dev Agent Record

### Iteration 1

**Status:** Complete
**Started:** 2026-07-01
**Completed:** 2026-07-01
**Changes:** Created README.md with 14 sections covering all ACs (quickstart, env vars, API reference, options, error handling, non-P100 models, concurrency, support matrix, testing, license, disclaimer). Created MIT LICENSE file. Updated doc.go with expanded godoc including quick start examples and error handling section. Created example_test.go for compile-time API validation.
**Notes:** All acceptance criteria satisfied. README structure follows story spec exactly. API signatures verified against actual codebase. DeviceInfo includes SSID field (decoded from base64) beyond original spec.
**Test results:** Files created and verified. example_test.go compiles against actual API signatures.
