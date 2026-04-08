---
phase: 03-fix-discord-presence-session-count-and-enhance-demo-mode
plan: 01
subsystem: session
tags: [go, enum, json-marshal, dedup, session-source, discord-presence]

# Dependency graph
requires:
  - phase: 03-00
    provides: "RED test stubs for Source enum and dedup behavior in registry_dedup_test.go"
provides:
  - "SessionSource enum (SourceJSONL, SourceHTTP, SourceClaude) with String()/MarshalJSON()"
  - "sourceFromID() prefix-based source inference"
  - "sourceRank() for upgrade comparison"
  - "Session.Source field with json:\"source\" tag"
  - "StartSessionWithSource() method with upgrade/no-downgrade logic"
  - "foundExisting() helper searching by ProjectName"
  - "upgradeSession() helper with copy-before-modify pattern"
affects: [03-02, 03-03, 03-04]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Go iota enum with String()/MarshalJSON() for JSON-safe serialization"
    - "strings.HasPrefix for safe prefix detection in sourceFromID"
    - "Copy-before-modify pattern in upgradeSession (immutable struct update)"
    - "Source-rank-based upgrade chain: JSONL(0) < HTTP(1) < Claude(2)"

key-files:
  created:
    - "session/source.go"
  modified:
    - "session/types.go"
    - "session/registry.go"
    - "session/registry_dedup_test.go"

key-decisions:
  - "Same-rank different-PID sessions coexist as separate windows (D-01 honored)"
  - "upgradeSession calls notifyChange() to trigger presence update on upgrade"
  - "foundExisting inlined into StartSessionWithSource loop for rank-aware matching"

patterns-established:
  - "SessionSource enum with iota for ranked source types"
  - "Explicit source parameter in StartSessionWithSource vs inferred in StartSession"
  - "Upgrade chain: JSONL -> HTTP -> Claude (no downgrade allowed)"

requirements-completed: [D-03, D-24]

# Metrics
duration: 4min
completed: 2026-04-06
---

# Phase 3 Plan 01: SessionSource Enum and Registry Helpers Summary

**SessionSource enum (JSONL/HTTP/Claude) with JSON string serialization, Session.Source field, and StartSessionWithSource upgrade/dedup logic**

## Performance

- **Duration:** 4 min
- **Started:** 2026-04-06T10:06:45Z
- **Completed:** 2026-04-06T10:10:50Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Created SessionSource enum with 3 values (SourceJSONL=0, SourceHTTP=1, SourceClaude=2) and JSON string serialization
- Extended Session struct with Source field that serializes as "jsonl"/"http"/"claude" in JSON (not numeric)
- Added StartSessionWithSource with full upgrade chain logic: higher-ranked sources replace lower-ranked ones, no downgrades allowed
- Added foundExisting and upgradeSession helpers using copy-before-modify immutable pattern
- All 8 Phase 3 dedup subtests pass GREEN, zero regressions on existing tests

## Task Commits

Each task was committed atomically:

1. **Task 1: Create SessionSource enum with String() and MarshalJSON()** - `1cbc740` (feat)
2. **Task 2: Extend Session struct + add registry helper stubs** - `af09063` (feat)

## Files Created/Modified
- `session/source.go` - SessionSource enum, String(), MarshalJSON(), sourceFromID(), sourceRank()
- `session/types.go` - Added Source SessionSource field to Session struct
- `session/registry.go` - Added StartSessionWithSource(), foundExisting(), upgradeSession(); set Source in StartSession
- `session/registry_dedup_test.go` - Removed //go:build phase16 tag to enable all 8 subtests

## Decisions Made
- Same-rank sessions with different PIDs coexist as separate Claude Code windows (D-01 honored in StartSessionWithSource)
- upgradeSession notifies onChange callback to trigger Discord presence update on session upgrade
- foundExisting logic inlined into StartSessionWithSource loop for rank-aware matching (avoids returning a match when same-rank different-PID should create new session)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed same-rank different-PID session handling in StartSessionWithSource**
- **Found during:** Task 2 (registry helper implementation)
- **Issue:** Initial foundExisting matched by ProjectName only, causing Same_project_different_PID_keeps_both test to fail (SessionCount=1 instead of 2)
- **Fix:** Refactored StartSessionWithSource to inline the search loop with rank-aware logic: only upgrade on strictly higher rank, return existing on lower rank, and for equal rank check PID to distinguish separate windows vs dedup
- **Files modified:** session/registry.go
- **Verification:** All 8 Phase16 subtests pass, including subtest 6 (Same_project_different_PID_keeps_both)
- **Committed in:** af09063 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Auto-fix necessary for correct dedup behavior. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- SessionSource enum and helpers ready for Plan 02 to wire into StartSession dedup logic
- StartSessionWithSource provides the upgrade chain that Plans 02-04 build upon
- foundExisting and upgradeSession are unexported helpers ready for internal use

## Self-Check: PASSED

- FOUND: session/source.go
- FOUND: 03-01-SUMMARY.md
- FOUND: commit 1cbc740
- FOUND: commit af09063

---
*Phase: 03-fix-discord-presence-session-count-and-enhance-demo-mode*
*Completed: 2026-04-06*
