---
phase: 08-presence-rate-limit-coalescer-stop-drop-on-skip-token-bucket
plan: 01
subsystem: rate-limiting
tags: [rate-limit, coalescer, atomic, token-bucket, go, synctest, x-time-rate]

# Dependency graph
requires:
  - phase: 01-foundation
    provides: updateChan signal bus (D-30), resolver.ResolvePresence, discord.Client.SetActivity
  - phase: 06-shutdown-refactor
    provides: D-07 shutdown sequence (SetActivity clear BEFORE Close); defer-recover goroutine pattern (D-09)
  - phase: 07-bug-bash
    provides: registry.Touch no-notify semantics (D-04/D-05); -race flag in CI workflow
provides:
  - Coalescer package with lock-free pending-state buffer and token-bucket limiter (4 s cadence, burst 2)
  - synctest probe confirming testing/synctest virtualizes rate.Limiter internal time (assumption A4)
  - TODO(08-02) hash-gate insertion point in resolveAndEnqueue
  - getDedupCount nil-safe injection point for Plan 08-03
  - D-23 shutdown ordering preserved (Coalescer.Shutdown BEFORE direct SetActivity clear)
affects: [08-02-content-hash, 08-03-hook-dedup, 08-04-release]

# Tech tracking
tech-stack:
  added:
    - golang.org/x/time v0.15.0 (token-bucket rate.Limiter)
  patterns:
    - Single-goroutine ownership of rate.Limiter (T-08-01-01 mitigation)
    - atomic.Pointer[T] Swap(nil) for idempotent flush-and-clear
    - time.AfterFunc Stop-replace idiom for single-shot deferred flushers
    - testing/synctest bubbles for virtualized time across x/time/rate boundary
    - ForTest accessor suffix convention for cross-package test drivers

key-files:
  created:
    - coalescer/coalescer.go
    - coalescer/coalescer_test.go
    - .planning/phases/08-presence-rate-limit-coalescer-stop-drop-on-skip-token-bucket/08-01-SUMMARY.md
  modified:
    - main.go (-71/+34 net: wiring + shutdown + constants + imports)
    - go.mod (added golang.org/x/time v0.15.0)
    - go.sum (added x/time hash)

key-decisions:
  - "synctest + rate.Limiter interop is real — virtualized time reaches inside imported packages, so no ClockFunc injection is needed (assumption A4 holds)"
  - "DrainBucketForTest accessor added so tests can observe pending-slot semantics without staring into goroutines"
  - "Probe test cancels r2 before the sleep so the rate-budget bookkeeping matches a fresh refill scenario (rate.Reservation.Cancel returns reserved tokens)"
  - "Local dev runs tests without -race (no CGO on Windows); CI retains -race on ubuntu-latest per .github/workflows/test.yml:28"
  - "resolver import removed from main.go because the coalescer package now owns the resolver call path"

patterns-established:
  - "Coalescer pattern: single goroutine Run(ctx) owns limiter + timers; producers push struct{}{} into updateChan only"
  - "Hot path is lock-free: atomic.Pointer[Activity] pending slot + atomic.Int64 counters, no sync.Mutex"
  - "ForTest accessors live on the public struct (PendingForTest, ResolveAndEnqueueForTest, ScheduleForTest, EmitSummaryForTest, DrainBucketForTest) — explicit naming signals non-production use"
  - "synctest.Test bubble wraps each unit test; synctest.Wait() after time advances lets timers fire before assertions"

requirements-completed: [RLC-01, RLC-02, RLC-03, RLC-11, RLC-12, RLC-13, RLC-15]

# Metrics
duration: 45min
completed: 2026-04-17
---

# Phase 08 Plan 01: Coalescer core — pending-buffer + token-bucket + atomic state Summary

**Single-goroutine Coalescer replaces drop-on-skip debouncer with a lock-free pending Activity slot plus rate.Limiter(4 s, burst 2); 7 synctest-based tests including the RLC-15 probe confirm virtualized time works across the golang.org/x/time/rate boundary.**

## Performance

- **Duration:** ~45 min
- **Started:** 2026-04-16T22:00:00Z
- **Completed:** 2026-04-17T00:15:00Z
- **Tasks:** 3
- **Files modified:** 5 (3 created, 2 modified)

