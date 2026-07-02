# Story 2.2: NegotiatingTransport & Transport Override

> **Epic:** 2 - Legacy Transport & Protocol Negotiation
> **Status:** done
> **Depends on:** Story 1.2 (KLAP Transport), Story 2.1 (Legacy Transport)
> **Blocked by:** None (assuming 1.2 and 2.1 are complete)

---

## Story

As a Go developer,
I want automatic protocol detection and the option to force a specific protocol,
So that I don't need to know which firmware my plug runs, but can override for debugging.

---

## Acceptance Criteria

### AC-1: NegotiatingTransport Implements Transport Interface
**Given** the `Transport` interface defined in `internal/transport/transport.go`
**When** the `NegotiatingTransport` is implemented in `internal/transport/negotiate.go`
**Then** it satisfies the `Transport` interface with `Login` and `Send` methods
**And** a compile-time check `var _ Transport = (*NegotiatingTransport)(nil)` passes
**And** the implementation conforms to AD-7 (lazy transport negotiation)

### AC-2: KLAP-First Protocol Negotiation on Login
**Given** a `NegotiatingTransport` with no cached transport
**When** `Login` is called for the first time
**Then** it attempts KLAP handshake first
**And** if KLAP succeeds, the KLAP transport is cached as the winner
**And** the cached transport is used for all subsequent `Send` calls without re-negotiation

### AC-3: Fallback to Legacy on Handshake Failure
**Given** a `NegotiatingTransport` attempting KLAP login
**When** the KLAP transport returns an error wrapping `ErrHandshake` (distinguishable protocol failure)
**Then** the `NegotiatingTransport` retries with the legacy transport
**And** if legacy succeeds, the legacy transport is cached as the winner
**And** the cached transport is used for all subsequent `Send` calls

### AC-4: No Fallback on Authentication or Network Errors
**Given** a `NegotiatingTransport` attempting KLAP login
**When** the KLAP transport returns an error wrapping `ErrAuth` (invalid credentials)
**Then** the error is returned immediately without attempting legacy
**When** the KLAP transport returns a network error (connection refused, timeout, DNS failure)
**Then** the error is returned immediately without attempting legacy

### AC-5: Send Delegates to Cached Transport
**Given** a `NegotiatingTransport` that has completed negotiation (transport cached)
**When** `Send` is called
**Then** the call is delegated directly to the cached concrete transport's `Send` method
**And** no negotiation logic runs on `Send` calls
**And** `Send` returns an error if `Login` has not been called (no cached transport)

### AC-6: Plug Holds Single Transport Reference
**Given** the `Plug` struct in `plug.go`
**When** `NewPlug` is called without a `WithTransport` option
**Then** `Plug` creates a `NegotiatingTransport` as its transport
**And** `Plug` calls `Login` once on its transport reference
**And** `Plug` does not import or reference `klap` or `legacy` packages directly

### AC-7: WithTransport Option Bypasses Negotiation
**Given** a developer constructing a Plug with `WithTransport("klap")`
**When** the option is applied
**Then** the `Plug` uses a KLAP transport directly instead of `NegotiatingTransport`
**And** no negotiation occurs
**Given** a developer constructing a Plug with `WithTransport("legacy")`
**When** the option is applied
**Then** the `Plug` uses a legacy transport directly instead of `NegotiatingTransport`
**And** no negotiation occurs
**Given** a developer constructing a Plug with `WithTransport("invalid")`
**When** `NewPlug` is called
**Then** it returns a descriptive error indicating the transport name is not recognized

### AC-8: KLAP Firmware Connects Without Fallback
**Given** a P100 on KLAP firmware (v1.4.4 Build 20240514 Rel 35017)
**When** `NegotiatingTransport.Login` is called
**Then** it connects via KLAP on the first attempt
**And** no fallback to legacy occurs
**And** KLAP is cached as the winning transport

### AC-9: Legacy-Only Firmware Falls Back Correctly
**Given** a P100 on legacy-only firmware
**When** `NegotiatingTransport.Login` is called
**Then** KLAP fails with a distinguishable `ErrHandshake` error
**And** the legacy transport is attempted and succeeds
**And** the legacy transport is cached as the winner

