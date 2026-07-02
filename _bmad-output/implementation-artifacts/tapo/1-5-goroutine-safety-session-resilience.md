# Story 1.5: Goroutine Safety & Session Resilience

**Epic:** Epic 1 — Core P100 Control Library
**Status:** ready-for-dev
**Priority:** High
**Story Points:** 3
**Depends On:** Story 1.4 (Plug Power Control)

---

## Story

As a Go developer running concurrent automation,
I want thread-safe client usage and automatic session recovery,
So that I can use the library in long-running services without data races or manual re-auth.

---

## Acceptance Criteria

### AC-1: Race-free concurrent access

**Given** a single Plug client shared across multiple goroutines
**When** concurrent `TurnOn`, `TurnOff`, `Toggle`, and `DeviceInfo` calls are made from separate goroutines
**Then** no data races are triggered when tested under `go test -race`

### AC-2: Plug-level mutex serializes re-authentication

**Given** a single Plug client shared across multiple goroutines
**When** a session-expiry error is detected during command execution
**Then** the Plug-level `sync.Mutex` serializes re-authentication so only one goroutine re-logins at a time per AD-5

### AC-3: Automatic re-auth on session expiry

**Given** an authenticated Plug client with an expired session
**When** a command returns error code `9999` (or transport-equivalent session expiry)
**Then** the client automatically re-authenticates once before surfacing the failure to the caller per NFR-3

### AC-4: Re-auth failure preserves original error context

**Given** an authenticated Plug client with an expired session
**When** a command returns session-expiry error and re-authentication also fails
**Then** the original command's error is returned with context (wrapped with re-auth failure detail)

### AC-5: Concurrent session expiry — single re-auth, all retry

**Given** three goroutines simultaneously hitting session expiry
**When** all three detect error `9999` concurrently
**Then** exactly one goroutine performs re-auth while the other two block on the mutex
**And** after re-auth completes, all three retry with the fresh session

---

## Tasks / Subtasks

### Task 1: Add Plug-level re-auth mutex

- [ ] Add a `sync.Mutex` field to the `Plug` struct dedicated to gating re-authentication (separate from any transport-level mutex)
- [ ] Add a session generation counter or timestamp field to detect whether another goroutine has already re-authed since the current goroutine's command failed

### Task 2: Define session-expiry error type

- [ ] In `internal/transport/transport.go` (or appropriate location), define a way for `Transport.Send` to surface session-expiry errors distinctly from other errors
- [ ] Option A: Define a sentinel `ErrSessionExpired` in the transport package
- [ ] Option B: Define an `IsSessionExpired(error) bool` helper function
- [ ] Ensure KLAP transport's `Send` method returns this distinct error when the device responds with error code `9999`

### Task 3: Implement re-auth logic in Plug command methods

- [ ] Create an internal `execute` (or similar) helper on `Plug` that wraps `Transport.Send` with re-auth retry logic
- [ ] On session-expiry error from `Send`:
  1. Acquire the re-auth mutex
  2. Check if another goroutine already re-authed (compare session generation counter)
  3. If session is still stale, call `Transport.Login` to re-authenticate
  4. Increment the session generation counter
  5. Release the re-auth mutex
  6. Retry the original command exactly once
  7. If the retry also fails, return the error to the caller with context
- [ ] If `Transport.Login` itself fails during re-auth, return the original command error wrapped with the re-auth failure
- [ ] Wire `TurnOn`, `TurnOff`, `Toggle`, and `DeviceInfo` through this helper

### Task 4: Race-condition tests

- [ ] Write a test that spawns N goroutines (e.g., 10+) calling `TurnOn`, `TurnOff`, `Toggle`, and `DeviceInfo` concurrently on a single Plug with a mock transport
- [ ] Ensure the test passes under `go test -race` with zero data race reports
- [ ] Test both the happy path (no session expiry) and contended path (intermittent session expiry)

### Task 5: Concurrent session-expiry test

- [ ] Create a mock transport that returns error `9999` on the first `Send` call from each goroutine, then succeeds after a `Login` call
- [ ] Spawn 3+ goroutines that all hit the session-expiry error concurrently
- [ ] Assert that `Login` is called exactly once (not once per goroutine)
- [ ] Assert that all goroutines eventually succeed after the single re-auth
- [ ] Use an atomic counter or similar mechanism in the mock to verify the single-login invariant

### Task 6: Re-auth failure test

- [ ] Create a mock transport where `Send` returns error `9999` and `Login` also fails
- [ ] Assert that the returned error wraps both the original command context and the re-auth failure
- [ ] Verify the error is inspectable with `errors.Is` against the appropriate sentinel (e.g., `ErrAuth`)

---

## Dev Notes

### AD-5: Two-layer mutex architecture

This story completes the second mutex layer defined in AD-5. The first layer already exists:

1. **Transport-level mutex (already done in Story 1.2):** Each `Transport` implementation (e.g., KLAP) guards its internal session state (cookies, AES keys, sequence counter) with its own `sync.Mutex`. This prevents concurrent `Send` calls from corrupting transport-internal state.

