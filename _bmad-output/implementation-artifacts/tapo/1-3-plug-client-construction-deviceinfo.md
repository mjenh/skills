# Story 1.3: Plug Client Construction & DeviceInfo

> **Epic:** 1 - Core P100 Control Library
> **Status:** done
> **Depends on:** Story 1.2 (KLAP Transport)
> **Blocked by:** None (assuming 1.1 and 1.2 are complete)

---

## Story

As a Go developer,
I want to construct a Plug client and retrieve device information,
So that I can verify connectivity and read my plug's state programmatically.

---

## Acceptance Criteria

### AC-1: NewPlug returns Plug on valid inputs

**Given** the KLAP transport from Story 1.2
**When** a developer calls `NewPlug(ctx, host, email, password)`
**Then** the function returns a `*Plug` and `nil` error when host and credentials are all non-empty

### AC-2: NewPlug rejects empty host or missing credentials

**Given** a call to `NewPlug`
**When** host is empty, or email is empty, or password is empty
**Then** the function returns a descriptive error identifying which field is missing
**And** `*Plug` is `nil`

### AC-3: NewPlug performs no network I/O (lazy negotiation)

**Given** a call to `NewPlug` with valid arguments
**When** the constructor returns
**Then** zero network requests have been issued (per AD-7)
**And** the KLAP transport's `Login` has not been called

### AC-4: NewPlugFromEnv reads environment variables

**Given** the environment variables `TAPO_HOST` (preferred) / `TAPO_IP` (alias), `TAPO_EMAIL`, and `TAPO_PASSWORD` are set
**When** a developer calls `NewPlugFromEnv(ctx)`
**Then** the function reads host from `TAPO_HOST` first; if absent, falls back to `TAPO_IP`
**And** constructs a `*Plug` using the resolved host, email, and password
**And** returns `nil` error on success

### AC-5: NewPlugFromEnv reports missing environment variables

**Given** one or more required environment variables are absent
**When** `NewPlugFromEnv(ctx)` is called
**Then** the returned error lists which required variables are missing
**And** `*Plug` is `nil`

### AC-6: Functional options (WithTimeout)

**Given** both `NewPlug` and `NewPlugFromEnv`
**When** called with variadic `Option` arguments
**Then** `WithTimeout(d time.Duration)` sets the per-request timeout (applied as context deadline on each command)
**And** the default timeout is 10 seconds when no `WithTimeout` option is provided

### AC-7: DeviceInfo triggers lazy login and returns device data

**Given** a constructed `Plug` that has not yet authenticated
**When** `plug.DeviceInfo(ctx)` is called for the first time
**Then** the KLAP transport's `Login` is invoked (lazy authentication)
**And** the transport sends `get_device_info`
**And** the response is deserialized into a `*DeviceInfo` struct
**And** `*DeviceInfo` is returned with `nil` error on success

### AC-8: DeviceInfo struct contains required fields

**Given** a successful `DeviceInfo` call
**When** the `*DeviceInfo` struct is inspected
**Then** it contains at minimum: `DeviceOn` (bool), `Model` (string), `Nickname` (string), `DeviceID` (string), `FirmwareVersion` (string), `HardwareVersion` (string), `IPAddress` (string), `MAC` (string)
**And** each field is tagged with the corresponding Tapo JSON key

### AC-9: Base64-encoded fields decoded before return

**Given** the device returns base64-encoded strings for `Nickname` and `SSID`
**When** `DeviceInfo` deserializes the response
**Then** `Nickname` and `SSID` are decoded from base64 to plain UTF-8 before populating the struct (per AD-8)
**And** invalid base64 does not cause a hard error (the raw value is preserved)

### AC-10: ErrUnsupportedModel on non-P100 device

**Given** a successful `DeviceInfo` call where the device reports `Model != "P100"`
**When** the response is processed
**Then** the returned error wraps `ErrUnsupportedModel` (detectable via `errors.Is`)
**And** `*DeviceInfo` is still fully populated and returned alongside the error (per FR-8)

### AC-11: Sentinel errors exported from errors.go

**Given** the root `tapo` package
**When** `errors.go` is inspected
**Then** it exports four sentinel error variables: `ErrAuth`, `ErrTimeout`, `ErrUnsupportedModel`, `ErrHandshake`
**And** each is a distinct `errors.New` value usable with `errors.Is`

---

## Tasks / Subtasks

### Task 1: Implement errors.go -- Sentinel Errors [AC-11]

