# skills

Importable Go modules for MCP servers, AI agent runtimes, and CLIs.

Each module lives in its own directory with an independent `go.mod` and semver releases tagged as `<module>/vX.Y.Z` (e.g. `weather/v1.0.0`).

## Modules

| Module | Import path | Latest | Description |
|--------|-------------|--------|-------------|
| [weather](weather/) | `github.com/mjenh/skills/weather` | [v1.0.0](weather/CHANGELOG.md#100---2026-07-02) | Current weather by location via Google Weather and Geocoding APIs; includes optional ADK adapter, CLI, and MCP integration docs |

## Install

```bash
go get github.com/mjenh/skills/weather@v1.0.0
```

See each module's README for setup, API usage, and integration examples.

## Versioning

Modules in this monorepo are versioned independently with prefixed git tags:

```bash
git tag weather/v1.0.0
git push origin weather/v1.0.0
```

Consumers pin a specific module release:

```bash
go get github.com/mjenh/skills/weather@v1.0.0
```

Future modules follow the same pattern (`<module>/vX.Y.Z`).

## Repository layout

```
skills/
└── weather/              # Go module — weather lookups
    ├── adk/              # Google ADK adapter (optional)
    ├── cmd/weather/      # CLI
    └── skill/            # MCP/agent integration docs
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the development workflow, module layout, and release process.
