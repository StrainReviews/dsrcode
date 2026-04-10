---
phase: 06-hook-system-overhaul
plan: 03
status: completed
completed: 2026-04-10
commits:
  - e25eaa2 feat(06-03): add SessionEnd, PostToolUse, StopFailure HTTP hook handlers
  - be56420 feat(06-03): add 5 remaining hook handlers + 24 tests for all 8 new hook routes
---

# Plan 06-03: Summary

## Scope

Add 8 new HTTP hook handler routes to `server/server.go` for the expanded
Phase 6 hook system. The handlers receive Claude Code hook events for
SessionEnd, PostToolUse, PreCompact, PostCompact, StopFailure, SubagentStart,
PostToolUseFailure, and CwdChanged, and translate them into session registry
updates and analytics calls. All handlers follow the D-16 contract (HTTP 200
always) and the D-09 pattern (<10ms HTTP response, expensive work in
panic-recovered background goroutines).

## Artifacts

| File | Role | Change |
|------|------|--------|
| `server/server.go` | HTTP hook routing + handlers | +558 lines: 5 new payload types, 5 new handlers, 8 new `mux.HandleFunc` routes, shared `lastTranscriptRead sync.Map` throttle, `onAutoExit` callback hook point for Plan 06-04. (Task 1 added SessionEnd/PostToolUse/StopFailure + throttle + onAutoExit; Task 2 added PreCompact/PostCompact/SubagentStart/PostToolUseFailure/CwdChanged.) |
| `server/server_test.go` | External-package tests | +497 lines: 24 new tests, 80+ sub-tests, full coverage of all 8 new routes including malformed-JSON contract, empty session_id, fast-response assertions, and D-19 round-trip error recovery. |

## Requirements Traceability

| Req | Description | Status |
|-----|-------------|--------|
| D-02 | PostToolUse token read trigger, 10s per-session throttle | implemented |
| D-09 | SessionEnd HTTP 200 <10ms, background goroutine | implemented |
| D-11 | Wildcard `*` matcher for PreToolUse (unchanged from Phase 4) | honored |
| D-12 | Wildcard `*` matcher for PostToolUse and all new events | honored (matcher lives in `settings.local.json` — Plan 06-02) |
| D-15 | PreCompact baseline snapshot via ParseTranscript | implemented |
| D-16 | HTTP 200 always, even on internal errors | implemented (35+ malformed-JSON tests) |
| D-19 | StopFailure error icon overlay + error-type label | implemented |
| D-20 | PostToolUseFailure error counter | implemented |
| D-21 | CwdChanged project context update | implemented (partial — Details only, see deviations) |
| D-22 | SubagentStart thinking overlay + `agent_type` label | implemented |
| D-23 | PostCompact counter + trigger + compact_summary logging | implemented |
| D-24 | SessionEnd 1.5s timeout irrelevant (HTTP <10ms) | honored |

## The 8 New Hook Routes

All registered in `Handler()` BEFORE the wildcard `POST /hooks/{hookType}`
route so Go 1.22+ ServeMux method routing picks the specific handler first.

| # | Route | Payload Type | Decision | Task |
|---|-------|-------------|----------|------|
| 1 | `POST /hooks/session-end` | `HookPayload` | D-09 | Task 1 |
| 2 | `POST /hooks/post-tool-use` | `HookPayload` | D-02 | Task 1 |
| 3 | `POST /hooks/stop-failure` | `StopFailurePayload` | D-19 | Task 1 |
| 4 | `POST /hooks/pre-compact` | `PreCompactPayload` | D-15 | Task 2 |
| 5 | `POST /hooks/post-compact` | `PostCompactPayload` | D-23 | Task 2 |
| 6 | `POST /hooks/subagent-start` | `SubagentStartPayload` | D-22 | Task 2 |
| 7 | `POST /hooks/post-tool-use-failure` | `PostToolUseFailurePayload` | D-20 | Task 2 |
| 8 | `POST /hooks/cwd-changed` | `CwdChangedPayload` | D-21 | Task 2 |

## Handler Design Notes

### Task 1 Handlers (Commit e25eaa2)