File: `errors.go`

- [ ] **1.1** Verify `errors.go` exists (created as placeholder in Story 1.1, may have sentinels from Story 1.2). If sentinels are already defined from Story 1.2, confirm all four are present and correctly defined. Otherwise, implement:
  ```go
  package tapo

  import "errors"

  var (
      // ErrAuth is returned when authentication with the Tapo device fails
      // due to invalid credentials.
      ErrAuth = errors.New("tapo: authentication failed")

      // ErrTimeout is returned when an operation exceeds its deadline.
      ErrTimeout = errors.New("tapo: operation timed out")

      // ErrUnsupportedModel is returned as a warning when the connected device
      // reports a model other than P100. The operation result is still valid.
      ErrUnsupportedModel = errors.New("tapo: unsupported device model")

      // ErrHandshake is returned when the transport handshake with the device
      // fails for non-credential reasons (network, protocol mismatch).
      ErrHandshake = errors.New("tapo: handshake failed")
  )
  ```
- [ ] **1.2** Verify all four sentinels are distinct and each is usable with `errors.Is`

### Task 2: Implement options.go -- Functional Options [AC-6]

File: `options.go`

- [ ] **2.1** Define the unexported config struct and `Option` type:
  ```go
  package tapo

  import "time"

  // config holds configuration applied via functional options.
  type config struct {
      timeout   time.Duration
      transport interface{} // internal transport.Transport; typed as interface{} to avoid exporting internal types (AD-1)
  }

  // defaultConfig returns the configuration with default values.
  func defaultConfig() config {
      return config{
          timeout: 10 * time.Second,
      }
  }

  // Option configures a Plug client.
  type Option func(*config)
  ```

- [ ] **2.2** Implement `WithTimeout`:
  ```go
  // WithTimeout sets the per-request timeout for all commands issued by the Plug.
  // The default is 10 seconds. The timeout is applied as a context deadline
  // on each individual command (Login, Send), not as an overall client timeout.
  func WithTimeout(d time.Duration) Option {
      return func(c *config) {
          c.timeout = d
      }
  }
  ```

- [ ] **2.3** Implement `WithTransport` (internal use, for testing and future Story 2.2):
  ```go
  // withTransport injects a specific transport implementation, bypassing
  // the default KLAP transport. Unexported because the transport.Transport
  // interface is internal (AD-1). Used by tests and by NegotiatingTransport
  // in Story 2.2.
  func withTransport(t interface{}) Option {
      return func(c *config) {
          c.transport = t
      }
  }
  ```
  Note: `withTransport` is unexported because `transport.Transport` is an internal type that must not appear in the public API per AD-1. It will be exposed indirectly via `WithTransport("klap"|"legacy")` in Story 2.2.

### Task 3: Implement plug.go -- Plug Type and Constructors [AC-1, AC-2, AC-3, AC-4, AC-5, AC-6]

File: `plug.go`

- [ ] **3.1** Define the `Plug` struct:
  ```go
  package tapo

  import (
      "context"
      "sync"

      "github.com/mjenh/tapo/internal/transport"
  )

  // Plug is a client for a single Tapo smart plug. It is safe for concurrent
  // use from multiple goroutines (AD-5). Create one with NewPlug or NewPlugFromEnv.
  type Plug struct {
      host      string
      email     string
      password  string
      cfg       config
      transport transport.Transport

      mu       sync.Mutex // guards loggedIn and re-auth serialization
      loggedIn bool
  }
  ```

- [ ] **3.2** Implement `NewPlug`:
  ```go
  // NewPlug creates a new Plug client for the device at the given host.
  // host is the IP address (and optional port) of the Tapo device on the LAN.
  // email and password are the Tapo account credentials used by the mobile app.
  //
  // NewPlug validates that host, email, and password are non-empty but performs
  // no network I/O (AD-7). Authentication occurs lazily on the first command.
  //
  // Use functional options to override defaults:
  //
  //   plug, err := tapo.NewPlug(ctx, "192.168.1.42", email, pass, tapo.WithTimeout(5*time.Second))
  func NewPlug(ctx context.Context, host, email, password string, opts ...Option) (*Plug, error)
  ```
  - Validate `host` is non-empty; return `fmt.Errorf("tapo: host must not be empty")` if empty
  - Validate `email` is non-empty; return `fmt.Errorf("tapo: email must not be empty")` if empty
  - Validate `password` is non-empty; return `fmt.Errorf("tapo: password must not be empty")` if empty
  - Apply `defaultConfig()`, then apply each `Option`
  - If `cfg.transport` is nil, create a new `klap.New(host, &http.Client{})` transport
  - Store host, email, password (private fields; never logged per NFR-4), config, and transport
  - Return `*Plug` with `loggedIn = false` (no network I/O)

