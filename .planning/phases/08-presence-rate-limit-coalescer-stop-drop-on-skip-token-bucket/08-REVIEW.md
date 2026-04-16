---
phase: 08-presence-rate-limit-coalescer-stop-drop-on-skip-token-bucket
reviewed: 2026-04-16T00:00:00Z
depth: standard
files_reviewed: 14
files_reviewed_list:
  - .claude-plugin/marketplace.json
  - .claude-plugin/plugin.json
  - CHANGELOG.md
  - coalescer/coalescer.go
  - coalescer/coalescer_test.go
  - coalescer/hash.go
  - go.mod
  - go.sum
  - main.go
  - scripts/phase-08/verify.ps1
  - scripts/phase-08/verify.sh
  - scripts/start.ps1
  - scripts/start.sh
  - server/hook_dedup.go
  - server/hook_dedup_test.go
  - server/server.go
  - server/server_test.go
findings:
  critical: 1
  warning: 4
  info: 7
  total: 12
status: issues_found
---

# Phase 8: Code Review Report

**Reviewed:** 2026-04-16
**Depth:** standard
**Files Reviewed:** 14 source + 3 test files (17 total)
**Status:** issues_found

## Summary

Phase 8 introduces a lock-free presence-update coalescer (`coalescer/`), a content-hash skip gate, and an HTTP hook-dedup middleware (`server/hook_dedup.go`). The core design is sound — atomic slot for pending Activity, FNV-64a content hash, token-bucket limiter, and a `sync.Map`-backed TTL cache for hook dedup are all standard, well-understood primitives. Tests use Go 1.25's `testing/synctest` for deterministic virtual-clock behaviour.

One critical correctness issue was found: **the verify harness (T3 in `scripts/phase-08/verify.sh` and `verify.ps1`) searches the daemon log for a `"presence updated"` message that is never emitted anywhere in the codebase.** T3 therefore always observes 0 matches and will either always pass (trivially, since 0 ≤ 3) or always fail, depending on the assertion polarity — masking real regressions the harness is supposed to catch.

Several warnings cover concurrency contract gaps (timer fields mutated from multiple goroutines without guard, `time.AfterFunc` leaking an unobserved goroutine when context is cancelled mid-delay, a test that exits a `synctest` bubble while a `go c.Run(ctx)` goroutine is still running, and a body-cap fail-open path that silently loses bytes). Info-level items flag magic numbers, dead comment blocks, and minor log-hygiene points.

Nothing in scope is a security vulnerability (inbound `/hooks/*` is loopback-only, `http.MaxBytesReader` bounds body reads, no SQL/shell injection vectors, no hardcoded secrets). The `0x1F` separator convention is correctly applied in both `coalescer/hash.go` and `server/hook_dedup.go`, overriding the original `\x00` proposal from CONTEXT.md D-12 per research discrepancy D3.

## Critical Issues

### CR-01: Verify harness T3 greps for a log line that is never emitted

**File:** `scripts/phase-08/verify.sh:64`, `scripts/phase-08/verify.ps1:122`
**Issue:** Both verify scripts assert the T3 coalescing contract by grepping the daemon log for the literal string `"presence updated"`:

```bash
# verify.sh:64
T3_COUNT=$(log_tail | grep -c '"presence updated"' || true)
if [[ "$T3_COUNT" -le 3 ]]; then
    pass "T3 10 distinct signals produced $T3_COUNT SetActivity calls (<=3 coalesced)"
```

```powershell
# verify.ps1:122
$count = Count-LogMatches '"presence updated"'
...
return ($count -le 3)
```

Grep-verified: the string `"presence updated"` **does not appear anywhere in the Go source** (`coalescer/coalescer.go`, `discord/client.go`, or `main.go`). `flushPending` only emits a `slog.Debug("discord SetActivity failed", ...)` on error and has no log line on success. `emitSummary` emits `"coalescer status"`, `resolveAndEnqueue` emits `"presence update skipped"`, but nothing ever emits `"presence updated"`.

**Impact on validation:**
- `T3_COUNT` is always 0 ⇒ the `-le 3` branch is always taken ⇒ T3 reports PASS no matter how the coalescer actually behaves. A regression that doubled the SetActivity rate would not be caught.
- Worse, since `T3_COUNT` never increases during genuine activity, the scripts give a false sense of coverage for RLC-01, the single most important invariant of this phase (burst of 10 → ≤ 3 flushes).

