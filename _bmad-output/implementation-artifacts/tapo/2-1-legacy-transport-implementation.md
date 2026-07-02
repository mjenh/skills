# Story 2.1: Legacy Transport Implementation

Status: done

## Story

As a library developer,
I want a legacy transport implementation,
So that P100 plugs on older firmware are supported.

## Acceptance Criteria

1. **Given** the Transport interface and crypto helpers from Epic 1, **When** the legacy transport is implemented, **Then** `internal/transport/legacy/legacy.go` implements the `Transport` interface with `Login(ctx context.Context, email, password string) error` and `Send(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error)`.

2. **Given** a Tapo device on legacy firmware, **When** `Login` is called with valid credentials, **Then** it performs an RSA handshake with the device (`POST /app` with `method: "handshake"` containing a client-generated RSA public key) and decrypts the returned AES key material (key + IV) using the client's RSA private key.

3. **Given** a successful RSA handshake, **When** `Login` proceeds to authentication, **Then** it calls `login_device` with `base64(SHA1(email))` as the username and `base64(password)` as the password, both AES-CBC encrypted inside a `securePassthrough` envelope.

4. **Given** a successful `login_device` response, **When** the session token is extracted, **Then** `Login` stores the session token internally per AD-4 and appends it as `?token=<session_token>` to all subsequent requests.

5. **Given** an authenticated legacy transport, **When** `Send` is called with a method and payload, **Then** it wraps the command in a `securePassthrough` envelope with AES-CBC encryption using the handshake-derived key/IV and appends the session token to the request URL.

6. **Given** invalid credentials, **When** `Login` calls `login_device` and receives an authentication error, **Then** it returns an error wrapping `ErrAuth` (detectable via `errors.Is`).

7. **Given** a device that rejects the RSA handshake, **When** `Login` fails during the handshake phase, **Then** it returns an error wrapping `ErrHandshake` (detectable via `errors.Is`).

8. **Given** the `internal/crypto/auth.go` file from Story 1.1, **When** the legacy transport requires SHA1+base64 login encoding, **Then** `auth.go` is extended with a `LegacyLoginHash(email string) string` function that returns `base64(SHA1(email))` (if not already present).

9. **Given** the legacy transport struct, **When** multiple goroutines access session state concurrently, **Then** the transport holds its own `sync.Mutex` guarding all session state (AES key, IV, session token, HTTP cookie jar) per AD-5.

10. **Given** any legacy transport operation, **When** a `context.Context` is provided, **Then** all HTTP requests and cryptographic operations honor the context for cancellation and deadlines.

11. **Given** the legacy transport implementation, **When** unit tests are run, **Then** all tests pass verifying handshake flow, AES encrypt/decrypt round-trip through `securePassthrough`, login error paths, and session token management using a mock HTTP server.

12. **Given** any code path in the legacy transport, **When** credentials (email, password) are handled, **Then** they are never logged, printed, or included in error messages.

## Tasks / Subtasks

### Task 1: Extend Crypto Auth Helpers [AC-8]

File: `internal/crypto/auth.go`

- [x] Add `LegacyLoginHash(email string) string`:
  - Compute `SHA1([]byte(email))` using `crypto/sha1`
  - Base64-encode the 20-byte SHA1 digest using `encoding/base64.StdEncoding`
  - Return the base64 string
  - This produces the `username` field for `login_device` requests
- [x] Ensure `crypto/sha1` and `encoding/base64` are in the import block (may already be present from KLAP auth hash)
- [x] No changes to existing `KLAPAuthHash` or `KLAPDeriveKeyAndIV` functions

### Task 2: Define Legacy Transport Struct [AC-1, AC-9]

File: `internal/transport/legacy/legacy.go`

