# Story 3.2: Test Coverage & v1.0.0 Release

Status: done

## Story

As a library maintainer,
I want comprehensive unit tests and a tagged release,
So that contributors can trust the library and consumers can pin a stable version.

## Acceptance Criteria

1. **Given** the completed library from Epics 1 and 2, **When** the test suite is audited and gaps are filled, **Then** unit test coverage is >=80% on protocol and crypto helpers (excluding integration-only paths) per SM-3.

2. **Given** transport-level tests exist, **When** the test suite is reviewed, **Then** all transport tests use mocked HTTP servers via `httptest.NewServer` — no real hardware is required to run `go test ./...`.

3. **Given** concurrent Plug usage patterns, **When** `go test -race ./...` is executed, **Then** zero data races are reported for concurrent `TurnOn`, `TurnOff`, `Toggle`, and `DeviceInfo` calls on a shared `*Plug` instance.

4. **Given** the library targets real hardware for validation, **When** integration test instructions are reviewed, **Then** a documented procedure exists for manually running integration tests against a real P100 on the LAN, including environment variable setup and expected output.

5. **Given** the full test suite and linting, **When** `go vet ./...` and `go test ./...` are run, **Then** both pass cleanly on Linux and macOS with zero errors and zero warnings.

6. **Given** all tests pass and documentation is complete, **When** the release is prepared, **Then** the module is tagged `v1.0.0` following semver conventions.

7. **Given** the `v1.0.0` tag is applied, **When** the module is inspected, **Then** `go.sum` is committed and the module is publishable via `go get github.com/mjenh/tapo@v1.0.0`.

## Tasks / Subtasks

### Task 1: Test Gap Audit [AC-1]

- [ ] Run `go test -cover ./...` on the entire module and record per-package coverage percentages
- [ ] Identify packages below the 80% threshold — focus on:
  - `internal/crypto/` (crypto.go, auth.go)
  - `internal/transport/klap/` (klap.go)
  - `internal/transport/legacy/` (legacy.go)
  - `internal/transport/` (negotiate.go)
  - Root package (plug.go, device_info.go, options.go, errors.go)
- [ ] Catalogue untested code paths: error branches, edge cases, boundary conditions
- [ ] Document which paths are integration-only (require real hardware) and exclude them from the coverage target
- [ ] Create a coverage improvement plan prioritizing high-impact gaps

### Task 2: Crypto Test Coverage [AC-1, AC-5]

File: `internal/crypto/crypto_test.go`, `internal/crypto/auth_test.go`

- [ ] Verify existing tests from Story 1.1 cover:
  - AES-CBC encrypt/decrypt round-trip with known test vectors
  - PKCS7 padding edge cases (block-aligned, empty, single-byte)
  - `Encrypt`/`Decrypt` error paths (invalid key length, non-block-aligned ciphertext, corrupted padding)
  - `KLAPAuthHash` with known inputs, empty inputs, and Unicode characters
  - `KLAPDeriveKeyAndIV` determinism and output size validation
- [ ] Add any missing tests identified in Task 1 audit
- [ ] Add tests for legacy auth helpers if extended in Story 2.1 (SHA1+base64 login encoding)
- [ ] Verify coverage >= 80% on `internal/crypto/` package after additions
- [ ] Run `go test -v ./internal/crypto/...` and confirm all pass

### Task 3: KLAP Transport Test Coverage [AC-1, AC-2]

File: `internal/transport/klap/klap_test.go`

- [ ] Verify existing tests from Story 1.2 cover:
  - Successful two-stage handshake with mock HTTP server returning valid seeds and `TP_SESSIONID` cookie
  - `Login` returns wrapped `ErrAuth` on invalid credentials (mock returns 401 or auth failure response)
  - `Login` returns wrapped `ErrHandshake` on handshake failure (mock returns malformed response)
  - `Send` encrypts payload, includes sequence counter in URL, decrypts response
  - `Send` with expired session returns appropriate error
  - Context cancellation during `Login` and `Send`
- [ ] Add any missing tests identified in Task 1 audit
- [ ] Add error-path tests:
  - Mock server returning HTTP 500
  - Mock server returning truncated/malformed handshake response
  - Mock server closing connection mid-handshake
  - Network timeout during handshake (context with short deadline)
- [ ] Verify all mock HTTP servers are created with `httptest.NewServer` and properly closed in test cleanup
- [ ] Verify coverage >= 80% on `internal/transport/klap/` package

### Task 4: Legacy Transport Test Coverage [AC-1, AC-2]

File: `internal/transport/legacy/legacy_test.go`