**Fix:** Add an `slog.Info("presence updated", ...)` (or `slog.Debug` + run verify at debug log level) inside the `err == nil` success path of `coalescer/coalescer.go:flushPending`, emitted after the successful `SetActivity` call and before `c.sent.Add(1)`:

```go
// coalescer/coalescer.go flushPending (around line 208-220)
func (c *Coalescer) flushPending() {
    a := c.pending.Swap(nil)
    if a == nil {
        return
    }
    if err := c.discord.SetActivity(*a); err != nil {
        slog.Debug("discord SetActivity failed", "error", err)
        return
    }
    c.lastSentHash.Store(HashActivity(a))
    c.sent.Add(1)
    // ADD: phase-08 verify harness T3 greps for this exact token. Keep
    // the key name "presence updated" stable — see scripts/phase-08/verify.sh:64.
    slog.Info("presence updated", "details", a.Details, "state", a.State)
}
```

Alternatively (less invasive), switch both verify scripts to grep for `"coalescer status"`'s `"sent":N` field after T3's 10-second wait — but that still requires reading the summary, so the 60-second wait in T6 would become a prerequisite for T3, inflating total harness time.

The log-line fix is cheaper and restores a direct observable for the single most important invariant of the phase.

## Warnings

### WR-01: `debounce` and `flushTimer` fields are mutated across goroutines without synchronization

**File:** `coalescer/coalescer.go:73-74, 141-145, 181-196, 245-253`
**Issue:** The fields `c.debounce *time.Timer` and `c.flushTimer *time.Timer` are plain pointers with no mutex or atomic wrapping. They are written from:

1. **Run goroutine** via `onSignal` (line 141–145) — writes `c.debounce`.
2. **Timer callback goroutine** for `c.debounce` → calls `c.resolveAndEnqueue` → calls `c.schedule()` (line 181) — reads/writes `c.flushTimer`.
3. **Main goroutine** (`main.go:369`) via `presenceCoalescer.Shutdown()` — reads both `c.debounce` and `c.flushTimer` (lines 247 + 250).
4. **Run-defer** path — calls `c.Shutdown()` (line 120), also reads both fields.

Go's `time.AfterFunc` callbacks run on a separate goroutine from the Run loop. `schedule()` is called from that callback goroutine, while `onSignal()` is called from the Run goroutine. `Shutdown()` is called from *main.go's main goroutine* after `cancel()` is pending but before Run actually exits (main.go:368–379).