- [x] Package declaration: `package legacy`
- [x] Define `Transport` struct with unexported fields:
  - `host string` -- device IP/host
  - `client *http.Client` -- HTTP client for device communication
  - `mu sync.Mutex` -- guards all session state below
  - `aesKey []byte` -- 16-byte AES key from RSA handshake (session state)
  - `aesIV []byte` -- 16-byte AES IV from RSA handshake (session state)
  - `token string` -- session token from `login_device` (session state)
  - `cookieJar http.CookieJar` -- HTTP cookies from handshake (session state, for `TP_SESSIONID` if set)
- [x] Implement constructor `New(host string, timeout time.Duration) *Transport`:
  - Creates `http.Client` with the given timeout and a `cookiejar.Jar`
  - Stores host
  - Returns the transport pointer
- [x] Ensure the struct satisfies `transport.Transport` interface (compile-time check: `var _ transport.Transport = (*Transport)(nil)`)

### Task 3: Implement RSA Handshake [AC-2, AC-7, AC-10]

File: `internal/transport/legacy/legacy.go`

- [x] Implement internal method `handshake(ctx context.Context) error`:
  - Generate a 1024-bit RSA key pair using `crypto/rsa.GenerateKey` with `crypto/rand`
  - Extract the public key in PKCS1 DER format using `crypto/x509.MarshalPKCS1PublicKey`
  - Base64-encode the DER bytes and wrap in PEM-style `-----BEGIN PUBLIC KEY-----` / `-----END PUBLIC KEY-----` markers
  - Build the handshake JSON request:
    ```json
    {
      "method": "handshake",
      "params": {
        "key": "<PEM-encoded RSA public key>"
      }
    }
    ```
  - `POST` to `http://{host}/app` with `Content-Type: application/json`
  - Use `http.NewRequestWithContext(ctx, ...)` to honor context
  - Parse the JSON response; extract `result.key` (base64-encoded encrypted AES key material)
  - Base64-decode the response key
  - Decrypt using `rsa.DecryptPKCS1v15` with the generated private key
  - The decrypted payload is 32 bytes: first 16 bytes = AES key, last 16 bytes = AES IV
  - Store `aesKey`, `aesIV` on the transport struct (under mutex)
  - On RSA decryption failure or non-200 HTTP status: return `fmt.Errorf("legacy: handshake failed: %w", tapo.ErrHandshake)`
  - On context cancellation: return context error
  - **Note:** RSA-1024 is at Go 1.24's enforced minimum. Works now but may break in future Go versions. No action needed until it breaks.

### Task 4: Implement securePassthrough Helper [AC-5]

File: `internal/transport/legacy/legacy.go`

- [x] Implement internal method `encryptRequest(payload []byte) (string, error)`:
  - Encrypt `payload` using `crypto.Encrypt(t.aesKey, t.aesIV, payload)` (from `internal/crypto`)
  - Base64-encode the ciphertext
  - Return the base64 string
- [x] Implement internal method `decryptResponse(encryptedPayload string) ([]byte, error)`:
  - Base64-decode the encrypted payload string
  - Decrypt using `crypto.Decrypt(t.aesKey, t.aesIV, ciphertext)` (from `internal/crypto`)
  - Return the plaintext bytes
- [x] Implement internal method `securePassthrough(ctx context.Context, innerJSON []byte) (json.RawMessage, error)`:
  - Encrypt `innerJSON` via `encryptRequest`
  - Build the `securePassthrough` envelope:
    ```json
    {
      "method": "securePassthrough",
      "params": {
        "request": "<base64-encoded-encrypted-inner-json>"
      }
    }
    ```
  - `POST` to `http://{host}/app?token={t.token}` with `Content-Type: application/json`
  - Use `http.NewRequestWithContext(ctx, ...)` to honor context
  - Parse the outer JSON response; check `error_code` field (0 = success)
  - If `error_code != 0`: return appropriate wrapped error
  - Extract `result.response` (base64-encoded encrypted response)
  - Decrypt via `decryptResponse`
  - Return the decrypted inner JSON as `json.RawMessage`

### Task 5: Implement Login [AC-2, AC-3, AC-4, AC-6, AC-7, AC-10, AC-12]