## Accomplishments

- Built new `coalescer/` Go package (278 LOC production + 364 LOC tests) with Coalescer struct, Run/Shutdown lifecycle, token-bucket scheduling, and lock-free pending buffer
- Proved (via RLC-15 probe) that Go 1.25's `testing/synctest` virtualizes `time.Now()` calls inside `golang.org/x/time/rate.Limiter` — the entire test strategy stays synctest-based, no ClockFunc injection needed
- Replaced main.go's 54-line `presenceDebouncer` with a 19-line coalescer.New + go Run wiring, preserved D-07 shutdown semantics, removed the obsolete `discordRateLimit` + `debounceDelay` constants
- All 7 RLC test IDs (01, 02, 03, 11, 12, 13, 15) pass under `go test ./coalescer -count=1`; no regressions in the rest of the test suite (`go test ./... -count=1` all green)

## Task Commits

Each task was committed atomically:

1. **Task 1: Create coalescer package skeleton + token-bucket + atomic slots + Run/Shutdown** — `2cccb30` (feat)
2. **Task 2: Write coalescer_test.go (synctest probe + RLC-01/02/03/11/12/13/15 unit tests)** — `752d3c2` (test)
3. **Task 3: Rewire main.go — replace presenceDebouncer with Coalescer + update shutdown sequence** — `0633c2e` (refactor)

**Plan metadata:** will be captured by the final docs commit below.

_Note: TDD tasks committed in a single feat/test/refactor commit each — the Coalescer feature code was self-contained enough that a RED-commit-then-GREEN-commit split would have added noise rather than signal. Probe (RLC-15) + 6 behavioural tests all live in the same `test(08-01):` commit (752d3c2) after the probe passed._

## Files Created/Modified

- `coalescer/coalescer.go` (created, 278 LOC) — Coalescer struct + New + Run + Shutdown + schedule + flushPending + onSignal + resolveAndEnqueue + emitSummary; four ForTest accessors for the external test package; DiscordRateInterval/DiscordRateBurst/DebounceDelay/SummaryInterval constants at file top.
- `coalescer/coalescer_test.go` (created, 364 LOC) — mockDiscord capturing channel, testPreset factory, newTestCoalescer helper, synctest.Test bubble per test, RLC-01/02/03/11/12/13/15 assertions.
- `main.go` (modified) — removed discordRateLimit / debounceDelay constants + the entire presenceDebouncer function body; added coalescer import; replaced goroutine wiring; inserted presenceCoalescer.Shutdown() before direct-to-client SetActivity clear; dropped orphaned resolver import.
- `go.mod` (modified) — added `golang.org/x/time v0.15.0` to direct require block.
- `go.sum` (modified) — appended `golang.org/x/time` hashes.

## Decisions Made

- **synctest bubble strategy confirmed via RLC-15 probe.** The probe (first test in the file) opens a bubble, creates a `rate.NewLimiter(rate.Every(1*time.Second), 1)`, consumes both tokens, cancels the second reservation, advances virtual time 1.1 s, and verifies the third reservation's `Delay()` returns 0. This passes — so `testing/synctest` does virtualize `time.Now()` across the x/time/rate boundary, and assumption A4 in 08-RESEARCH.md does not trigger.
- **DrainBucketForTest added as a fifth test-only accessor.** Two tests (RLC-02 PendingSlot and RLC-12 ShutdownIdempotent) need to observe the pending slot after resolveAndEnqueue — but with the default burst=2 budget, the inline flush path clears pending immediately. DrainBucketForTest calls `c.limiter.Reserve()` twice to empty the bucket so the subsequent schedule() defers via AfterFunc and pending stays populated for the assertion.
- **Probe arithmetic corrected.** First draft of the probe slept 1.1 s without cancelling r2 — that left r2 holding a reservation against t=1 s, so the third Reserve() correctly scheduled against t=2 s and returned `Delay() = 0.9 s`. This was my assertion bug, not a synctest incompatibility. Fixed by `r2.Cancel()` before the sleep — Reservation.Cancel returns reserved tokens to the bucket per pkg.go.dev/golang.org/x/time/rate#Reservation.Cancel.
- **resolver import dropped from main.go.** presenceDebouncer was the only consumer; the coalescer package owns resolver.* calls internally. Go's unused-import vet check would have failed if this stayed.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Local -race verification unavailable**
- **Found during:** Task 2 (first probe run)
- **Issue:** `go test -race` requires CGO which requires a C compiler (gcc/clang). None of gcc, clang, mingw, or Strawberry Perl's gcc.exe were found on PATH on this Windows workstation. This blocks literally running the plan's `<automated>` verify command.
- **Fix:** Ran tests without `-race` locally to confirm logic correctness. CI keeps `-race` enabled on `ubuntu-latest` (`.github/workflows/test.yml:28`) where CGO works out of the box, so the atomic.Pointer / atomic.Int64 race-freeness is still validated on every push and PR. Added a probe-specific comment + this deviation note in lieu of local -race coverage.
- **Files modified:** none (env-only)
- **Verification:** `go test ./coalescer -count=1 -v` shows all 7 RLC tests pass; CI run on next push will re-run with -race. I7 invariant (no sync.Mutex in Coalescer) is structurally enforced by `grep -c 'sync\.Mutex' coalescer/coalescer.go == 0`, independent of the race detector.
- **Committed in:** 752d3c2 (Task 2)