- [ ] **3.3** Implement `NewPlugFromEnv`:
  ```go
  // NewPlugFromEnv creates a new Plug client using environment variables.
  //
  // Required variables:
  //   - TAPO_HOST (preferred) or TAPO_IP (alias): device IP address
  //   - TAPO_EMAIL: Tapo account email
  //   - TAPO_PASSWORD: Tapo account password
  //
  // Returns an error listing all missing variables.
  func NewPlugFromEnv(ctx context.Context, opts ...Option) (*Plug, error)
  ```
  - Read `TAPO_HOST` from `os.Getenv`; if empty, fall back to `TAPO_IP`
  - Read `TAPO_EMAIL` from `os.Getenv`
  - Read `TAPO_PASSWORD` from `os.Getenv`
  - Collect names of all missing/empty variables into a slice
  - If any are missing, return `fmt.Errorf("tapo: missing required environment variables: %s", strings.Join(missing, ", "))`
  - Otherwise, delegate to `NewPlug(ctx, host, email, password, opts...)`

- [ ] **3.4** Implement the `login` helper (unexported, lazy authentication):
  ```go
  // login ensures the transport is authenticated. Called before every command.
  // Uses the Plug-level mutex to serialize authentication attempts (AD-5).
  func (p *Plug) login(ctx context.Context) error
  ```
  - Acquire `p.mu`; if `p.loggedIn`, release mutex and return nil
  - Release mutex before calling `p.transport.Login(ctx, p.email, p.password)` (never hold mutex during I/O)
  - On success, acquire mutex, set `p.loggedIn = true`, release mutex
  - On failure, return the error from `Login` (which wraps `ErrAuth` or `ErrHandshake`)

- [ ] **3.5** Implement `applyTimeout` helper:
  ```go
  // applyTimeout wraps the given context with the configured per-request timeout.
  func (p *Plug) applyTimeout(ctx context.Context) (context.Context, context.CancelFunc)
  ```
  - If `p.cfg.timeout > 0`, return `context.WithTimeout(ctx, p.cfg.timeout)`
  - Otherwise, return `ctx` and a no-op cancel func

### Task 4: Implement device_info.go -- DeviceInfo Struct and Decoding [AC-7, AC-8, AC-9, AC-10]

File: `device_info.go`

- [ ] **4.1** Define the `DeviceInfo` struct with JSON tags matching the Tapo wire format:
  ```go
  package tapo

  // DeviceInfo contains the device state and metadata returned by a Tapo plug.
  // Base64-encoded fields (Nickname, SSID) are automatically decoded to plain
  // UTF-8 strings.
  type DeviceInfo struct {
      DeviceOn        bool   `json:"device_on"`
      Model           string `json:"model"`
      Nickname        string `json:"nickname"`
      DeviceID        string `json:"device_id"`
      FirmwareVersion string `json:"fw_ver"`
      HardwareVersion string `json:"hw_ver"`
      IPAddress       string `json:"ip"`
      MAC             string `json:"mac"`
      SSID            string `json:"ssid"`
  }
  ```

- [ ] **4.2** Define the unexported wire struct for initial JSON unmarshaling (before base64 decode):
  ```go
  // deviceInfoWire mirrors DeviceInfo but keeps base64-encoded fields as raw
  // strings for two-pass decoding.
  type deviceInfoWire struct {
      DeviceOn        bool   `json:"device_on"`
      Model           string `json:"model"`
      Nickname        string `json:"nickname"`
      DeviceID        string `json:"device_id"`
      FirmwareVersion string `json:"fw_ver"`
      HardwareVersion string `json:"hw_ver"`
      IPAddress       string `json:"ip"`
      MAC             string `json:"mac"`
      SSID            string `json:"ssid"`
  }
  ```