File: `internal/transport/legacy/legacy.go`

- [x] Implement `Login(ctx context.Context, email, password string) error`:
  - Acquire `t.mu.Lock()` at entry, defer `t.mu.Unlock()`
  - Call `t.handshake(ctx)` -- on failure, return the handshake error (wraps `ErrHandshake`)
  - Build the `login_device` inner request:
    ```json
    {
      "method": "login_device",
      "params": {
        "username": "<base64(SHA1(email))>",
        "password": "<base64(password)>"
      },
      "requestTimeMils": <unix_milliseconds>
    }
    ```
  - Use `crypto.LegacyLoginHash(email)` for the username field
  - Use `base64.StdEncoding.EncodeToString([]byte(password))` for the password field
  - Send via `t.securePassthrough(ctx, loginJSON)` (this uses the handshake-derived AES key/IV, no token yet for this first request)
  - Note: the `login_device` request is sent without `?token=` in the URL since no token exists yet
  - Parse the inner response JSON; extract `result.token` as the session token
  - Check `error_code` in the inner response:
    - `0`: success -- store `t.token`
    - `-1501` (or other auth-related codes): return `fmt.Errorf("legacy: invalid credentials: %w", tapo.ErrAuth)`
  - On success, store `t.token` for use in subsequent `Send` calls
  - Credentials (`email`, `password`) must not appear in any error message or log output

### Task 6: Implement Send [AC-1, AC-5, AC-10, AC-12]

File: `internal/transport/legacy/legacy.go`

- [x] Implement `Send(ctx context.Context, method string, payload json.RawMessage) (json.RawMessage, error)`:
  - Acquire `t.mu.Lock()` at entry, defer `t.mu.Unlock()`
  - Validate that `t.token` is non-empty (return error if `Login` has not been called)
  - Build the inner command JSON:
    ```json
    {
      "method": "<method>",
      "params": <payload>
    }
    ```
  - If `payload` is nil or empty, omit the `params` field (some commands like `get_device_info` have no params)
  - Send via `t.securePassthrough(ctx, commandJSON)`
  - Parse the inner response; check `error_code`:
    - `0`: return `result` field as `json.RawMessage`
    - `9999`: return an error indicating session expiry (the Plug-level re-auth logic from Story 1.5 handles retry)
    - Other non-zero: return error with code context
  - Return the response `json.RawMessage`

### Task 7: Wire Format Types [AC-1, AC-5]

File: `internal/transport/legacy/legacy.go`

- [x] Define unexported wire-format structs for JSON marshaling/unmarshaling:
  - `handshakeRequest` -- `method`, `params.key`
  - `handshakeResponse` -- `error_code`, `result.key`
  - `securePassthroughRequest` -- `method`, `params.request`
  - `securePassthroughResponse` -- `error_code`, `result.response`
  - `loginRequest` -- `method`, `params.username`, `params.password`, `requestTimeMils`
  - `deviceResponse` -- `error_code`, `result` (json.RawMessage)
- [x] All struct fields use `json:"snake_case"` tags matching Tapo device JSON keys
- [x] All types are unexported (lowercase) -- they never cross the package boundary

### Task 8: Unit Tests -- RSA Handshake [AC-11, AC-7]

File: `internal/transport/legacy/legacy_test.go`

- [x] Package declaration: `package legacy` (internal test for unexported access)
- [x] Set up `httptest.NewServer` that simulates the legacy device handshake endpoint:
  - Accept `POST /app` with `method: "handshake"`
  - Extract the client's RSA public key from the request
  - Parse the PEM-encoded public key
  - Generate a known 32-byte AES key material (16-byte key + 16-byte IV)
  - Encrypt it with the client's public key using `rsa.EncryptPKCS1v15`
  - Return the base64-encoded encrypted key in the response JSON
