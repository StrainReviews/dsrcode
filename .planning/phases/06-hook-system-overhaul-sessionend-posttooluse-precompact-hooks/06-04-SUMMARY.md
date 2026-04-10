---
phase: 06-hook-system-overhaul
plan: 04
status: completed
completed: 2026-04-10
commits:
  - 03b97b6 refactor(06-04): remove JSONL fallback code from main.go and main_test.go
  - 8d13c43 feat(06-04): add auto-exit goroutine with grace period and shutdown sequence
---

# Plan 06-04: Summary

## Scope

Remove the entire JSONL polling/watching/discovery pipeline from `main.go`
and `main_test.go` (now that `analytics.ParseTranscript` from Plan 06-01 is
the single source of truth for hook-triggered transcript reads), and add a
dual-trigger grace-period auto-exit goroutine that shuts the daemon down
cleanly when all sessions end. Implements decisions D-01 (JSONL removal),
D-04 (dual-trigger auto-exit), D-05 (configurable grace period, 0=disabled),
D-06 (grace-abort on new SessionStart), D-07 (ordered shutdown sequence),
D-08 (timer stays primary), D-17 (exact file/line list for removal), and
D-18 (analytics/ untouched).

## Artifacts

| File | Role | Change |
|------|------|--------|
| `main.go` | Daemon entry point | -390 lines (JSONL removal) + ~148 lines (auto-exit goroutine + shutdown sequence); net 879 -> 637 |
| `main_test.go` | Main package tests | -378 lines (TestParseJSONLSession, TestPathDecoding, TestFindMostRecentJSONL, TestReadStatusLineData + helpers); net 547 -> 169 |
| `server/server.go` | HTTP server | +10 lines (`SetOnAutoExit(f func())` public setter for D-04 SessionEnd trigger path) |

## Requirements Traceability

| Req | Description | Status |
|-----|-------------|--------|
| D-01 | Remove JSONL polling/watching/discovery, keep parsing in `analytics/` | implemented |
| D-04 | Dual-trigger auto-exit (SessionEnd hook + stale detection) | implemented |
| D-05 | Configurable `shutdownGracePeriod` (30s default, 0=disabled) consumed | implemented |
| D-06 | Grace-abort on new SessionStart during grace period | implemented |
| D-07 | Shutdown sequence: clear activity -> Close Discord IPC -> cancel ctx -> drain | implemented |
| D-08 | Stale detection timer remains primary (30s) | honored (no changes to `session.CheckStaleSessions`) |
| D-10 | PID liveness check unchanged | honored |
| D-17 | Exact deletion list from `06-CONTEXT.md` followed | implemented (+1 auto-fix for TestReadStatusLineData) |
| D-18 | `analytics/` package untouched | honored |

## Task 1: JSONL Fallback Removal (commit 03b97b6)

### Deleted from main.go (-390 lines)

**Functions:**
- `jsonlFallbackWatcher()` — fsnotify watcher + 3s polling ticker
- `jsonlPollLoop()` — polling fallback when fsnotify unavailable
- `ingestJSONLFallback()` — synthetic "jsonl-*" session bootstrap
- `readStatusLineData()` — reader for `~/.claude/dsrcode-data.json`
- `findMostRecentJSONL()` — walker over `~/.claude/projects/*/`
- `parseJSONLSession()` — the JSONL line-scanner (migrated to `analytics.ParseTranscript` in Plan 06-01)
- `readSessionData()` — dispatcher (statusline first, then JSONL)

**Types:**
- `StatusLineData`, `SessionData`, `JSONLMessage`

**Variables and constants:**
- `projectsDir`, `dataFilePath`, `sessionStartTime`, `jsonlDeprecationOnce`
- `PollInterval` const
- `go jsonlFallbackWatcher(...)` goroutine launch in `main()`

**Imports cleaned:**
- `bufio`, `io/fs`, `sort`, `github.com/fsnotify/fsnotify` (orphaned)