### AC-10: Unit Tests with Error Discrimination
**Given** the `NegotiatingTransport` implementation
**When** unit tests are run
**Then** tests verify KLAP-first success (no fallback) using mock transports
**And** tests verify fallback to legacy on `ErrHandshake`
**And** tests verify no fallback on `ErrAuth` (error returned immediately)
**And** tests verify no fallback on network errors (error returned immediately)
**And** tests verify `Send` delegates to the cached transport
**And** tests verify `Send` errors when `Login` has not been called
**And** tests verify `WithTransport` bypasses negotiation
**And** all tests pass with `go test -race`

---

## Tasks / Subtasks

### Task 1: Implement NegotiatingTransport Struct [AC-1]

File: `internal/transport/negotiate.go`

- [ ] **1.1** Define the `NegotiatingTransport` struct:
  ```go
  package transport

  import (
      "net/http"
      "sync"
  )

  // NegotiatingTransport implements Transport by trying KLAP first,
  // falling back to legacy on handshake failure, and caching the winner.
  type NegotiatingTransport struct {
      host   string
      client *http.Client

      mu     sync.Mutex  // guards cached
      cached Transport    // the winning transport after negotiation
  }
  ```
- [ ] **1.2** Implement constructor:
  ```go
  // NewNegotiating creates a NegotiatingTransport for the given host.
  // The transport is not authenticated until Login is called.
  func NewNegotiating(host string, client *http.Client) *NegotiatingTransport
  ```
  - If `client` is nil, create a new `http.Client`
  - Store host and client; `cached` starts as nil
- [ ] **1.3** Add compile-time interface check:
  ```go
  var _ Transport = (*NegotiatingTransport)(nil)
  ```

### Task 2: Implement Login with Protocol Negotiation [AC-2, AC-3, AC-4]

File: `internal/transport/negotiate.go`

- [ ] **2.1** Implement `Login(ctx context.Context, email, password string) error`:
  - Acquire mutex, check if `cached` is already set; if so, return nil (already negotiated)
  - Release mutex before any network I/O
  - Create a KLAP transport: `klap.New(host, client)`
  - Call `klapTransport.Login(ctx, email, password)`
  - On success: acquire mutex, set `cached = klapTransport`, release mutex, return nil
  - On error: inspect the error for negotiation decision

- [ ] **2.2** Implement error discrimination logic:
  ```go
  // After KLAP Login fails:
  if errors.Is(err, tapo.ErrHandshake) {
      // Protocol mismatch — try legacy
  } else {
      // ErrAuth, network error, context cancellation — return immediately
      return err
  }
  ```
  - Only `ErrHandshake` triggers fallback — this means the device doesn't speak KLAP
  - `ErrAuth` means credentials are wrong; trying legacy won't help
  - Network errors mean the device is unreachable; trying legacy won't help

- [ ] **2.3** Implement legacy fallback path:
  - Create a legacy transport: `legacy.New(host, client)`
  - Call `legacyTransport.Login(ctx, email, password)`
  - On success: acquire mutex, set `cached = legacyTransport`, release mutex, return nil
  - On error: return the legacy error (optionally wrapping both errors for diagnostics)

- [ ] **2.4** Ensure the mutex is never held during HTTP I/O (per AD-5):
  - Lock only to read/write the `cached` field
  - All `Login` calls on concrete transports happen outside the lock

### Task 3: Implement Send Delegation [AC-5]

File: `internal/transport/negotiate.go`

- [ ] **3.1** Implement `Send(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error)`:
  - Acquire mutex, read `cached`, release mutex
  - If `cached` is nil, return an error: `fmt.Errorf("negotiate: Login has not been called")`
  - Delegate to `cached.Send(ctx, method, payload)` and return its result
- [ ] **3.2** No negotiation logic in `Send` — it is a pure passthrough to the cached transport

### Task 4: Add WithTransport Option [AC-7]

File: `options.go`

- [ ] **4.1** Ensure the `Option` type and `config` struct exist (from Story 1.3):
  ```go
  type Option func(*config)

  type config struct {
      timeout       time.Duration
      transportName string  // "klap", "legacy", or "" (auto-negotiate)
  }
  ```
- [ ] **4.2** Implement `WithTransport` option function:
  ```go
  // WithTransport forces a specific transport protocol, bypassing automatic
  // negotiation. Valid values are "klap" and "legacy". Any other value
  // causes NewPlug to return an error.
  func WithTransport(name string) Option {
      return func(c *config) {
          c.transportName = name
      }
  }
  ```