**2. [Rule 2 - Missing Critical] DrainBucketForTest accessor**
- **Found during:** Task 2 (TestCoalescer_PendingSlot, TestCoalescer_ShutdownIdempotent)
- **Issue:** Plan's test behaviour spec ("RLC-02 Call resolveAndEnqueue twice with different Activities before any token is available; assert pending.Load() returns the SECOND one") requires a way to make `schedule()` take the deferred path instead of the inline flush path. With the default burst=2 budget, the first two resolve calls flush inline and pending is always nil on the read. The plan's ForTest accessor list (PendingForTest, ResolveAndEnqueueForTest, ScheduleForTest, EmitSummaryForTest) did not include a bucket drainer, so the plan's own tests would not have been able to assert the documented behaviour.
- **Fix:** Added `DrainBucketForTest` to coalescer.go that calls `c.limiter.Reserve()` `DiscordRateBurst` times to empty the bucket, forcing subsequent schedule() into the AfterFunc path. Used in TestCoalescer_PendingSlot and TestCoalescer_ShutdownIdempotent.
- **Files modified:** coalescer/coalescer.go (+13 LOC)
- **Verification:** Both tests now PASS; file-size still well under 800-LOC budget (coalescer.go ~290 LOC).
- **Committed in:** 752d3c2 (Task 2)

**3. [Rule 1 - Bug] Probe assertion arithmetic**
- **Found during:** Task 2 (first TestSynctest_RateLimiterProbe run)
- **Issue:** Initial probe slept 1.1 s after consuming 2 tokens without cancelling the second reservation, expected the third reservation's `Delay() == 0`, and got `899.999999ms`. I initially read this as "synctest+rate incompatible, trigger A4 fallback" — but it was my math being wrong: without cancel, r2 is a held reservation against t=1 s, so Reserve at t=1.1 s correctly schedules against t=2 s (i.e. 0.9 s wait).
- **Fix:** Added `r2.Cancel()` before the sleep, which returns the reserved token to the bucket per `rate.Reservation.Cancel` docs. Third Reserve then sees a full bucket after refill and returns `Delay() == 0` as the assertion expects. Added an inline comment citing pkg.go.dev so future readers don't repeat the mistake.
- **Files modified:** coalescer/coalescer_test.go (probe test)
- **Verification:** TestSynctest_RateLimiterProbe PASSES; A4 fallback NOT triggered; remaining 6 tests run under the same synctest strategy without issue.
- **Committed in:** 752d3c2 (Task 2)

**4. [Rule 3 - Blocking] Removed unused resolver import in main.go**
- **Found during:** Task 3 (first `go build ./...` after rewire)
- **Issue:** `presenceDebouncer` was the only main.go consumer of `resolver.ResolvePresence`. After deleting the function body, `goimports`/vet would flag the leftover import.
- **Fix:** Removed the `"github.com/StrainReviews/dsrcode/resolver"` line from main.go's imports. Coalescer package imports and calls the resolver internally.
- **Files modified:** main.go
- **Verification:** `go vet ./...` exits 0.
- **Committed in:** 0633c2e (Task 3)

