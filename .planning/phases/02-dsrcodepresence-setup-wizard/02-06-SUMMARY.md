---
phase: 02-dsrcodepresence-setup-wizard
plan: 06
subsystem: infra
tags: [hooks, http, discord, plugin, auto-update, bash]

requires:
  - phase: 02-01
    provides: "Go binary with HTTP server on port 19460"
provides:
  - "HTTP hooks replacing command+bash hooks (10x faster)"
  - "Notification hook for idle_prompt state tracking"
  - "plugin.json v3.0.0 manifest"
  - "start.sh auto-update and first-run hint"
affects: [02-07, 02-08]

tech-stack:
  added: []
  patterns: [http-hooks, auto-update-binary, first-run-hint]

key-files:
  created: []
  modified:
    - hooks/hooks.json
    - .claude-plugin/plugin.json
    - scripts/start.sh
  deleted:
    - scripts/hook-forward.sh

key-decisions:
  - "HTTP hooks with 1s timeout replace command hooks (bash+curl) for 10x faster activity tracking"
  - "Notification hook with idle_prompt matcher added for idle state detection"
  - "SessionStart/SessionEnd remain command type per D-15 (lifecycle hooks need shell execution)"
  - "No userConfig in plugin.json per D-37"
  - "start.sh auto-update kills existing daemon before binary replacement"

patterns-established:
  - "HTTP hook type for activity tracking (native JSON POST, no shell intermediary)"
  - "Version check + auto-update pattern in start.sh for seamless binary upgrades"
  - "First-run hint pattern suggesting /dsrcode:setup for new users"

requirements-completed: [DSR-08, DSR-09, DSR-10, DSR-11, DSR-12, DSR-13, DSR-14, DSR-35, DSR-36, DSR-37, DSR-39]

duration: 3min
completed: 2026-04-03
---

# Phase 2 Plan 06: Hook Migration & Plugin Update Summary

**HTTP hooks replacing bash+curl command hooks (4 hooks: PreToolUse, UserPromptSubmit, Stop, Notification), plugin.json v3.0.0, start.sh auto-update + first-run hint**

## Performance

- **Duration:** 3 min
- **Started:** 2026-04-03T18:47:17Z
- **Completed:** 2026-04-03T18:50:33Z
- **Tasks:** 2
- **Files modified:** 3 (+ 1 deleted)

## Accomplishments
- Migrated all activity hooks from command type (bash + curl) to native HTTP type with 1s timeout
- Added Notification hook with idle_prompt matcher for idle state tracking
- Deleted hook-forward.sh intermediary script (no longer needed)
- Updated plugin.json to v3.0.0, keeping SessionStart/SessionEnd as command hooks per D-15
- Added version check + auto-update logic to start.sh (detects version mismatch, kills daemon, re-downloads)
- Added first-run hint suggesting /dsrcode:setup when no config file exists

## Task Commits

Each task was committed atomically:

1. **Task 1: Migrate hooks.json to HTTP type + add Notification hook + delete hook-forward.sh** - `063a70e` (feat)
2. **Task 2: Update plugin.json to v3.0.0 + start.sh version check and first-run hint** - `4817e2f` (feat)

## Files Created/Modified
- `hooks/hooks.json` - Rewritten: 4 HTTP hooks (PreToolUse, UserPromptSubmit, Stop, Notification) targeting localhost:19460
- `.claude-plugin/plugin.json` - Version bumped to 3.0.0, no userConfig added
- `scripts/start.sh` - VERSION v3.0.0, auto-update logic, first-run hint
- `scripts/hook-forward.sh` - Deleted (replaced by native HTTP hooks)

## Decisions Made
- HTTP hooks with 1s timeout replace command hooks for 10x faster activity tracking (eliminates bash + curl overhead)
- Notification hook with idle_prompt matcher added to fix the "no activity" problem when Claude waits for input
- SessionStart/SessionEnd remain command type per D-15 (lifecycle hooks need shell execution for binary management)
- No userConfig in plugin.json per D-37
- Auto-update in start.sh kills existing daemon before binary replacement to avoid file-in-use issues

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- HTTP hooks are in place and ready for the Go server to receive native JSON payloads
- Plugin manifest is v3.0.0 matching the new feature set
- start.sh auto-update ensures users always get the correct binary version on session start
- First-run hint guides new users to /dsrcode:setup

## Self-Check: PASSED

- FOUND: hooks/hooks.json
- FOUND: .claude-plugin/plugin.json
- FOUND: scripts/start.sh
- CONFIRMED DELETED: scripts/hook-forward.sh
- FOUND: 02-06-SUMMARY.md
- FOUND: commit 063a70e
- FOUND: commit 4817e2f

---
*Phase: 02-dsrcodepresence-setup-wizard*
*Completed: 2026-04-03*