2. **Plug-level mutex (this story):** `Plug` holds a separate `sync.Mutex` that gates re-authentication. When a `Send` call returns a session-expiry error, only one goroutine acquires this mutex and calls `Transport.Login`. The other goroutines block until re-auth completes, then all retry with the fresh session.

These two mutexes serve different purposes and must not be collapsed into one. The transport mutex protects per-request encryption state; the Plug mutex coordinates the re-auth decision across goroutines.

### NFR-3: Re-auth semantics

Per NFR-3, automatic re-login on session expiry happens **at most once per command**. The flow is:

1. `Plug.execute(method, payload)` calls `Transport.Send`
2. `Send` returns a session-expiry error (error code `9999`)
3. `Plug` acquires re-auth mutex, checks session freshness, calls `Login` if needed
4. `Plug` retries `Send` exactly once
5. If the retry fails, the error surfaces to the caller — no further retries

This prevents infinite re-auth loops if the device is in a bad state.

### Error code 9999

The Tapo protocol uses error code `9999` to indicate session expiry. Both KLAP and legacy transports should surface this as a distinct, detectable error from `Send`. The recommended approach is a package-level sentinel or a typed error in `internal/transport` that `Plug` can check with `errors.Is`.

### Session generation counter pattern

To avoid redundant re-auths when multiple goroutines hit session expiry simultaneously, use a generation counter:

```go
type Plug struct {
    // ...
    authMu     sync.Mutex
    authGen    uint64  // incremented on each successful re-auth
    // ...
}
```

When a goroutine detects session expiry, it snapshots the current `authGen`, acquires `authMu`, then checks if `authGen` has changed. If it has, another goroutine already re-authed — skip `Login` and go straight to retry.

### No golang.org/x/sync dependency

Per AR-9 (zero external dependencies), use only `sync.Mutex` from the standard library. Do not introduce `golang.org/x/sync/singleflight` or similar.

### What exists from prior stories

From **Story 1.2** (KLAP Transport):
- `internal/transport/transport.go` — `Transport` interface with `Login` and `Send` methods
- `internal/transport/klap/klap.go` — KLAP implementation with its own `sync.Mutex`

From **Story 1.3** (Plug Client):
- `plug.go` — `Plug` struct, `NewPlug`, `NewPlugFromEnv`, `DeviceInfo` method
- `errors.go` — `ErrAuth`, `ErrTimeout`, `ErrUnsupportedModel`, `ErrHandshake` sentinels
- `options.go` — `Option` type, `WithTimeout`

From **Story 1.4** (Power Control):
- `plug.go` — `TurnOn`, `TurnOff`, `Toggle` methods on `Plug`

---

## Project Structure Notes

### Files to create or modify

| File | Action | Description |
|------|--------|-------------|
| `plug.go` | Modify | Add `authMu sync.Mutex` and `authGen uint64` fields to `Plug` struct. Add `execute` helper method with re-auth retry logic. Wire `TurnOn`, `TurnOff`, `Toggle`, `DeviceInfo` through `execute`. |
| `internal/transport/transport.go` | Modify | Add session-expiry error detection — either a sentinel `ErrSessionExpired` or an `IsSessionExpired(error) bool` helper. |
| `internal/transport/klap/klap.go` | Modify | Update `Send` to return the session-expiry error when the device responds with error code `9999`. |
| `plug_test.go` | Modify | Add race-condition tests (concurrent goroutines calling all methods), concurrent session-expiry test (single re-auth verification), and re-auth failure test. |

### Files NOT modified

| File | Reason |
|------|--------|
| `errors.go` | No new public sentinels needed. Session-expiry is an internal concern, not a caller-facing error category. |
| `options.go` | No new options for this story. |
| `device_info.go` | No changes to DeviceInfo struct or decoding. |
| `internal/crypto/*` | No crypto changes needed for concurrency. |

---

## References

- **AD-5 (Mutex-based concurrency):** `tapo-architecture.md` — Two mutex layers: transport-level and Plug-level
- **NFR-3 (Auto re-login):** `tapo-spec.md` — Automatic re-login on session expiry at most once per command
- **FR-10 (Goroutine safety):** `tapo-prd.md` — Concurrent calls do not trigger race detector; session serialized internally
- **CAP-2 (Authenticate):** `tapo-spec.md` — Session expiry (error code `9999`) triggers exactly one automatic re-authentication
- **CAP-10 (Goroutine-safe client):** `tapo-spec.md` — Concurrent calls from separate goroutines, no race detector triggers
- **AR-9 (Zero external deps):** `tapo-architecture.md` — stdlib only; no `golang.org/x/sync`
- **Story 1.2 (KLAP Transport):** Transport interface and KLAP implementation with transport-level mutex
- **Story 1.3 (Plug Client):** Plug struct, constructors, DeviceInfo, sentinel errors
- **Story 1.4 (Power Control):** TurnOn, TurnOff, Toggle methods

---

## Dev Agent Record

<!-- Record of dev agent interactions during implementation -->
<!-- Format: date | agent | action | outcome -->
