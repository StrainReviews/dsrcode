---
phase: 01-discord-rich-presence-activity-status-plugin-merge
plan: 07
subsystem: discord-presence
tags: [go, discord-rpc, presence-resolver, knuth-hash, anti-flicker, layout-d]

requires:
  - phase: 01-03
    provides: "Session types (Session, ActivityCounts, SessionStatus)"
  - phase: 01-04
    provides: "Preset types (MessagePreset, Button) and embedded JSON presets"
provides:
  - "ResolvePresence function converting session state to Discord Activity"
  - "StablePick anti-flicker message rotation with 5-minute Knuth hash buckets"
  - "FormatStatsLine with singular/plural activity counts and duration"
  - "DetectDominantMode for >50% activity icon detection"
affects: [01-08, 01-09, 01-10]

tech-stack:
  added: []
  patterns: [knuth-multiplicative-hash, placeholder-template-replacement, tier-based-message-dispatch]

key-files:
  created:
    - "resolver/resolver.go"
    - "resolver/stable_pick.go"
    - "resolver/stats.go"
  modified:
    - "resolver/resolver_test.go"

key-decisions:
  - "HashString for StablePick seed instead of timestamp (sessionID-based for determinism)"
  - "Stats line as multi-session state (aggregated counts replace model/tokens/cost)"
  - "getMostRecentSession prefers active over idle for SmallImage icon selection"

patterns-established:
  - "StablePick(pool, seed, now): Deterministic rotation within 5-min buckets using Knuth hash 2654435761"
  - "replacePlaceholders: Generic {key}->value template substitution for preset messages"
  - "Tier-based dispatch: map[string][]string with keys 2/3/4 and overflow slice for 5+"

requirements-completed: [D-08, D-09, D-10, D-11, D-12, D-13, D-15, D-23, D-25, D-26, D-27]

duration: 8min
completed: 2026-04-02
---

# Phase 1 Plan 07: Presence Resolver Summary

**Presence resolver converting session state to Discord Activity with Knuth-hash anti-flicker rotation, Layout D format, and tier-based multi-session messages**

## Performance

- **Duration:** 8 min
- **Started:** 2026-04-02T18:54:36Z
- **Completed:** 2026-04-02T19:03:17Z
- **Tasks:** 2
- **Files modified:** 4

## Accomplishments
- StablePick anti-flicker using Knuth multiplicative hash with 5-minute rotation buckets (D-23)
- FormatStatsLine with singular/plural activity counts joined by middle dot separator (D-25)
- DetectDominantMode returning activity icon for >50% dominant or "multi-session"/"idle" (D-25)
- Single-session resolver producing Layout D format: Details from preset, State with model/tokens/cost (D-08 to D-15)
- Multi-session resolver with tier-based message dispatch (2/3/4/overflow) and aggregated stats (D-27)
- All 8 resolver tests passing

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement StablePick and stats formatting** - `fa74361` (feat)
2. **Task 2: Implement presence resolver for single and multi session** - `f800b98` (feat)

## Files Created/Modified
- `resolver/stable_pick.go` - StablePick Knuth hash rotation + HashString seed helper
- `resolver/stats.go` - FormatStatsLine activity counter formatting + DetectDominantMode
- `resolver/resolver.go` - ResolvePresence dispatcher, resolveSingle/resolveMulti, formatTokens/formatCost/formatDuration helpers
- `resolver/resolver_test.go` - 8 tests: StablePick, StablePickDeterministic, FormatStatsLine, FormatStatsLineSingular, FormatStatsLineWithDuration, DetectDominantMode, ResolveSingleSession, ResolveMultiSession

## Decisions Made
- Used HashString(sessionID) as StablePick seed rather than timestamp, ensuring different sessions get independent rotation even within the same time bucket
- Multi-session state line uses FormatStatsLine with aggregated counts instead of model/tokens/cost (those are per-session and don't make sense aggregated)
- getMostRecentSession prefers active sessions over idle ones when selecting the SmallImage icon

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
- Race detector (`-race`) unavailable on Windows without CGO_ENABLED=1; tests verified without race flag (no concurrency in resolver package)

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness
- resolver/ package complete with ResolvePresence as the public entry point
- Ready for 01-08 (server integration) to wire resolver into the polling loop
- StablePick and FormatStatsLine exported for use by other packages

## Self-Check: PASSED

- All 4 created/modified files exist on disk
- Commit fa74361 verified in git history
- Commit f800b98 verified in git history
- All 8 resolver tests pass

---
*Phase: 01-discord-rich-presence-activity-status-plugin-merge*
*Completed: 2026-04-02*
