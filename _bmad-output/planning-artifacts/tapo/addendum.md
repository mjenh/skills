# Addendum: Tapo Go Client Library

Technical context and design rationale that inform the PRD but are not themselves product requirements.

---

## Reference library comparison

| Aspect | fabiankachlock/tapo-api | tess1o/tapo-go | rk295/tapo-go (deprecated baseline) |
|--------|-------------------------|----------------|-------------------------------------|
| Activity | Active (v1.4.x) | Pre-1.0, active | Stale (v0.0.9, 2022); repo unavailable |
| Protocol | KLAP | KLAP + ssl_aes | Legacy only |
| P100 | Explicit `P100()` factory | Generic SmartPlug | Generic plug API |
| Context | Partial | Full `context.Context` | None |
| Scope | Plugs, lights, hubs | P110/P115, H200, T315 | Plugs + P110 energy |
| License | MIT | MIT | MIT |

**tapo v1 positioning:** Smaller scope than tapo-api (P100 only), similar idiomatic Go patterns to tess1o/tapo-go (context, options), greenfield API with semver intent.

---

## Local protocol overview

Tapo devices expose an HTTP endpoint on the LAN (typically `http://{host}/app`). There is no official public specification; community libraries reverse-engineer the mobile app behavior.

### KLAP (primary for v1)

Used by many current firmware builds. Rough flow:

1. Two-stage seed handshake establishing `TP_SESSIONID` cookie.
2. Auth hash derived from username/password (community convention: nested MD5 of encoded credentials).
3. AES-encrypted payloads with derived key/IV; sequence counter in request URL.
4. Commands such as `get_device_info`, `set_device_info` issued inside encrypted envelopes.

Implementations to study: fabiankachlock/tapo-api KLAP client, python-kasa `KlapTransport`.

### Legacy protocol (fallback)

Older devices/firmware:

1. RSA handshake → device returns AES key material.
2. All commands wrapped in `securePassthrough` with AES-CBC.
3. `login_device` with base64(SHA1(email)) username encoding.
4. Session token appended to subsequent requests; error `9999` → re-login.

**PRD decision (FR-9):** On first connect, attempt KLAP handshake; if it fails with a distinguishable protocol error, retry legacy. Document behavior in README support matrix.

**v1.0.0 certified firmware:** P100 — v1.4.4 Build 20240514 Rel 35017 (KLAP).

---

## DeviceInfo field mapping (informative)

Based on community `get_device_info` responses for plugs. Minimum v1 fields map to Tapo JSON keys:

| Go field | JSON key | Notes |
|----------|----------|-------|
| DeviceOn | device_on | Relay state |
| Model | model | e.g. "P100" |
| Nickname | nickname | Often base64-encoded on wire |
| DeviceID | device_id | |
| FirmwareVersion | fw_ver | |
| HardwareVersion | hw_ver | |
| IPAddress | ip | |
| MAC | mac | |
| SSID | ssid | Optional; base64 on wire |
| RSSI | rssi | Optional signal metadata |

Additional fields may be added in minor versions if backward-compatible.

---

## Rejected alternatives

| Alternative | Why not v1 |
|-------------|------------|
| Fork rk295/tapo-go | Stale, legacy-only, no context, unavailable source repo |
| Support P110 energy in v1 | Expands scope and API surface; user scoped v1 to P100 |
| Cloud API | Different auth model, latency, privacy profile |
| Non-commercial OSS license only | MIT chosen for launch adoption; user permitted MIT |
| Drop-in rk295 API compatibility | Greenfield rewrite; clean semver API preferred |

---

## Suggested implementation order (for downstream architecture/epics)

1. KLAP transport + login + `get_device_info`
2. `set_device_info` on/off
3. Legacy fallback transport
4. Env helper + options (timeout, retry)
5. Toggle convenience (v1.0.0)
6. Goroutine-safe session/command serialization (FR-10)
7. README quickstart + support matrix (P100 v1.4.4 Build 20240514 Rel 35017)
