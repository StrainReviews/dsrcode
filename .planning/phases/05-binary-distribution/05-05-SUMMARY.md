---
phase: 05-binary-distribution
plan: 05
subsystem: scripts
tags: [stop-scripts, daemon-lifecycle, dsrcode-rename, backward-compat]
dependency_graph:
  requires: ["05-01"]
  provides: ["stop-scripts-dsrcode"]
  affects: ["scripts/stop.sh", "scripts/stop.ps1", "scripts/setup-statusline.sh"]
tech_stack:
  added: []
  patterns: ["SIGTERM-wait-SIGKILL graceful shutdown", "new-path-first fallback migration"]
key_files:
  created: []
  modified:
    - scripts/stop.sh
    - scripts/stop.ps1
    - scripts/setup-statusline.sh
decisions:
  - "SIGTERM with 5s grace period before SIGKILL for clean daemon shutdown"
  - "Session counting spans both old and new directories during migration"
  - "Refcount writes migrate forward to new path even when read from old"
metrics:
  duration: "16m"
  completed: "2026-04-09T17:56:48Z"
  tasks_completed: 2
  tasks_total: 2
  files_modified: 3
---

# Phase 5 Plan 5: Stop Scripts and Setup References Summary

Overhauled stop.sh, stop.ps1, and setup-statusline.sh for dsrcode rename with backward-compatible fallback to old discord-presence paths and graceful SIGTERM/wait/SIGKILL shutdown pattern.

## One-liner

Stop scripts use dsrcode paths (primary) with discord-presence fallback, SIGTERM+5s wait+SIGKILL for clean shutdown.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | c33f986 | Overhaul stop.sh with dsrcode paths and graceful shutdown |
| 2 | e6a44d1 | Overhaul stop.ps1 and update setup-statusline.sh references |

## Task Details

### Task 1: Overhaul stop.sh with dsrcode paths + SIGTERM/wait/SIGKILL + fallback

**Changes to scripts/stop.sh:**
- Configuration paths updated: `dsrcode.pid`, `dsrcode-sessions`, `dsrcode.refcount` as primary
- Old paths retained as fallback: `discord-presence.pid`, `discord-presence-sessions`, `discord-presence.refcount`
- `kill_process()` now sends SIGTERM, waits up to 5 seconds checking `kill -0`, then sends SIGKILL if still alive
- Windows refcount reads from new file first, falls back to old; writes always go to new path (migrate forward)
- Unix session counting iterates both `dsrcode-sessions/` and `discord-presence-sessions/` directories
- Process name fallback uses `dsrcode` (primary) and `cc-discord-presence` (old) for both pkill and taskkill
- Both old and new PID files cleaned up on stop

### Task 2: Overhaul stop.ps1 + update setup-statusline.sh references

**Changes to scripts/stop.ps1:**
- Configuration paths updated to dsrcode with Old* fallback variables
- Refcount logic tries new file first, falls back to old
- PID file lookup tries new path first, falls back to old
- Process name search uses `dsrcode*` primary, `cc-discord-presence*` fallback
- Both old and new PID/refcount files cleaned up on stop

**Changes to scripts/setup-statusline.sh:**
- Comment updated from "cc-discord-presence" to "dsrcode (Discord Rich Presence for Claude Code)"
- User-facing echo message updated to reference "dsrcode"
- Functional behavior unchanged (wrapper script copy, settings.json update)

## Decisions Made

1. **SIGTERM with 5s grace period**: Sends SIGTERM first, polls with `kill -0` every second for up to 5 seconds, then SIGKILL only if process still alive. Prevents zombie daemons while allowing clean shutdown.
2. **Migrate-forward pattern for refcount**: When reading from old refcount file, writes go to new path. Ensures gradual migration without user intervention.
3. **Dual-directory session counting**: Active sessions counted across both old and new session directories to avoid premature daemon termination during migration.

## Deviations from Plan

None - plan executed exactly as written.

## Verification Results

- stop.sh passes `bash -n` syntax check
- stop.sh contains `dsrcode.pid` as primary PID file
- stop.sh contains `discord-presence.pid` as fallback PID file
- stop.sh contains `dsrcode-sessions` as primary sessions dir
- stop.sh contains `discord-presence-sessions` as fallback sessions dir
- stop.sh contains `dsrcode.refcount` as primary refcount file
- stop.sh contains SIGTERM then wait then SIGKILL pattern (`kill -9`)
- stop.sh fallback kill uses process name `dsrcode` (primary) and `cc-discord-presence` (fallback)
- stop.ps1 contains `dsrcode.pid` as primary, `discord-presence.pid` as fallback
- stop.ps1 contains `dsrcode.refcount` as primary, `discord-presence.refcount` as fallback
- stop.ps1 process name search uses `dsrcode*` (primary) and `cc-discord-presence*` (fallback)
- stop.ps1 cleans up both old and new PID files
- setup-statusline.sh mentions "dsrcode" in user-facing messages
- setup-statusline.sh does NOT contain "tsanva"

## Self-Check: PASSED

All 3 modified files verified present. Both task commits (c33f986, e6a44d1) verified in git log.
