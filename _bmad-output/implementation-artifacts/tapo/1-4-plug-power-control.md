# Story 1.4: Plug Power Control

> **Epic:** 1 - Core P100 Control Library
> **Status:** ready-for-dev
> **Depends on:** Story 1.3 (Plug Client Construction & DeviceInfo)
> **Blocked by:** None (assuming 1.3 is complete)

---

## Story

As a Go developer,
I want to turn my plug on, off, or toggle its state,
So that I can control devices programmatically in scripts and automation.

---

## Acceptance Criteria

### AC-1: TurnOn Sends set_device_info with device_on true
**Given** a constructed Plug client from Story 1.3
**When** a developer calls `plug.TurnOn(ctx)`
**Then** the client sends `set_device_info` with `{"device_on": true}` via the transport
**And** the call returns `nil` error on success
**And** the relay state is verifiable as on via a subsequent `DeviceInfo` call

### AC-2: TurnOff Sends set_device_info with device_on false
**Given** a constructed Plug client
**When** a developer calls `plug.TurnOff(ctx)`
**Then** the client sends `set_device_info` with `{"device_on": false}` via the transport
**And** error behavior is symmetric with `TurnOn`

### AC-3: Toggle Reads State Then Inverts
**Given** a constructed Plug client with the plug currently on
**When** a developer calls `plug.Toggle(ctx)`
**Then** the client reads current state via `get_device_info`, then sets the inverse via `set_device_info`
**And** the plug state changes from on to off (or off to on)

### AC-4: Toggle Fails Safely When State Unreadable
**Given** a constructed Plug client where the device is unreachable
**When** `Toggle` cannot read the current state
**Then** it returns an error without guessing the state
**And** no `set_device_info` call is made

### AC-5: ErrUnsupportedModel Warning on Non-P100 Device
**Given** a non-P100 device that accepts the command
**When** `TurnOn`, `TurnOff`, or `Toggle` is called
**Then** the command succeeds (the device state changes) and the returned error wraps `ErrUnsupportedModel` (detectable via `errors.Is`) per FR-8
**And** the caller can distinguish success-with-warning from actual failure

### AC-6: Error on Auth Failure, Unreachable Host, or Device Rejection
**Given** a Plug client that is not authenticated, whose host is unreachable, or whose device rejects the command
**When** `TurnOn`, `TurnOff`, or `Toggle` is called
**Then** the call returns a descriptive error wrapping the appropriate sentinel (`ErrAuth`, `ErrTimeout`, `ErrHandshake`)

---

## Tasks / Subtasks

### Task 1: Implement setDeviceOn Internal Helper [AC-1, AC-2, AC-5, AC-6]

- [ ] **1.1** Add an unexported method to `Plug` in `plug.go`:
  ```go
  func (p *Plug) setDeviceOn(ctx context.Context, on bool) error
  ```
  - Builds the `set_device_info` command JSON:
    ```json
    {"device_on": true}
    ```
    or `{"device_on": false}` depending on the `on` parameter
  - Calls `p.send(ctx, "set_device_info", payload)` -- reuses the existing `send` helper that handles lazy login, context propagation, and Transport.Send
  - On success, checks `p.model` (or the cached model from a prior DeviceInfo/login): if model is not `"P100"` and not empty, returns `fmt.Errorf("tapo: set_device_info: device model %q may not be fully supported: %w", model, ErrUnsupportedModel)`
  - On transport or network error, returns the error from `send` (which already wraps sentinels appropriately)

- [ ] **1.2** Define the params struct (unexported):
  ```go
  type setDeviceInfoParams struct {
      DeviceOn bool `json:"device_on"`
  }
  ```
  Use `json.Marshal` to serialize this struct as the payload for `send`

### Task 2: Implement TurnOn [AC-1]