- [ ] **4.3** Implement the `parseDeviceInfo` function:
  ```go
  // parseDeviceInfo deserializes the JSON response from get_device_info into
  // a DeviceInfo struct, decoding base64 fields (Nickname, SSID) in one pass
  // per AD-8. Returns a populated *DeviceInfo even when the model is not P100
  // (in that case, the returned error wraps ErrUnsupportedModel per FR-8).
  func parseDeviceInfo(data []byte) (*DeviceInfo, error)
  ```
  - Unmarshal `data` into `deviceInfoWire`
  - Decode `wire.Nickname` from standard base64 to UTF-8; if decoding fails, preserve the raw value (do not return an error)
  - Decode `wire.SSID` from standard base64 to UTF-8; if decoding fails, preserve the raw value
  - Copy all fields into a `*DeviceInfo`
  - If `info.Model != "P100"`, return `info, fmt.Errorf("tapo: device model %q is not P100: %w", info.Model, ErrUnsupportedModel)`
  - Otherwise, return `info, nil`

### Task 5: Implement DeviceInfo Method on Plug [AC-7, AC-10]

File: `plug.go`

- [ ] **5.1** Implement `DeviceInfo`:
  ```go
  // DeviceInfo retrieves the current state and metadata of the plug.
  // On the first call, it triggers KLAP authentication (lazy login per AD-7).
  //
  // When the device reports a model other than P100, the returned error wraps
  // ErrUnsupportedModel but the DeviceInfo is still fully populated (FR-8).
  func (p *Plug) DeviceInfo(ctx context.Context) (*DeviceInfo, error)
  ```
  - Apply timeout: `ctx, cancel := p.applyTimeout(ctx); defer cancel()`
  - Call `p.login(ctx)` to ensure authentication; return error if login fails
  - Call `p.transport.Send(ctx, "get_device_info", nil)` to send the command
  - Pass the response to `parseDeviceInfo(result)` to deserialize and decode
  - Return the `*DeviceInfo` and any error (including `ErrUnsupportedModel` warnings)

### Task 6: Implement doc.go -- Package Documentation [N/A]

File: `doc.go`

- [ ] **6.1** Verify `doc.go` exists with proper package documentation (created in Story 1.1). If it is a bare placeholder, update it:
  ```go
  // Package tapo provides a Go client for controlling Tapo P100 smart plugs
  // over the local network.
  //
  // Construct a client with NewPlug or NewPlugFromEnv, then call DeviceInfo,
  // TurnOn, TurnOff, or Toggle to interact with the device. The client
  // authenticates lazily on the first command and is safe for concurrent use.
  //
  // All errors are inspectable with errors.Is using the package sentinels:
  // ErrAuth, ErrTimeout, ErrUnsupportedModel, and ErrHandshake.
  package tapo
  ```

### Task 7: Unit Tests -- Sentinel Errors [AC-11]

File: `errors_test.go`

- [ ] **7.1** Test that all four sentinels are distinct values:
  - Assert `ErrAuth != ErrTimeout`, `ErrAuth != ErrUnsupportedModel`, `ErrAuth != ErrHandshake`, etc.
- [ ] **7.2** Test that wrapped errors are detectable via `errors.Is`:
  - Wrap each sentinel with `fmt.Errorf("context: %w", sentinel)`
  - Assert `errors.Is(wrapped, sentinel)` returns `true`
- [ ] **7.3** Test that `errors.Is` does not cross-match between sentinels:
  - Assert `errors.Is(wrappedAuth, ErrTimeout)` returns `false`

### Task 8: Unit Tests -- NewPlug [AC-1, AC-2, AC-3]

File: `plug_test.go`

- [ ] **8.1** Test: **NewPlug succeeds with valid inputs**
  - Call `NewPlug(ctx, "192.168.1.1", "test@example.com", "password")`
  - Assert `err == nil` and `plug != nil`

- [ ] **8.2** Test: **NewPlug rejects empty host**
  - Call `NewPlug(ctx, "", "test@example.com", "password")`
  - Assert `err != nil` and error message mentions "host"
  - Assert `plug == nil`

- [ ] **8.3** Test: **NewPlug rejects empty email**
  - Call `NewPlug(ctx, "192.168.1.1", "", "password")`
  - Assert `err != nil` and error message mentions "email"
  - Assert `plug == nil`

- [ ] **8.4** Test: **NewPlug rejects empty password**
  - Call `NewPlug(ctx, "192.168.1.1", "test@example.com", "")`
  - Assert `err != nil` and error message mentions "password"
  - Assert `plug == nil`

- [ ] **8.5** Test: **NewPlug performs no network I/O**
  - Use a mock transport (via `withTransport`) that records calls
  - After `NewPlug` returns, assert `Login` has not been called
  - Assert `Send` has not been called

