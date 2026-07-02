## Summary

<!-- What this PR does and why -->

## Type of change

- [ ] Bug fix (patch version bump)
- [ ] New feature (minor version bump)
- [ ] Breaking change (major version bump)
- [ ] New module
- [ ] Changelog/docs only (no version bump required)
- [ ] Other (describe below)

## Module(s) affected

<!-- e.g. weather, tapo, or repository root -->

-

## BMad planning

<!-- Complete BMad planning artifacts before opening a PR for non-trivial work -->

- [ ] Not applicable (trivial change)
- [ ] BMad planning artifacts are linked or included

## Checklist

### All changes

- [ ] `go build ./...` passes inside the affected module directory(ies)
- [ ] `go vet ./...` passes inside the affected module directory(ies)

### Module code changes

<!-- Required when files under `<module>/` change (excluding CHANGELOG.md-only edits).
     CI validates the version in CHANGELOG.md against existing `<module>/v*` tags. -->

- [ ] `CHANGELOG.md` updated with a new `## [X.Y.Z] - YYYY-MM-DD` section
- [ ] Version follows [semver](https://semver.org/) and is greater than the latest tag

### New module

<!-- Complete when adding a new top-level module -->

- [ ] `go.mod` with module path `github.com/mjenh/skills/<module>`
- [ ] Core package free of ADK/MCP imports (adapters live in subpackages)
- [ ] `README.md` with install, setup, API, and integration examples
- [ ] `CHANGELOG.md` following [Keep a Changelog](https://keepachangelog.com/)
- [ ] Module added to the table in root [README.md](README.md)
- [ ] `.gitignore` and `.env.example` added if the module needs secrets or config

## Test plan

<!-- How you verified this change -->

-