- [ ] **2.1** Add the exported method to `Plug` in `plug.go`:
  ```go
  func (p *Plug) TurnOn(ctx context.Context) error {
      return p.setDeviceOn(ctx, true)
  }
  ```
  - Thin wrapper that delegates to `setDeviceOn(ctx, true)`
  - No additional logic needed -- error wrapping, lazy login, and model check are all in `setDeviceOn`

### Task 3: Implement TurnOff [AC-2]

- [ ] **3.1** Add the exported method to `Plug` in `plug.go`:
  ```go
  func (p *Plug) TurnOff(ctx context.Context) error {
      return p.setDeviceOn(ctx, false)
  }
  ```
  - Thin wrapper that delegates to `setDeviceOn(ctx, false)`
  - Symmetric with `TurnOn` in behavior and error handling

### Task 4: Implement Toggle [AC-3, AC-4, AC-5]

- [ ] **4.1** Add the exported method to `Plug` in `plug.go`:
  ```go
  func (p *Plug) Toggle(ctx context.Context) error
  ```
  - Step 1: Call `p.DeviceInfo(ctx)` to read the current device state
  - Step 2: If `DeviceInfo` returns an error that is NOT `ErrUnsupportedModel`, return the error immediately (AC-4: do not guess state)
  - Step 3: If `DeviceInfo` returns `ErrUnsupportedModel` (warning), the `DeviceInfo` result is still populated -- proceed with the toggle
  - Step 4: Call `p.setDeviceOn(ctx, !info.DeviceOn)` to set the inverse state
  - Step 5: Return the result from `setDeviceOn` (which may include its own `ErrUnsupportedModel` warning)

- [ ] **4.2** Handle the `ErrUnsupportedModel` edge case in Toggle correctly:
  - `DeviceInfo` may return `(info, ErrUnsupportedModel)` for non-P100 devices
  - `setDeviceOn` will also return `ErrUnsupportedModel` for non-P100 devices
  - Toggle should return the `ErrUnsupportedModel` from `setDeviceOn` (the write operation), since the caller cares about the outcome of the state change
  - Pattern:
    ```go
    info, err := p.DeviceInfo(ctx)
    if err != nil && !errors.Is(err, ErrUnsupportedModel) {
        return fmt.Errorf("tapo: toggle: %w", err)
    }
    return p.setDeviceOn(ctx, !info.DeviceOn)
    ```

### Task 5: Unit Tests -- TurnOn and TurnOff [AC-1, AC-2, AC-6]

- [ ] **5.1** Add tests in `plug_test.go` (or a new test file if `plug_test.go` already exists from Story 1.3)

- [ ] **5.2** Create a mock transport that implements `transport.Transport` for testing:
  - Records the method and payload of each `Send` call
  - Returns configurable responses and errors
  - This mock may already exist from Story 1.3 tests -- reuse it if so

- [ ] **5.3** Test: **TurnOn sends correct command**
  - Create a `Plug` with the mock transport
  - Call `TurnOn(ctx)`
  - Assert mock received `Send` with method `"set_device_info"` and payload containing `"device_on":true`
  - Assert no error returned

- [ ] **5.4** Test: **TurnOff sends correct command**
  - Create a `Plug` with the mock transport
  - Call `TurnOff(ctx)`
  - Assert mock received `Send` with method `"set_device_info"` and payload containing `"device_on":false`
  - Assert no error returned

- [ ] **5.5** Test: **TurnOn returns transport error**
  - Configure mock to return an error wrapping `ErrAuth`
  - Call `TurnOn(ctx)` and assert `errors.Is(err, ErrAuth)` is true

- [ ] **5.6** Test: **TurnOff returns transport error**
  - Configure mock to return an error wrapping `ErrTimeout`
  - Call `TurnOff(ctx)` and assert `errors.Is(err, ErrTimeout)` is true

- [ ] **5.7** Test: **TurnOn error symmetry with TurnOff**
  - Verify both methods produce structurally identical errors for the same failure modes (auth, timeout, handshake)

### Task 6: Unit Tests -- Toggle [AC-3, AC-4, AC-5]