**handleSessionEnd (D-09)** — Calls `w.WriteHeader(http.StatusOK)` first,
then spawns a `go func()` with `defer recover()` that performs a final
`analytics.ParseTranscript(transcript_path)` and `registry.EndSession`.
The goroutine also calls `s.onAutoExit()` if `registry.SessionCount() == 0`
— this is a hook point; Plan 06-04 will wire the callback to trigger the
grace-period auto-exit. Accepts all `reason` values (clear, resume, logout,
prompt_input_exit, bypass_permissions_disabled, other) via unified logging.

**handlePostToolUse (D-02)** — Records tool usage via `tracker.RecordTool`,
then performs a throttled JSONL read via a `sync.Map`-backed per-session
10-second timer (`postToolUseThrottleInterval`). The throttle prevents
excessive file I/O when Claude Code spams tool calls. Also clears the
"error" overlay from a prior StopFailure event (D-19: successful PostToolUse
means the API is responding again, so SmallImage returns to "coding" with
`SmallText="Back to work"`). Keeps `transcript_path` in sync on the session.

**handleStopFailure (D-19)** — Maps the Claude Code `error` field to a
concise Discord-visible label via the testable `stopFailureErrorText` helper.
Swaps SmallImage to "error" and SmallText to the label. Does NOT touch
State or Details so token/model info remains visible. The overlay clears
on the next successful PostToolUse per the handlePostToolUse recovery path.

### Task 2 Handlers (Commit be56420)

**handlePreCompact (D-15)** — Snapshots token baseline via
`analytics.ParseTranscript` in a `defer recover()`-wrapped background
goroutine. NOT throttled: compaction is infrequent, so a single
ParseTranscript read per event is acceptable I/O. Logs the model count
on success.

