---
phase: 05-binary-distribution
plan: "03"
subsystem: ci-cd
tags: [github-actions, goreleaser, ci, testing, linting]
dependency_graph:
  requires: ["05-02"]
  provides: ["release-pipeline", "test-pipeline"]
  affects: [".github/workflows/release.yml", ".github/workflows/test.yml"]
tech_stack:
  added: ["goreleaser-action@v7", "golangci-lint-action@v7"]
  patterns: ["go-version-file", "minimal-permissions"]
key_files:
  created: [".github/workflows/test.yml"]
  modified: [".github/workflows/release.yml"]
decisions:
  - "goreleaser-action@v7 with version ~> v2 for GoReleaser semver range"
  - "go-version-file: go.mod in both workflows instead of hardcoded Go version"
  - "workflow_dispatch on release.yml for manual release trigger"
  - "golangci-lint-action@v7 with version: latest for PR linting"
  - "Minimal permissions: contents:write for release, contents:read for test"
metrics:
  duration_seconds: 244
  completed: "2026-04-09T17:43:35Z"
  tasks_completed: 2
  tasks_total: 2
  files_changed: 2
---

# Phase 5 Plan 03: GitHub Actions CI/CD Pipelines Summary

GoReleaser release pipeline replacing manual go build, plus new PR test workflow with vet, test, and golangci-lint

## What Was Done

### Task 1: Replace release.yml with GoReleaser pipeline
- **Commit:** `1ced0f0`
- **File:** `.github/workflows/release.yml`
- Replaced manual `go build` cross-compilation + `softprops/action-gh-release@v2` with `goreleaser/goreleaser-action@v7`
- Added `fetch-depth: 0` (required for GoReleaser changelog and version detection)
- Switched from hardcoded `go-version: '1.26'` to `go-version-file: go.mod`
- Added `workflow_dispatch` trigger for manual releases from GitHub UI
- Uses `version: "~> v2"` semver range (resolves to GoReleaser v2.15+)
- Uses `--clean` flag (v2 replacement for `--rm-dist`)
- `GITHUB_TOKEN` auto-provisioned by GitHub Actions with `contents: write` scope

### Task 2: Create test.yml for PR validation
- **Commit:** `7139d7a`
- **File:** `.github/workflows/test.yml` (new)
- Triggers on pull requests to main and pushes to main
- Runs `go vet ./...` for static analysis
- Runs `go test ./... -count=1 -race -v` for thorough testing (-count=1 disables cache, -race enables race detector)
- Runs `golangci/golangci-lint-action@v7` which reads `.golangci.yml` from repo root
- Uses `go-version-file: go.mod` consistent with release workflow
- Minimal permissions: `contents: read`

## Decisions Made

| Decision | Rationale |
|----------|-----------|
| goreleaser-action@v7 with `~> v2` | Latest official action (Node 24 + ESM), semver range gets v2.15+ without breaking on v3 |
| go-version-file: go.mod | Single source of truth for Go version; existing workflow had wrong hardcoded version (1.26 vs go.mod's 1.25) |
| workflow_dispatch on release | Per DIST-14: enables manual releases from GitHub UI without pushing a tag |
| golangci-lint-action@v7 | Official action, reads .golangci.yml config from repo root per DIST-16 |
| Minimal permissions blocks | contents:write for release (needed to create releases), contents:read for test (read-only) |
| -count=1 -race -v test flags | Disable caching in CI, enable race detector, verbose output for debugging failures |

## Deviations from Plan

None - plan executed exactly as written.

## Commits

| # | Hash | Message | Files |
|---|------|---------|-------|
| 1 | `1ced0f0` | feat(05-03): replace release.yml with GoReleaser pipeline | `.github/workflows/release.yml` |
| 2 | `7139d7a` | feat(05-03): create test.yml for PR validation pipeline | `.github/workflows/test.yml` |

## Verification Results

- release.yml contains `goreleaser/goreleaser-action@v7` -- PASS
- release.yml contains `version: "~> v2"` -- PASS
- release.yml contains `args: release --clean` -- PASS
- release.yml contains `fetch-depth: 0` -- PASS
- release.yml contains `go-version-file: go.mod` -- PASS
- release.yml contains `workflow_dispatch` trigger -- PASS
- release.yml does NOT contain `softprops/action-gh-release` -- PASS
- release.yml does NOT contain hardcoded `go-version: '1.26'` -- PASS
- test.yml contains `pull_request:` and `push:` triggers on `branches: [main]` -- PASS
- test.yml contains `go vet ./...` -- PASS
- test.yml contains `go test ./... -count=1 -race -v` -- PASS
- test.yml contains `golangci/golangci-lint-action@v7` -- PASS
- test.yml contains `go-version-file: go.mod` -- PASS
- test.yml contains `permissions: contents: read` -- PASS

## Requirements Coverage

| Requirement | Status | How |
|-------------|--------|-----|
| DIST-09 | Covered | goreleaser-action@v7 with version ~> v2 in release.yml |
| DIST-10 | Covered | go-version-file: go.mod in both workflows |
| DIST-14 | Covered | workflow_dispatch trigger on release.yml |
| DIST-15 | Covered | test.yml with go vet, go test, golangci-lint on PRs |
| DIST-16 | Covered | golangci-lint-action@v7 reads .golangci.yml from repo root |

## Self-Check: PASSED

- [x] `.github/workflows/release.yml` exists
- [x] `.github/workflows/test.yml` exists
- [x] `.planning/phases/05-binary-distribution/05-03-SUMMARY.md` exists
- [x] Commit `1ced0f0` exists (Task 1)
- [x] Commit `7139d7a` exists (Task 2)