- [ ] **6.1** Test: **Toggle from on to off**
  - Configure mock to return `get_device_info` response with `"device_on": true`
  - Call `Toggle(ctx)`
  - Assert mock received `Send` with `"set_device_info"` and `"device_on": false`
  - Assert no error returned

- [ ] **6.2** Test: **Toggle from off to on**
  - Configure mock to return `get_device_info` response with `"device_on": false`
  - Call `Toggle(ctx)`
  - Assert mock received `Send` with `"set_device_info"` and `"device_on": true`
  - Assert no error returned

- [ ] **6.3** Test: **Toggle fails when DeviceInfo fails**
  - Configure mock to return an error on `get_device_info`
  - Call `Toggle(ctx)`
  - Assert error is returned and no `set_device_info` call was made (AC-4)

- [ ] **6.4** Test: **Toggle does not guess state on error**
  - Configure mock to return a transport error on the first `Send` (the `get_device_info` call)
  - Call `Toggle(ctx)`
  - Assert that `set_device_info` was never called
  - Assert the returned error wraps the original transport error

### Task 7: Unit Tests -- ErrUnsupportedModel Warning [AC-5]

- [ ] **7.1** Test: **TurnOn on non-P100 device returns ErrUnsupportedModel**
  - Configure mock to return a `get_device_info` response with `"model": "P110"` (or set the model on the Plug directly if there is a cached model field)
  - Call `TurnOn(ctx)` and assert `errors.Is(err, ErrUnsupportedModel)` is true
  - Assert the operation itself succeeded (mock confirms `set_device_info` was sent)

- [ ] **7.2** Test: **TurnOff on non-P100 device returns ErrUnsupportedModel**
  - Same setup as 7.1 but with `TurnOff`
  - Assert `errors.Is(err, ErrUnsupportedModel)` is true

- [ ] **7.3** Test: **Toggle on non-P100 device returns ErrUnsupportedModel**
  - Configure mock to return DeviceInfo with `"model": "P110"` and `"device_on": true`
  - Call `Toggle(ctx)`
  - Assert `errors.Is(err, ErrUnsupportedModel)` is true
  - Assert `set_device_info` was called with `"device_on": false`

- [ ] **7.4** Test: **TurnOn on P100 device returns nil error**
  - Configure mock to return a response with `"model": "P100"`
  - Call `TurnOn(ctx)` and assert `err == nil`

### Task 8: Run All Tests with Race Detector [AC-1 through AC-6]

- [ ] **8.1** Run `go test -race ./...` and confirm all tests pass with no race conditions
- [ ] **8.2** Run `go vet ./...` and confirm no warnings

---

## Dev Notes

### What Exists from Prior Stories

From Story 1.3, the `Plug` struct in `plug.go` already has:
- **`send` helper method:** Handles lazy login (triggers Transport.Login on first call), context propagation, and calls Transport.Send. TurnOn/TurnOff/Toggle should use this helper -- do NOT call Transport.Send directly.
- **`DeviceInfo` method:** Sends `get_device_info` and returns a populated `*DeviceInfo` struct with decoded base64 fields. Toggle reuses this to read current `DeviceOn` state.
- **`DeviceInfo` struct:** Contains `DeviceOn bool`, `Model string`, and other fields. The `DeviceOn` field is what Toggle reads and inverts.
- **Model check pattern:** `DeviceInfo` already implements the `ErrUnsupportedModel` warning when `Model != "P100"`. The same pattern must be applied in `setDeviceOn`.

From Story 1.2, the transport layer:
- **Transport interface:** `Login(ctx, email, password) error` and `Send(ctx, method, payload) (json.RawMessage, error)` in `internal/transport/transport.go`.
- **KLAP implementation:** Handles session management, encryption/decryption, sequence counting.

From Story 1.1, sentinel errors in `errors.go`:
- `ErrAuth`, `ErrTimeout`, `ErrUnsupportedModel`, `ErrHandshake`

