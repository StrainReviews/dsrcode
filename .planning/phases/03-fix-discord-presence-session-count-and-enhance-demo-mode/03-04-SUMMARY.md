---
phase: 03-fix-discord-presence-session-count-and-enhance-demo-mode
plan: 04
subsystem: api
tags: [go, http, preview, discord-activity, stablepick, message-rotation]

# Dependency graph
requires:
  - phase: 03-02
    provides: "Session dedup logic and source-aware StartSession"
  - phase: 03-03
    provides: "Unique project counting for resolver tier selection"
provides:
  - "Extended PreviewPayload with all Discord Activity fields"
  - "FakeSession struct for multi-session demo preview"
  - "ButtonDef struct for preview buttons"
  - "GET /preview/messages endpoint for message rotation preview"
  - "onPreview callback accepts full PreviewPayload"
affects: [03-05-demo-skill-rewrite]

# Tech tracking
tech-stack:
  added: []
  patterns: ["preview placeholder resolution separate from resolver to avoid circular deps", "query param clamping for DoS mitigation"]

key-files:
  created: []
  modified:
    - "server/server.go"
    - "server/server_preview_test.go"
    - "server/server_test.go"
    - "main.go"

key-decisions:
  - "onPreview callback changed from func(string, string, time.Duration) to func(PreviewPayload, time.Duration) for full Activity control"
  - "No 'Preview Mode' default text per D-11 -- empty details/state lets resolver use preset messages"
  - "Separate buildPreviewPlaceholderValues/replacePlaceholdersSimple in server package to avoid circular dependency with resolver"
  - "Fake session uses realistic demo data: my-saas-app, Opus 4.6, 250K tokens, $2.50, feature/auth"

patterns-established:
  - "Preview endpoint returns resolved messages with placeholder substitution"
  - "Query param clamping pattern (count max 20, invalid preset returns 400)"

requirements-completed: [D-06, D-07, D-08, D-10, D-11]

# Metrics
duration: 11min
completed: 2026-04-06
---

# Phase 3 Plan 04: Preview API Extension Summary

**Extended PreviewPayload with full Discord Activity control (smallImage, smallText, largeText, buttons, startTimestamp, sessionCount, fakeSessions), added GET /preview/messages for StablePick rotation preview, removed "Preview Mode" indicator per D-11**

## Performance

- **Duration:** 11 min
- **Started:** 2026-04-06T10:38:52Z
- **Completed:** 2026-04-06T10:49:31Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- Extended PreviewPayload with 7 new fields for full Discord Activity control in demo mode
- Added GET /preview/messages endpoint that simulates StablePick message rotation across time windows
- Removed "Preview Mode" default text so screenshots look authentic (D-11)
- Changed onPreview callback to pass full payload, enabling rich demo screenshots from Plan 05

## Task Commits

Each task was committed atomically:

1. **Task 1: Extend PreviewPayload and handlePostPreview** - `35ce699` (feat)
2. **Task 2: Add GET /preview/messages endpoint** - `3c16f79` (feat)

## Files Created/Modified
- `server/server.go` - Added FakeSession, ButtonDef structs; extended PreviewPayload; added GET /preview/messages handler with buildPreviewPlaceholderValues, formatPreviewDuration, replacePlaceholdersSimple helpers
- `server/server_preview_test.go` - Removed //go:build phase16 tag; all 3 preview tests now compile and pass GREEN
- `server/server_test.go` - Updated newTestServerWithPreview helper and TestPostPreview for new callback signature
- `main.go` - Updated onPreview callback to accept PreviewPayload; wires SmallImage, SmallText, StartTimestamp into Activity

## Decisions Made
- onPreview callback signature changed from `func(string, string, time.Duration)` to `func(PreviewPayload, time.Duration)` for full Activity field control
- No "Preview Mode" default text per D-11: empty details/state means resolver uses preset messages for authentic screenshots
- Server-side placeholder resolution kept separate from resolver package to avoid circular dependency
- Fake session uses realistic demo data matching D-10 requirements: my-saas-app, Opus 4.6, 250K tokens, $2.50, feature/auth branch

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed Activity.StartTime pointer assignment in main.go**
- **Found during:** Task 1 (updating main.go callback)
- **Issue:** `activity.StartTime = time.Unix(...)` failed because StartTime is `*time.Time`, not `time.Time`
- **Fix:** Assigned via intermediate variable: `ts := time.Unix(...); activity.StartTime = &ts`
- **Files modified:** main.go
- **Verification:** `go build ./...` succeeds
- **Committed in:** 35ce699 (Task 1 commit)

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Minor type fix required for pointer assignment. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Preview API fully extended and ready for Plan 05 (demo skill rewrite)
- GET /preview/messages provides message rotation preview for demo screenshots
- FakeSession struct enables multi-session simulation in demo mode
- All 3 Phase 3 preview tests pass GREEN alongside existing test suite

## Self-Check: PASSED

- All 4 modified files exist on disk
- Commit 35ce699 verified in git log
- Commit 3c16f79 verified in git log
- 03-04-SUMMARY.md created successfully

---
*Phase: 03-fix-discord-presence-session-count-and-enhance-demo-mode*
*Completed: 2026-04-06*