- [ ] **4.3** The validation of the transport name happens in `NewPlug` (Task 5), not in the option function itself

### Task 5: Wire NegotiatingTransport into Plug [AC-6, AC-7]

File: `plug.go`

- [ ] **5.1** Modify `NewPlug` to create the transport based on options:
  ```go
  func NewPlug(ctx context.Context, host, email, password string, opts ...Option) (*Plug, error) {
      cfg := defaultConfig()
      for _, opt := range opts {
          opt(&cfg)
      }

      var t transport.Transport
      switch cfg.transportName {
      case "":
          // Default: auto-negotiate
          t = transport.NewNegotiating(host, httpClient)
      case "klap":
          t = klap.New(host, httpClient)
      case "legacy":
          t = legacy.New(host, httpClient)
      default:
          return nil, fmt.Errorf("tapo: unknown transport %q (valid: \"klap\", \"legacy\")", cfg.transportName)
      }

      return &Plug{
          host:      host,
          email:     email,
          password:  password,
          transport: t,
          // ...
      }, nil
  }
  ```
- [ ] **5.2** Ensure `plug.go` imports `internal/transport` but does NOT import `internal/transport/klap` or `internal/transport/legacy` when `WithTransport` is not used
  - When `WithTransport` IS used, the `plug.go` switch statement must import the concrete packages
  - This is acceptable: `Plug` still interacts only through the `Transport` interface; imports are for construction only
- [ ] **5.3** Verify `Plug` calls `Login` on the transport reference without any protocol-specific logic
- [ ] **5.4** Verify `NewPlugFromEnv` also respects `WithTransport` (it passes options to the same config)

### Task 6: Unit Tests for NegotiatingTransport [AC-10]

File: `internal/transport/negotiate_test.go`

- [ ] **6.1** Create mock transport helpers for testing:
  ```go
  // mockTransport is a test double implementing Transport.
  type mockTransport struct {
      loginErr error
      sendResp json.RawMessage
      sendErr  error
      loginCalls int
      sendCalls  int
  }

  func (m *mockTransport) Login(ctx context.Context, email, password string) error {
      m.loginCalls++
      return m.loginErr
  }

  func (m *mockTransport) Send(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error) {
      m.sendCalls++
      return m.sendResp, m.sendErr
  }
  ```

- [ ] **6.2** Refactor `NegotiatingTransport` to accept a transport factory for testability:
  - The production code creates KLAP and legacy transports internally
  - For testing, provide an internal-only mechanism (unexported field or constructor) to inject mock transport factories:
  ```go
  // newTransportFunc creates a concrete Transport for a given host and client.
  type newTransportFunc func(host string, client *http.Client) Transport

  // For testing: allow overriding the KLAP and legacy constructors
  type NegotiatingTransport struct {
      host   string
      client *http.Client

      mu     sync.Mutex
      cached Transport

      // Factory functions, set to defaults in NewNegotiating
      newKLAP   newTransportFunc
      newLegacy newTransportFunc
  }
  ```

- [ ] **6.3** Test: **KLAP succeeds on first attempt (no fallback)**
  - Inject a mock KLAP that returns nil from Login
  - Call `NegotiatingTransport.Login`
  - Assert no error, KLAP mock's `loginCalls == 1`, legacy mock's `loginCalls == 0`
  - Call `Send` and assert it delegates to the KLAP mock

- [ ] **6.4** Test: **Fallback to legacy on ErrHandshake**
  - Inject a mock KLAP that returns `fmt.Errorf("klap: handshake1 failed: %w", tapo.ErrHandshake)`
  - Inject a mock legacy that returns nil from Login
  - Call `NegotiatingTransport.Login`
  - Assert no error, KLAP mock's `loginCalls == 1`, legacy mock's `loginCalls == 1`
  - Call `Send` and assert it delegates to the legacy mock

- [ ] **6.5** Test: **No fallback on ErrAuth**
  - Inject a mock KLAP that returns `fmt.Errorf("klap: authentication failed: %w", tapo.ErrAuth)`
  - Call `NegotiatingTransport.Login`
  - Assert error wraps `ErrAuth`
  - Assert legacy mock's `loginCalls == 0`

- [ ] **6.6** Test: **No fallback on network error**
  - Inject a mock KLAP that returns `errors.New("dial tcp: connection refused")`
  - Call `NegotiatingTransport.Login`
  - Assert error is returned as-is
  - Assert legacy mock's `loginCalls == 0`

