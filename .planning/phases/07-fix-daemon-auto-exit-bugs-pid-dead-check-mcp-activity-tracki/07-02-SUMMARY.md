---
phase: 7
plan: 07-02
subsystem: session-registry + hook-server
tags: [bugfix, mcp-activity, stale-detector, registry, post-tool-use]
requires: [session/registry.go existing mutation patterns, server/server.go handlePostToolUse]
provides: [SessionRegistry.Touch method, PostToolUse activity-clock wiring]
affects: [session.SessionRegistry, server.handlePostToolUse]
tech_stack_added: []
patterns_used: [immutable-copy-before-modify, write-lock-serialized-mutation, silent-no-op-for-unknown-id]
key_files_created: []
key_files_modified:
  - session/registry.go
  - session/registry_test.go
  - server/server.go
  - server/server_test.go
decisions: [D-04, D-05, D-06]
duration_minutes: ~10
completed_date: 2026-04-13
commits:
  - 41401b8 feat(07-02): add registry.Touch()
  - 6ddf6dd feat(07-02): wire Touch into handlePostToolUse
---

# Phase 7 Plan 02: registry.Touch() + handlePostToolUse activity-clock update — Summary

**One-liner:** Added `SessionRegistry.Touch(sessionID)` that refreshes `LastActivityAt` under a write lock without firing `notifyChange`, and wired it into `handlePostToolUse` so every MCP tool call keeps the stale-detector's clock fresh with zero Discord presence side-effects.

## What changed

### `session/registry.go`
- Added exported `func (r *SessionRegistry) Touch(sessionID string)` directly below `UpdateActivity` (registry.go:187 region). Acquires `r.mu.Lock()` (write lock), copies the session struct, sets `LastActivityAt = time.Now()`, and writes back — explicitly does **not** call `notifyChange()` per D-05. No-op if sessionID unknown.

### `session/registry_test.go`
- `TestTouchUpdatesLastActivityWithoutNotify` — confirms LastActivityAt advances and onChange counter stays at 0.
- `TestTouchIsNoOpForUnknownSession` — confirms no panic and no callback for unknown IDs (fatal-callback pattern).

### `server/server.go`
- Inserted `s.registry.Touch(payload.SessionID)` with a `Phase 7 D-04` comment block immediately after the empty-session_id guard in `handlePostToolUse` (before the analytics-throttle block), so activity-clock freshness is not gated by the 10s transcript-read throttle.

### `server/server_test.go`
- `TestHandlePostToolUseUpdatesLastActivity` — forces session 5 minutes stale, POSTs, asserts `LastActivityAt` back within 5s (D-04 regression guard).
- `TestHandlePostToolUseDoesNotChangeUiFields` — pre-sets SmallImageKey/SmallText/Details, POSTs with an `mcp__*` tool_name, asserts all UI fields and `ActivityCounts` unchanged (D-05 invariance guard).

## Test results

```
go test ./session/... ./server/... ./...
ok  github.com/StrainReviews/dsrcode/session   4.804s
ok  github.com/StrainReviews/dsrcode/server    14.237s
ok  github.com/StrainReviews/dsrcode           (cached)
... (all other packages green)
```

Touch-specific tests: 2/2 PASS. Server PostToolUse tests: 7/7 PASS (including 2 new ones and 5 existing regressions, no breakage). `go vet ./...` clean. `gofmt -l` on modified files: empty.

Note: `-race` could not be run because CGO is disabled on this Windows host (`go: -race requires cgo; enable cgo by setting CGO_ENABLED=1`). Tests still validate correctness; race detection deferred to CI Linux runners (standard project posture for Windows dev machines).

## Must-Haves verified

| # | Truth | Status |
|---|-------|--------|
| 1 | `Touch("known")` advances LastActivityAt AND onChange stays at 0 | PASS — TestTouchUpdatesLastActivityWithoutNotify |
| 2 | `Touch("unknown")` silent no-op, no panic, no callback | PASS — TestTouchIsNoOpForUnknownSession |
| 3 | POST /hooks/post-tool-use refreshes LastActivityAt within 5s | PASS — TestHandlePostToolUseUpdatesLastActivity |
| 4 | POST does NOT change SmallImageKey/SmallText/Details/ActivityCounts | PASS — TestHandlePostToolUseDoesNotChangeUiFields |
| 5 | No hook-config file changed | VERIFIED — `git status --short hooks/ scripts/start.sh` empty; start.sh:470 wildcard matcher pre-existing |
| A1 | `Touch` method exists with correct signature below UpdateActivity | VERIFIED |
| A2 | Both Touch tests in registry_test.go | VERIFIED |
| A3 | `s.registry.Touch(payload.SessionID)` with D-04 comment in handlePostToolUse | VERIFIED |
| A4 | Both server tests in server_test.go | VERIFIED |
| Key | Touch uses `r.mu.Lock()` not `RLock`, and does NOT call `notifyChange` | VERIFIED — code inspection + test |

## Deviations from Plan

**None.** Plan executed exactly as written. One pre-existing `gofmt` drift in `session/types.go` (unrelated, untouched by this plan) is out of scope per the Scope Boundary rule; logged for future cleanup but not fixed here.

## Threat Flags

None. No new I/O, no new network surface, no new auth boundary. Threat Model entries T-07-02 (Tampering via concurrent Touch+UpdateActivity) and T-07-03 (DoS via Touch storm) are both addressed by the write-lock serialization and O(1) lock-hold cost respectively.

## MCP citations

- **mcp__sequential-thinking__sequentialthinking** — Decomposed the plan into 2 code tasks + 1 verification task; confirmed Touch belongs as sibling of UpdateActivity (same file, 8-line mechanical twin of SetLastActivityForTest). Verified the write-lock requirement and the no-`notifyChange` guarantee as the critical invariants.
- Context7 / Exa not required for final implementation — the plan's `read_first` sections (registry.go:137-185 sibling UpdateActivity, registry.go:431-445 twin SetLastActivityForTest, 07-PATTERNS.md excerpt, 07-RESEARCH.md Example 2) provided verbatim reference material. Pattern is a direct local analog.

## Known Stubs

None — no placeholder UI fields, no hardcoded mock data introduced.

## Self-Check: PASSED

- `session/registry.go` Touch method present — FOUND (grep at line 187 region)
- `session/registry_test.go` both tests present — FOUND
- `server/server.go` Touch invocation present — FOUND
- `server/server_test.go` both new tests present — FOUND
- Commit 41401b8 — FOUND in git log
- Commit 6ddf6dd — FOUND in git log