- [x] Test successful handshake: call `handshake(ctx)`, verify `t.aesKey` and `t.aesIV` are set to expected values
- [x] Test handshake failure (server returns error): verify returned error wraps `ErrHandshake`
- [x] Test handshake with cancelled context: verify context error is returned

### Task 9: Unit Tests -- Login Flow [AC-11, AC-6, AC-12]

File: `internal/transport/legacy/legacy_test.go`

- [x] Extend mock HTTP server to handle both handshake and `login_device`:
  - For `securePassthrough` requests, decrypt the inner payload using the known AES key/IV
  - Parse the inner `login_device` request
  - Verify the username is `base64(SHA1(email))` format
  - Verify the password is `base64(password)` format
  - Return an encrypted response with `error_code: 0` and a mock session token
- [x] Test successful login: call `Login(ctx, email, password)`, verify `t.token` is set
- [x] Test login with invalid credentials: mock server returns auth error code, verify returned error wraps `ErrAuth` and `errors.Is(err, tapo.ErrAuth)` is true
- [x] Test login with handshake failure: mock server rejects handshake, verify returned error wraps `ErrHandshake`
- [x] Test that credentials do not appear in error messages: capture error strings, assert they do not contain email or password values

### Task 10: Unit Tests -- Send / securePassthrough [AC-11, AC-5]

File: `internal/transport/legacy/legacy_test.go`

- [x] Test successful `Send`:
  - Login first (via mock handshake + login_device)
  - Call `Send(ctx, "get_device_info", nil)`
  - Mock server decrypts `securePassthrough`, verifies inner method is `"get_device_info"`, returns encrypted mock device info
  - Verify the returned `json.RawMessage` contains expected device info JSON
- [x] Test `Send` with session token in URL: verify the mock server receives `?token=<expected_token>` in the request URL
- [x] Test `Send` without prior Login: verify an error is returned indicating no active session
- [x] Test `Send` with error code 9999 in response: verify an error is returned indicating session expiry
- [x] Test AES encrypt/decrypt round-trip through `securePassthrough`: verify plaintext survives the encrypt-send-decrypt pipeline

### Task 11: Unit Tests -- Crypto Auth Extension [AC-8]

File: `internal/crypto/auth_test.go`

- [x] Test `LegacyLoginHash` with known inputs:
  - Input: `email = "test@example.com"`
  - Manually compute expected output: `base64(SHA1("test@example.com"))`
  - Assert the function returns the expected base64 string
- [x] Test `LegacyLoginHash` with empty email: should not panic, returns valid base64 string
- [x] Test `LegacyLoginHash` output is valid base64: decode the result and verify it is exactly 20 bytes (SHA1 digest size)
- [x] Test `LegacyLoginHash` determinism: same input always produces same output

### Task 12: Verify Build and Tests [AC-1, AC-11]

- [x] Run `go build ./...` and confirm zero errors
- [x] Run `go test ./internal/transport/legacy/...` and confirm all tests pass
- [x] Run `go test ./internal/crypto/...` and confirm existing + new tests pass
- [x] Run `go test -race ./...` and confirm no race conditions detected
- [x] Run `go vet ./...` and confirm no warnings
- [x] Verify `go.mod` still has zero external dependencies (no `require` block added)

## Dev Notes

### Architecture Constraints

- **AD-1 (Layered with internal boundary):** The legacy transport lives under `internal/transport/legacy/`. No type from this package appears in any exported signature. The root package interacts only through the `Transport` interface.
- **AD-2 (Package split: transport and crypto):** The legacy transport imports `internal/crypto` for AES encrypt/decrypt and the new `LegacyLoginHash` helper. The crypto package never imports transport.
- **AD-3 (Thin Transport interface):** The legacy transport implements exactly two methods: `Login` and `Send`. All legacy protocol specifics (RSA handshake, `securePassthrough` wrapping, session token management) are internal to the package.
- **AD-4 (Transport-owned session state):** The legacy transport owns its session state: AES key, AES IV, session token, and cookie jar. `Plug` never inspects or stores this data. No session type crosses the `Transport` interface boundary.
- **AD-5 (Mutex-based concurrency):** The legacy transport holds its own `sync.Mutex` guarding all session state. This is the transport-level mutex (layer 1). The Plug-level mutex (layer 2, from Story 1.5) gates re-authentication.
- **AD-6 (Sentinel errors with wrapping):** Login errors wrap `ErrAuth` or `ErrHandshake` using `fmt.Errorf("legacy: ...: %w", sentinel)`. Callers use `errors.Is`.