**`init()` reduced to one line:** only `claudeDir` is initialized now (still
needed for `analytics.TrackerConfig.DataDir`).

### Deleted from main_test.go (-378 lines)

- `TestParseJSONLSession`
- `TestPathDecoding` + its helpers (`replaceDoubleDash`, `replaceSingleDash`, `replacePlaceholder`)
- `TestFindMostRecentJSONL`
- `TestReadStatusLineData` (hard dependency on deleted `dataFilePath` and `sessionStartTime`) — auto-fix per Rule 3 since the plan did not explicitly list it but it was otherwise a compile failure
- `os`, `path/filepath`, `time` imports (orphaned)

### Preserved per plan

- `formatModelName()`, `formatNumber()`, `getGitBranch()` (still referenced at
  least from the tests or the legacy export — note: Plan 06-05 will re-wire
  `syncAnalyticsToRegistry` which is kept in Task 1 for that reason)
- `syncAnalyticsToRegistry()` — no callers after Task 1, but Plan 06-05 will
  re-wire it. Go allows unused package-level funcs.
- `modelDisplayNames` map (still used by `formatModelName`)
- All other main.go code paths: `discordConnectionLoop`, `presenceDebouncer`,
  HTTP server wiring, config watcher, stale checker
- `TestCalculateCost`, `TestFormatModelName`, `TestFormatNumber`,
  `TestModelDisplayNames`
- `analytics/` package (entirely untouched per D-18)

### Line-count outcome

| File | Before | After | Delta |
|------|--------|-------|-------|
| `main.go` | 879 | 489 | -390 |
| `main_test.go` | 547 | 169 | -378 |
| **Total** | **1426** | **658** | **-768** |

## Task 2: Auto-Exit Goroutine (commit 8d13c43)

### Channel + goroutine wiring

1. **`autoExitChan`** — buffered channel (size 1) for refcount=0 pulses.
   Created in `main()` before `session.NewRegistry` so the wrapped
   `onChange` callback can reference it.

2. **`session.Registry.onChange` wrapper** — now pushes to BOTH `updateChan`
   (existing presence debounce) and `autoExitChan` (new D-04 stale detector
   trigger). Both sends are non-blocking selects with `default`.

3. **`Server.SetOnAutoExit(f)`** — new public setter on `server.Server` that
   installs the D-04 SessionEnd trigger path. `main.go` calls it immediately
   after `srv.SetTracker(tracker)` and BEFORE `srv.Start(...)` so there is
   no race where a SessionEnd event could land before the callback is wired.
   The callback pushes into `autoExitChan`.

4. **`autoExitLoop` goroutine function** (top-level, named):
   - If `gracePeriod == 0` (D-05 sentinel): log and return immediately.
   - Loop body: `select { case <-ctx.Done(): return; case <-autoExitChan: ... }`
   - On pulse, re-query `registry.SessionCount()`.
   - `count == 0` and no timer pending: `context.WithCancel(ctx)` +
     `time.AfterFunc(gracePeriod, ...)`. The timer callback checks
     `graceCtx.Err() != nil` as a race guard before calling `cancel()`.
   - `count == 0` and timer already pending: continue (no change, still
     counting down).
   - `count > 0` and timer pending: grace-abort per D-06 via the local
     `abortFn` closure which cancels `graceCtx` FIRST (so an in-flight
     timer callback takes its early-exit branch) THEN stops the timer.
   - On ctx.Done: deferred cleanup fires `abortFn` to cancel any pending
     graceCtx and stop the timer.

5. **Updated shutdown sequence** at the tail of `main()` per D-07:
   ```
   1. SetActivity(discord.Activity{}) — clears Rich Presence over live IPC
   2. discordClient.Close() — closes the IPC conn (with err log)
   3. cancel() — triggers server.Server.Start's existing 5s
      httpServer.Shutdown drain, stops stale checker, debouncer, config
      watcher, and the auto-exit loop itself.
   ```
   The pre-existing "cancel(); Close(); log" order is replaced because D-07
   explicitly requires clearing Discord activity BEFORE tearing down the
   IPC conn. `cancel()` remains idempotent (multiple paths call it) per
   Go stdlib guarantee.

