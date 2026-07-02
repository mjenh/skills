# Glossary

Terms used across this spec and its companions. Canonical definitions — downstream skills should not redefine these.

- **Tapo account** — Email and password pair registered with the Tapo mobile app; used for local device authentication. Not a separate API key.
- **Device** — Any Tapo hardware endpoint addressable on the LAN. v1 supports only the Plug subtype.
- **Plug** — A single-outlet Tapo smart plug. v1 target model: P100.
- **P100** — Tapo Wi-Fi smart plug without energy monitoring. Supports on/off control and device info via local API commands `get_device_info` and `set_device_info`.
- **Host** — IP address (optional port) of the Plug on the local network, e.g. `192.168.1.42` or `192.168.1.42:80`.
- **Client** — A configured library instance bound to one Host and one set of Tapo account credentials, responsible for session lifecycle and command dispatch.
- **Session** — Authenticated state between Client and Plug after successful login, including cookies/tokens required for subsequent commands.
- **KLAP** — Newer Tapo local transport protocol (seed handshake, derived AES keys, sequence counter). Required by many current firmware builds. Primary transport for v1.
- **Legacy protocol** — Older Tapo local transport (RSA handshake + AES `securePassthrough` wrapper). Fallback transport for v1.
- **DeviceInfo** — Structured snapshot returned by `get_device_info`: on/off state, model, nickname, firmware versions, network metadata.
- **ErrUnsupportedModel** — Sentinel error indicating the connected device reports a model other than P100. Warning semantics — inspectable via `errors.Is`; does not by itself mean a command failed.
- **Module** — The Go package published at `github.com/mjenh/tapo`.
