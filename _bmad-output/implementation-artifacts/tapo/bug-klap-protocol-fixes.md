# Bug Report: KLAP Protocol Implementation Fixes

> **Epic:** 1 - Core P100 Control Library (Story 1.2: KLAP Transport)
> **Status:** in-progress
> **Discovered during:** Story 3.1 (README & Quickstart) — manual integration testing
> **Severity:** Critical — KLAP transport non-functional against real hardware
> **Affected files:** `internal/crypto/auth.go`, `internal/transport/klap/klap.go`

---

## Summary

Six bugs in the KLAP transport prevented communication with real Tapo P100 hardware. The handshake completed successfully, but all `Send` requests failed with HTTP 403 or produced garbled decrypted output. Root cause: the implementation was built from incomplete protocol documentation and never validated against a physical device.

All bugs were identified by comparing runtime behaviour against the [python-kasa](https://github.com/python-kasa/python-kasa) reference implementation and analysing debug logs from a real P100 (firmware v1.4.4).

---

## Bugs

### Bug 1: Sequence number URL formatting (HTTP 403)

**File:** `internal/transport/klap/klap.go` — `Send`

**Symptom:** Device returned HTTP 403 on every request.

**Root cause:** The `seq` field is `uint32`, but the device expects a signed 32-bit integer in the URL query parameter. Large unsigned values produced positive numbers the device rejected.

**Fix:** Cast to `int32` in the URL format string:
```go
// Before
url := fmt.Sprintf("http://%s/app/request?seq=%d", t.host, seq)
// After
url := fmt.Sprintf("http://%s/app/request?seq=%d", t.host, int32(seq))
```

---

### Bug 2: Missing request signature (HTTP 403)

**File:** `internal/transport/klap/klap.go` — `Send`

**Symptom:** Device returned HTTP 403 even with the int32 URL fix.

**Root cause:** KLAP requires every request body to be prefixed with a 32-byte SHA-256 signature: `SHA256(sigBase + seqBytes + encrypted) || encrypted`. The original implementation sent only the encrypted payload.

**Fix:** Derive a 28-byte signature base during key derivation and prepend the computed signature to each request body.

---

### Bug 3: Wrong signature base length and sequence byte source

**File:** `internal/crypto/auth.go` — `KLAPDeriveKeyIVSeqSig`

**Symptom:** Device returned HTTP 403 — signature verification failed on the device side.

**Root cause (a):** Signature base was 32 bytes (full SHA-256 hash). The protocol uses only the first 28 bytes.

**Root cause (b):** Sequence number was derived from the first 4 bytes of a hash. The protocol uses the last 4 bytes of the IV hash.

**Fix:**
```go
// sig: first 28 bytes, not 32
sig = sigHash[:28]

// seq: last 4 bytes of IV hash, not first 4 of a separate hash
seq = binary.BigEndian.Uint32(ivHash[28:])
```

---

### Bug 4: Sequence increment timing (HTTP 403)

**File:** `internal/transport/klap/klap.go` — `Send`

**Symptom:** Device returned HTTP 403 — the derived sequence number (from handshake) is the "base" value; the first request must use base+1.

**Root cause:** `t.seq++` was placed after the request. The protocol requires incrementing before each request — the handshake-derived seq is never used directly.

**Fix:** Move `t.seq++` to the beginning of `Send`, before reading the seq value. Remove redundant post-response increments.

---

### Bug 5: Wrong IV derivation prefix (garbled decryption)

**File:** `internal/crypto/auth.go` — `KLAPDeriveKeyIVSeqSig`

**Symptom:** HTTP 200 received but decrypted output was garbled (first AES-CBC block corrupted = wrong IV).

**Root cause (a):** IV hash prefix was `"ivb"` instead of `"iv"`.

**Root cause (b):** Sequence number was derived from a separate `SHA256("seq" + payload)` hash. It should come from the last 4 bytes of the same IV hash.

**Fix:**
```go
// Before
ivHash := sha256.Sum256(append([]byte("ivb"), payload...))
seqHash := sha256.Sum256(append([]byte("seq"), payload...))
seq = binary.BigEndian.Uint32(seqHash[:4])

// After
ivHash := sha256.Sum256(append([]byte("iv"), payload...))
iv = ivHash[:12]
seq = binary.BigEndian.Uint32(ivHash[28:]) // last 4 bytes of same hash
```

---

### Bug 6: Response body parsing (garbled decryption)

**File:** `internal/transport/klap/klap.go` — `Send`

**Symptom:** Decryption of response body produced garbled output even with correct keys.

**Root cause:** The code attempted to decrypt the full 64-byte response body. The response format is `signature (32 bytes) || encrypted data` — the first 32 bytes must be stripped before decryption. The full-body attempt sometimes "succeeded" (PKCS7 unpadding didn't error by coincidence) and returned garbage.

**Fix:** Always strip the 32-byte signature prefix before decrypting:
```go
// Before: tried full body first, fell back to stripping
decrypted, err := crypto.Decrypt(key, fullIV, respBody)
if err != nil {
    decrypted, err = crypto.Decrypt(key, fullIV, respEncrypted)
}

// After: always strip signature
respEncrypted := respBody[32:]
decrypted, err := crypto.Decrypt(key, fullIV, respEncrypted)
```

---

## Remaining Work

- [ ] Verify fix end-to-end against real hardware (user must run `TestExample`)
- [ ] Remove all debug logging (`log.Printf`, `"log"` and `"encoding/hex"` imports) from `klap.go`
- [ ] Update unit tests in `klap_test.go` and `auth_test.go` for renamed function and changed signatures
- [ ] Commit fixes on current branch or a dedicated bug-fix branch

---

## Test Evidence

**Device:** Tapo P100, firmware v1.4.4 Build 20240514 Rel 35017
**Test:** `go test -run ^TestExample$ github.com/mjenh/tapo`

| Stage | Before fix | After fix |
|---|---|---|
| Handshake | 200 OK | 200 OK (no change) |
| Send request | 403 Forbidden | 200 OK |
| Response decrypt | N/A | Pending verification (Bug 6 fix not yet tested) |

---

## References

- python-kasa KLAP implementation: `kasa/transports/klaptransport.py`
- Story 1.2 artifact: `_bmad-output/implementation-artifacts/tapo/1-2-klap-transport.md`
- Affected source: `internal/crypto/auth.go`, `internal/transport/klap/klap.go`
