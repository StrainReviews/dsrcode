---
phase: 7
plan: 07-05
subsystem: release
tags: [release, changelog, version-bump, validation]
requires: [07-01, 07-02, 07-03, 07-04]
provides: [v4.1.2-release-artifacts, nyquist-validation]
affects: [main.go, .claude-plugin/plugin.json, .claude-plugin/marketplace.json, scripts/start.sh, scripts/start.ps1, CHANGELOG.md, .planning/phases/07-fix-daemon-auto-exit-bugs-pid-dead-check-mcp-activity-tracki/07-VALIDATION.md]
tech-stack:
  added: []
  patterns: [single-source-version-bump-via-script, keep-a-changelog-1.1.0]
key-files:
  created:
    - .planning/phases/07-fix-daemon-auto-exit-bugs-pid-dead-check-mcp-activity-tracki/07-05-SUMMARY.md
  modified:
    - main.go
    - .claude-plugin/plugin.json
    - .claude-plugin/marketplace.json
    - scripts/start.sh
    - scripts/start.ps1
    - CHANGELOG.md
    - .planning/phases/07-fix-daemon-auto-exit-bugs-pid-dead-check-mcp-activity-tracki/07-VALIDATION.md
decisions: [D-13, D-14, D-15]
metrics:
  duration: ~15min
  completed: 2026-04-13
  task_count: 3
  file_count: 8
  commits: 2
---

# Phase 7 Plan 05: Release v4.1.2 Summary

**One-liner:** Land v4.1.2 hotfix — propagate version constant via `bump-version.sh` to 5 canonical files, add Keep-a-Changelog-style v4.1.2 entry with cross-referenced D-IDs and upstream issue links, and finalize Nyquist-compliant Per-Task Verification Map. Tag/push deferred to user per CLAUDE.md §Releasing.

## Tasks Completed

| # | Task | Commit |
|---|------|--------|
| 07-05-01 | Version bump to v4.1.2 (5 files via `bump-version.sh`) | `3bc35d9` |
| 07-05-02 | CHANGELOG v4.1.2 entry + 07-VALIDATION.md Per-Task Verification Map finalization | `4e376e7` |
| 07-05-03 | Manual cross-platform verification | **🟡 PENDING USER** (Truth 5/6/7) |

## What Shipped

### Version Propagation (Task 07-05-01)
`./scripts/bump-version.sh 4.1.2` updated all 5 canonical files in a single sed pass:
- `main.go` — `version = "4.1.2"` (inside grouped var block; gofmt-safe leading whitespace preserved)
- `.claude-plugin/plugin.json` — `"version": "4.1.2"`
- `.claude-plugin/marketplace.json` — `"version": "4.1.2"`
- `scripts/start.sh` — `VERSION="v4.1.2"`
- `scripts/start.ps1` — `$Version = "v4.1.2"`

JSON validity confirmed (`node -e JSON.parse`). `go build` green.

### CHANGELOG v4.1.2 Section (Task 07-05-02)
Inserted between `## [Unreleased]` and `## [4.1.1]` per Keep-a-Changelog 1.1.0:

**Fixed (4 bullets):**
1. Daemon self-termination during long MCP-heavy sessions — Bug #2 (D-04/D-05)
2. PID-liveness check false-positives for HTTP-sourced sessions — Bug #1 (D-01/D-02)
3. Refcount drift via missing SessionEnd command hook — Bug #3 (D-07/D-09), with upstream issue links to [anthropics/claude-code#17885](https://github.com/anthropics/claude-code/issues/17885) (SessionEnd hook doesn't fire on `/exit`) and [anthropics/claude-code#16288](https://github.com/anthropics/claude-code/issues/16288) (plugin hooks not loaded from external `hooks.json`).
4. Log overwrite on daemon restart — Bug #4 (D-10/D-11/D-12), with [PowerShell/PowerShell#15031](https://github.com/PowerShell/PowerShell/issues/15031) link explaining the `Start-Process` no-append constraint that motivated the rotation pattern.

**Changed (1 bullet):**
- Unix log redirection now splits stdout/stderr (`dsrcode.log` + `dsrcode.log.err`) and uses append mode to match Windows behavior shipped in v4.1.1.

### Nyquist Validation (Task 07-05-02)
`07-VALIDATION.md` Per-Task Verification Map filled with 16 concrete Task IDs (07-01-01..07-05-03), all Wave-1 rows marked ✅ green based on the SUMMARY artifacts of plans 07-01..07-04. Frontmatter flipped: `status: draft → final`, `nyquist_compliant: false → true`.

## Automated Must-Haves Verified

| Truth | Check | Result |
|-------|-------|--------|
| **Truth 1** | `grep -l '4.1.2' main.go .claude-plugin/plugin.json .claude-plugin/marketplace.json scripts/start.sh scripts/start.ps1 \| wc -l` returns 5 | ✅ PASS (5) |
| **Truth 2** | CHANGELOG has `## [4.1.2]` with 4 Fixed + 1 Changed bullet | ✅ PASS |
| **Truth 3** | CHANGELOG references each fix by D-ID + upstream issues #17885/#16288 | ✅ PASS |
| **Truth 4** | VALIDATION map has all task IDs + `nyquist_compliant: true` | ✅ PASS (16 task-ID rows) |
| **Artifact 1** | All 5 version files updated | ✅ PASS |
| **Artifact 2** | CHANGELOG.md `## [4.1.2]` section | ✅ PASS |
| **Artifact 3** | 07-VALIDATION.md Per-Task Verification Map filled | ✅ PASS |
| **Key link** | `bump-version.sh` is the ONE TRUE WAY (no hand-edits) | ✅ PASS |