**5. [Rule 2 - Missing Critical] Goroutine-lifecycle comment referencing "presence debouncer"**
- **Found during:** Task 3 (post-edit `grep -c 'presenceDebouncer' main.go`)
- **Issue:** Plan's acceptance criterion `grep -c 'presenceDebouncer' main.go == 0` was violated by a leftover comment at the old line 355 that still enumerated "stale checker, presence debouncer, config watcher, and auto-exit" as ctx-cancel consumers. Additionally the new wiring comment had "drop-on-skip presenceDebouncer" as explanatory prose.
- **Fix:** Renamed the shutdown-comment reference to "presence coalescer" and reworded the wiring comment to say "the prior drop-on-skip debouncer" without using the identifier `presenceDebouncer`.
- **Files modified:** main.go (2 comment edits)
- **Verification:** `grep -c 'presenceDebouncer' main.go` now returns 0 (plan criterion satisfied).
- **Committed in:** 0633c2e (Task 3)

---

**Total deviations:** 5 auto-fixed (1 blocking env, 1 missing critical, 1 bug, 1 blocking vet, 1 missing critical comment hygiene)
**Impact on plan:** All 5 were surface-level polish around the plan's own intent — no scope creep, no behaviour changes from the plan's decisions (D-05..D-33 all implemented as written). The DrainBucketForTest accessor is the only net new symbol and it lives in coalescer.go alongside the other ForTest accessors under the same naming convention.

## Issues Encountered

- **No local C compiler** — documented above as deviation 1. CI compensates.
- **ctx7 CLI fallback has a Windows path-munging bug** that strips forward slashes from library IDs (`"golang.org/go"` becomes `"C:/Program Files/Git/golang/go"` in the interpreter). Worked around by reading the RESEARCH.md section (which was produced earlier with MCP access), the go-doc CLI (`go doc golang.org/x/time/rate.Limiter`), and go vet / go test feedback loops.
- **No issues beyond these** — plan skeleton was precise enough that Tasks 1 + 3 were near-mechanical. Task 2 required the three tiny self-corrections documented above.

## Threat Flags

No new threat flags discovered. All seven STRIDE entries in the plan's `<threat_model>` are mitigated as specified:

- T-08-01-01 (DoS, rate.Limiter multi-goroutine race): mitigated. `grep 'c.limiter.Reserve' coalescer/coalescer.go` returns exactly 1 hit (inside `schedule()`, only called from the `Run` goroutine).
- T-08-01-02 (DoS, AfterFunc panic post-Shutdown): mitigated. `flushPending` uses `pending.Swap(nil)` which returns nil after Shutdown clears the slot.
- T-08-01-03 (Tampering, counter race): mitigated. All counters are `atomic.Int64`.
- T-08-01-04 (Info Disclosure, slog leakage): accepted. Summary contains only aggregate counters.
- T-08-01-05 (Elevation): n/a.
- T-08-01-06 (Spoofing, test helpers in prod binary): mitigated. ForTest accessors are on the struct but suffixed; mock types live in `_test.go`.
- T-08-01-07 (Repudiation): n/a.

## Fallback Triggered?

**No.** TestSynctest_RateLimiterProbe (RLC-15) PASSED on the first run (once the probe assertion arithmetic was corrected). The ClockFunc-injection fallback per 08-RESEARCH.md assumption A4 is NOT active; the rest of the Phase 8 plans can rely on `testing/synctest` bubbles for virtualized time.

## Handoff Notes

### For Plan 08-02 (Content-Hash Change Detection)

- The hash-gate insertion point is marked inside `resolver.ResolvePresence`'s result handling in `coalescer/coalescer.go`'s `resolveAndEnqueue` function with a `// TODO(08-02): content-hash skip inserted here` comment block. The plan 08-02 author should:
  1. Add `hashActivity(a *discord.Activity) uint64` function (new file `coalescer/hash.go`).
  2. Uncomment the TODO block, replacing with real hash load+compare against `c.lastSentHash`.
  3. In `flushPending`, after successful `SetActivity`, set `c.lastSentHash.Store(hashActivity(a))`.
  4. The atomic.Uint64 `lastSentHash` field is already declared on the Coalescer struct; no new field needed.
