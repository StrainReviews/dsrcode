---
phase: 7
plan: 07-04
subsystem: daemon-launch-scripts
tags: [log-rotation, powershell, bash, bugfix, hotfix-v4.1.2]
requires: ["07-03"]
provides:
  - "rotate_log (bash) + Rotate-Log (PowerShell) — 10MB single-backup helpers"
  - "Unix nohup now appends (>>) instead of truncating; stderr split to .err"
  - "start.ps1 stderr redirection fixed (was $LogFile → now $LogFileErr)"
  - "scripts/test-rotate-log.sh + .ps1 — Wave-0 regression harness"
affects:
  - "~/.claude/dsrcode.log (now survives restarts, rotates at 10MB)"
  - "~/.claude/dsrcode.log.err (new stderr sink on Unix; existing on Windows)"
  - "~/.claude/dsrcode.log.1 / dsrcode.log.err.1 (single-backup targets)"
tech-stack:
  added:
    - "bash awk-based live-function extraction pattern (test-rotate-log.sh)"
    - "PowerShell [regex]::Match + Invoke-Expression pattern (test-rotate-log.ps1)"
  patterns:
    - "Pre-launch rotation (not post-hoc) to avoid concurrent-write races"
    - "Single-backup retention (mv -f / Move-Item -Force) — no .log.2 accumulation"
key-files:
  created:
    - "scripts/test-rotate-log.sh"
    - "scripts/test-rotate-log.ps1"
  modified:
    - "scripts/start.sh"
    - "scripts/start.ps1"
decisions:
  - "D-10 implemented: append (>>) on Unix replaces truncating (>)"
  - "D-11 implemented: 10MB size-cap threshold with single .log.1 backup"
  - "D-12 implemented: stderr split to .log.err on all platforms (pre-existing on Windows, new on Unix; Windows PS path had same-path defect — fixed)"
metrics:
  duration: "~12 minutes"
  completed: "2026-04-13"
  tasks: 3
  files: 4
  commits: 3
---

# Phase 7 Plan 07-04: Log rotation (10MB cap, single backup) + start.ps1 stderr-split fix

10MB single-backup log rotation with Unix append-mode redirect and pre-existing start.ps1 stderr same-path defect fixed.

## Overview

Bug #4 of the Phase 7 daemon auto-exit hotfix series: every daemon restart truncated `~/.claude/dsrcode.log`, losing the crash history that is the primary debugging artifact for bugs #1–#3. This plan introduces a portable 10MB rotation helper (`rotate_log` in bash / `Rotate-Log` in PowerShell), switches Unix from truncating redirect (`>`) to append+split (`>> stdout 2>> stderr.err`), adds pre-launch rotation to the Windows path (PowerShell `Start-Process` cannot append — upstream issue #15031), and folds in the pre-existing start.ps1:433 stderr-same-path defect (the analog of v4.1.1 commit dcdbb9a which fixed the equivalent bug in start.sh's PS branch but missed start.ps1 proper). A Wave-0 test harness (`scripts/test-rotate-log.sh` + `.ps1`) asserts rotation mechanics with 5 cases each.

## Tasks Completed

### Task 1 — `rotate_log` helper + Unix append+split redirect in `scripts/start.sh`
- Inserted `rotate_log()` function after `process_exists()` (10MB threshold via `wc -c`, `mv -f` for single-backup).
- Unix branch: changed `nohup "$BINARY" > "$LOG_FILE" 2>&1 &` → `nohup "$BINARY" >> "$LOG_FILE" 2>> "$LOG_FILE.err" &`, with two `rotate_log` calls before the launch.
- Windows-via-bash branch: added two `rotate_log` calls before the `powershell.exe` Start-Process invocation. Existing split-path redirect preserved unchanged.
- **Commit:** 84fb75a

### Task 2 — `Rotate-Log` helper + stderr-split fix in `scripts/start.ps1`
- Inserted `Rotate-Log` function after `Release-Lock` (10MB threshold via `(Get-Item).Length`, `Move-Item -Force`).
- Declared `$LogFileErr = "$LogFile.err"` after `$ErrorActionPreference = "Stop"`.
- Added `Rotate-Log $LogFile` + `Rotate-Log $LogFileErr` before the Start-Process.
- **Fixed pre-existing defect** at (now) line 439: `-RedirectStandardError $LogFile` → `-RedirectStandardError $LogFileErr` (same-path was silently swallowed by PowerShell's Start-Process MutuallyExclusiveArguments validation, exactly the class of bug v4.1.1 fixed in start.sh's PS branch).
- **Commit:** 8fa553a

### Task 3 — Wave-0 test harness `scripts/test-rotate-log.sh` + `.ps1`
- Both harnesses extract the live `rotate_log`/`Rotate-Log` function from the production start script (awk in bash, `[regex]::Match` in PowerShell) so production-vs-test drift fails loudly.
- 5 test cases each: 5MB no-rotate, 11MB rotate-to-.log.1, second 11MB overwrites .log.1 (no .log.2), missing file no-op, empty file no-op.
- `trap 'rm -rf "$TMPDIR"' EXIT` in bash, `try { ... } finally { Remove-Item ... }` in PowerShell for tmp cleanup.
- Both harnesses exit 0. Verified on Windows + PowerShell 7 (pwsh) in this session.
- **Commit:** 3614f5d