- [ ] **6.7** Test: **No fallback on context cancellation**
  - Inject a mock KLAP that returns `context.Canceled`
  - Call `NegotiatingTransport.Login` with a cancelled context
  - Assert error indicates cancellation
  - Assert legacy mock's `loginCalls == 0`

- [ ] **6.8** Test: **Send errors when Login not called**
  - Create a fresh `NegotiatingTransport` (no Login called)
  - Call `Send`
  - Assert error indicates Login has not been called

- [ ] **6.9** Test: **Send delegates to cached transport**
  - After successful negotiation (KLAP wins), call `Send` multiple times
  - Assert all calls go to the cached KLAP mock
  - Assert the response from Send matches the mock's configured response

- [ ] **6.10** Test: **Login is idempotent after negotiation**
  - After successful negotiation, call `Login` again
  - Assert no additional KLAP or legacy Login calls (cached transport is reused)
  - Assert no error returned

- [ ] **6.11** Test: **Legacy fallback also fails**
  - Inject a mock KLAP that returns `ErrHandshake`
  - Inject a mock legacy that returns `ErrAuth`
  - Call `NegotiatingTransport.Login`
  - Assert the returned error wraps the legacy error (the final failure)

- [ ] **6.12** Run all tests with race detector: `go test -race ./internal/transport/...`

### Task 7: Unit Tests for WithTransport Option [AC-7]

File: `options_test.go` or `plug_test.go`

- [ ] **7.1** Test: **WithTransport("klap") creates KLAP transport directly**
  - Call `NewPlug` with `WithTransport("klap")`
  - Assert no error
  - Assert the Plug's internal transport is a `*klap.Transport` (not a `*NegotiatingTransport`)

- [ ] **7.2** Test: **WithTransport("legacy") creates legacy transport directly**
  - Call `NewPlug` with `WithTransport("legacy")`
  - Assert no error
  - Assert the Plug's internal transport is a `*legacy.Transport`

- [ ] **7.3** Test: **WithTransport("invalid") returns error**
  - Call `NewPlug` with `WithTransport("invalid")`
  - Assert error is returned with a descriptive message

- [ ] **7.4** Test: **No WithTransport creates NegotiatingTransport**
  - Call `NewPlug` without WithTransport
  - Assert the Plug's internal transport is a `*NegotiatingTransport`

### Task 8: Verify Build and Race Safety [AC-1, AC-10]

- [ ] **8.1** Run `go build ./...` and confirm zero errors
- [ ] **8.2** Run `go vet ./...` and confirm no warnings
- [ ] **8.3** Run `go test -race ./...` and confirm all tests pass with no data races
- [ ] **8.4** Confirm no new external dependencies in `go.mod`

---

## Dev Notes

### Architecture Constraints

- **AD-7 (Lazy Transport Negotiation):** `NewPlug` returns immediately with no network I/O. `NegotiatingTransport` defers protocol selection to the first `Login` call. The `Plug` holds one `Transport` reference and interacts with it exclusively through the `Transport` interface. It does not know whether it is talking to KLAP, legacy, or the negotiator.

- **AD-2 (Package Split):** `NegotiatingTransport` lives in `internal/transport/` (the same package as the `Transport` interface). It imports both `internal/transport/klap` and `internal/transport/legacy` to create concrete transports. This is the only place where both concrete packages are imported together.

- **AD-5 (Mutex-Based Concurrency):** `NegotiatingTransport` uses its own `sync.Mutex` to guard the `cached` field. The mutex is never held during network I/O. The concrete transports have their own mutexes for their session state. The Plug-level mutex (AD-5 layer 2) serializes re-authentication, which means `NegotiatingTransport.Login` will only be called by one goroutine at a time in practice, but the implementation must still be safe for concurrent calls.

- **AD-4 (Transport-Owned Session State):** `NegotiatingTransport` does not hold session state itself. It delegates all session management to the cached concrete transport. The only state it manages is which concrete transport won.

### Error Discrimination Logic

The critical design point of this story is correctly distinguishing "wrong protocol" from "wrong credentials" and "unreachable device":

