# skills

Importable Go modules for MCP servers, AI agent runtimes, and CLIs.

Each module lives in its own directory with an independent `go.mod` and semver releases tagged as `<module>/vX.Y.Z` (e.g. `weather/v1.0.0`).

## Modules

| Module | Import path | Latest | Description |
|--------|-------------|--------|-------------|
| [weather](weather/) | `github.com/mjenh/skills/weather` | [v1.0.1](weather/CHANGELOG.md#101---2026-07-02) | Current weather by location via Google Weather and Geocoding APIs; includes optional ADK adapter, CLI, and MCP integration docs |
| [tapo](tapo/) | `github.com/mjenh/skills/tapo` | [v1.0.0](tapo/CHANGELOG.md#100---2026-07-02) | Tapo P100 smart plug control over the LAN; includes optional ADK adapter, CLI, and MCP integration docs |

## Install

```bash
go get github.com/mjenh/skills/weather@v1.0.1
go get github.com/mjenh/skills/tapo@v1.0.0
```

See each module's README for setup, API usage, and integration examples.

## Versioning

Modules in this monorepo are versioned independently with prefixed git tags. Merging a PR with a new `CHANGELOG.md` version section to `main` triggers automated tagging, GitHub Releases, and README updates (see [CONTRIBUTING.md](CONTRIBUTING.md#automated-releases)).

```bash
git tag weather/v1.0.0
git tag tapo/v1.0.0
git push origin weather/v1.0.0 tapo/v1.0.0
```

Consumers pin a specific module release:

```bash
go get github.com/mjenh/skills/weather@v1.0.1
```

Future modules follow the same pattern (`<module>/vX.Y.Z`).

## Repository layout

```
skills/
├── weather/              # Go module — weather lookups
│   ├── adk/              # Google ADK adapter (optional)
│   ├── cmd/weather/      # CLI
│   └── skill/            # MCP/agent integration docs
└── tapo/                 # Go module — Tapo P100 plug control
    ├── adk/              # Google ADK adapter (optional)
    ├── cmd/tapo/         # CLI
    └── skill/            # MCP/agent integration docs
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the development workflow, module layout, and release process.