- [ ] Verify existing tests from Story 2.1 cover:
  - RSA handshake with mock HTTP server
  - `Login` returns wrapped `ErrAuth` on invalid credentials
  - `Login` returns wrapped `ErrHandshake` on RSA handshake failure
  - `Send` wraps commands in `securePassthrough` with AES-CBC encryption
  - `Send` appends session token to requests
  - Session token stored correctly after successful login
  - Context cancellation during `Login` and `Send`
- [ ] Add any missing tests identified in Task 1 audit
- [ ] Add error-path tests:
  - Mock server returning invalid RSA key material
  - Mock server returning error code 9999 (session expiry)
  - `securePassthrough` decryption failure (corrupted response)
- [ ] Verify all mock HTTP servers use `httptest.NewServer` and are properly cleaned up
- [ ] Verify coverage >= 80% on `internal/transport/legacy/` package

### Task 5: NegotiatingTransport Test Coverage [AC-1, AC-2]

File: `internal/transport/negotiate_test.go`

- [ ] Verify existing tests from Story 2.2 cover:
  - KLAP-first success: mock KLAP handshake succeeds, no legacy fallback
  - KLAP failure + legacy fallback: KLAP mock returns distinguishable protocol error, legacy mock succeeds
  - Transport caching: after successful negotiation, subsequent `Send` calls use the cached transport
  - `WithTransport("klap")` bypasses negotiation, uses KLAP directly
  - `WithTransport("legacy")` bypasses negotiation, uses legacy directly
  - Both transports fail: returns error with context from both attempts
- [ ] Add any missing tests identified in Task 1 audit
- [ ] Add table-driven tests for negotiation scenarios:
  - KLAP succeeds immediately
  - KLAP fails with handshake error, legacy succeeds
  - Both KLAP and legacy fail
  - Override to KLAP only
  - Override to legacy only
- [ ] Verify coverage >= 80% on negotiation logic in `internal/transport/`

### Task 6: Root Package Test Coverage [AC-1, AC-2]

File: `plug_test.go`

- [ ] Verify existing tests from Stories 1.3, 1.4, 1.5 cover:
  - `NewPlug` construction with valid host, email, password
  - `NewPlug` returns error when host is empty
  - `NewPlug` returns error when credentials are missing
  - `NewPlug` performs no network I/O (returns immediately)
  - `NewPlugFromEnv` reads TAPO_HOST, TAPO_EMAIL, TAPO_PASSWORD from environment
  - `NewPlugFromEnv` reads TAPO_IP as alias for TAPO_HOST
  - `NewPlugFromEnv` returns error listing missing variables
  - `DeviceInfo` triggers login on first call, returns populated struct
  - `DeviceInfo` decodes base64 Nickname and SSID fields
  - `DeviceInfo` returns wrapped `ErrUnsupportedModel` for non-P100 devices
  - `TurnOn` sends `set_device_info` with `{"device_on": true}`
  - `TurnOff` sends `set_device_info` with `{"device_on": false}`
  - `Toggle` reads current state then sets inverse
  - `Toggle` returns error when current state cannot be read
  - `WithTimeout` option sets per-request timeout
  - `WithTransport` option overrides transport selection
  - Sentinel errors (`ErrAuth`, `ErrTimeout`, `ErrUnsupportedModel`, `ErrHandshake`) are detectable via `errors.Is`
- [ ] Add any missing tests identified in Task 1 audit
- [ ] Add table-driven tests for option validation and error wrapping
- [ ] Verify coverage >= 80% on root package (excluding integration-only code paths)

### Task 7: Race Condition Tests [AC-3]

File: `plug_test.go`

- [ ] Verify existing race-condition tests from Story 1.5 cover:
  - Multiple goroutines calling `TurnOn`, `TurnOff`, `Toggle`, and `DeviceInfo` concurrently on a shared `*Plug`
  - Concurrent session expiry and re-authentication (three goroutines hitting 9999 simultaneously)
  - Serialized re-authentication: only one goroutine re-logins, others block and retry
- [ ] Add additional race-condition test scenarios if gaps exist:
  - Concurrent `NewPlug` construction with same host (independent instances, no shared state)
  - Rapid alternating `TurnOn`/`TurnOff` from separate goroutines
  - Concurrent `DeviceInfo` calls racing with `Toggle`
- [ ] Run `go test -race ./...` and confirm zero data race warnings
- [ ] Add a test comment documenting the `-race` requirement: `// Run with: go test -race ./...`

### Task 8: Go Vet & Static Analysis [AC-5]

- [ ] Run `go vet ./...` and fix any warnings
- [ ] Common issues to check and resolve:
  - Unreachable code
  - Incorrect `Printf` format verbs
  - Unused variables or imports
  - Struct field alignment suggestions (informational only)
  - Suspicious `sync.Mutex` copies