### Command JSON Format

The `set_device_info` command sent via `Transport.Send`:

```json
{"method": "set_device_info", "params": {"device_on": true}}
```

The `Plug.send` helper already wraps the method and params into the Tapo command envelope (as established in Story 1.3 for `get_device_info`). The `TurnOn`/`TurnOff` methods only need to provide:
- **method:** `"set_device_info"`
- **payload:** `json.RawMessage` serialized from `{"device_on": true/false}`

The `send` helper constructs the full envelope. Verify this pattern by reading the existing `DeviceInfo` implementation to see how it calls `send`.

### ErrUnsupportedModel Warning Semantics (FR-8)

This is a WARNING, not a failure. The semantics:

1. The control command (`set_device_info`) is always attempted regardless of device model.
2. If the device accepts the command AND the device model is not `"P100"`, return an error wrapping `ErrUnsupportedModel`.
3. The caller can detect this with `errors.Is(err, ErrUnsupportedModel)`.
4. The caller knows the operation succeeded because `ErrUnsupportedModel` is defined to mean "operation completed but on an untested model."
5. If the command fails for transport/auth reasons, return that error instead -- `ErrUnsupportedModel` is only returned on success with a non-P100 model.

Implementation pattern:
```go
if err := p.send(ctx, "set_device_info", payload); err != nil {
    return fmt.Errorf("tapo: set_device_info: %w", err)
}
if p.model != "" && p.model != "P100" {
    return fmt.Errorf("tapo: set_device_info: device model %q may not be fully supported: %w", p.model, ErrUnsupportedModel)
}
return nil
```

The model value may come from a cached field on `Plug` (populated by a prior `DeviceInfo` call or login response). If the model is unknown (empty string), do not warn -- the device may be a P100 that hasn't been queried yet. Examine the existing `DeviceInfo` implementation to confirm how the model is cached.

### Error Wrapping Convention

Follow the established pattern from prior stories:
```go
fmt.Errorf("tapo: <method>: %w", err)
```

Examples:
```go
fmt.Errorf("tapo: set_device_info: %w", err)        // transport/auth error
fmt.Errorf("tapo: toggle: %w", err)                  // toggle-specific read failure
fmt.Errorf("tapo: set_device_info: device model %q may not be fully supported: %w", model, ErrUnsupportedModel)
```

### Toggle Implementation Detail

Toggle performs two transport operations:
1. `get_device_info` (via `DeviceInfo`) -- read current state
2. `set_device_info` (via `setDeviceOn`) -- write inverse state

These are NOT atomic. Between the read and write, the device state could change (e.g., physical button press, another client). This is acceptable for v1 -- the toggle is a convenience method, not a transactional operation.

If `DeviceInfo` returns `ErrUnsupportedModel`, the `DeviceInfo` struct is still populated (this is the warning semantic from Story 1.3). Toggle must handle this:
```go
info, err := p.DeviceInfo(ctx)
if err != nil && !errors.Is(err, ErrUnsupportedModel) {
    return fmt.Errorf("tapo: toggle: %w", err)
}
// info is populated even when err is ErrUnsupportedModel
return p.setDeviceOn(ctx, !info.DeviceOn)
```

### Testing Approach

- **Mock Transport:** Reuse or extend the mock transport from Story 1.3 tests. The mock should record each `Send` call's method and payload so tests can assert the correct command was sent.
- **No Real Hardware:** All tests use mocked transports. Integration tests against real P100 are deferred to Story 3.2.
- **Race Detection:** Run `go test -race ./...` -- the methods in this story are thin wrappers and do not introduce new concurrency concerns, but race detection confirms no regressions.
- **Use stdlib `testing` only.** No testify or external test frameworks.

### Implementation Checklist (Quick Reference)

