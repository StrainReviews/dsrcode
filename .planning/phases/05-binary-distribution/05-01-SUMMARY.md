---
phase: 05-binary-distribution
plan: 01
subsystem: core
tags: [rename, module, migration, goreleaser, identity]
dependency_graph:
  requires: []
  provides: [dsrcode-module-path, goreleaser-ldflags, config-migration]
  affects: [go.mod, main.go, config/config.go, all-import-paths]
tech_stack:
  added: []
  patterns: [copy-on-first-find-migration, goreleaser-default-ldflags]
key_files:
  created: []
  modified:
    - go.mod
    - go.sum
    - main.go
    - config/config.go
    - config/config_test.go
    - resolver/resolver.go
    - resolver/resolver_test.go
    - resolver/resolver_unique_test.go
    - resolver/stats.go
    - server/server.go
    - server/server_test.go
    - server/server_preview_test.go
    - discord/backoff_test.go
    - preset/preset_test.go
    - main_test.go
    - analytics/compaction_test.go
    - analytics/context_test.go
    - analytics/persist_test.go
    - analytics/pricing_test.go
    - analytics/subagent_test.go
    - analytics/tokens_test.go
    - analytics/tools_test.go
    - analytics/tracker_test.go
    - session/registry_analytics_test.go
    - session/registry_dedup_test.go
    - session/registry_test.go
decisions:
  - "GoReleaser default ldflags convention: lowercase version/commit/date vars"
  - "Copy-on-first-find migration: read old -> write new -> log warning -> return new path"
  - "Fail-safe fallback: if copy fails, return old path (user not locked out)"
  - "Log file has no migration (append-only, new sessions use new path)"
metrics:
  duration: 23m
  completed: "2026-04-09T17:17:28Z"
  tasks: 2
  files: 26
---

# Phase 5 Plan 1: Go Module Rename + Identity + Config Migration Summary

Go module renamed from tsanva/cc-discord-presence to StrainReviews/dsrcode with GoReleaser-compatible version ldflags and copy-on-first-find config file migration

## Tasks Completed

| Task | Name | Commit | Key Changes |
|------|------|--------|-------------|
| 1 | Go module rename + import path update | 7e4b6d8 | Module path updated in go.mod + 46 import occurrences across 23 Go files |
| 2 (RED) | Failing tests for rename/migration | f1c2ea7 | 4 new config tests: new install path, migration copy, both-exist, log path |
| 2 (GREEN) | Version refactor + file rename + migration | 3fdb22d | GoReleaser ldflags, dsrcode identity, runtime file rename, config copy-on-first-find |

## Key Implementation Details

### Go Module Rename (Task 1)
- `go mod edit -module github.com/StrainReviews/dsrcode` updated go.mod
- sed replaced all 46 occurrences of `github.com/tsanva/cc-discord-presence` across 23 files
- Test fixture data `StrainReviewsScanner` in resolver_unique_test.go correctly preserved (not an import path)
- `go mod tidy` regenerated go.sum; `go build ./...` and `go vet ./...` pass clean

### Version Variable Refactor (Task 2)
- Replaced `var Version = "3.2.0"` (exported, single var) with lowercase var block:
  ```go
  var (
      version = "dev"
      commit  = "none"
      date    = "unknown"
  )
  ```
- GoReleaser default ldflags (`-X main.version={{.Version}}`) work without custom config
- `--version` now prints: `dsrcode dev (none, unknown)` format
- All `Version` references in main.go (slog, server.NewServer) updated to `version`

### Runtime File Rename (Task 2)
- `discord-presence-data.json` -> `dsrcode-data.json` (statusline data)
- `discord-presence-analytics` -> `dsrcode-analytics` (analytics directory)
- `discord-presence.log` -> `dsrcode.log` (log file, no migration needed)
- fsnotify watcher comparison updated to match new data file name

### Config Migration (Task 2)
- `DefaultConfigPath()` implements copy-on-first-find per DIST-30:
  1. Check for new `dsrcode-config.json` -- return immediately if found
  2. Check for old `discord-presence-config.json` -- if found, copy to new name
  3. Old file kept as backup (not deleted)
  4. If copy fails, return old path (fail-safe, not fail-hard)
  5. Fresh installs get new name directly
- Log warning on successful migration and on migration failure
- 4 new tests verify all paths: new install, migration copy, both-exist, log path

## Deviations from Plan

None - plan executed exactly as written.

## Verification Results

All 7 plan verification checks passed:
1. `go build ./...` - exits 0
2. `go vet ./...` - exits 0
3. `go test ./... -count=1 -short` - all 10 packages pass
4. `grep -r "tsanva/cc-discord-presence" --include="*.go" .` - zero results
5. `grep "dsrcode-config.json" config/config.go` - matches found
6. `grep "discord-presence-config.json" config/config.go` - backward compat present
7. `grep "os.WriteFile" config/config.go` - copy-on-first-find confirmed

## Self-Check: PASSED

All created files exist. All 3 commits verified in git log.