- [ ] Run `go test ./...` and confirm all tests pass cleanly
- [ ] Verify identical behavior on both Linux and macOS:
  - Run tests via `go test ./...` on Linux
  - Run tests via `go test ./...` on macOS
  - Confirm `go vet ./...` is clean on both platforms

### Task 9: Integration Test Documentation [AC-4]

File: `INTEGRATION_TESTING.md` (or documented in README per Story 3.1)

- [ ] Document integration test instructions for manual execution against real hardware:
  - Prerequisites: Go 1.24+, a P100 smart plug on the same LAN, Tapo account credentials
  - Environment variable setup:
    ```bash
    export TAPO_HOST="192.168.1.X"
    export TAPO_EMAIL="your-tapo-email@example.com"
    export TAPO_PASSWORD="your-tapo-password"
    ```
  - Command to run integration tests: `go test -tags=integration -v ./...`
  - Expected output: successful connection, DeviceInfo with Model "P100", power control verification
  - Troubleshooting: common issues (wrong IP, firewall, incorrect credentials)
- [ ] Add build tag `//go:build integration` to any integration test files to exclude them from default `go test ./...` runs
- [ ] Document that integration tests are NOT required in CI — they require physical hardware
- [ ] Include note about certified firmware: P100 v1.4.4 Build 20240514 Rel 35017

### Task 10: Release Preparation [AC-6, AC-7]

- [ ] Verify `go.mod` is correct:
  - Module path: `github.com/mjenh/tapo`
  - Go version: 1.24 or later
  - No unnecessary `require` blocks (zero external dependencies)
- [ ] Run `go mod tidy` to clean up `go.mod` and generate/update `go.sum`
- [ ] Commit `go.sum` to the repository
- [ ] Verify module is publishable:
  - `go build ./...` succeeds
  - `go test ./...` passes
  - `go vet ./...` is clean
  - `go test -race ./...` passes
  - `go test -cover ./...` shows >= 80% on protocol and crypto helpers
- [ ] Tag the release:
  ```bash
  git tag -a v1.0.0 -m "v1.0.0: Initial stable release of tapo Go client library"
  git push origin v1.0.0
  ```
- [ ] Verify the module is fetchable: `go get github.com/mjenh/tapo@v1.0.0`
- [ ] Confirm semver compliance: `v1.0.0` is the first stable release, no prior v0.x.x tags that would conflict

## Dev Notes

### Tests Already Implemented in Prior Stories

This story is a gap-filling and finalization story, not a greenfield test-writing story. Significant test coverage already exists from prior stories:

- **Story 1.1** implemented: `internal/crypto/crypto_test.go` (AES-CBC round-trip, known test vectors, PKCS7 edge cases, error paths) and `internal/crypto/auth_test.go` (KLAPAuthHash with known inputs, KLAPDeriveKeyAndIV determinism and output sizes)
- **Story 1.2** implemented: `internal/transport/klap/klap_test.go` (mock HTTP server handshake, encrypt/decrypt round-trip, ErrAuth/ErrHandshake error paths, context cancellation)
- **Story 1.3** implemented: `plug_test.go` (construction validation, NewPlugFromEnv, DeviceInfo with base64 decoding, ErrUnsupportedModel, sentinel error detection)
- **Story 1.4** implemented: `plug_test.go` additions (TurnOn/TurnOff/Toggle via mock transport, error propagation)
- **Story 1.5** implemented: `plug_test.go` additions (concurrent goroutine tests, race-condition validation, session re-auth serialization)
- **Story 2.1** implemented: `internal/transport/legacy/legacy_test.go` (RSA handshake mock, securePassthrough, ErrAuth/ErrHandshake)
- **Story 2.2** implemented: `internal/transport/negotiate_test.go` (KLAP-first, fallback, caching, override)

The primary work in this story is:
1. Auditing coverage gaps across all packages
2. Adding missing error-path and edge-case tests
3. Ensuring the race-condition test suite is thorough
4. Verifying cross-platform clean builds
5. Documenting integration tests
6. Tagging the v1.0.0 release

### Testing Patterns

