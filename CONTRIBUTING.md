# Contributing

Thank you for contributing to the Go modules in this repository.

## Development workflow

Use the **BMad Method** from planning through implementation. Do not jump straight to code for new modules or significant features — work through the BMad workflow first:

1. **Planning** — product brief, PRD, architecture, epics/stories as appropriate
2. **Implementation** — dev story workflow with test-first discipline
3. **Review** — code review and course correction when needed

BMad skills and artifacts are maintained internally in this repository. Invoke `bmad-help` from your agent environment for routing and next steps.

## Adding a new module

Each module is a self-contained Go package under its own directory:

```
<module>/
├── go.mod
├── README.md
├── CHANGELOG.md
├── .gitignore
├── .env.example          # if the module needs secrets or config
├── doc.go
├── ...                   # package source files
├── adk/                  # optional Google ADK adapter
├── cmd/<module>/         # optional CLI
└── skill/                # optional MCP/agent integration docs
    ├── SKILL.md
    └── tool.schema.json
```

### Checklist

- [ ] Create `go.mod` with module path `github.com/mjenh/skills/<module>`
- [ ] Keep the core package free of ADK/MCP imports — adapters live in subpackages
- [ ] Add `README.md` with install, setup, API, and integration examples
- [ ] Add `CHANGELOG.md` following [Keep a Changelog](https://keepachangelog.com/)
- [ ] Add the module to the table in the root [README.md](README.md)
- [ ] Tag the release with a prefixed semver tag (see below)

## Versioning

Modules are versioned **independently** within this monorepo using prefixed git tags:

```bash
git tag <module>/v1.0.0
git push origin <module>/v1.0.0
```

Examples:

```bash
git tag weather/v1.0.0
go get github.com/mjenh/skills/weather@v1.0.0
```

Follow [Semantic Versioning](https://semver.org/):

| Change | Tag bump |
|--------|----------|
| Backward-compatible API additions or fixes | `v1.x.y` patch/minor |
| Breaking API changes | major bump; use `/v2` module path if needed |

## Code conventions

- Pure library code uses `context.Context` and explicit configuration (no hidden `os.Getenv` in core packages)
- Prefer structured return types over formatted strings in the library; format at the adapter/CLI layer
- Use typed errors for domain failures consumers may handle
- Expose test hooks via functional options (e.g. overridable base URLs, injectable HTTP client)
- Match existing module layout and naming before introducing new patterns

## Pull requests

1. Complete BMad planning artifacts for the change before opening a PR for non-trivial work
2. Ensure `go build ./...` and `go vet ./...` pass inside the module directory
3. Update `CHANGELOG.md` under `[Unreleased]` or the target version section
4. Update the root `README.md` modules table when adding a new module or releasing a version