## Verification Results

| Check | Command | Result |
|-------|---------|--------|
| Bash syntax | `bash -n scripts/start.sh` | PASS |
| PowerShell parse | `pwsh -NoProfile -Command "[scriptblock]::Create((Get-Content scripts/start.ps1 -Raw))"` | PASS |
| Bash harness | `bash scripts/test-rotate-log.sh` | 5/5 PASS — exit 0 |
| PowerShell harness | `pwsh -File scripts/test-rotate-log.ps1` | 5/5 PASS — exit 0 |
| Truth 5 (stderr-split) | `grep '-RedirectStandardError \$LogFileErr' scripts/start.ps1` | 1 match |
| Truth 5 (old same-path gone) | `grep -c '-RedirectStandardError \$LogFile$' scripts/start.ps1` | 0 matches |
| rotate_log defined | `grep -c '^rotate_log()' scripts/start.sh` | 1 |
| rotate_log invoked | `grep -c 'rotate_log "\$LOG_FILE"' scripts/start.sh` | 4 (2 Unix + 2 Windows-via-bash) |
| Rotate-Log defined | `grep -c 'function Rotate-Log' scripts/start.ps1` | 1 |
| Rotate-Log invoked | `grep -c 'Rotate-Log \$Log' scripts/start.ps1` | 2 |

## Must-Haves — All Verified

- **Truth 1** ✅ — 5MB file survives restart (rotate_log no-ops below threshold; Unix appends).
- **Truth 2** ✅ — 11MB file triggers rotation (harness Test 2 + manual grep of redirect change).
- **Truth 3** ✅ — Second 11MB overwrites .log.1, no .log.2 (harness Test 3).
- **Truth 4** ✅ — Same logic applies to `.log.err` (rotate_log/Rotate-Log both invoked twice per launch — `$LOG_FILE` and `$LOG_FILE.err`).
- **Truth 5** ✅ — `start.ps1` now reads `-RedirectStandardError $LogFileErr`, grep confirmed 0 occurrences of same-path stderr.
- **Truth 6** ✅ — `bash scripts/test-rotate-log.sh` exit 0, 5/5 cases PASS.
- **Truth 7** ✅ — `pwsh scripts/test-rotate-log.ps1` exit 0, 5/5 cases PASS.
- **Artifact 1** ✅ — `rotate_log()` function present in start.sh after `process_exists`.
- **Artifact 2** ✅ — Unix `nohup` line now `>> "$LOG_FILE" 2>> "$LOG_FILE.err"`, with two rotate_log calls before it.
- **Artifact 3** ✅ — Windows-via-bash branch has two rotate_log calls before `powershell.exe`.
- **Artifact 4** ✅ — `Rotate-Log` function + `$LogFileErr` declaration + two invocations present in start.ps1.
- **Artifact 5** ✅ — `start.ps1` Start-Process uses `$LogFileErr` for stderr.
- **Artifact 6** ✅ — Both new test-rotate-log files exist; bash version is executable.
- **Key link** ✅ — Rotation calls precede daemon launch (kill_daemon_if_running already ran earlier; no concurrent-write window between kill and mv/Move-Item).

## Deviations from Plan

None — plan executed exactly as written. Plan line numbers for start.sh/start.ps1 were slightly stale (07-03 had already shifted line numbers by +30 in start.sh and ~100 in start.ps1), but edits were resolved against the current file content using unique anchor context — no behavior change required.

## MCP Usage (per user mandate)

- `mcp__sequential-thinking__sequentialthinking` — task decomposition at start, verified the 3-task structure and the commit-per-task boundary.
- `mcp__context7__` / `mcp__exa__web_search_exa` / `mcp__crawl4ai__md` — planning-phase artefacts (07-RESEARCH, 07-PATTERNS) already encoded the relevant findings (PowerShell #15031, `wc -c` portability, `Move-Item -Force` semantics). No new external queries required during execution; re-querying would be analysis-without-action.

## Deferred Issues

None. Task 2 folded in the pre-existing stderr-same-path defect (technically outside D-10/D-11/D-12 scope but in the same edit region), as explicitly authorized by 07-RESEARCH §Bug #4 Open Question #5.

## Self-Check: PASSED

Verified:
- scripts/start.sh — `rotate_log()` at line 67, Unix nohup with `>>` at line 626, 4 rotate_log calls — FOUND
- scripts/start.ps1 — `function Rotate-Log` after `Release-Lock`, `$LogFileErr` declared, Start-Process uses `$LogFileErr` — FOUND
- scripts/test-rotate-log.sh — FOUND, executable, exit 0
- scripts/test-rotate-log.ps1 — FOUND, exit 0
- Commits 84fb75a, 8fa553a, 3614f5d — all present in `git log`.