- [ ] **8.6** Test: **NewPlug applies WithTimeout option**
  - Call `NewPlug(ctx, host, email, pass, WithTimeout(5*time.Second))`
  - Assert the configured timeout is 5 seconds (verify via test behavior or internal inspection)

- [ ] **8.7** Test: **NewPlug uses default timeout of 10s**
  - Call `NewPlug(ctx, host, email, pass)` without options
  - Assert the configured timeout is 10 seconds

### Task 9: Unit Tests -- NewPlugFromEnv [AC-4, AC-5]

File: `plug_test.go` (same file)

- [ ] **9.1** Test: **NewPlugFromEnv succeeds with TAPO_HOST**
  - Set `TAPO_HOST=192.168.1.1`, `TAPO_EMAIL=test@example.com`, `TAPO_PASSWORD=password`
  - Call `NewPlugFromEnv(ctx)` and assert success
  - Use `t.Setenv` for automatic cleanup

- [ ] **9.2** Test: **NewPlugFromEnv falls back to TAPO_IP**
  - Unset `TAPO_HOST`, set `TAPO_IP=10.0.0.1`, `TAPO_EMAIL=test@example.com`, `TAPO_PASSWORD=password`
  - Call `NewPlugFromEnv(ctx)` and assert success

- [ ] **9.3** Test: **TAPO_HOST takes precedence over TAPO_IP**
  - Set `TAPO_HOST=192.168.1.1`, `TAPO_IP=10.0.0.1`
  - Call `NewPlugFromEnv(ctx)` and verify the host used is `192.168.1.1`

- [ ] **9.4** Test: **NewPlugFromEnv reports missing TAPO_HOST and TAPO_IP**
  - Unset both `TAPO_HOST` and `TAPO_IP`, set `TAPO_EMAIL` and `TAPO_PASSWORD`
  - Assert error message mentions `TAPO_HOST`

- [ ] **9.5** Test: **NewPlugFromEnv reports missing TAPO_EMAIL**
  - Set `TAPO_HOST` and `TAPO_PASSWORD`, unset `TAPO_EMAIL`
  - Assert error message mentions `TAPO_EMAIL`

- [ ] **9.6** Test: **NewPlugFromEnv reports missing TAPO_PASSWORD**
  - Set `TAPO_HOST` and `TAPO_EMAIL`, unset `TAPO_PASSWORD`
  - Assert error message mentions `TAPO_PASSWORD`

- [ ] **9.7** Test: **NewPlugFromEnv reports all missing variables at once**
  - Unset all env vars
  - Assert the error message lists all missing variables (host, email, password)

- [ ] **9.8** Test: **NewPlugFromEnv accepts variadic options**
  - Call `NewPlugFromEnv(ctx, WithTimeout(3*time.Second))` with valid env vars
  - Assert the timeout option is applied

### Task 10: Unit Tests -- DeviceInfo [AC-7, AC-8, AC-9, AC-10]

File: `plug_test.go` (same file) or `device_info_test.go`

- [ ] **10.1** Create a mock transport for testing:
  ```go
  type mockTransport struct {
      loginCalled bool
      loginErr    error
      sendResult  json.RawMessage
      sendErr     error
      sendMethod  string
  }
  ```
  - Implements `transport.Transport` interface
  - Records whether `Login` and `Send` are called, and what arguments are passed

- [ ] **10.2** Test: **DeviceInfo triggers login on first call**
  - Construct a Plug with mock transport (via `withTransport`)
  - Call `DeviceInfo(ctx)`
  - Assert `mockTransport.loginCalled == true`

- [ ] **10.3** Test: **DeviceInfo sends get_device_info**
  - Configure mock transport to return valid DeviceInfo JSON
  - Call `DeviceInfo(ctx)`
  - Assert `mockTransport.sendMethod == "get_device_info"`

- [ ] **10.4** Test: **DeviceInfo returns populated struct**
  - Mock transport returns:
    ```json
    {
      "device_on": true,
      "model": "P100",
      "nickname": "TXkgUGx1Zw==",
      "device_id": "abc123",
      "fw_ver": "1.4.4 Build 20240514 Rel 35017",
      "hw_ver": "1.0",
      "ip": "192.168.1.42",
      "mac": "AA:BB:CC:DD:EE:FF",
      "ssid": "TXlXaUZp"
    }
    ```
  - Assert `DeviceOn == true`, `Model == "P100"`, `Nickname == "My Plug"` (decoded from base64)
  - Assert `SSID == "MyWiFi"` (decoded from base64)
  - Assert `FirmwareVersion == "1.4.4 Build 20240514 Rel 35017"`
  - Assert all other fields match expected values