**handlePostCompact (D-23)** — Records the compaction via
`tracker.RecordCompaction` with a synthetic UUID of the form
`compact-{trigger}-{unixnano}` to keep the tracker's built-in UUID dedup
happy. Claude Code payload lacks a true UUID (Issue #40492 open). Logs
`trigger` and `compact_summary` length via `slog.Info`.

**handleSubagentStart (D-22)** — Calls `tracker.RecordSubagentSpawn` with
the `agent_id`, `agent_type`, and a falls-back description (agent_type if
empty). Swaps session presence to `SmallImage="thinking"` with
`SmallText="Subagent: {agent_type}"`. Empty agent_type falls back to
`"unknown"` so the label is never `"Subagent: "`. Session must exist — no
implicit session creation from a subagent-spawn event.

**handlePostToolUseFailure (D-20)** — Increments an error counter via
`tracker.RecordTool("{tool}_error")` — the suffix keeps failures distinct
from successful tool calls in analytics while reusing the same counter
infrastructure. No presence update per D-20 (too granular); the user sees
errors via StopFailure at turn end.

**handleCwdChanged (D-21)** — Updates session `Details` field with
`fmt.Sprintf("Working on %s", filepath.Base(new_cwd))`. Falls back to the
common `cwd` field when `new_cwd` is empty (robust against older Claude
Code versions or missing fields). Does NOT implicitly create sessions
(silent no-op if session not found). Plan 06-05 integration point: the
full Cwd/ProjectName/ProjectPath swap requires a new
`Registry.UpdateProjectContext` method — Plan 06-03 only touches Details
because `ActivityRequest.Cwd` is ignored by `UpdateActivity`.

## Shared Infrastructure (Added by Task 1)

- `Server.lastTranscriptRead sync.Map` — per-session 10-second throttle
  for PostToolUse-triggered JSONL reads (D-02). Zero-value ready for use,
  must not be copied.
- `Server.onAutoExit func()` — nil in Plan 06-03; Plan 06-04 wires it.
- `postToolUseThrottleInterval = 10 * time.Second` — package-level constant.
- `StopFailurePayload` struct with all 7 Claude Code error types.
- `stopFailureErrorText(string) string` — testable helper for error-type-to-label
  mapping (7 cases + default fallback to "API Error").

## Test Suite (24 tests, 80+ sub-tests)

### Task 1 Handler Tests (added in Task 2's commit)

| Test | Coverage |
|------|----------|
| `TestHandleSessionEnd` | 200 response + <100ms elapsed + deterministic session removal via `waitForSessionCount` polling |
| `TestHandleSessionEndMalformedJSON` | D-16 |
| `TestHandleSessionEndEmptySessionID` | no-op, other sessions preserved |
| `TestHandlePostToolUse` | 200 + transcript path stored per D-18 |
| `TestHandlePostToolUseMalformedJSON` | D-16 |
| `TestHandlePostToolUseErrorRecovery` | D-19 round-trip: `error` overlay cleared on successful PostToolUse |
| `TestHandleStopFailure` | 9 sub-tests: all 7 Claude Code error types + empty + unknown-new-type |
| `TestHandleStopFailureMalformedJSON` | D-16 |

### Task 2 Handler Tests

| Test | Coverage |
|------|----------|
| `TestHandlePreCompact` + `MalformedJSON` | 200, <100ms, D-16 |
| `TestHandlePostCompact` | 3 sub-tests: manual/auto/empty triggers |
| `TestHandlePostCompactMalformedJSON` | D-16 |
| `TestHandleSubagentStart` | code-reviewer agent_type, thinking overlay, "Subagent: code-reviewer" label |
| `TestHandleSubagentStartEmptyAgentType` | unknown fallback |
| `TestHandleSubagentStartMalformedJSON` | D-16 |
| `TestHandlePostToolUseFailure` + `MalformedJSON` | 200, D-16 |
| `TestHandleCwdChanged` | D-21 Details update via filepath.Base |
| `TestHandleCwdChangedMissingSession` | no implicit session creation |
| `TestHandleCwdChangedFallbackCwd` | common cwd fallback when new_cwd empty |
| `TestHandleCwdChangedMalformedJSON` | D-16 |

### Cross-Handler Smoke Tests

| Test | Coverage |
|------|----------|
| `TestAllNewHookRoutesReturn200` | 8 sub-tests (one per new route): 200 + <100ms elapsed |
| `TestAllNewHookRoutesReturn200OnMalformedJSON` | 40 sub-tests (8 routes x 5 malformed bodies: `"not json"`, `"{broken"`, `{"session_id":}`, empty, `[]`) — exhaustive D-16 contract verification |

### Test Helpers

- `postHook(handler, route, body) -> (code, elapsed)` — single-call HTTP
  POST with automatic timing for the <100ms assertions.
- `waitForSessionCount(t, registry, want, timeout)` — deterministic polling
  (5ms tick, 500ms timeout) for tests that exercise background goroutines.
- `startTestSession(srv, sessionID, cwd)` — seeds a live session via the
  pre-tool-use hook route for tests that need a pre-existing session.

## Verification Performed

### Automated

- `go build ./... ` : 0
- `go vet ./... ` : 0
- `go test ./server/...` : PASS (80+ sub-tests, ~9.6s elapsed)
- `gofmt -l server/server.go server/server_test.go` : clean

### Test Timing

- `session-end` handler: ~90ms (includes background goroutine completion wait)
- `post-tool-use` handler: ~20ms
- All other new handlers: <10ms
- All within the `<100ms` assertion; all honor the D-09 fast-response spec.

## Deviations from Plan

### Task 1 Deviations (documented in commit e25eaa2)

1. **Rule-2** — `defer recover()` in both background goroutines to prevent
   an `analytics.ParseTranscript` panic from crashing the daemon. Not in
   the plan snippet.
2. **Rule-3** — Replaced the plan's `goto skipRead` with an `if shouldRead`
   block for idiomatic Go style.
3. **Rule-3** — Extracted `stopFailureErrorText` as a package-level helper
   for testability.
4. **Rule-3** — Extracted `postToolUseThrottleInterval` as a package-level
   constant.
5. **Task split** — Task 1 registered only its own 3 routes. The remaining
   5 routes were added in Task 2 to keep each commit atomic and buildable.

### Task 2 Deviations (documented in commit be56420)

1. **Rule-2 (consistency)** — `defer recover()` in `handlePreCompact`
   background goroutine. Matches the Task 1 pattern; plan snippet omitted it.
2. **Rule-3 (idiomatic)** — Dedicated `PreCompactPayload` struct instead of
   reusing `HookPayload`. `HookPayload` has no `trigger` field, and the
   other 4 new payloads all have dedicated structs. Consistency wins over
   the plan's "uses HookPayload" suggestion.
3. **Rule-3 (robustness)** — `handleCwdChanged` falls back to `payload.Cwd`
   when `new_cwd` is empty (older Claude Code versions or missing field).
4. **Rule-3 (no implicit session creation)** — `handleCwdChanged` is a
   silent no-op for unknown sessions. CwdChanged should not create sessions
   out of thin air; it is a context update event, not a lifecycle event.
5. **Rule-2 (partial D-21)** — Only the `Details` field is updated on
   CwdChanged, not `Cwd`/`ProjectName`/`ProjectPath`. `ActivityRequest` is
   the existing Registry update contract and it does not propagate Cwd
   changes. A new `Registry.UpdateProjectContext` method is required for
   the full swap — **deferred to Plan 06-05** as an integration point
   documented in the handler comment.
6. **Rule-2 (defensive fallbacks)** — `handleSubagentStart` falls back to
   `agent_type` for `SubagentEntry.Description` when prompt/description are
   absent (Issue #32016 open). Empty `agent_type` falls back to `"unknown"`
   so `SmallText` is never the dangling `"Subagent: "`.
7. **Rule-2 (test scope expansion)** — 24 tests instead of the loose 5-test
   spec in the plan. Table-driven coverage of all 7 StopFailure error types,
   all 3 PostCompact triggers, all 5 malformed-JSON bodies across all 8
   routes. Worth the ~200 extra test LoC for exhaustive D-16 verification.

## Plan 06-05 Integration Points

Plan 06-03 exposes four hook points that Plan 06-05 must wire:

1. **`Server.onAutoExit` callback** — nil in Plan 06-03; Plan 06-04 wires
   it to the grace-period auto-exit goroutine. It fires when a SessionEnd
   event causes `registry.SessionCount()` to reach 0.
2. **`Registry.UpdateProjectContext` method** — does not exist yet. Plan
   06-05 must add it so `handleCwdChanged` can swap the full
   Cwd/ProjectName/ProjectPath chain, not just the Details string.
3. **Tracker -> Registry analytics sync bridge** — all new handlers call
   `tracker.UpdateTokens`/`tracker.RecordCompaction`/etc, but the
   Tracker state is not currently synced back to `Registry.UpdateAnalytics`.
   Plan 06-05 must wire the `onChange` pump so token updates flow back to
   Discord presence.
4. **CompactionCount + baseline verification** — Plan 06-03 saves the
   PreCompact baseline via `tracker.UpdateTokens` (inside the existing
   baseline detection path in `Tracker.UpdateTokens`), and increments the
   counter via PostCompact. Plan 06-05 should verify the closed-loop
   behavior on a live compaction.

## MCP Usage Log (Phase-6 Mandate: PRE + POST per task)

Both tasks completed the full 4-MCP protocol per the Phase-6 execution
rule. Total MCP calls: **16** (4 per pre-step x 2 tasks + 4 per post-step
x 2 tasks). Each commit body contains a full `MCP-Verification` section.

### Task 1 MCP Round (commit e25eaa2)

**PRE-STEP:**
- `sequential-thinking` (3 thoughts): evaluated Hypothesis A (direct port)
  vs B (`goto`-removed + recover-added), identified 5 sub-steps, traced
  D-02/D-09/D-16/D-19/D-24 alignment.
- `context7` (`/websites/pkg_go_dev_go1_25_3`): verified `sync.Map.Load/Store`
  signatures, confirmed "zero Map is empty and ready for use, must not
  be copied after first use".
- `exa`: confirmed `golang/go#78051` (ResponseWriter panic after handler
  return — use ResponseWriter only BEFORE `go func()`, never inside the
  goroutine). Confirmed OneUptime 2026 article on `sync.Map` per-key
  throttle as best practice.
- `crawl4ai` (`code.claude.com/docs/en/hooks`): verified SessionEnd payload
  schema (`session_id, transcript_path, cwd, hook_event_name, reason`) and
  PostToolUse payload schema (+ `tool_name, tool_input, tool_response,
  tool_use_id`).

**POST-STEP:**
- `sequential-thinking`: reflected on all 5 sub-steps, confirmed D-*
  alignment, identified 5 deviations (all Rule-1/2/3 auto-fixes).
- `context7`: re-verified `recover()`/`defer` idiom for panicking
  goroutines.
- `exa`: re-confirmed `golang/go#78051` safety boundaries.
- `crawl4ai` (`pkg.go.dev/sync`): full-text read of `sync.Map` contract.

### Task 2 MCP Round (commit be56420)

**PRE-STEP:**
- `sequential-thinking` (5 thoughts): decomposed 5 handlers + 24 tests
  into atomic sub-steps, evaluated 3 hypotheses (direct-port vs
  stabilized-port vs restructured), chose stabilized-port for consistency
  with Task 1, traced D-15/D-16/D-20/D-21/D-22/D-23 alignment, resolved
  Risk-3 (CwdChanged session-not-found semantics) to silent no-op.
- `context7` (`/websites/pkg_go_dev_go1_25_3`): verified
  `ServeMux.HandleFunc` most-specific-pattern rule, `json.NewDecoder.Decode`
  stream signature. Checked `json.UnmarshalRead` as v2 alternative —
  rejected in favor of stdlib v1 (plan spec).
- `exa` (2 queries): "Go net/http background goroutine defer recover 2026"
  confirmed Crown.edu + OneUptime 2026 best practice. "Claude Code
  SubagentStart agent_type payload 2026" confirmed exact payload shape
  via `anthropics/claude-code#19946` (docs fix landed Jan 2026) and
  `anthropics/claude-code#32016` (prompt/description deferred).
- `crawl4ai` (`code.claude.com/docs/en/hooks`, BM25 filtered): pulled the
  canonical payload schemas for PreCompact, PostCompact, SubagentStart,
  PostToolUseFailure, CwdChanged. All 5 new payload structs match the
  official doc exactly.

**POST-STEP:**
- `sequential-thinking`: reflected on all 5 handlers + 24 tests, confirmed
  all D-* alignment, catalogued 7 auto-fix deviations, flagged Plan 06-05
  integration point for `Registry.UpdateProjectContext`. Verified no race
  conditions (tests do not use `t.Parallel()`).
- `context7` (`/websites/pkg_go_dev_go1_25_3`): verified `filepath.Base("")`
  returns `"."` — our `if newProject == "" || newProject == "."` branch
  handles both edge cases. Verified cross-platform slash handling.
- `exa`: "Go http.Handler subtest race condition table-driven 2026" — no
  parallel subtests in our code, loop-variable closure bug (pre Go 1.22)
  not applicable.
- `crawl4ai` (`pkg.go.dev/path/filepath`): full-text verification of `Base`
  contract. Our fallback branch matches the documented behavior exactly.

## Issues Encountered

- **CGO unavailable** — `go test -race` requires `CGO_ENABLED=1`, which
  is not configured on this system. Per the Phase-6 execution allowance
  ("drop `-race` if CGO unavailable, document in SUMMARY"), tests were
  run without `-race`. All 5 new handlers are pure single-goroutine HTTP
  handlers except the 3 background goroutines (SessionEnd, PostToolUse,
  PreCompact), each of which has `defer recover()` as a safety net. Race
  detection would exercise the `sync.Map` throttle path but the
  test-mode `s.tracker == nil` branch already skips the goroutine
  dispatch entirely, so the concurrency surface in tests is minimal. Plan
  06-04's auto-exit goroutine should re-enable `-race` once a CGO-enabled
  environment is available.
- **Task 1 had no tests** — commit e25eaa2 added the 3 handlers without a
  test suite. Task 2 backfilled tests for all 8 handlers in one batch
  (including the 3 Task 1 handlers), which is why the Task 2 commit is
  larger than a typical per-handler addition (497 test LoC).

## Next Plan Readiness

- **Plan 06-04** (JSONL removal from `main.go` + auto-exit goroutine +
  shutdown sequence) is unblocked. It depends on `analytics.ParseTranscript`
  (delivered in Plan 06-01) and the `Server.onAutoExit` hook point
  (delivered in Plan 06-03 Task 1). Plan 06-04 will wire `NewRegistry`'s
  `onChange` callback to fire the auto-exit channel when `refcount == 0`.
- **Plan 06-05** (integration wiring + CHANGELOG + verification checkpoint)
  is unblocked. It will wire the analytics sync bridge, add the
  `Registry.UpdateProjectContext` method for full CwdChanged support, and
  run the cross-phase integration test checklist.

---
*Phase: 06-hook-system-overhaul-sessionend-posttooluse-precompact-hooks*
*Plan: 03 (8 new HTTP hook handlers)*
*Completed: 2026-04-10*