Two concrete race windows:
- **Window A (Run vs callback):** Run reads `c.flushTimer` inside the next invocation of `schedule()` (via `resolveAndEnqueue`'s nested `schedule()`), while a still-firing timer's callback is simultaneously writing to `c.flushTimer` via `time.AfterFunc` (line 195). This is technically safe *today* because `resolveAndEnqueue` only runs in one goroutine (the debounce-callback goroutine), but a future refactor that calls `schedule()` from `Run` directly would trip the race.
- **Window B (Shutdown vs callback):** `main.go:369` calls `presenceCoalescer.Shutdown()` *before* cancel() lands, so the Run loop might still be emitting signals that schedule new timers. `Shutdown()` reads `c.flushTimer` non-atomically while a concurrent `schedule()` could be writing it.

The CHANGELOG claims "`go test -race ./...` is green" (line 14), but `coalescer/coalescer_test.go` never drives `c.Shutdown()` from one goroutine while another is still actively pumping `updateChan` — so the race detector has no way to observe Window B.

**Fix (option 1 — minimal):** Add a `sync.Mutex timerMu` field guarding both timer pointers:

```go
type Coalescer struct {
    // ... existing fields ...
    timerMu    sync.Mutex  // guards debounce and flushTimer
    debounce   *time.Timer
    flushTimer *time.Timer
}

func (c *Coalescer) onSignal() {
    c.timerMu.Lock()
    defer c.timerMu.Unlock()
    if c.debounce != nil {
        c.debounce.Stop()
    }
    c.debounce = time.AfterFunc(DebounceDelay, c.resolveAndEnqueue)
}
```

This does NOT violate Invariant I7 ("no `sync.Mutex` on the hot path") because the mutex only guards the coarse timer-management path, not the per-signal hot path (pending.Store, limiter.Reserve, pending.Swap, lastSentHash.Store/Load — all still lock-free).

**Fix (option 2 — design):** Route *all* timer manipulation through the Run goroutine by sending signals to an internal control channel, so the timer fields are only ever touched by one goroutine. More invasive but structurally eliminates the race class.

Related documentation: the struct field comment "100ms front-aggregator (Stop-replace idiom)" does not call out that `onSignal` and `Shutdown` run on different goroutines — the code-as-written is subtly unsafe even though the tests pass today.

### WR-02: `TestCoalescer_TokenBucketRate` exits the synctest bubble with `go c.Run(ctx)` still running

**File:** `coalescer/coalescer_test.go:136-175`
**Issue:** The test launches `go c.Run(ctx)` inside a `synctest.Test` bubble at line 138 and relies on `defer cancel()` at line 137 to stop it. But `synctest.Test` requires ALL goroutines launched inside the bubble to exit (Run's defer-recover path must complete) before `synctest.Test` returns, otherwise the bubble will hang forever waiting for the goroutine or fail with a durably-blocked error.

Looking at `Run`: after `cancel()`, the goroutine hits the `<-ctx.Done()` select branch and returns, triggering `defer Shutdown()`. But the Shutdown path accesses `c.debounce` and `c.flushTimer` without synchronization (see WR-01), which in a `synctest` bubble is observable because the scheduler aggressively interleaves goroutines.

Additionally, `TestCoalescer_FlushSchedule` (line 222–260), `TestCoalescer_ReconnectDiscard` (line 315–333), and `TestCoalescer_HashSkip` (line 445–473) all follow the same pattern. Tests pass locally today, but the ordering contract is fragile — any future change that extends Run's defer (e.g., to log summary stats or flush pending analytics) risks introducing a synctest hang.

**Fix:** Explicitly synctest-wait for Run's exit before the bubble returns. Current test pattern:

```go
go c.Run(ctx)
// ... test body ...
cancel()  // via defer
// synctest.Test returns immediately; no guarantee Run's defer completed
```

Replace with:

```go
done := make(chan struct{})
go func() {
    defer close(done)
    c.Run(ctx)
}()
// ... test body ...
cancel()
synctest.Wait() // flushes all goroutines in the bubble
<-done           // ensure Run returned before asserting final state
```

This eliminates a whole class of ordering flakiness.

### WR-03: Body-cap fail-open path silently drops bytes to the downstream handler

**File:** `server/hook_dedup.go:131-141`
**Issue:** When `http.MaxBytesReader` rejects a body larger than `hookBodyCap`:

```go
r.Body = http.MaxBytesReader(w, r.Body, hookBodyCap)
bodyBytes, err := io.ReadAll(r.Body)
if err != nil {
    // Fail-open: if we couldn't read the body (cap exceeded or
    // client disconnect), pass through to the handler which
    // will return its own 400/413. Do NOT dedup on partial
    // reads — could miss a legitimate subsequent request.
    r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
    next.ServeHTTP(w, r)
    return
}
```

Two issues with this fail-open path:

1. **`bodyBytes` contains a truncated body** (`MaxBytesReader` returns `http.MaxBytesError` after copying up to `hookBodyCap` bytes). The middleware then rewraps `bodyBytes` (the truncated prefix, not the original stream) and passes it downstream. The downstream handler (`handleHook`, `handlePostToolUse`, etc.) will try to `json.Unmarshal(bodyBytes, ...)` on the *truncated* bytes, which will fail with `unexpected end of JSON input` — the client sees a 400, not a 413. This is user-visible: a legitimate 70 KiB request becomes an incomprehensible 400 instead of the expected "body too large" response.

2. **`MaxBytesReader` itself writes headers on error** in some Go versions (`MaxBytesReader` invokes `maxBytesError.Write` which sets `Connection: close` and 413 status under certain paths). Combined with the downstream handler's own `http.Error(...)` call, you may get a duplicate `WriteHeader` warning from `net/http`.

**Fix:** Detect `*http.MaxBytesError` specifically and short-circuit with 413 before touching downstream:

```go
r.Body = http.MaxBytesReader(w, r.Body, hookBodyCap)
bodyBytes, err := io.ReadAll(r.Body)
if err != nil {
    var mbe *http.MaxBytesError
    if errors.As(err, &mbe) {
        http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
        return
    }
    // Other read errors (client disconnect): just pass through with
    // whatever partial bytes we already have; downstream will 400.
    r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
    next.ServeHTTP(w, r)
    return
}
```

This keeps the defence-in-depth cap meaningful (real 413 returned) and keeps the fail-open contract for non-size errors.

### WR-04: `time.AfterFunc` in `schedule()` leaks callback on ctx cancel

**File:** `coalescer/coalescer.go:191-196`
**Issue:** In `schedule()`:

```go
if d := r.Delay(); d == 0 {
    c.flushPending()
} else {
    c.skipRate.Add(1) // deferred flush == "rate-limit in effect"
    c.flushTimer = time.AfterFunc(d, c.flushPending)
}
```

If `ctx` is cancelled between setting `c.flushTimer` and the timer firing, Run's defer will call `Shutdown()` which calls `c.flushTimer.Stop()`. Good so far. **But** if the timer has already fired and the `flushPending` callback has started executing (is about to call `c.discord.SetActivity(*a)` on a possibly-closed IPC), `Stop()` returns false and the flush proceeds against a connection that main.go has already closed (main.go:376 calls `discordClient.Close()` after the Coalescer shutdown).

In practice `Close()` runs *after* `presenceCoalescer.Shutdown()`, so a pre-firing timer will race with the `Close()` call. `discord.Client.SetActivity` will return an error which the Coalescer currently logs at `slog.Debug` and swallows — no crash, but the shutdown sequence's claim "no new SetActivity from the Coalescer can race with the clear" (main.go:363–367) is only best-effort, not guaranteed.

**Fix:** Drain before close. Add an explicit `c.flushTimer.Stop() + synctest.Wait()`-equivalent — for production, the simplest fix is to make `flushPending` check `ctx.Done()` at entry:

```go
// Option 1: pass ctx down
func (c *Coalescer) flushPending() {
    // (requires Run to stash ctx; alternative: accept ctx arg)
    select {
    case <-c.runCtx.Done():
        return
    default:
    }
    a := c.pending.Swap(nil)
    // ...
}
```

**Option 2 (simpler):** accept that the current behaviour is "flushPending may log one debug line about a failed SetActivity during shutdown" and document it explicitly in the Shutdown comment. The current comment at lines 241–244 does hint at this ("BEFORE the direct-to-Discord clear-activity call, so no new SetActivity from the Coalescer can race with the clear"), but the race window against `discordClient.Close()` (main.go:376) is not documented. Add a line noting that a single in-flight SetActivity may still fire against a closed client, which is harmless (logged and discarded).

Recommend Option 2 (document-only) unless there is observed noise in production shutdown logs — the race is theoretical and the blast radius is one debug log line.

## Info

### IN-01: `MapHookToActivity` key case mismatch with JSON hook_event_name convention

**File:** `server/server.go:310-325`
**Issue:** The hook dedup middleware is keyed off `r.URL.Path` (e.g., `/hooks/pre-tool-use`), but `MapHookToActivity` switches on kebab-case (`"pre-tool-use"`) while Claude Code sends `hook_event_name` in CamelCase (`"PreToolUse"`). The dedup middleware is fine (it uses URL path), but the interaction is unobvious: a caller who routes via `hook_event_name` header instead of URL would silently fall through to the `default: "thinking", "Processing..."` branch.

**Fix:** Add a case-normalization comment at the top of `MapHookToActivity` explaining the convention, or accept both forms:

```go
func MapHookToActivity(hookType string, toolName string) (icon string, text string) {
    // hookType here is the URL path slug (kebab-case): "pre-tool-use",
    // "stop", etc. NOT the Claude Code hook_event_name field which uses
    // CamelCase ("PreToolUse"). Callers that have the CamelCase form
    // must convert via strings.ToLower + dashing before calling.
    switch hookType {
    // ...
    }
}
```

### IN-02: Magic numbers in verify scripts

**File:** `scripts/phase-08/verify.sh:65, 73-80, 105`, `scripts/phase-08/verify.ps1:124, 132-133, 153`
**Issue:** Thresholds `-le 3`, `-ge 9`, `sleep 65`, `sleep 0.6` are inlined with no constant/variable. Tuning tolerances (e.g., when CI runs slower) requires editing multiple call sites.

**Fix:** Extract to top-of-file constants:

```bash
T3_MAX_FLUSHES=3
T4_MIN_HASH_SKIPS=9
T6_SUMMARY_WAIT_SECONDS=65
T4_SPACING_SECONDS=0.6
```

Not a bug — makes tuning safer.

### IN-03: `emitSummary` counter reset is non-atomic w.r.t. the INFO log

**File:** `coalescer/coalescer.go:225-239`
**Issue:** The function calls `.Swap(0)` on each of the four counters, then logs. Between the four Swaps, an in-flight `resolveAndEnqueue` or `flushPending` may increment a counter that the current summary has already read-reset; that increment lands in the *next* summary window. This is fine (correct aggregation over time), but the log's "this 60s window" claim is slightly fuzzy.

**Fix:** Document this is an acceptable fuzziness ("summary windows may overlap by up to one `Run` tick") — no code change needed.

### IN-04: Commented "D-12, which originally specified \x00" in `hash.go` and `hook_dedup.go` will become stale

**File:** `coalescer/hash.go:11-15`, `server/hook_dedup.go:44-48`
**Issue:** Both separator comment blocks reference "08-CONTEXT.md D-12, which originally specified \x00" as a historical note. Once 08-CONTEXT.md is updated (or archived), these comments lose their anchor and become orphaned.

**Fix:** Trim the "originally specified" clause and keep only the active rationale:

```go
// sep is the ASCII Unit Separator (0x1F). 0x1F is reserved in the ASCII
// control-code block for field delimiters — it effectively never appears
// in user content. See 08-RESEARCH.md §Pattern 4.
const sep byte = 0x1F
```

This is a minor maintainability nit.

### IN-05: `DrainBucketForTest` uses `for range DiscordRateBurst` (Go 1.22+ integer range loop) without version guard

**File:** `coalescer/coalescer.go:290-294`
**Issue:** `for range DiscordRateBurst` is valid Go 1.22+ syntax. `go.mod` declares `go 1.25.0` so this is fine, but the pattern is still new enough that some readers may not recognize it. No fix required; noting for future maintainers.

### IN-06: `hook_dedup.go` eviction walks the whole sync.Map every 60s — O(n) per cycle

**File:** `server/hook_dedup.go:99-106`
**Issue:** `evictOlderThan` walks every entry in the sync.Map on each tick regardless of how many are actually expired. For a 60-second tick with 500 ms TTL, the map typically holds ~(hook_rate × 60s / 60s) = one cycle's worth of keys — bounded. But if a burst pushes 10k hooks into a 60-second window, the next cleanup walk touches all 10k. No correctness issue; just a note that the design assumes hook rates stay bounded.

**Fix:** None required. If hook rates ever exceed ~100/s sustained, consider switching to a priority-queue based eviction (e.g., `golang.org/x/sync/singleflight`-style sharded TTL map or ristretto). Performance work is out of v1 scope per the review rubric.

### IN-07: `CHANGELOG.md` reference link block references `v4.1.0` but release is `4.2.0`

**File:** `CHANGELOG.md:186-195`
**Issue:** The reference-link block at the bottom of CHANGELOG.md has a `[Unreleased]: compare/v4.1.0...HEAD` entry (line 186) but the Unreleased section at line 8 is now empty because 4.2.0 has been promoted to a dated entry. Missing: the `[4.2.0]` compare link.

**Fix:** Add:

```markdown
[Unreleased]: https://github.com/DSR-Labs/cc-discord-presence/compare/v4.2.0...HEAD
[4.2.0]: https://github.com/DSR-Labs/cc-discord-presence/compare/v4.1.2...v4.2.0
[4.1.2]: https://github.com/DSR-Labs/cc-discord-presence/compare/v4.1.1...v4.1.2
[4.1.1]: https://github.com/DSR-Labs/cc-discord-presence/compare/v4.1.0...v4.1.1
```

And fix the existing `[Unreleased]` line to compare against `v4.2.0`, not `v4.1.0`. Purely cosmetic but matters for release-note rendering on GitHub.

---

_Reviewed: 2026-04-16_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
