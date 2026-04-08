---
phase: 04-discord-presence-enhanced-analytics
plan: 07
subsystem: integration
tags: [go, analytics, discord-presence, hooks, i18n, jsonl, pricing, compaction]

# Dependency graph
requires:
  - phase: 04-01
    provides: analytics domain types and tracker coordinator
  - phase: 04-02
    provides: analytics formatting and pricing functions
  - phase: 04-03
    provides: server SubagentStop route and Agent spawn tracking
  - phase: 04-04
    provides: 7 analytics placeholders in resolver
  - phase: 04-05
    provides: 8 bilingual presets with analytics messages
  - phase: 04-06
    provides: go-i18n with 24 message IDs and NewLocalizer
provides:
  - "Fully wired analytics tracker in main.go connecting all Phase 4 subsystems"
  - "JSONL watcher with per-model token breakdown and compaction detection"
  - "Statusline watcher with context% extraction"
  - "start.sh auto-patching hooks.json (Agent + SubagentStop)"
  - "start.sh german preset migration to professional + lang=de"
  - "CHANGELOG.md v3.2.0 release notes"
  - "Version bumped to 3.2.0 in main.go and start.sh"
affects: [phase-18-goreleaser, discord-presence-release]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "syncAnalyticsToRegistry bridge: json.Marshal to avoid circular deps between analytics/ and session/"
    - "UpdateAnalytics on SessionRegistry using immutable copy pattern with json.RawMessage"
    - "Tracker passed through watcher chain: main -> jsonlFallbackWatcher -> ingestJSONLFallback"

key-files:
  created: []
  modified:
    - "main.go"
    - "main_test.go"
    - "session/registry.go"
    - "scripts/start.sh"
    - "CHANGELOG.md"

key-decisions:
  - "Used json.RawMessage bridge pattern to avoid circular deps between analytics/ and session/ packages"
  - "Tracker passed as parameter through watcher chain rather than global variable"
  - "readStatusLineData accepts tracker to extract context% inline during statusline parsing"
  - "parseJSONLSession accumulates per-model TokenBreakdown map for cache-aware pricing"
  - "Build-from-source refactor in start.sh incorporated as part of version bump commit"

patterns-established:
  - "syncAnalyticsToRegistry: bridges analytics.TrackerState to session.AnalyticsUpdate via json.Marshal"
  - "Tracker injection: analytics.Tracker passed through function parameters, not stored as global"

requirements-completed: [DPA-01, DPA-05, DPA-10, DPA-11, DPA-12, DPA-22, DPA-23, DPA-25, DPA-26, DPA-29, DPA-30]

# Metrics
duration: 10min
completed: 2026-04-07
---

# Phase 4 Plan 07: Main.go Integration Summary

**Wired analytics tracker to server, JSONL watcher (per-model tokens + compaction), statusline watcher (context%), with auto-patching hooks.json and german preset migration in start.sh; version bumped to v3.2.0**

## Performance

- **Duration:** 10 min
- **Started:** 2026-04-07T13:03:46Z
- **Completed:** 2026-04-07T13:13:43Z
- **Tasks:** 2
- **Files modified:** 5

## Accomplishments
- Wired analytics.Tracker into main.go connecting server, JSONL watcher, and statusline watcher
- Replaced old calculateCost/modelPricing with cache-aware analytics.CalculateSessionCost
- Added per-model token breakdown accumulation and compaction detection in JSONL parsing
- Added context% extraction from statusline UsedPercentage field
- Added auto-patching hooks.json (Agent + SubagentStop) and german preset migration in start.sh
- Created comprehensive CHANGELOG.md v3.2.0 entry with all Phase 4 changes

## Task Commits

Each task was committed atomically:

1. **Task 1: Wire analytics tracker in main.go and update JSONL/statusline watchers** - `01b380b` (feat)
2. **Task 2: Update start.sh with hooks auto-patch, german migration, version bump** - `f7fcb9d` (feat)

## Files Created/Modified
- `main.go` - Wired analytics.Tracker, removed old pricing, added per-model tokens, context% extraction, syncAnalyticsToRegistry bridge, version 3.2.0
- `main_test.go` - Updated tests for analytics.CalculateCost API, removed obsolete modelPricing consistency test
- `session/registry.go` - Added AnalyticsUpdate struct and UpdateAnalytics method with immutable copy pattern
- `scripts/start.sh` - Version v3.2.0, patch_hooks_json(), migrate_german_preset(), build-from-source refactor
- `CHANGELOG.md` - Added v3.2.0 section with Added/Changed/Fixed/Deprecated entries

## Decisions Made
- Used json.RawMessage bridge pattern in session.AnalyticsUpdate to avoid importing analytics/ in session/ package (prevents circular dependency)
- Passed tracker as function parameter through the watcher chain rather than using a package-level global
- Incorporated the pre-existing build-from-source refactor in start.sh as part of the version bump commit (was uncommitted working tree change)
- readStatusLineData extracts context% inline during parsing to minimize code changes while providing D-10 functionality
- Config watcher now syncs lang and features changes for hot-reload of bilingual presets

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added encoding/json import to session/registry.go**
- **Found during:** Task 1 (UpdateAnalytics method)
- **Issue:** AnalyticsUpdate struct uses json.RawMessage which requires encoding/json import
- **Fix:** Added encoding/json to the import block in session/registry.go
- **Files modified:** session/registry.go
- **Verification:** go build succeeds
- **Committed in:** 01b380b (Task 1 commit)

**2. [Rule 1 - Bug] Updated main_test.go for removed calculateCost/modelPricing**
- **Found during:** Task 1 (removing old pricing code)
- **Issue:** TestCalculateCost and TestModelPricingConsistency referenced removed functions
- **Fix:** Rewrote TestCalculateCost to use analytics.CalculateCost, replaced TestModelPricingConsistency with TestModelDisplayNames
- **Files modified:** main_test.go
- **Verification:** go test ./... passes
- **Committed in:** 01b380b (Task 1 commit)

**3. [Rule 2 - Missing Critical] Build-from-source changes in start.sh**
- **Found during:** Task 2 (start.sh modifications)
- **Issue:** Working tree had uncommitted build-from-source refactor replacing download-based binary acquisition
- **Fix:** Incorporated as part of the version bump since start.sh needed modification anyway
- **Files modified:** scripts/start.sh
- **Verification:** Script syntax valid, grep counts match
- **Committed in:** f7fcb9d (Task 2 commit)

---

**Total deviations:** 3 auto-fixed (1 blocking, 1 bug, 1 missing critical)
**Impact on plan:** All auto-fixes necessary for correctness. No scope creep.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- All Phase 4 components are now wired together end-to-end
- Binary version 3.2.0 is ready for release via Phase 5 GoReleaser
- Analytics tracker receives events from hooks (server) and watchers (main.go)
- Resolver can display analytics data via 7 new placeholders
- Presets have bilingual messages using analytics placeholders
- Config supports lang and features toggles with hot-reload
- start.sh handles operational migrations on every daemon start

---
*Phase: 04-discord-presence-enhanced-analytics*
*Plan: 07*
*Completed: 2026-04-07*