### Legacy Protocol Implementation Details

- **RSA-1024 Key Size:** The Tapo legacy protocol uses RSA-1024 for the handshake. Go 1.24 enforces a minimum 1024-bit key, so this works at the boundary. Future Go versions may raise the floor. No action needed now; monitor across Go releases. If Go raises the minimum, the legacy transport will need a workaround or the minimum Go version constraint will need to be pinned.
- **Handshake Flow:** Client generates RSA-1024 keypair, sends public key PEM to device. Device encrypts 32 bytes (16-byte AES key + 16-byte IV) with the client's public key and returns base64-encoded ciphertext. Client decrypts with private key to get AES key material.
- **Login Encoding:** Username is `base64(SHA1(email))` -- the email is SHA1-hashed, then the raw 20-byte digest is base64-encoded. Password is simply `base64(password)` -- the raw password bytes are base64-encoded directly.
- **securePassthrough Envelope:** All commands after handshake are wrapped: the inner command JSON is AES-CBC encrypted, base64-encoded, and placed in `{"method": "securePassthrough", "params": {"request": "<encrypted>"}}`. The response follows the same pattern in reverse.
- **Session Token:** The `login_device` response returns a `token` field. This token is appended as `?token=<value>` to all subsequent request URLs. Error code `9999` in any response indicates session expiry; the Plug-level re-auth logic (Story 1.5) handles retry.
- **Request URL:** All requests go to `http://{host}/app` (handshake, login) or `http://{host}/app?token={token}` (subsequent commands).
- **Cookies:** The device may set cookies (e.g., `TP_SESSIONID`) during handshake. The HTTP client's cookie jar handles these automatically.

### What to Reuse from KLAP (Story 1.2)

- **Transport interface:** The legacy transport implements the exact same `transport.Transport` interface defined in `internal/transport/transport.go`. No changes to the interface.
- **AES-CBC encrypt/decrypt:** Reuse `internal/crypto.Encrypt` and `internal/crypto.Decrypt` from Story 1.1 for `securePassthrough` payload encryption. The legacy protocol uses the same AES-CBC with PKCS7 padding.
- **Error sentinels:** Reuse `ErrAuth` and `ErrHandshake` from `errors.go` (Story 1.3). Wrap with context via `fmt.Errorf`.
- **Mock HTTP server pattern:** Follow the same `httptest.NewServer` approach used in KLAP transport tests for the legacy transport tests.
- **Mutex pattern:** Same `sync.Mutex` on the transport struct guarding session state, same pattern as KLAP.

### What Already Exists from Prior Stories

- `internal/transport/transport.go` -- Transport interface (Story 1.2): `Login` + `Send` methods
- `internal/transport/klap/klap.go` -- KLAP implementation (Story 1.2): pattern reference
- `internal/crypto/crypto.go` -- AES-CBC `Encrypt`/`Decrypt` with PKCS7 padding (Story 1.1)
- `internal/crypto/auth.go` -- `KLAPAuthHash`, `KLAPDeriveKeyAndIV` (Story 1.1); needs `LegacyLoginHash` extension
- `errors.go` -- `ErrAuth`, `ErrTimeout`, `ErrUnsupportedModel`, `ErrHandshake` (Story 1.3)
- `internal/transport/legacy/legacy.go` -- placeholder file with `package legacy` (Story 1.1)

### Testing Requirements