- [ ] **10.5** Test: **DeviceInfo decodes base64 Nickname**
  - Mock transport returns `"nickname": "TXkgUGx1Zw=="` (base64 of "My Plug")
  - Assert `info.Nickname == "My Plug"`

- [ ] **10.6** Test: **DeviceInfo decodes base64 SSID**
  - Mock transport returns `"ssid": "TXlXaUZp"` (base64 of "MyWiFi")
  - Assert `info.SSID == "MyWiFi"`

- [ ] **10.7** Test: **DeviceInfo preserves raw value on invalid base64**
  - Mock transport returns `"nickname": "Not-Valid-Base64!!!"` (not valid base64)
  - Assert `info.Nickname == "Not-Valid-Base64!!!"` (raw value preserved, no error)

- [ ] **10.8** Test: **DeviceInfo returns ErrUnsupportedModel for non-P100**
  - Mock transport returns `"model": "P110"`
  - Call `DeviceInfo(ctx)`
  - Assert `errors.Is(err, ErrUnsupportedModel)` is `true`
  - Assert `info != nil` (struct is still populated)
  - Assert `info.Model == "P110"`

- [ ] **10.9** Test: **DeviceInfo returns nil error for P100**
  - Mock transport returns `"model": "P100"`
  - Call `DeviceInfo(ctx)` and assert `err == nil`

- [ ] **10.10** Test: **DeviceInfo propagates login errors**
  - Configure mock transport with `loginErr = fmt.Errorf("mock: %w", ErrAuth)`
  - Call `DeviceInfo(ctx)` and assert `errors.Is(err, ErrAuth)` is `true`

- [ ] **10.11** Test: **DeviceInfo propagates send errors**
  - Configure mock transport with `sendErr = fmt.Errorf("mock: network error")`
  - Call `DeviceInfo(ctx)` and assert `err != nil`

- [ ] **10.12** Test: **DeviceInfo skips login on subsequent calls**
  - Call `DeviceInfo(ctx)` twice
  - Assert `Login` was called exactly once (lazy, not repeated)

### Task 11: Compile Check and Build Verification [All ACs]

- [ ] **11.1** Run `go build ./...` and confirm zero errors
- [ ] **11.2** Run `go vet ./...` and confirm no warnings
- [ ] **11.3** Run `go test ./...` and confirm all tests pass
- [ ] **11.4** Run `go test -race ./...` and confirm no race conditions
- [ ] **11.5** Run `go mod tidy` and confirm no external dependencies added to `go.mod`

---

## Dev Notes

### Architecture Constraints

- **AD-1 (Internal boundary):** The `transport.Transport` interface is internal. No `internal/` type may appear in any exported signature. The `Plug` struct references `transport.Transport` as a private field. The `Option` type and all exported functions use only public types. The `withTransport` option helper is unexported for this reason.

- **AD-7 (Lazy transport negotiation):** `NewPlug` and `NewPlugFromEnv` return immediately with zero network I/O. The transport's `Login` is called on the first command (e.g., `DeviceInfo`). This decouples construction from connection errors, letting callers construct clients at startup and handle failures at command time.

- **AD-8 (DeviceInfo decoding in root package):** JSON-to-struct mapping and base64 decoding happen in `device_info.go` in the root package. There is no `internal/device` package. The `parseDeviceInfo` function handles the full pipeline: JSON unmarshal, base64 decode of `Nickname` and `SSID`, and model validation.

- **AD-5 (Mutex-based concurrency):** The `Plug` struct has its own `sync.Mutex` that serializes the `loggedIn` flag check and re-authentication. Critical rule: never hold the Plug mutex during network I/O. Pattern:
  ```
  mu.Lock() -> check loggedIn -> mu.Unlock() -> do Login I/O -> mu.Lock() -> set loggedIn -> mu.Unlock()
  ```
  Full goroutine safety (concurrent command serialization, re-auth on session expiry) is completed in Story 1.5.

- **AD-6 (Sentinel errors with wrapping):** All four sentinels are defined in `errors.go`. Internal code wraps them with `fmt.Errorf("context: %w", sentinel)` for additional detail. Callers use `errors.Is`. For the `ErrUnsupportedModel` warning, the error is returned alongside a fully populated `*DeviceInfo` so both the operation result and the warning are observable.

### DeviceInfo JSON Field Mapping

