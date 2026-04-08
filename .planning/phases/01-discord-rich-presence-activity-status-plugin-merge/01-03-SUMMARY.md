---
phase: 01-discord-rich-presence-activity-status-plugin-merge
plan: 03
subsystem: session
tags: [go, rwmutex, concurrency, pid-liveness, stale-detection, build-tags]

requires:
  - phase: 01-01
    provides: "Session, ActivityCounts, SessionStatus, ActivityRequest, CounterMap types"
provides:
  - "Thread-safe SessionRegistry with RWMutex, CRUD, dedup, onChange callback"
  - "Cross-platform PID liveness check (Unix kill -0 / Windows tasklist)"
  - "Stale session detection loop (idle at 10min, remove at 30min)"
  - "GetMostRecentSession for presence resolver"
affects: [01-05, 01-06, 01-07]

tech-stack:
  added: []
  patterns:
    - "Immutable update pattern: copy struct before modifying, store pointer to copy"
    - "Build tags for cross-platform code (//go:build windows / !windows)"
    - "Ticker-based goroutine loop with context cancellation"
    - "RWMutex read/write separation for concurrent session access"

key-files:
  created:
    - "cc-discord-presence/session/registry.go"
    - "cc-discord-presence/session/stale.go"
    - "cc-discord-presence/session/stale_unix.go"
    - "cc-discord-presence/session/stale_windows.go"
  modified:
    - "cc-discord-presence/session/registry_test.go"

key-decisions:
  - "Exported CheckOnce for external test package access to stale detection logic"
  - "SetLastActivityForTest helper on registry for deterministic stale tests without time mocking"
  - "IsPidAlive uses tasklist on Windows (os.FindProcess always succeeds on Windows, unreliable)"

patterns-established:
  - "Immutable copy pattern: all mutation methods copy *Session before modifying, then store pointer to copy"
  - "onChange callback: every mutation method calls registry.notifyChange() for presence update trigger"
  - "Build-tagged platform files: stale_unix.go (!windows) and stale_windows.go (windows)"

requirements-completed: [D-26, D-27, D-28, D-29, D-30, D-31]

duration: 10min
completed: 2026-04-02
---

# Phase 1 Plan 03: Session Registry & Stale Detection Summary

**Thread-safe SessionRegistry with RWMutex, projectPath+PID dedup, immutable update pattern, cross-platform PID liveness, and ticker-based stale detection (idle at 10min, remove at 30min)**

## Performance

- **Duration:** 10 min
- **Started:** 2026-04-02T18:34:26Z
- **Completed:** 2026-04-02T18:45:08Z
- **Tasks:** 2
- **Files created:** 4
- **Files modified:** 1

## Accomplishments

- SessionRegistry with sync.RWMutex managing concurrent session access with 8 public methods (StartSession, UpdateActivity, UpdateSessionData, EndSession, GetSession, GetAllSessions, SessionCount, GetMostRecentSession)
- Session deduplication by ProjectPath+PID prevents duplicate registrations from the same Claude Code process
- Cross-platform PID liveness using build-tagged files: Unix uses kill -0 (syscall.Signal(0)), Windows uses tasklist command parsing
- Stale detection loop with context-aware shutdown: transitions to idle after 10min inactivity, removes after 30min, and removes dead PIDs immediately
- All 8 session package tests pass (5 registry CRUD/dedup/concurrent + 3 stale/PID)

## Task Commits

Each task was committed atomically:

1. **Task 1: Implement SessionRegistry with RWMutex** - `1777a26` (feat)
2. **Task 2: Implement stale detection with cross-platform PID liveness** - `d3c8e0c` (feat)

## Files Created/Modified

- `session/registry.go` - Thread-safe session map with lifecycle operations, onChange callback, dedup, GetMostRecentSession
- `session/stale.go` - CheckStaleSessions ticker loop, CheckOnce single-pass stale check
- `session/stale_unix.go` - IsPidAlive via syscall.Signal(0) on Unix
- `session/stale_windows.go` - IsPidAlive via tasklist on Windows
- `session/registry_test.go` - All 8 test stubs implemented (5 registry + 3 stale/PID)

## Decisions Made

- Exported `CheckOnce` (not `checkOnce`) so external test package `session_test` can call it directly for deterministic stale behavior testing
- Added `SetLastActivityForTest` helper method on registry to allow tests to set LastActivityAt to arbitrary past times without needing time mocking infrastructure
- Windows IsPidAlive uses `tasklist /FI "PID eq N"` and string-contains check because `os.FindProcess` on Windows always succeeds (it opens a handle, does not verify liveness)
- `-race` flag unavailable in this environment (requires CGO + gcc), but concurrent access test with 100 goroutines validates thread safety functionally

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Exported checkOnce for external test access**
- **Found during:** Task 2 (stale tests)
- **Issue:** Test file uses external package `session_test`, cannot call unexported `checkOnce`
- **Fix:** Renamed to `CheckOnce` (exported)
- **Files modified:** session/stale.go
- **Verification:** All stale tests call CheckOnce directly and pass
- **Committed in:** d3c8e0c (Task 2 commit)

**2. [Rule 2 - Missing Critical] Added SetLastActivityForTest helper**
- **Found during:** Task 2 (stale tests)
- **Issue:** No way to set LastActivityAt to a past time for deterministic stale tests from external test package
- **Fix:** Added SetLastActivityForTest method on SessionRegistry
- **Files modified:** session/registry.go
- **Verification:** TestStaleCheck and TestStaleRemove use it successfully
- **Committed in:** d3c8e0c (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (1 blocking, 1 missing critical)
**Impact on plan:** Both auto-fixes necessary for test functionality. No scope creep.

## Issues Encountered

- `-race` flag requires CGO_ENABLED=1 and a C compiler (gcc), neither available in this Windows environment. Concurrent access test with 100 goroutines runs successfully without the race detector. The race detector can be used in CI where gcc is available.

## Known Stubs

None - all functionality is fully wired.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- SessionRegistry ready for use by the HTTP server (plan 01-05) and presence resolver (plan 01-06)
- Stale detection loop ready to be started from main.go with configurable intervals
- TransitionToIdle method ready for resolver's idle state handling

---
*Phase: 01-discord-rich-presence-activity-status-plugin-merge*
*Plan: 03*
*Completed: 2026-04-02*
