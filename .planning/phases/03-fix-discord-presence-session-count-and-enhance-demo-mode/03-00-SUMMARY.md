---
phase: 03-fix-discord-presence-session-count-and-enhance-demo-mode
plan: 00
subsystem: testing
tags: [go, tdd, table-driven-tests, session-dedup, build-tags]

requires:
  - phase: 01-discord-rich-presence-activity-status-plugin-merge
    provides: session registry, resolver, server with external test package pattern
  - phase: 02-dsrcodepresence-setup-wizard
    provides: preview endpoint, display detail enum, HTTP hooks

provides:
  - 13 TDD test stubs (RED phase) across 3 test files
  - Test specification for session Source-aware dedup upgrade chain
  - Test specification for unique-project tier selection in resolver
  - Test specification for enhanced preview/demo endpoints

affects: [03-01, 03-02, 03-03, 03-04]

tech-stack:
  added: []
  patterns:
    - "Build tag gating (//go:build phase16) for incremental TDD across plans"
    - "External test packages for black-box testing (session_test, resolver_test, server_test)"

key-files:
  created:
    - "session/registry_dedup_test.go"
    - "resolver/resolver_unique_test.go"
    - "server/server_preview_test.go"
  modified: []

key-decisions:
  - "Build-tagged tests for session dedup and server preview (phase16 tag) to avoid breaking CI until implementation plans add types"
  - "Resolver unique-project tests compile without build tag and FAIL intentionally (RED phase demonstrates current bug)"
  - "All 3 server preview tests in single build-tagged file since tests 1-2 reference non-existent PreviewPayload fields"

patterns-established:
  - "TDD RED stubs with build tags: tests reference future types gated by //go:build so CI stays green"
  - "Intentionally-failing tests document expected behavior (resolver unique project count vs raw session count)"

requirements-completed: [D-20, D-25]

duration: 8min
completed: 2026-04-06
---

# Phase 3 Plan 00: Wave 0 TDD Test Stubs Summary

**13 table-driven test stubs across 3 Go test files documenting Source-aware session dedup, unique-project tier selection, and enhanced preview endpoints for Phase 3 RED phase**

## Performance

- **Duration:** 8 min
- **Started:** 2026-04-06T09:49:51Z
- **Completed:** 2026-04-06T09:58:00Z
- **Tasks:** 1
- **Files created:** 3

## Accomplishments

- Scaffolded 8 registry dedup subtests covering the full Source upgrade chain (JSONL->HTTP->UUID), no-downgrade rules, same-project-different-PID coexistence, and Source enum serialization
- Scaffolded 2 resolver subtests that demonstrate the current bug: 2 sessions on same project incorrectly triggers multi-session tier instead of single-session
- Scaffolded 3 server preview subtests for enhanced demo mode fields (smallImage, fakeSessions, GET /preview/messages)
- Verified build-tagged tests are excluded from normal CI (go test ./... passes except for intentional resolver RED failure)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create 13 test stubs across 3 test files** - `5dc36df` (test)

## Files Created/Modified

- `session/registry_dedup_test.go` - 8 subtests: JSONL->HTTP upgrade, HTTP->UUID upgrade, JSONL->UUID full chain, no-downgrade UUID->JSONL, no-downgrade HTTP->JSONL, same-project-different-PID keeps both, Source enum String(), Source enum MarshalJSON
- `resolver/resolver_unique_test.go` - 2 subtests: two sessions same project uses single-session tier, two sessions different projects uses multi-session tier
- `server/server_preview_test.go` - 3 subtests: Preview_SmallImage, Preview_FakeSessions, GET_preview_messages

## Decisions Made

- **Build tag strategy:** Used `//go:build phase16` on session and server test files because they reference types that don't exist yet (SessionSource, SourceJSONL, SourceHTTP, SourceClaude, StartSessionWithSource). Plan 01 will add these types and remove the tag.
- **Resolver tests untagged:** The resolver unique-project tests compile against existing code but FAIL because current ResolvePresence uses `len(sessions)` instead of unique project count. This is the correct RED phase behavior -- the test documents the bug.
- **Server tests all tagged:** All 3 server preview tests placed in one build-tagged file. Tests 1-2 send JSON with unknown fields to existing /preview endpoint (currently ignored by Go decoder), test 3 hits non-existent GET /preview/messages (404). All excluded from normal CI.

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Test stubs ready as targets for Plans 01-04
- Plan 01 can add SessionSource enum and Source field, then remove //go:build phase16 from session tests
- Plan 02 can implement registry dedup logic to make session tests pass (GREEN)
- Plan 03 can fix resolver unique-project counting to make resolver tests pass (GREEN)
- Plan 04 can extend PreviewPayload and add GET /preview/messages to make server tests pass (GREEN)

## Self-Check: PASSED

- [x] session/registry_dedup_test.go exists at C:\Users\ktown\Projects\cc-discord-presence\session\registry_dedup_test.go
- [x] resolver/resolver_unique_test.go exists at C:\Users\ktown\Projects\cc-discord-presence\resolver\resolver_unique_test.go
- [x] server/server_preview_test.go exists at C:\Users\ktown\Projects\cc-discord-presence\server\server_preview_test.go
- [x] Commit 5dc36df exists in git log

---
*Phase: 03-fix-discord-presence-session-count-and-enhance-demo-mode*
*Plan: 00*
*Completed: 2026-04-06*