### Server.SetOnAutoExit setter (out-of-scope modification)

`server/server.go` is not listed in Plan 06-04's `files_modified` but Task 2
required adding a single public setter (+7 lines, +3 lines doc) so `main.go`
can wire the SessionEnd trigger. Without it, the Plan 06-03 hook point
`s.onAutoExit` remains dead code and D-04 dual-trigger is violated. Added
as a minimum-viable surface matching the existing `SetTracker` pattern.

Rule 2 (auto-add missing critical functionality) + Rule 3 (auto-fix blocking
issue) are the justification. The plan's own Task 2 text explicitly
acknowledges the need: "The Shutdown call needs to exist on server.Server.
Let's check if it already exists. If srv.Shutdown doesn't exist yet...".

### Key design decisions

1. **`abortFn` closure pattern for vet lostcancel.** The plan's literal
   snippet used outer-scope `graceCtx`/`graceCancel` variables, which fails
   `go vet`'s `lostcancel` check because the compiler cannot prove every
   control-flow path through the outer for-select reaches a cancel call.
   Refactored to a single `abortFn func()` closure that captures the local
   `graceCancel` and `timer`. Both the abort path and the deferred cleanup
   call `abortFn()`, so vet sees every branch going through one function.
   See `context.WithCancel` docs: "The go vet tool checks that CancelFuncs
   are used on all control-flow paths."

2. **Cancel BEFORE Stop ordering.** `time.AfterFunc(f).Stop()` can return
   `false` if `f` is already running. `abortFn` cancels `graceCtx` FIRST so
   `f`'s `graceCtx.Err() != nil` race guard takes the early-exit branch,
   THEN calls `timer.Stop()`. The order is load-bearing — inverting it
   would open a window where `f` fires `cancel()` after a new session has
   arrived.

3. **`CancelFunc` idempotency.** `cancel()` is called from three paths:
   the sigChan select, the auto-exit timer callback, and the srv.Start
   error path. Per Go stdlib: "A CancelFunc may be called by multiple
   goroutines simultaneously. After the first call, subsequent calls to
   a CancelFunc do nothing." No coordination needed.

4. **Buffered channel + non-blocking send.** `autoExitChan` has buffer 1
   and all sends use `select { case ch <- struct{}{}: default: }`. If the
   buffer is full, the pulse is dropped — which is harmless because the
   loop will re-query `SessionCount()` on any later mutation anyway.

5. **Reusing server.Server.Start's existing Shutdown drain.** The
   `server.Server.Start` method already implements
   `httpServer.Shutdown(5s timeout)` on `ctx.Done()`. Rather than
   duplicating the `http.Server.Shutdown` contract in `main.go`, the D-07
   sequence just calls `cancel()` and lets Start handle the drain.

## Verification Performed

### Automated (final state, after both commits)

- `go build ./...` — exit 0
- `go vet ./...` — exit 0 (lostcancel passes after abortFn refactor)
- `go test ./...` — all packages pass:
  ```
  ok   github.com/StrainReviews/dsrcode           0.619s
  ok   github.com/StrainReviews/dsrcode/analytics (cached)
  ok   github.com/StrainReviews/dsrcode/config    (cached)
  ok   github.com/StrainReviews/dsrcode/discord   (cached)
  ok   github.com/StrainReviews/dsrcode/preset    (cached)
  ok   github.com/StrainReviews/dsrcode/resolver  (cached)
  ok   github.com/StrainReviews/dsrcode/server    8.409s
  ok   github.com/StrainReviews/dsrcode/session   (cached)
  ```
- `gofmt -l` — clean on all three modified files
- `-race` flag — not run (CGO unavailable on this environment, documented
  in Plan 06-01 and 06-03 SUMMARYs); the auto-exit goroutine and its
  channel/timer interactions are single-consumer on `autoExitChan` so the
  race-detector surface is minimal.