The `DeviceInfo` struct maps to the Tapo `get_device_info` JSON response as follows:

| Go Field | JSON Key | Go Type | Notes |
|---|---|---|---|
| `DeviceOn` | `device_on` | `bool` | Relay state (on/off) |
| `Model` | `model` | `string` | e.g. `"P100"` |
| `Nickname` | `nickname` | `string` | Base64-encoded on wire; decoded to UTF-8 |
| `DeviceID` | `device_id` | `string` | Unique device identifier |
| `FirmwareVersion` | `fw_ver` | `string` | e.g. `"1.4.4 Build 20240514 Rel 35017"` |
| `HardwareVersion` | `hw_ver` | `string` | e.g. `"1.0"` |
| `IPAddress` | `ip` | `string` | LAN IP address |
| `MAC` | `mac` | `string` | MAC address |
| `SSID` | `ssid` | `string` | Optional; base64-encoded on wire; decoded to UTF-8 |

### Base64 Decoding Strategy

The Tapo device base64-encodes `Nickname` and `SSID` in the `get_device_info` response. The decoding approach:

1. Unmarshal JSON into `deviceInfoWire` (all fields as raw strings).
2. Attempt `base64.StdEncoding.DecodeString()` on `Nickname` and `SSID`.
3. If decoding succeeds and the result is valid UTF-8, use the decoded value.
4. If decoding fails (not valid base64), preserve the raw string value without error. This is a graceful fallback for devices that may send plain-text values in future firmware.

### Error Wrapping Patterns

```go
// Validation errors (no sentinel)
fmt.Errorf("tapo: host must not be empty")
fmt.Errorf("tapo: missing required environment variables: %s", names)

// Unsupported model warning (includes model name for debugging)
fmt.Errorf("tapo: device model %q is not P100: %w", model, ErrUnsupportedModel)

// Login errors propagated from transport (already wrapped with ErrAuth or ErrHandshake)
// DeviceInfo returns them as-is; no re-wrapping
```

### Environment Variable Precedence

- `TAPO_HOST` is checked first (preferred).
- `TAPO_IP` is checked only when `TAPO_HOST` is empty (alias for backward compatibility).
- If neither is set, the error reports `TAPO_HOST` as the missing variable.
- `TAPO_EMAIL` and `TAPO_PASSWORD` are always required.

### Timeout Behavior

- Default timeout: 10 seconds (per NFR-2).
- Applied as a `context.WithTimeout` wrapping the caller's context on each command, not as an `http.Client.Timeout`.
- The transport's HTTP requests inherit the deadline via context propagation (AC-6 of Story 1.2).
- `WithTimeout(0)` disables the automatic timeout, relying solely on the caller's context.

### Dependencies from Prior Stories

**From Story 1.1 (internal/crypto):**
- `crypto.KLAPAuthHash(email, password string) []byte` -- used indirectly via transport
- `crypto.Encrypt` / `crypto.Decrypt` -- used indirectly via transport

**From Story 1.2 (internal/transport):**

| Symbol | File | Purpose |
|---|---|---|
| `transport.Transport` | `internal/transport/transport.go` | Interface with `Login` + `Send` |
| `klap.New(host string, client *http.Client) *Transport` | `internal/transport/klap/klap.go` | KLAP transport constructor |
| `ErrAuth`, `ErrHandshake` | `errors.go` | Sentinel errors (may already be defined) |

**From Story 1.1 (root package placeholders):**
- `doc.go`, `plug.go`, `device_info.go`, `options.go`, `errors.go` -- exist as `package tapo` placeholders to be replaced with full implementations

### What This Story Does NOT Implement

- `TurnOn`, `TurnOff`, `Toggle` methods -- deferred to Story 1.4
- Re-authentication on session expiry (error 9999) -- deferred to Story 1.5
- Full Plug-level mutex serialization for concurrent commands -- deferred to Story 1.5
- `NegotiatingTransport` / transport override via public `WithTransport` -- deferred to Story 2.2
- The `login` helper in this story provides basic lazy-login; Story 1.5 upgrades it with re-auth and concurrent serialization

---

## Project Structure Notes

### Files Created / Modified in This Story