- All tests use `testing` stdlib package only. No testify or other external test frameworks.
- Mock HTTP server via `net/http/httptest` for handshake and `securePassthrough` simulation.
- Test files use internal test package (`package legacy`) to access unexported helpers and struct fields.
- The mock server must perform real RSA encryption (using the client's public key from the request) to simulate the actual handshake flow.
- The mock server must perform real AES-CBC encryption/decryption for `securePassthrough` tests to validate the full encrypt/decrypt pipeline.
- Run `go test -race ./...` to confirm no race conditions with the mutex-guarded session state.
- Test error messages to confirm they do not contain credential values.

### Conventions

- Follow Go stdlib naming conventions. Exported: `Transport`, `New`. Unexported: `handshake`, `securePassthrough`, `encryptRequest`, `decryptResponse`, wire-format structs.
- Error messages follow Go conventions: lowercase, no punctuation at end, descriptive. Format: `"legacy: <detail>: %w"`.
- No `init()` functions. No global mutable state.
- The file should have a brief package-level comment: `// Package legacy implements the Tapo legacy transport protocol using RSA handshake and AES-CBC securePassthrough.`
- Import grouping: stdlib first, then internal packages, separated by blank line.

## Project Structure Notes

### New Files

```
internal/
  transport/
    legacy/
      legacy.go           # Legacy Transport implementation (RSA handshake, login, securePassthrough, Send)
      legacy_test.go       # Unit tests for legacy transport (mock HTTP server)
```

### Files to Modify

```
internal/
  crypto/
    auth.go               # Add LegacyLoginHash(email string) string function
    auth_test.go           # Add tests for LegacyLoginHash
```

### Existing Files (no changes needed)

```
internal/
  transport/
    transport.go           # Transport interface -- no changes
  crypto/
    crypto.go              # AES-CBC Encrypt/Decrypt -- reused as-is
errors.go                  # ErrAuth, ErrHandshake sentinels -- reused as-is
```

## References

- **PRD:** `_bmad-output/planning-artifacts/tapo/tapo-prd.md` (FR-9, NFR-4)
- **Spec:** `_bmad-output/planning-artifacts/tapo/tapo-spec.md` (CAP-2, CAP-9)
- **Architecture:** `_bmad-output/planning-artifacts/tapo/tapo-architecture.md` (AD-1, AD-2, AD-3, AD-4, AD-5, AD-6, Structural Seed, Stack, RSA-1024 deferred item)
- **Epics:** `_bmad-output/planning-artifacts/tapo/tapo-epics.md` (Epic 2, Story 2.1)
- **Addendum:** `_bmad-output/planning-artifacts/tapo/addendum.md` (Legacy protocol overview, securePassthrough, login_device encoding)
- **Glossary:** `_bmad-output/planning-artifacts/tapo/glossary.md` (Legacy protocol, Session, KLAP definitions)
- **Story 1.1:** `_bmad-output/implementation-artifacts/tapo/1-1-module-scaffold-shared-crypto-helpers.md` (crypto foundation)
- **Implementation Readiness Report:** `_bmad-output/planning-artifacts/tapo/implementation-readiness-report-2026-07-01.md`

## Dev Agent Record

### Iteration 1

**Status:** Complete
**Started:** 2026-07-01
**Completed:** 2026-07-01
**Changes:**
- Added `LegacyLoginHash` to `internal/crypto/auth.go`
- Created `internal/transport/legacy/legacy.go` — full legacy transport (RSA handshake, securePassthrough, Login, Send)
- Created `internal/transport/legacy/legacy_test.go` — 12 tests with mock HTTP device server
- Added 4 tests for `LegacyLoginHash` in `internal/crypto/auth_test.go`
**Notes:** Followed KLAP transport patterns for consistency. RSA-1024 used per protocol spec. All error wrapping uses sentinels from `internal/transport`. Zero external dependencies maintained.
**Test results:** All tests pass including `go test -race ./...`, `go vet ./...`, `go build ./...`
