---
phase: 05-binary-distribution
plan: 06
subsystem: release-tooling
tags: [version-bump, plugin-manifest, build-cleanup]
dependency_graph:
  requires: ["05-01", "05-04"]
  provides: ["bump-version-script", "v4.0.0-manifests"]
  affects: ["main.go", "plugin.json", "marketplace.json", "start.sh", "start.ps1"]
tech_stack:
  added: []
  patterns: ["sed-based version replacement", "5-file coordinated update"]
key_files:
  created:
    - scripts/bump-version.sh
  modified:
    - .claude-plugin/plugin.json
    - .claude-plugin/marketplace.json
  deleted:
    - scripts/build.sh
decisions:
  - "bump-version.sh uses sed -i.bak with cleanup for cross-platform compat (macOS sed requires -i suffix)"
  - "Script strips v prefix from input to normalize (v4.1.0 -> 4.1.0) but adds v for start.sh/start.ps1 VERSION"
  - "Script prints git instructions but does NOT auto-commit per DIST-42"
  - "author.name stays DSR-Labs (brand), repository points to StrainReviews/dsrcode (actual repo)"
metrics:
  duration: "5m"
  completed: "2026-04-09T18:22:42Z"
  tasks_completed: 2
  tasks_total: 2
  files_changed: 4
---

# Phase 5 Plan 6: Version Bump Script & Plugin Manifest Update Summary

Coordinated version bump script for 5-file updates (main.go, plugin.json, marketplace.json, start.sh, start.ps1) with manifests set to v4.0.0 and repository migrated to StrainReviews/dsrcode

## Tasks Completed

### Task 1: Create bump-version.sh (5 files) + delete build.sh
**Commit:** `6d8eafa`

- Created `scripts/bump-version.sh` with executable permission
- Script takes a version argument (e.g., `4.1.0` or `v4.1.0`)
- Updates 5 files via sed: main.go (var version), plugin.json, marketplace.json, start.sh (VERSION), start.ps1 ($Version)
- Prints step-by-step git instructions for commit, tag, and push
- Does NOT auto-commit (per DIST-42)
- Deleted `scripts/build.sh` (superseded by GoReleaser per DIST-43)

### Task 2: Update plugin.json and marketplace.json to v4.0.0 + StrainReviews repo
**Commit:** `b0f40f3`

- Updated plugin.json version from 3.1.10 to 4.0.0
- Updated plugin.json repository from DSR-Labs/cc-discord-presence to StrainReviews/dsrcode
- Updated marketplace.json plugins[0].version from 3.1.10 to 4.0.0
- Both files validated as parseable JSON
- author.name stays DSR-Labs (brand identity), author.url stays github.com/DSR-Labs

## Deviations from Plan

None - plan executed exactly as written.

## Verification Results

| Check | Result |
|-------|--------|
| bash -n scripts/bump-version.sh | PASS (syntax valid) |
| test ! -f scripts/build.sh | PASS (deleted) |
| grep main.go bump-version.sh | PASS (6 references) |
| plugin.json version 4.0.0 | PASS |
| marketplace.json version 4.0.0 | PASS |
| plugin.json repo StrainReviews/dsrcode | PASS |
| Both JSON files parseable | PASS |

## Known Stubs

None.

## Self-Check: PASSED

- FOUND: scripts/bump-version.sh
- FOUND: .claude-plugin/plugin.json
- FOUND: .claude-plugin/marketplace.json
- FOUND: .planning/phases/05-binary-distribution/05-06-SUMMARY.md
- FOUND: commit 6d8eafa (Task 1)
- FOUND: commit b0f40f3 (Task 2)
