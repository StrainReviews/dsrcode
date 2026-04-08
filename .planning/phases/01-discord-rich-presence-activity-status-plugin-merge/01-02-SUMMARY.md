---
phase: 01-discord-rich-presence-activity-status-plugin-merge
plan: 02
subsystem: discord-ipc
tags: [go, discord-rpc, rich-presence, exponential-backoff, buttons]

# Dependency graph
requires:
  - phase: 01-00
    provides: "Test stubs for backoff_test.go (t.Skip placeholders)"
provides:
  - "Button type and Activity.Buttons field for Discord Rich Presence"
  - "SetActivity serializes up to 2 buttons with label validation"
  - "Backoff struct with exponential doubling (1s->30s) and Reset"
  - "DefaultBackoff() factory for standard D-42 configuration"
affects: [01-03, 01-04, 01-06]

# Tech tracking
tech-stack:
  added: []
  patterns: ["exponential backoff with configurable min/max", "custom JSON serialization via map construction"]

key-files:
  created:
    - "discord/backoff.go"
  modified:
    - "discord/client.go"
    - "discord/backoff_test.go"

key-decisions:
  - "Buttons field uses json:\"-\" tag with custom map serialization in SetActivity (matches existing StartTime pattern)"
  - "Backoff.Next() returns current then doubles (caller sees 1s on first call, not 2s)"

patterns-established:
  - "Button validation: label 1-32 chars, non-empty URL, max 2 items enforced at serialization"
  - "Backoff pattern: NewBackoff(min, max) constructor, Next()/Reset() interface"

requirements-completed: [D-14, D-32, D-42, D-43, D-44]

# Metrics
duration: 9min
completed: 2026-04-02
---

# Phase 1 Plan 02: Discord Client Extensions Summary

**Activity struct extended with Button support (max 2, label-validated) and exponential backoff utility doubling from 1s to 30s cap**

## Performance

- **Duration:** 9 min
- **Started:** 2026-04-02T18:10:53Z
- **Completed:** 2026-04-02T18:19:43Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- Extended Activity struct with Buttons field and Button type (Label/URL) for Discord Rich Presence clickable links
- SetActivity serializes up to 2 buttons with label length validation (1-32 chars) and non-empty URL check
- Created Backoff utility implementing exponential doubling from configurable min to max (default 1s to 30s)
- Removed t.Skip stubs from backoff_test.go, all 3 tests pass with real assertions

## Task Commits

Each task was committed atomically:

1. **Task 1: Add Button type and Buttons field to Activity, update SetActivity serialization** - `20b6310` (feat)
2. **Task 2: Create exponential backoff utility** - `7d5b511` (feat)

## Files Created/Modified
- `discord/client.go` - Added Button struct, Buttons field to Activity, button serialization in SetActivity
- `discord/backoff.go` - New file: Backoff struct with NewBackoff, Next, Reset, DefaultBackoff
- `discord/backoff_test.go` - Replaced t.Skip stubs with full test implementations for sequence, reset, max

## Decisions Made
- Buttons field uses `json:"-"` tag (same pattern as StartTime) since buttons need custom map serialization within the activityData map, not standard JSON struct marshaling
- Backoff.Next() returns the current value then advances, so first call returns min (1s), not min*2 (2s) -- matches the TypeScript reference pattern semantics

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## Known Stubs

None - all code is fully functional with no placeholder data.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- Button type ready for use in preset configurations (01-03, 01-04)
- Backoff utility ready for integration into session reconnection logic (01-06)
- All 15 discord package tests passing (12 existing + 3 new backoff tests)

## Self-Check: PASSED

- All 3 created/modified files verified on disk
- Both commit hashes (20b6310, 7d5b511) verified in git log
- All 15 discord package tests pass

---
*Phase: 01-discord-rich-presence-activity-status-plugin-merge*
*Completed: 2026-04-02*