### Symbol grep clean-up

All of the following produce zero matches anywhere in the codebase:
- `jsonlFallbackWatcher`, `jsonlPollLoop`, `ingestJSONLFallback`
- `readStatusLineData`, `findMostRecentJSONL`, `parseJSONLSession`
- `readSessionData`, `StatusLineData`, `JSONLMessage`, `SessionData` (struct)
- `projectsDir`, `dataFilePath`, `sessionStartTime`, `jsonlDeprecationOnce`
- `PollInterval`
- `TestParseJSONLSession`, `TestPathDecoding`, `TestFindMostRecentJSONL`,
  `TestReadStatusLineData`
- `replaceDoubleDash`, `replaceSingleDash`, `replacePlaceholder`

### Acceptance criteria (from plan)

- [x] `main.go` does NOT contain any of the 7 JSONL helper function names
- [x] `main.go` does NOT contain `StatusLineData`, `JSONLMessage`,
      `SessionData` type declarations
- [x] `main.go` does NOT contain `PollInterval`
- [x] `main.go` DOES contain `formatModelName` (preserved)
- [x] `main.go` DOES contain `syncAnalyticsToRegistry` (preserved)
- [x] `main_test.go` does NOT contain the three JSONL test names
- [x] `main.go` contains `autoExitChan` channel
- [x] `main.go` contains `ShutdownGracePeriod` reference (via
      `cfg.ShutdownGracePeriod` passed to `autoExitLoop`)
- [x] `main.go` contains "grace period" in slog messages (3 occurrences)
- [x] `main.go` contains `SetActivity(discord.Activity{})` for clearing
- [x] `main.go` contains the D-07 shutdown sequence
- [x] `main.go` `onChange` callback signals both `updateChan` and
      `autoExitChan`
- [x] `go build ./...` exits 0
- [x] `go test ./...` exits 0

## Deviations from Plan

### Rule 2 (missing critical functionality)

1. **Added `Server.SetOnAutoExit(f func())` to `server/server.go`** —
   out-of-scope file per frontmatter, but D-04 dual-trigger is a must-have
   and the Plan 06-03 hook point is dead code without a public installer.
   Minimal surface (+10 lines) matching existing `SetTracker` pattern.
2. **Discord `Close()` return value logged** via `slog.Debug` — plan snippet
   omitted the error check.
3. **Grace-period `abortFn` closure pattern** — plan's literal snippet
   failed `go vet` lostcancel; refactored to the closure pattern so every
   control-flow path reaches exactly one cancel closure.

### Rule 3 (auto-fix blocking issues)

4. **`TestReadStatusLineData` removed from `main_test.go`** — plan did not
   list it but its `dataFilePath`/`sessionStartTime` references became
   compile errors after variable deletion. Hard dependency.
5. **`main_test.go` imports cleaned** — `os`/`path/filepath`/`time` became
   orphaned after the four test deletions. `go vet` flagged them.
6. **`main.go` imports cleaned** — `bufio`/`io/fs`/`sort`/`fsnotify` became
   orphaned after the six JSONL function deletions.
7. **`autoExitLoop` extracted as named top-level function** — plan snippet
   used an anonymous goroutine inside `main()`. Named function is
   doc-visible, potentially unit-testable, and keeps `main()` shorter.
8. **"shutdown initiated (auto-exit or fatal error)" log added** to the
   `<-ctx.Done()` path to distinguish auto-exit from signal.
9. **Fixed typo** in `SetTracker` docstring ("edpoint" -> "endpoint") while
   editing the adjacent block.

**Total deviations:** 9 auto-fixes (3 Rule 2 for missing functionality,
6 Rule 3 for blocking issues / consistency). All are forced by the plan
literal failing to compile, failing `go vet`, or requiring a plan-scope
extension to satisfy a must-have decision. No architectural changes beyond
what D-04/D-05/D-06/D-07 already specified.

## Issues Encountered