Build/test gate:
- `go build -o /tmp/dsrcode-412 .` → exit 0
- `go test ./...` → all packages OK (cached green)
- JSON validity (plugin.json + marketplace.json) → exit 0

## Manual Checkpoints Pending (Task 07-05-03)

Per `<resume-signal>` in 07-05-PLAN.md, user must run and report results:

| Truth | What to verify | Platform |
|-------|---------------|----------|
| **Truth 5** | Bug #1 + #2 — 30+ min MCP-only session (sequential-thinking + exa + context7 + crawl4ai). Daemon process stays alive; refcount stays at 1; Discord presence stays visible. | Windows critical (also macOS/Linux nice-to-have) |
| **Truth 6** | Bug #3 SessionEnd → refcount decrement on `/exit`, `/clear`, VS Code close-button, `/resume`. PID-file cleanup on macOS/Linux. | Windows + macOS/Linux |
| **Truth 7** | Bug #4 log rotation: synthetic 11 MB `~/.claude/dsrcode.log` rotates to `.log.1` on daemon restart; second rotation overwrites `.log.1` (no `.log.2`); same for `.log.err`. Bonus: stderr fix smoke (Windows) — error trace lands in `.log.err`, not `.log`. | All platforms |

If any FAIL, do NOT tag v4.1.2 — open a follow-up plan.

## Deviations from Plan

**Auto-fixed Issues:**
- **[Rule 3 — blocking issue] `go test -race` requires CGO on Windows.** The plan's pre-flight specified `go test -race ./...`. The Windows dev machine has no cgo toolchain (`CGO_ENABLED=1` not configured). Fell back to `go test ./...` (no race) which all 10 packages passed (cached). Documented here as the race-detector check is properly handled on Linux CI per `.github/workflows/*`. Out-of-scope to install MinGW for this hotfix release.

**Folded-in scope:**
- The CHANGELOG `Fixed` bullet for Bug #4 absorbs the start.ps1 stderr same-path defect fix as a sub-clause (per Plan §Notes "5th bullet folded into Bug #4"). The plan's pre-drafted text had it as a separate 5th bullet; consolidating into Bug #4's bullet keeps the 4-bullets-per-bug shape per Truth 2.

**Out-of-scope (deliberately deferred):**
- Tagging `v4.1.2` and `git push origin main --tags` — explicitly user-gated per CLAUDE.md §Releasing AND per Plan §Notes "Did NOT tag v4.1.2 in this plan".

## MCP Citations

Per Phase-7 user mandate (PRE+POST MCP round per task):

- **`mcp__sequential-thinking__sequentialthinking`** — Decomposed Task 2 into (A) CHANGELOG edit, (B) VALIDATION frontmatter+table edit, (C) commit. Confirmed pre-drafted bullets from Plan 07-05 §Step A could be used verbatim with minor consolidation per `<notes>` "5th bullet folded into Bug #4".
- **`mcp__github__get_issue` × 3** — Confirmed live issue titles/states for CHANGELOG accuracy:
  - `anthropics/claude-code#17885` (closed-stale, "SessionEnd hook doesn't fire on /exit command")
  - `anthropics/claude-code#16288` (open, bug, "Plugin hooks not loaded from external hooks.json file")
  - `PowerShell/PowerShell#15031` (closed, "Start-Process: Provide the ability to append when redirecting stdout/stderr streams to file")
  - All three URLs live and resolve; titles match the wording used in CHANGELOG.

## Release Execution (FOR USER, after manual verifications pass)

Per CLAUDE.md §Releasing — Plan 07-05 deliberately stops short of these:

```bash
git log -3 --stat                        # confirm last commits
git tag v4.1.2
git push origin main --tags              # triggers GoReleaser CI (5 platform binaries)
gh release view v4.1.2                   # confirm release published
```

Reminder from `~/.claude/.../MEMORY.md`: never push >3 new tags at once (GitHub silently drops tag webhook events). v4.1.2 is a single tag — safe.

## Self-Check: PASSED

- ✅ `CHANGELOG.md` modified — `grep '## \[4.1.2\]'` returns 1 line
- ✅ `07-VALIDATION.md` modified — `grep 'nyquist_compliant: true'` matches
- ✅ All 5 version files at v4.1.2 — `grep -l '4.1.2' [...] | wc -l` returns 5
- ✅ Commit `3bc35d9` exists in `git log`
- ✅ Commit `4e376e7` exists in `git log`
- ✅ `07-05-SUMMARY.md` created at `.planning/phases/07-fix-daemon-auto-exit-bugs-pid-dead-check-mcp-activity-tracki/07-05-SUMMARY.md`
