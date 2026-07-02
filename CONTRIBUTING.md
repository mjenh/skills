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

### Version policy (author-driven, CI-enforced)

You choose the semver in the PR. Automation reads the version from `CHANGELOG.md` and validates it — it does not guess bump levels from commit messages.

1. **Declare the version explicitly** — add a dated section to `<module>/CHANGELOG.md`:
   ```markdown
   ## [1.0.1](https://github.com/mjenh/skills/releases/tag/tapo/v1.0.1) - 2026-07-02

   ### Fixed
   - Describe the change
   ```
2. **Pick the bump** using semver rules above (patch for fixes, minor for backward-compatible features, major for breaking changes).
3. **Must be greater than the latest tag** — CI compares your new section against existing `<module>/v*` tags. Reusing or downgrading a version fails the PR check.
4. **Required when code changes** — if files under `<module>/` change (excluding `CHANGELOG.md`), the PR must also update `CHANGELOG.md` with a new untagged version section.
5. **Changelog-only PRs are allowed** — typo fixes or release-note edits without a version bump pass; a new release only happens when an untagged version section is present.

Any directory with a `go.mod` is treated as an independent module automatically.

## Automated releases

Two GitHub Actions workflows handle validation and publishing:

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| `changelog-check.yml` | Pull request → `main` | Ensures module changes include a valid CHANGELOG version bump |
| `release-module.yml` | Push → `main` | Tags, creates GitHub Releases, and updates root `README.md` |

### Release flow

1. Open a PR that changes module code and adds a new `## [X.Y.Z] - YYYY-MM-DD` section to `<module>/CHANGELOG.md`.
2. Merge to `main` after the changelog check passes.
3. On merge, `release-module.yml`:
   - Creates git tag `<module>/vX.Y.Z`
   - Opens a [GitHub Release](https://docs.github.com/en/repositories/releasing-projects-on-github) with notes from the CHANGELOG section
   - Commits an update to the root `README.md` modules table and `go get` examples (`[skip ci]` avoids a loop)

Consumers can then:

```bash
go get github.com/mjenh/skills/tapo@v1.0.1
```

No manual tagging is required for routine releases.

## Code conventions

- Pure library code uses `context.Context` and explicit configuration (no hidden `os.Getenv` in core packages)
- Prefer structured return types over formatted strings in the library; format at the adapter/CLI layer
- Use typed errors for domain failures consumers may handle
- Expose test hooks via functional options (e.g. overridable base URLs, injectable HTTP client)
- Match existing module layout and naming before introducing new patterns

## Pull requests

1. Complete BMad planning artifacts for the change before opening a PR for non-trivial work
2. Ensure `go build ./...` and `go vet ./...` pass inside the module directory
3. Update `CHANGELOG.md` with a new `## [X.Y.Z] - YYYY-MM-DD` section (automation updates root `README.md` on release)
4. Add new modules to the root `README.md` modules table manually (version links are updated automatically on release)