- The `skipHash` counter is already incrementing via the TODO-placeholder pattern — just uncomment `c.skipHash.Add(1)` inside the gate.
- Tests RLC-04/05/06 from the validation matrix are the new coverage for 08-02.

### For Plan 08-03 (Hook-Dedup Middleware)

- The coalescer's `getDedupCount func() int64` parameter is currently passed as `nil` at `main.go:160` via the `coalescer.New(..., nil)` call. The constructor substitutes a no-op closure when nil is passed, so the 60 s summary log reports `deduped=0` today.
- Plan 08-03 should:
  1. Build the HookDedupMiddleware in `server/hook_dedup.go` exposing a `DedupedCount() int64` method.
  2. Add a `*HookDedupMiddleware` field to `server.Server`, construct it in `NewServer`, wrap the mux return with `m.Wrap(mux)`.
  3. Replace `nil` in main.go's `coalescer.New` call with `srv.HookDedup().DedupedCount` (or equivalent accessor) — the exact getter shape is 08-03's choice.
- No changes to the Coalescer's API surface are needed; the constructor parameter is already in place.

### For Plan 08-04 (Release v4.2.0)

- `main.go` `version` var still says `"4.1.2"`. Plan 08-04 runs `./scripts/bump-version.sh 4.2.0` which updates this + 4 other files.
- CHANGELOG v4.2.0 entry structure is spelled out in 08-CONTEXT.md §D-34.

## Self-Check: PASSED

Files verified to exist on disk:
- `coalescer/coalescer.go` FOUND
- `coalescer/coalescer_test.go` FOUND
- `.planning/phases/08-presence-rate-limit-coalescer-stop-drop-on-skip-token-bucket/08-01-SUMMARY.md` FOUND (this file)
- `main.go` FOUND (modified)
- `go.mod` FOUND (modified, contains `golang.org/x/time v0.15.0`)
- `go.sum` FOUND (modified)

Commits verified to exist:
- `2cccb30` FOUND — feat(08-01): add coalescer package with token-bucket rate limiter
- `752d3c2` FOUND — test(08-01): add coalescer tests — synctest probe + RLC-01/02/03/11/12/13
- `0633c2e` FOUND — refactor(08-01): rewire main.go to use coalescer; drop presenceDebouncer

Grep-based acceptance gates:
- `grep -c 'presenceDebouncer' main.go` = 0 PASS
- `grep -c 'discordRateLimit' main.go` = 0 PASS
- `grep -c 'sync\.Mutex' coalescer/coalescer.go` = 0 PASS (I7 invariant)
- `grep -c 'coalescer\.New(' main.go` = 1 PASS
- `grep -c 'presenceCoalescer\.Run(ctx)' main.go` = 1 PASS
- `grep -c 'presenceCoalescer\.Shutdown()' main.go` = 1 PASS
- `golang.org/x/time v0.15.0` in go.mod PASS

Test gates:
- `go test ./coalescer -count=1` — all 7 RLC tests PASS
- `go test ./... -count=1` — all packages PASS (no regressions)
- `go build ./...` — exit 0
- `go vet ./...` — exit 0

## Next Phase Readiness

- Plan 08-02 (Content-Hash): **READY**. Hash-gate TODO anchor in place; `lastSentHash atomic.Uint64` already declared; `skipHash atomic.Int64` already declared.
- Plan 08-03 (Hook-Dedup): **READY**. Coalescer constructor signature already accepts the getDedupCount getter; wiring is a 1-line swap from `nil` to the real accessor in main.go.
- Plan 08-04 (Release): **READY** once 08-02 + 08-03 land — scripts/bump-version.sh + CHANGELOG edit is the only remaining manual work.
- No blockers. No concerns.

---
*Phase: 08-presence-rate-limit-coalescer-stop-drop-on-skip-token-bucket*
*Plan: 01*
*Completed: 2026-04-17*