1. Read the existing `plug.go` to understand the `send` helper signature and the model caching pattern
2. Add `setDeviceInfoParams` struct and `setDeviceOn` unexported method
3. Add `TurnOn` and `TurnOff` as thin wrappers
4. Add `Toggle` with DeviceInfo read + setDeviceOn write
5. Add/extend tests with mock transport
6. Run `go test -race ./...` and `go vet ./...`

---

## Project Structure Notes

### Files Modified in This Story

| File | Change |
|---|---|
| `plug.go` | Add `TurnOn`, `TurnOff`, `Toggle` exported methods and `setDeviceOn` unexported helper, `setDeviceInfoParams` struct |

### Files from Prior Stories (Already Exist -- Do Not Recreate)

| File | Contains | Story |
|---|---|---|
| `go.mod` | Module declaration `github.com/mjenh/tapo`, Go 1.24+ | 1.1 |
| `errors.go` | `ErrAuth`, `ErrTimeout`, `ErrUnsupportedModel`, `ErrHandshake` sentinels | 1.3 |
| `plug.go` | `Plug` struct, `NewPlug`, `NewPlugFromEnv`, `DeviceInfo`, `send` helper | 1.3 |
| `device_info.go` | `DeviceInfo` struct with JSON tags and base64 decoding | 1.3 |
| `options.go` | `Option` type, `WithTimeout` | 1.3 |
| `internal/transport/transport.go` | `Transport` interface (`Login`, `Send`) | 1.2 |
| `internal/transport/klap/klap.go` | KLAP transport implementation | 1.2 |
| `internal/crypto/crypto.go` | AES-CBC encrypt/decrypt | 1.1 |
| `internal/crypto/auth.go` | `KLAPAuthHash`, `KLAPDeriveKeyAndIV` | 1.1 |

### Test Files

| File | Purpose | New/Existing |
|---|---|---|
| `plug_test.go` | Tests for TurnOn, TurnOff, Toggle, ErrUnsupportedModel warning path | Existing (extend from Story 1.3) |

---

## References

- **Epics:** `_bmad-output/planning-artifacts/tapo/tapo-epics.md` (Story 1.4 definition, FR-4, FR-5, FR-6, FR-8)
- **Architecture:** `_bmad-output/planning-artifacts/tapo/tapo-architecture.md` (AD-3, AD-6, AD-8, Capability-Architecture Map for CAP-4/5/6/8)
- **PRD:** `_bmad-output/planning-artifacts/tapo/tapo-prd.md` (FR-4 Turn on, FR-5 Turn off, FR-6 Toggle, FR-8 ErrUnsupportedModel)
- **Spec:** `_bmad-output/planning-artifacts/tapo/tapo-spec.md` (CAP-4, CAP-5, CAP-6, CAP-8)
- **Addendum:** `_bmad-output/planning-artifacts/tapo/addendum.md` (set_device_info command format, DeviceInfo field mapping)
- **Glossary:** `_bmad-output/planning-artifacts/tapo/glossary.md`
- **Prior Stories:**
  - `_bmad-output/implementation-artifacts/tapo/1-1-module-scaffold-shared-crypto-helpers.md`
  - `_bmad-output/implementation-artifacts/tapo/1-2-klap-transport.md`
  - Story 1.3 (Plug Client & DeviceInfo) -- prerequisite, defines the Plug struct, send helper, and DeviceInfo method

---

## Dev Agent Record

### Completion Log

| Task | Status | Notes |
|---|---|---|
| Task 1: Implement setDeviceOn Internal Helper | not-started | |
| Task 2: Implement TurnOn | not-started | |
| Task 3: Implement TurnOff | not-started | |
| Task 4: Implement Toggle | not-started | |
| Task 5: Unit Tests -- TurnOn and TurnOff | not-started | |
| Task 6: Unit Tests -- Toggle | not-started | |
| Task 7: Unit Tests -- ErrUnsupportedModel Warning | not-started | |
| Task 8: Run All Tests with Race Detector | not-started | |

### Change Log

_No changes yet._

### Test Results

_No test runs yet._

### Notes

_No dev notes yet._
