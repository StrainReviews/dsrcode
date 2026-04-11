---
phase: quick-260411-iyf
plan: 01
subsystem: scripts
tags: [windows, logging, daemon, start.sh, quick-fix]
requirements:
  - QUICK-260411-IYF
dependency_graph:
  requires: []
  provides:
    - "Windows daemon stdout → ~/.claude/dsrcode.log"
    - "Windows daemon stderr → ~/.claude/dsrcode.log.err"
  affects:
    - scripts/start.sh
tech_stack:
  added: []
  patterns:
    - "cygpath -w path conversion for PowerShell consumption"
    - "Start-Process -RedirectStandardOutput/-RedirectStandardError with disjoint paths (MutuallyExclusiveArguments workaround)"
key_files:
  created: []
  modified:
    - scripts/start.sh
decisions:
  - "stderr routes to ${WIN_LOG_FILE}.err (sibling file) because PowerShell Start-Process rejects same-path for both streams"
metrics:
  duration: "~5 min"
  completed: "2026-04-11"
  tasks: 1
  files: 1
  commits: 1
---

# Quick Task 260411-iyf: Fix Windows Daemon Launch Log Redirect Summary

One-liner: Added `-RedirectStandardOutput`/`-RedirectStandardError` + new `WIN_LOG_FILE` (via cygpath -w) to the Windows Start-Process call in start.sh so dsrcode's slog output and crashes are captured to `~/.claude/dsrcode.log` / `.log.err` instead of vanishing.

## Context

The Windows launch branch in `scripts/start.sh` (lines 570-579) used `powershell.exe ... Start-Process ... -WindowStyle Hidden -PassThru` with NO stdout/stderr redirection. dsrcode's Go `slog` writes to stdout, so on Windows all daemon output disappeared into the void — crashes became silently invisible. The Unix branch already used `nohup "$BINARY" > "$LOG_FILE" 2>&1 &`. This fix brings the Windows branch to parity.

## Changes

### Modified: `scripts/start.sh`

**Lines 570-576** (before):
```bash
if $IS_WINDOWS; then
    WIN_BINARY=$(cygpath -w "$BINARY" 2>/dev/null || echo "$BINARY")
    WIN_PID_FILE=$(cygpath -w "$PID_FILE" 2>/dev/null || echo "$PID_FILE")

    powershell.exe -NoProfile -WindowStyle Hidden -Command \
        '$process = Start-Process -FilePath "'"$WIN_BINARY"'" -WindowStyle Hidden -PassThru; $process.Id | Out-File -FilePath "'"$WIN_PID_FILE"'" -Encoding ASCII -NoNewline' 2>/dev/null
```

**After**:
```bash
if $IS_WINDOWS; then
    WIN_BINARY=$(cygpath -w "$BINARY" 2>/dev/null || echo "$BINARY")
    WIN_PID_FILE=$(cygpath -w "$PID_FILE" 2>/dev/null || echo "$PID_FILE")
    WIN_LOG_FILE=$(cygpath -w "$LOG_FILE" 2>/dev/null || echo "$LOG_FILE")

    powershell.exe -NoProfile -WindowStyle Hidden -Command \
        '$process = Start-Process -FilePath "'"$WIN_BINARY"'" -WindowStyle Hidden -PassThru -RedirectStandardOutput "'"$WIN_LOG_FILE"'" -RedirectStandardError "'"${WIN_LOG_FILE}.err"'"; $process.Id | Out-File -FilePath "'"$WIN_PID_FILE"'" -Encoding ASCII -NoNewline' 2>/dev/null
```

Diff: +2 lines / -1 line (net +1 line). Unix `else` branch untouched. `start.ps1` untouched.

## Verification

Automated (all passed):
- `bash -n scripts/start.sh` → exit 0 (no syntax errors)
- `grep -c "RedirectStandardOutput" scripts/start.sh` → 1
- `grep -c "RedirectStandardError" scripts/start.sh` → 1
- `grep -c 'WIN_LOG_FILE=$(cygpath' scripts/start.sh` → 1
- Unix `nohup "$BINARY" > "$LOG_FILE" 2>&1 &` line → byte-identical to before

Manual (owner: user, after fix ships in a real SessionStart):
1. Close all Claude Code sessions, confirm dsrcode daemon stopped.
2. Delete/rotate `~/.claude/dsrcode.log`.
3. Start a new Claude Code session on Windows.
4. `tail ~/.claude/dsrcode.log` should show slog lines (server start, port bind, health endpoint).
5. On crash, `~/.claude/dsrcode.log.err` should contain stderr trace instead of vanishing.
6. `curl http://127.0.0.1:19460/health` should return 200.

## Commits

- `dcdbb9a` fix(start.sh): redirect Windows daemon stdout/stderr to dsrcode.log

## Deviations from Plan

None — plan executed exactly as written. Single task, single file, single commit, all done criteria met.

## Out of Scope (Flagged for Follow-up)

- `scripts/start.ps1` line 335 has a latent same-path bug (both `-RedirectStandardOutput` and `-RedirectStandardError` point to the same file, which PowerShell rejects with `MutuallyExclusiveArguments`). Flagged in the plan but NOT fixed here — requires a separate quick task.

## Self-Check: PASSED

- FOUND: scripts/start.sh (modified, contains all 3 required patterns)
- FOUND: commit dcdbb9a in git log
- FOUND: .planning/quick/260411-iyf-fix-windows-daemon-launch-log-redirect-i/260411-iyf-SUMMARY.md (this file)