- **Mock HTTP servers:** All transport tests use `httptest.NewServer` from the Go stdlib. Each test creates its own server, configures the handler to simulate the Tapo device HTTP responses, and defers `server.Close()`. No real hardware is contacted during `go test ./...`.
- **Table-driven tests:** Follow Go convention using `[]struct{ name string; ... }` slices with `t.Run(tc.name, ...)` subtests for systematic coverage of input variations.
- **Known test vectors:** Crypto tests use hardcoded byte slices or hex-encoded strings decoded in tests. Vectors are deterministic and reproducible. Do not generate test data at runtime.
- **Race detection:** Run `go test -race ./...` to validate goroutine safety. The `-race` flag instruments memory accesses and reports data races. Race tests should exercise concurrent access to shared `*Plug` instances with mock transports.
- **Build tags for integration tests:** Integration test files use `//go:build integration` to exclude them from default test runs. They are run manually with `go test -tags=integration -v ./...` against real hardware.
- **Internal test packages:** Crypto tests use `package crypto` (same package) to access unexported helpers like PKCS7 padding functions. Transport and root package tests use `package X_test` (external test package) for black-box testing of exported APIs, or `package X` when testing unexported internals.
- **No external test dependencies:** All tests use the `testing` stdlib package. No testify, gomock, or other external test frameworks. Assertions use `if got != want { t.Errorf(...) }` pattern.

### Coverage Target Details

- **SM-3 threshold:** >= 80% on protocol and crypto helpers
- **Scope:** `internal/crypto/`, `internal/transport/klap/`, `internal/transport/legacy/`, `internal/transport/` (negotiate.go), root package (plug.go, device_info.go, options.go, errors.go)
- **Exclusions:** Integration-only code paths that require real hardware (e.g., actual HTTP calls to a Tapo device). These paths should be clearly marked with comments explaining they are exercised by integration tests only.
- **Measurement:** `go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out` to get per-function coverage

### Release Process

1. Ensure all tests pass: `go test ./...`
2. Ensure race detector passes: `go test -race ./...`
3. Ensure vet is clean: `go vet ./...`
4. Ensure coverage meets threshold: `go test -cover ./...`
5. Clean up module: `go mod tidy`
6. Commit all changes including `go.sum`
7. Create annotated tag: `git tag -a v1.0.0 -m "v1.0.0: Initial stable release"`
8. Push tag: `git push origin v1.0.0`
9. Verify fetchability: `go get github.com/mjenh/tapo@v1.0.0` from a clean environment

### Cross-Platform Considerations

- Tests must pass on both Linux and macOS (NFR-9)
- Windows is best-effort — not a gating requirement for v1.0.0
- File path handling uses `path/filepath` where applicable (not relevant for this library, but verified)
- No OS-specific build tags in library code (crypto/net/http are cross-platform in Go stdlib)
- CI should run on both `ubuntu-latest` and `macos-latest` (if CI is configured)

## Project Structure Notes

Test files created or modified across all stories (this story audits and fills gaps):

```
github.com/mjenh/tapo/
  plug_test.go                             # Root package: construction, DeviceInfo, power control, env vars, race tests
  internal/
    crypto/
      crypto_test.go                       # AES-CBC encrypt/decrypt, PKCS7 padding, error paths
      auth_test.go                         # KLAPAuthHash, KLAPDeriveKeyAndIV, legacy auth helpers
    transport/
      negotiate_test.go                    # NegotiatingTransport: KLAP-first, fallback, caching, override
      klap/
        klap_test.go                       # KLAP transport: mock HTTP handshake, encrypt/decrypt, errors
      legacy/
        legacy_test.go                     # Legacy transport: mock HTTP RSA handshake, securePassthrough, errors
```

Coverage measurement output:

```
github.com/mjenh/tapo/
  coverage.out                             # Generated by go test -coverprofile (not committed)
```

Integration test documentation:

```
github.com/mjenh/tapo/
  INTEGRATION_TESTING.md                   # Manual integration test instructions (or in README)
```

## References

- **PRD:** `_bmad-output/planning-artifacts/tapo/tapo-prd.md`
- **Spec:** `_bmad-output/planning-artifacts/tapo/tapo-spec.md` (SM-3 coverage requirement, NFR-9 platform targets)
- **Architecture:** `_bmad-output/planning-artifacts/tapo/tapo-architecture.md` (AD-5 mutex concurrency, AD-7 lazy negotiation, Structural Seed for test file layout)
- **Epics:** `_bmad-output/planning-artifacts/tapo/tapo-epics.md` (Epic 3, Story 3.2)
- **Addendum:** `_bmad-output/planning-artifacts/tapo/addendum.md` (protocol overview for integration test context)
- **Glossary:** `_bmad-output/planning-artifacts/tapo/glossary.md`
- **Implementation Readiness Report:** `_bmad-output/planning-artifacts/tapo/implementation-readiness-report-2026-07-01.md`
- **Story 1.1:** `_bmad-output/implementation-artifacts/tapo/1-1-module-scaffold-shared-crypto-helpers.md` (template reference, initial crypto tests)

## Dev Agent Record

### Iteration 1

**Status:** Not started
**Started:** -
**Completed:** -
**Changes:** -
**Notes:** -
**Test results:** -