| File | Action | Purpose |
|---|---|---|
| `plug.go` | **Replace placeholder** | `Plug` struct, `NewPlug`, `NewPlugFromEnv`, `DeviceInfo`, `login`, `applyTimeout` |
| `device_info.go` | **Replace placeholder** | `DeviceInfo` struct, `deviceInfoWire`, `parseDeviceInfo` |
| `options.go` | **Replace placeholder** | `config` struct, `Option` type, `WithTimeout`, `withTransport` |
| `errors.go` | **Verify / update** | Four sentinel errors (may already be complete from Story 1.2) |
| `doc.go` | **Verify / update** | Package doc comment |
| `plug_test.go` | **Create** | Unit tests for `NewPlug`, `NewPlugFromEnv`, `DeviceInfo` |
| `device_info_test.go` | **Create** (optional) | Unit tests for `parseDeviceInfo` if separated from `plug_test.go` |
| `errors_test.go` | **Create** | Unit tests for sentinel error behavior |

### Files from Prior Stories (Already Exist -- Do Not Modify)

| File | Contains | Story |
|---|---|---|
| `go.mod` | Module `github.com/mjenh/tapo`, Go 1.24+ | 1.1 |
| `internal/crypto/crypto.go` | AES-CBC encrypt/decrypt | 1.1 |
| `internal/crypto/auth.go` | `KLAPAuthHash`, `KLAPDeriveKeyAndIV` | 1.1 |
| `internal/crypto/crypto_test.go` | Crypto unit tests | 1.1 |
| `internal/crypto/auth_test.go` | Auth hash unit tests | 1.1 |
| `internal/transport/transport.go` | `Transport` interface | 1.2 |
| `internal/transport/klap/klap.go` | KLAP transport implementation | 1.2 |
| `internal/transport/klap/klap_test.go` | KLAP transport unit tests | 1.2 |

---

## References

- **PRD:** `_bmad-output/planning-artifacts/tapo/tapo-prd.md` (FR-1, FR-3, FR-7, FR-8, NFR-1, NFR-2, NFR-4, Section 9.1 Public API)
- **Spec:** `_bmad-output/planning-artifacts/tapo/tapo-spec.md` (CAP-1, CAP-3, CAP-7, CAP-8, Constraints)
- **Architecture:** `_bmad-output/planning-artifacts/tapo/tapo-architecture.md` (AD-1, AD-5, AD-6, AD-7, AD-8, Structural Seed, Consistency Conventions)
- **Epics:** `_bmad-output/planning-artifacts/tapo/tapo-epics.md` (Epic 1, Story 1.3 definition and acceptance criteria)
- **Addendum:** `_bmad-output/planning-artifacts/tapo/addendum.md` (DeviceInfo field mapping table, KLAP protocol overview)
- **Glossary:** `_bmad-output/planning-artifacts/tapo/glossary.md` (Plug, DeviceInfo, ErrUnsupportedModel, Session, Client)

---

## Dev Agent Record

### Completion Log

| Task | Status | Notes |
|---|---|---|
| Task 1: Implement errors.go | done | Doc comments added; all 4 sentinels verified |
| Task 2: Implement options.go | done | config, Option, WithTimeout (10s default), withTransport |
| Task 3: Implement plug.go | done | Plug struct, NewPlug, NewPlugFromEnv, login, applyTimeout |
| Task 4: Implement device_info.go | done | DeviceInfo struct, base64 decode with UTF-8 validation |
| Task 5: Implement DeviceInfo method | done | Lazy login, get_device_info, parseDeviceInfo |
| Task 6: Implement doc.go | done | Updated package documentation |
| Task 7: Unit Tests -- Sentinel Errors | done | 3 tests: distinctness, wrapping, no cross-match |
| Task 8: Unit Tests -- NewPlug | done | 7 tests: validation, no I/O, timeout options |
| Task 9: Unit Tests -- NewPlugFromEnv | done | 8 tests: env vars, fallback, precedence, missing |
| Task 10: Unit Tests -- DeviceInfo | done | 13 tests: mock transport, base64, ErrUnsupportedModel |
| Task 11: Compile Check and Build | done | User verified locally |

### Change Log

- 2026-07-01: Initial implementation — all tasks complete
- 2026-07-01: Code review fixes — safe type assertion, UTF-8 validation in base64 decode, doc comment accuracy

### Test Results

All tests pass locally (verified by user). 31 tests total.

### Notes

- Code review (Sonnet) found: unchecked type assertion panic risk (H3), missing UTF-8 validation in decodeBase64 (M4), overclaimed concurrent safety doc comment (H1). All fixed.
- Concurrent first-login duplicate requests acknowledged as deferred to Story 1.5.
- Field name `tp` used instead of story pseudocode's `transport` to avoid shadowing the imported package name.