- **Initial `go vet` failure on Task 2:** the plan's literal snippet using
  outer-scope `graceCtx`/`graceCancel` variables failed `go vet`
  `lostcancel` check. Root cause: vet cannot prove every path through the
  outer for-select reaches a `cancel()` call because of the conditional
  `if graceTimer != nil { continue }` branch that skips the assignment.
  Refactored to the `abortFn` closure pattern per the context package
  docs ("The go vet tool checks that CancelFuncs are used on all
  control-flow paths"). Fix verified via `go vet ./...` exit 0.
- **CGO unavailable** — same limitation as Plans 06-01 and 06-03 prevents
  `-race`. Documented.

## MCP Usage Log (Phase-6 Mandate: PRE + POST per task)

Both tasks completed the full 4-MCP protocol per the Phase-6 execution
rule. Total MCP calls: **16** (4 per pre-step x 2 tasks + 4 per post-step
x 2 tasks). Each commit body contains a full `MCP-Verification` section.

### Task 1 MCP Round (commit 03b97b6)

**PRE-STEP:**
- `sequential-thinking` (3 thoughts): Decomposed the deletion task into
  ordered sub-steps (functions bottom-up to avoid line-number drift,
  then types, vars, consts, imports, main() call site). Verified that
  `getGitBranch` also exists in `server/server.go` (so the main.go copy
  becomes dead but legal per Go rules). Verified that
  `syncAnalyticsToRegistry` has no callers post-Task-1 but Plan 06-05
  will re-wire it. Evaluated HYPOTHESE A (targeted Edit) vs B (Write
  rewrite), chose A for reviewability. Traced D-01/D-17/D-18 alignment.
- `context7` (`/websites/pkg_go_dev_go1_25_3`): Verified that Go's
  `UnusedImport`/`UnusedVar`/`UnusedLabel` errors fire only at local
  scope (imports, local vars, labels) — package-level funcs and vars
  are silently allowed. Justifies keeping `syncAnalyticsToRegistry`,
  `getGitBranch`, `formatNumber` post-deletion.
- `exa` (2 queries): Confirmed the rule via Stack Overflow 32564935
  ("Why does golang allow compilation of unused functions?"),
  golang/go#26080, and OneUptime's 2026 "declared and not used" guide.
  Also confirmed Go refactoring best practice of verifying deletions
  via `go build` + `go vet` + `go test`.
- `crawl4ai` (pkg.go.dev/cmd/compile, BM25 filtered): Full-text read of
  the Go compiler command page. Confirmed "deadlocals pass removes
  assignments to unused local variables" — package scope is explicitly
  exempt from the unused-var rule.

**POST-STEP:**
- `sequential-thinking` (1 reflection): Confirmed grep-clean (no orphan
  references to 17 removed symbols), catalogued 5 Rule 2/3 auto-fixes
  (TestReadStatusLineData removal, main_test.go import cleanup, var
  block reduction, init() reduction, retaining `sync` import). D-*
  alignment confirmed.
- `context7`: Re-verified `go vet` check list to confirm build+vet+test
  covers the entire removal surface.
- `exa`: Go 1.26 `gofix` and `deadcode` analyzer blog post (Feb 2026)
  confirming the vet-based validation pipeline is correct.
- `crawl4ai` (pkg.go.dev/cmd/vet): Full vet check catalogue read.
  Confirms `unreachable` check covers dead code paths, `lostcancel`
  check covers `context.WithCancel` paths (relevant for Task 2!).

### Task 2 MCP Round (commit 8d13c43)

**PRE-STEP:**
- `sequential-thinking` (1 deep thought): Decomposed 5 architectural
  questions: autoExitChan design, onChange wrap vs Registry API change,
  Server.onAutoExit wiring strategy (OPTION A setter / B no-op /
  C constructor param), Timer.Stop vs cancel race ordering, shutdown
  sequence ordering. Chose OPTION A (minimal setter) after confirming
  D-04 dual-trigger is a must-have that can NOT be satisfied without a
  public installer on Server. Identified 4 race risks and mapped their
  mitigations to `CancelFunc` idempotency + `graceCtx.Err()` guard +
  the cancel-before-Stop ordering.
- `context7` (2 queries to `/websites/pkg_go_dev_go1_25_3`): Verified
  `context.CancelFunc` idempotency ("after the first call, subsequent
  calls do nothing", "may be called by multiple goroutines
  simultaneously"), `context.WithCancel` return contract, `time.AfterFunc`
  return contract ("waits for duration to elapse and then calls f in
  its own goroutine"), `Timer.Stop` return semantics ("returns false if
  the timer has already expired or been stopped"), and the vet
  lostcancel rule ("go vet tool checks that CancelFuncs are used on
  all control-flow paths" — directly relevant to the abortFn refactor).
- `exa` (2 queries): "Go time.AfterFunc Stop race already fired callback
  graceful cancellation" — rednafi 2026 article on WithCancelCause and
  manual-timer patterns confirming the graceCtx.Err() race guard is the
  canonical idiom; golang/go#75935 (October 2025) open issue on stopped
  timer arg retention (not a concern here because the `timer` variable
  is released when abortFn sets it nil-through-closure). "Go graceful
  shutdown context.WithCancel http.Server.Shutdown ordering" —
  unanswered.io 2026 guide confirming the "goroutine Start + main
  Shutdown on cancel" pattern which is exactly what server.Server.Start
  already implements.
- `crawl4ai` (pkg.go.dev/net/http, BM25 filtered): Full-text read of
  `Server.Shutdown` contract: "first closing all open listeners, then
  closing all idle connections, and then waiting indefinitely for
  connections to return to idle and then shut down". Confirms the 5s
  shutdown timeout in `server.Server.Start` is sufficient for dsrcode's
  HTTP hook handlers which all respond <100ms per Plan 06-03.

**POST-STEP:**
- `sequential-thinking` (1 reflection): Re-traced 5 race scenarios and
  confirmed mitigations. Catalogued 4 Rule 2/3 deviations including
  the server/server.go modification. Confirmed D-04 through D-07
  alignment. Verified concurrent-shutdown idempotency via CancelFunc
  guarantee.
- `context7`: Re-verified the vet lostcancel rule and confirmed the
  `abortFn` closure pattern is the idiomatic fix documented in the
  context package overview ("The go vet tool checks that CancelFuncs
  are used on all control-flow paths").
- `exa`: Stack Overflow 74354475 + 71206167 confirming the closure
  pattern is the community-accepted idiomatic lostcancel fix for
  conditional WithCancel inside loops.
- `crawl4ai` (pkg.go.dev/context): Full re-read of the CancelFunc
  concurrent-safety clause: "A CancelFunc may be called by multiple
  goroutines simultaneously. After the first call, subsequent calls
  to a CancelFunc do nothing." Justifies the multi-path `cancel()`
  design without extra coordination.

## Next Plan Readiness

- **Plan 06-05** (integration wiring: `syncAnalyticsToRegistry` bridge,
  `Registry.UpdateProjectContext` for D-21 full support, CHANGELOG.md
  v4.0.0 entries, cross-phase verification checklist) is unblocked. It
  will wire `syncAnalyticsToRegistry` into the Plan 06-03 hook handlers'
  `tracker.Update*` paths so analytics state flows back to Discord
  presence through the registry.
- Plan 06-05 also needs to verify the auto-exit goroutine with a live
  integration test: start daemon, fire PreToolUse (session count 1),
  fire SessionEnd (count 0 -> grace period), either wait 30s (auto-exit)
  or fire another PreToolUse (grace-abort).
- The `server/server.go` `SetOnAutoExit` setter is now public API — if
  Plan 06-05 adds any additional auto-exit triggers, it can reuse the
  same channel-pulse pattern without touching the Server type again.

---
*Phase: 06-hook-system-overhaul-sessionend-posttooluse-precompact-hooks*
*Plan: 04 (JSONL removal + auto-exit goroutine)*
*Completed: 2026-04-10*