| Error from KLAP Login | `errors.Is` check | Action |
|---|---|---|
| Handshake failure (device doesn't speak KLAP) | `errors.Is(err, tapo.ErrHandshake)` | Fallback to legacy |
| Authentication failure (wrong email/password) | `errors.Is(err, tapo.ErrAuth)` | Return error immediately |
| Network error (connection refused, DNS, timeout) | Neither `ErrHandshake` nor `ErrAuth` | Return error immediately |
| Context cancelled/deadline exceeded | Neither `ErrHandshake` nor `ErrAuth` | Return error immediately |

The discrimination relies on the wrapping patterns established in Story 1.2:
- `fmt.Errorf("klap: handshake1 failed (status %d): %w", status, tapo.ErrHandshake)` -- protocol failure
- `fmt.Errorf("klap: invalid credentials: %w", tapo.ErrAuth)` -- auth failure

Only `ErrHandshake` is a signal that the device might support a different protocol. All other errors are either definitively wrong credentials (trying another protocol won't help) or infrastructure issues (the device can't be reached at all).

### What Exists from Prior Stories

| Component | Story | File | What It Provides |
|---|---|---|---|
| Transport interface | 1.2 | `internal/transport/transport.go` | `Login` + `Send` methods |
| KLAP transport | 1.2 | `internal/transport/klap/klap.go` | `klap.New(host, client)`, KLAP handshake + AES session |
| Legacy transport | 2.1 | `internal/transport/legacy/legacy.go` | `legacy.New(host, client)`, RSA handshake + securePassthrough |
| Sentinel errors | 1.2 | `errors.go` | `ErrAuth`, `ErrHandshake`, `ErrTimeout`, `ErrUnsupportedModel` |
| Crypto helpers | 1.1 | `internal/crypto/` | AES-CBC, KLAP auth hash, key/IV derivation |
| Plug struct | 1.3 | `plug.go` | `NewPlug`, `NewPlugFromEnv`, `DeviceInfo` |
| Option type | 1.3 | `options.go` | `Option func(*config)`, `WithTimeout` |

### Test Scenarios Summary

| Scenario | KLAP Login Result | Legacy Login Result | Expected Outcome |
|---|---|---|---|
| KLAP firmware | Success | Not attempted | KLAP cached, no fallback |
| Legacy-only firmware | `ErrHandshake` | Success | Legacy cached after fallback |
| Wrong credentials | `ErrAuth` | Not attempted | Error returned immediately |
| Device unreachable | Network error | Not attempted | Error returned immediately |
| Context cancelled | `context.Canceled` | Not attempted | Error returned immediately |
| Both protocols fail | `ErrHandshake` | `ErrAuth` | Legacy error returned |
| WithTransport("klap") | Direct use | Not created | No negotiation |
| WithTransport("legacy") | Not created | Direct use | No negotiation |

### Testability Design

To test `NegotiatingTransport` without real HTTP servers, the implementation should use factory functions for creating concrete transports. The production constructor sets these to `klap.New` and `legacy.New`; tests inject mock factories. This avoids exporting the factory mechanism while keeping the public API clean:

```go
// Production usage:
t := transport.NewNegotiating(host, client)

// Internal test usage (via unexported fields or test-only constructor):
t := &NegotiatingTransport{
    host:      host,
    client:    client,
    newKLAP:   func(h string, c *http.Client) Transport { return mockKLAP },
    newLegacy: func(h string, c *http.Client) Transport { return mockLegacy },
}
```

### Conventions

- Follow Go stdlib naming: `NegotiatingTransport`, `NewNegotiating`, `WithTransport`
- Error messages are lowercase, no trailing punctuation: `"negotiate: login has not been called"`
- No `init()` functions. No global mutable state.
- File-level comment in `negotiate.go` explaining the negotiation pattern
- `WithTransport` accepts only lowercase string values: `"klap"`, `"legacy"`

---

## Project Structure Notes

### Files Created in This Story

| File | Purpose |
|---|---|
| `internal/transport/negotiate.go` | `NegotiatingTransport` composite: KLAP-first negotiation with legacy fallback, winner caching |
| `internal/transport/negotiate_test.go` | Unit tests with mock transports for all negotiation paths |

### Files Modified in This Story

| File | Changes |
|---|---|
| `options.go` | Add `WithTransport(name string) Option` function, add `transportName` field to `config` |
| `plug.go` | Wire `NegotiatingTransport` as default transport, add switch for `WithTransport` override, import `internal/transport/klap` and `internal/transport/legacy` |

### Files from Prior Stories (Already Exist)

| File | Story | Contains |
|---|---|---|
| `go.mod` | 1.1 | Module declaration, Go 1.24+ |
| `errors.go` | 1.2 | `ErrAuth`, `ErrHandshake`, `ErrTimeout`, `ErrUnsupportedModel` |
| `internal/transport/transport.go` | 1.2 | `Transport` interface |
| `internal/transport/klap/klap.go` | 1.2 | KLAP transport implementation |
| `internal/transport/legacy/legacy.go` | 2.1 | Legacy transport implementation |
| `internal/crypto/crypto.go` | 1.1 | AES-CBC encrypt/decrypt |
| `internal/crypto/auth.go` | 1.1 | KLAP auth hash, key/IV derivation |
| `plug.go` | 1.3 | `Plug` struct, `NewPlug`, `NewPlugFromEnv` |
| `options.go` | 1.3 | `Option` type, `WithTimeout` |

---

## References

- **Architecture:** `_bmad-output/planning-artifacts/tapo/tapo-architecture.md` (AD-2, AD-4, AD-5, AD-7)
- **Epics:** `_bmad-output/planning-artifacts/tapo/tapo-epics.md` (Story 2.2 definition)
- **PRD:** `_bmad-output/planning-artifacts/tapo/tapo-prd.md` (CAP-1, CAP-2, CAP-9)
- **Spec:** `_bmad-output/planning-artifacts/tapo/tapo-spec.md` (CAP-9, Constraints)
- **Addendum:** `_bmad-output/planning-artifacts/tapo/addendum.md` (Protocol details)
- **Glossary:** `_bmad-output/planning-artifacts/tapo/glossary.md` (KLAP, Transport, Session terms)
- **Story 1.2:** `_bmad-output/implementation-artifacts/tapo/1-2-klap-transport.md` (Transport interface, KLAP implementation, error wrapping patterns)
- **Story 1.1:** `_bmad-output/implementation-artifacts/tapo/1-1-module-scaffold-shared-crypto-helpers.md` (Module scaffold, crypto helpers)

---

## Dev Agent Record

### Completion Log

| Task | Status | Notes |
|---|---|---|
| Task 1: NegotiatingTransport Struct | done | Factory pattern with NewTransportFunc type |
| Task 2: Login with Protocol Negotiation | done | Single-flighted KLAP-first, ErrHandshake-only fallback |
| Task 3: Send Delegation | done | Snapshot cached under short lock |
| Task 4: WithTransport Option | done | transportSet bool distinguishes default from explicit "" |
| Task 5: Wire into Plug | done | Switch on transportName, factories injected from plug.go |
| Task 6: NegotiatingTransport Tests | done | 14 tests including concurrency and panic tests |
| Task 7: WithTransport Option Tests | done | 5 tests in plug_test.go |
| Task 8: Build and Race Safety | done | go build, go vet, go test -race all pass |

### Change Log

- `internal/transport/negotiate.go` — created NegotiatingTransport with factory pattern, single-flighted Login, both-fail error diagnostics
- `internal/transport/negotiate_test.go` — 14 tests: KLAP success, legacy fallback, no-fallback on ErrAuth/network/context, send delegation, idempotent login, both-fail, single-flight concurrency, waiter context cancel, nil factory panics
- `options.go` — added WithTransport option, transportSet bool for empty-string validation
- `plug.go` — wired NegotiatingTransport as default, switch on transportName, httpClient.Timeout set from cfg
- `plug_test.go` — 5 tests: default transport, klap, legacy, invalid, empty string
- `internal/transport/legacy/legacy_test.go` — fixed unused parameter r in handleSecurePassthrough

### Review Findings (Patched)

- [x] [H-1] Single-flighted concurrent Login via negotiating flag + channel
- [x] [M-1] httpClient.Timeout set from cfg.timeout for auto-negotiation path
- [x] [M-2] Nil factory arguments panic with clear message
- [x] [L-1] Both-fail error wraps legacy error (%w) and includes KLAP context (%v)
- [x] [L-2] WithTransport("") rejected via transportSet bool

### Test Results

All tests pass with `go test -race ./...`. Zero data races.

### Notes

- Circular import avoided via factory function pattern: negotiate.go imports no concrete transport packages; plug.go injects klap.New and legacy.New as factories
- Single-flight pattern chosen over sync.Once because failed negotiations must be retryable
