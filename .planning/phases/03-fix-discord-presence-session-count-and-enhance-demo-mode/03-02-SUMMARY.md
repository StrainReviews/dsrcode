---
phase: 03-fix-discord-presence-session-count-and-enhance-demo-mode
plan: 02
subsystem: session
tags: [go, dedup, session-source, discord-presence, slog, sync-once]

# Dependency graph
requires:
  - phase: 03-01
    provides: "SessionSource enum, StartSessionWithSource, foundExisting, upgradeSession"
provides:
  - "StartSession delegates to StartSessionWithSource for source-aware dedup (D-02)"
  - "slog.Info on session upgrade, slog.Debug on skip (D-17)"
  - "JSONL redundant dedup removed from main.go (D-05)"
  - "sync.Once deprecation warning for JSONL fallback (D-16)"
affects: [03-03, 03-04, 03-05]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "StartSession delegates to StartSessionWithSource (single enforcement point)"
    - "sync.Once for once-per-lifetime deprecation warnings"
    - "slog structured logging for session lifecycle audit trail"

key-files:
  created: []
  modified:
    - "session/registry.go"
    - "main.go"

key-decisions:
  - "StartSession delegates to StartSessionWithSource instead of duplicating logic (DRY)"
  - "sync.Once at package level sufficient since one project at a time in JSONL fallback"
  - "slog.Info for upgrades (actionable), slog.Debug for skips (diagnostic)"

patterns-established:
  - "Single enforcement point: all session creation funnels through StartSessionWithSource"
  - "Deprecation warnings via sync.Once to prevent log spam in poll loops"

requirements-completed: [D-01, D-02, D-04, D-05, D-13, D-16, D-17]

# Metrics
duration: 6min
completed: 2026-04-06
---

# Phase 3 Plan 02: Source-Aware Dedup in StartSession and JSONL Cleanup Summary

**StartSession delegates to source-aware dedup (D-02 single enforcement point), removes redundant JSONL dedup from main.go (D-05), adds sync.Once deprecation warning (D-16)**

## Performance

- **Duration:** 6 min
- **Started:** 2026-04-06T10:17:10Z
- **Completed:** 2026-04-06T10:22:44Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- StartSession now delegates to StartSessionWithSource, making it the single dedup enforcement point (D-02)
- Added slog.Info logging on session upgrades and slog.Debug on skip/no-downgrade events (D-17 audit trail)
- Removed the redundant JSONL dedup check from main.go's ingestJSONLFallback (D-05) -- this was the root cause of the asymmetric dedup bug
- Added sync.Once deprecation warning that fires once per daemon lifetime, not per 3-second poll cycle (D-16)
- All 8 Phase 3 dedup subtests pass GREEN, full project builds and tests pass with zero regressions

## Task Commits

Each task was committed atomically:

1. **Task 1: Wire source-aware dedup into StartSession** - `22f2c60` (fix)
2. **Task 2: Remove JSONL dedup check + add deprecation warning** - `b507cbf` (fix)

## Files Created/Modified
- `session/registry.go` - StartSession delegates to StartSessionWithSource; added slog import; slog.Info on upgrade, slog.Debug on skip
- `main.go` - Removed redundant JSONL dedup block (old lines 489-495); added jsonlDeprecationOnce sync.Once; deprecation warning in ingestJSONLFallback

## Decisions Made
- StartSession delegates to StartSessionWithSource rather than duplicating the upgrade logic (DRY principle, single enforcement point)
- sync.Once at package level is sufficient since JSONL fallback only tracks one project at a time
- slog.Info used for upgrades (actionable event worth monitoring), slog.Debug for skips (diagnostic only)

## Deviations from Plan

None -- plan executed exactly as written. Plan 01 had already created StartSessionWithSource with full upgrade logic, so Task 1 was a clean delegation rather than a full rewrite of StartSession's body.

## Issues Encountered
None

## User Setup Required
None -- no external service configuration required.

## Threat Surface Scan

No new threat surfaces introduced. Changes are internal logic refactoring (delegation pattern) and log output only. T-03-02-02 (elevation via upgradeSession) is mitigated by rank comparison. T-03-02-03 (repudiation) is mitigated by slog.Info audit trail now active.

## Next Phase Readiness
- Registry is now the single enforcement point for session dedup (D-02 fully implemented)
- JSONL fallback in main.go is clean with deprecation warning (D-16)
- Plans 03-05 can build on the source-aware registry without worrying about duplicate session counts

## Self-Check: PASSED

- FOUND: session/registry.go
- FOUND: main.go
- FOUND: commit 22f2c60
- FOUND: commit b507cbf

---
*Phase: 03-fix-discord-presence-session-count-and-enhance-demo-mode*
*Completed: 2026-04-06*
