---
phase: 08-presence-rate-limit-coalescer-stop-drop-on-skip-token-bucket
verified: 2026-04-16T23:59:00Z
status: passed
score: 17/17
overrides_applied: 0
re_verification:
  previous_status: human_needed
  previous_score: 15/17
  gaps_closed:
    - "CR-01: slog.Info(\"presence updated\", ...) now emitted in coalescer/coalescer.go flushPending() at line 224 (commit dc61b43); T3 grep assertion is no longer false-green"
  gaps_remaining: []
  regressions: []
gaps: []
human_verification:
  - test: "Run live daemon smoke: bash scripts/phase-08/verify.sh or powershell -File scripts/phase-08/verify.ps1 against a running v4.2.0 daemon"
    expected: "T1-T6 all PASS; T3 must now observe a non-zero SetActivity count (> 0 and <= 3) from the 10-signal burst, confirming that coalescing is actually happening — not trivially passing with 0"
    why_human: "External-infrastructure UAT step per CLAUDE.md §Releasing. Requires a live Discord-connected daemon. Automated tests confirm the log line fires during TestCoalescer_TokenBucketRate and TestCoalescer_FlushSchedule (see test output); the T3 false-green is resolved. This item is retained as a deliberate user-acceptance test to be run before/after git tag v4.2.0, not a gap in automated verification."
  - test: "Trigger 10+ rapid tool calls in a real Claude Code session with dsrcode v4.2.0 running and observe Discord Rich Presence updates"
    expected: "Presence updates smoothly ~every 4 seconds; no 'stuck' periods during MCP-heavy sessions; 'presence update skipped reason=content_hash' appears in log for redundant resolves; 'coalescer status' line appears every ~60s with meaningful counter values"
    why_human: "External-infrastructure UAT step per CLAUDE.md §Releasing. End-to-end behavior (70% skip rate improvement claim) requires live Discord client + active Claude Code MCP session; automated tests use mock Discord client."
---

# Phase 8: Presence Rate-Limit Coalescer Verification Report

**Phase Goal:** Replace drop-on-skip `presenceDebouncer` with a coalescing rate-limiter + pending buffer + FNV-64 content-hash gate + hook-dedup middleware so presence updates queued during cooldowns are flushed exactly once, identical content is skipped, and duplicate POST /hooks/* requests within 500ms TTL are deduped. Target v4.2.0.
**Verified:** 2026-04-16T23:59:00Z (re-verification after commit dc61b43)
**Status:** passed
**Re-verification:** Yes — post-fix for CR-01 gap (commit dc61b43)

## Post-Fix Update (CR-01 Resolution)

**Gap closed:** The CR-01 gap identified in the initial verification (and reported in 08-REVIEW.md) has been resolved.

**Fix committed:** `dc61b43 "fix(08-01): add slog.Info presence updated for verify.sh T3 smoke"`

**What changed:** `slog.Info("presence updated", "details", a.Details, "state", a.State)` was added to `coalescer/coalescer.go flushPending()` at line 224, between `c.lastSentHash.Store(HashActivity(a))` and `c.sent.Add(1)`. This is the exact success path that verify.sh T3 and verify.ps1 T3 grep the daemon log for.

**Verification of fix:**
- `grep -n "presence updated" coalescer/coalescer.go` returns line 224 — the string is present in the correct function and position.
- `go test -v ./coalescer/...` passes all 10 tests (10/10 PASS). `TestCoalescer_TokenBucketRate` and `TestCoalescer_FlushSchedule` both emit `INFO presence updated details=...state=...` lines, confirming the log fires on the live hot path during realistic flush sequences.
- Truth 16 is now fully VERIFIED (no longer partial).

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | Single-goroutine Coalescer replaces presenceDebouncer with no drop-on-skip behavior; latest pending Activity is always flushed within r.Delay() | VERIFIED | `grep -c 'presenceDebouncer' main.go == 0`; `coalescer/coalescer.go` has Run/Shutdown/flushPending/schedule/resolveAndEnqueue/onSignal all present; `pending.Swap(nil)` in flushPending |
| 2 | Token bucket (rate.Every(4s), burst 2) serializes Discord SetActivity calls; first two coalesced updates after daemon start are instant | VERIFIED | `rate.NewLimiter(rate.Every(DiscordRateInterval), DiscordRateBurst)` in coalescer.go:108; DiscordRateInterval=4s, DiscordRateBurst=2 as constants; TestCoalescer_TokenBucketRate (RLC-01) confirms burst behavior |
| 3 | Pending buffer via atomic.Pointer[discord.Activity] is lock-free on the hot path; Swap(nil) atomically flush-and-clears | VERIFIED | `atomic.Pointer[discord.Activity]` field in Coalescer struct; `c.pending.Swap(nil)` in flushPending; `grep -c 'sync.Mutex' coalescer/coalescer.go == 0`; Invariant I7 satisfied |
| 4 | Coalescer.Run honors ctx.Done() for shutdown; Shutdown() is idempotent and stops all timers + clears pending | VERIFIED | Run loop has `case <-ctx.Done(): return`; Shutdown() calls pending.Store(nil) + debounce.Stop() + flushTimer.Stop(); TestCoalescer_ShutdownIdempotent (RLC-12) passes |
| 5 | IPC disconnect (SetActivity error) discards pending; reconnect triggers fresh updateChan signal | VERIFIED | flushPending: `slog.Debug("discord SetActivity failed")` + early return on error; hash store only on success; TestCoalescer_ReconnectDiscard (RLC-13) passes |
| 6 | 60s summary INFO log fires only when any counter is non-zero; idle sessions produce no summary noise | VERIFIED | emitSummary() has `if sent+sr+sh+dd == 0 { return }` guard; TestCoalescer_SummaryIdleSkip (RLC-11) asserts no log when idle |
| 7 | synctest + rate.Limiter interop probe confirms virtualized time works across x/time/rate boundary (RLC-15) | VERIFIED | TestSynctest_RateLimiterProbe present and passing; ClockFunc fallback NOT triggered |
| 8 | FNV-1a 64-bit hashes of two Activities with identical user-visible content produce identical uint64 values | VERIFIED | `coalescer/hash.go` has HashActivity using `fnv.New64a()` + field-ordered bytes + `const sep byte = 0x1F`; TestHashActivity_Deterministic (RLC-04) passes |
| 9 | Changes to StartTime alone never change the hash — StartTime is excluded by design (RLC-05) | VERIFIED | `grep 'StartTime' coalescer/hash.go` returns only comment lines; HashActivity never references a.StartTime in code; TestHashActivity_ExcludesStartTime (RLC-05) passes |
| 10 | lastSentHash atomic.Uint64 gates every SetActivity call: identical content skipped with skipHash counter incremented and DEBUG log | VERIFIED | `HashActivity(a) == c.lastSentHash.Load()` at coalescer.go:167; `c.skipHash.Add(1)` + `slog.Debug("presence update skipped", "reason", "content_hash")` present; hash store `c.lastSentHash.Store(HashActivity(a))` at coalescer.go:218 AFTER successful SetActivity only; TestCoalescer_HashSkip (RLC-06) passes |
| 11 | Two POST /hooks/pre-tool-use with identical body within 500ms produce exactly ONE downstream handler invocation; second returns HTTP 200 silently | VERIFIED | HookDedupMiddleware.Wrap has sync.Map Load-check with `hookDedupTTL = 500ms`; dedup hit returns HTTP 200 + slog.Debug("hook deduped"); TestHookDedup_PathFilter (RLC-07) verifies exactly 1 dedup hit for duplicate POST; TestHookDedup_KeyComposition (RLC-08) verifies key distinctness |
| 12 | Non-hook routes pass through the middleware untouched | VERIFIED | `strings.HasPrefix(r.URL.Path, "/hooks/")` path filter in Wrap(); TestHookDedup_PathFilter asserts deduped counter stays 0 for /statusline, /config; TestPreview_BypassesCoalescer (RLC-14) confirms /preview bypasses |
| 13 | Middleware preserves r.Body so downstream can json.Unmarshal; /preview bypasses both dedup and Coalescer | VERIFIED | `r.Body = io.NopCloser(bytes.NewReader(bodyBytes))` after ReadAll; TestHookDedup_BodyPreserved (RLC-10) confirms session created with correct SmallImageKey; TestPreview_BypassesCoalescer confirms /preview bypass (RLC-14) |
| 14 | sync.Map-backed TTL cache evicts stale entries via 60s background goroutine; DedupedCount() int64 exposed | VERIFIED | `hookDedupCleanupPeriod = 60s` ticker in StartCleanup(); evictOlderThan(now) walks sync.Map; DedupedCount() returns deduped.Load(); TestHookDedup_TTLCleanup (RLC-09) asserts post-TTL request not deduped |
| 15 | main.go version, plugin.json, marketplace.json, start.sh, start.ps1 all report 4.2.0; CHANGELOG has [4.2.0] section | VERIFIED | version="4.2.0" in main.go:31; plugin.json has "4.2.0"; marketplace.json has "4.2.0"; start.sh has VERSION="v4.2.0"; start.ps1 has $Version="v4.2.0"; `grep -c '## \[4.2.0\]' CHANGELOG.md == 1` |
| 16 | verify.sh/verify.ps1 exist, are executable, and exercise T1-T6; T3 observes real flush events from the daemon log | VERIFIED | scripts/phase-08/verify.sh exists (100755); verify.ps1 exists; both have 6 PASS paths (T1-T6); slog.Info("presence updated", ...) confirmed at coalescer.go:224 (commit dc61b43); test run shows INFO presence updated lines in TestCoalescer_TokenBucketRate and TestCoalescer_FlushSchedule |
| 17 | CHANGELOG [4.2.0] Keep-a-Changelog 1.1.0 compliant with bold-lead-phrase + RLC-ID citations | VERIFIED | Fixed/Changed/Added subsections under [4.2.0]; all 8 bullets start with `- **`; 9 RLC-NN citations across bullets span RLC-01..RLC-16; [4.2.0] appears above [4.1.2] in file order |

**Score:** 17/17 truths verified

### Deferred Items

None.

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `coalescer/coalescer.go` | Coalescer struct + New + Run + Shutdown + schedule + flushPending + onSignal + resolveAndEnqueue + emitSummary | VERIFIED | 301 LOC post-fix; all functions present; no sync.Mutex; atomic.Pointer[discord.Activity] pending slot; slog.Info("presence updated") at line 224 |
| `coalescer/hash.go` | HashActivity using fnv.New64a + 0x1F separator | VERIFIED | 68 LOC; fnv.New64a() confirmed; const sep byte = 0x1F; StartTime absent from code lines |
| `coalescer/coalescer_test.go` | 10 tests: RLC-01/02/03/04/05/06/11/12/13/15 | VERIFIED | 10 test functions confirmed; all PASS; INFO presence updated emitted in TokenBucketRate and FlushSchedule tests |
| `server/hook_dedup.go` | HookDedupMiddleware + Wrap + StartCleanup + DedupedCount + http.MaxBytesReader | VERIFIED | 197 LOC; all required components present; dedupSep=0x1F; hookDedupTTL=500ms |
| `server/hook_dedup_test.go` | 6 integration tests: RLC-07/08/09/10/14 + synctest compat | VERIFIED | 6 test functions confirmed; package server_test (external); all tests target correct routes |
| `server/server.go` | hookDedup field + NewServer init + StartCleanup + Handler() Wrap + HookDedupedCount() | VERIFIED | hookDedup *HookDedupMiddleware at line 240; NewHookDedupMiddleware() at line 274; s.hookDedup.Wrap(mux) at line 355; HookDedupedCount() at line 362; StartCleanup at line 1552 |
| `main.go` | coalescer.New wiring; srv before presenceCoalescer; Shutdown before SetActivity clear; no presenceDebouncer | VERIFIED | srv at line 160, presenceCoalescer at line 275 (srv first); Shutdown at line 369, SetActivity clear at line 372; grep presenceDebouncer == 0; grep discordRateLimit == 0 |
| `go.mod` | golang.org/x/time v0.15.0 | VERIFIED | golang.org/x/time v0.15.0 confirmed in go.mod |
| `CHANGELOG.md` | [4.2.0] section with Fixed/Changed/Added | VERIFIED | Section present at line 10; 3 Fixed + 2 Changed + 3 Added bullets; 9 RLC citations |
| `scripts/phase-08/verify.sh` | Executable bash script with T1-T6; T3 greps for "presence updated" which is now emitted | VERIFIED | Exists, executable, set -euo pipefail, 6 PASS paths; CR-01 resolved — log line now present in source |
| `scripts/phase-08/verify.ps1` | PowerShell 5.1 script with T1-T6; T3 greps for "presence updated" which is now emitted | VERIFIED | Exists, #Requires -Version 5.1, Invoke-Test helper, 6 tests; CR-01 resolved — log line now present in source |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|-----|--------|---------|
| `main.go` | `coalescer.New` | constructor at line 275 replaces old presenceDebouncer goroutine | WIRED | `coalescer.New(` confirmed at main.go:275 |
| `coalescer/coalescer.go Run` | `discord.Client.SetActivity` | flushPending path after Reserve().Delay() | WIRED | `c.discord.SetActivity(*a)` in flushPending; discordSetter interface used |
| `coalescer/coalescer.go schedule` | `time.AfterFunc(r.Delay(), c.flushPending)` | single-shot flusher replaced on each pending.Store | WIRED | `c.flushTimer = time.AfterFunc(d, c.flushPending)` at coalescer.go:195 |
| `coalescer/coalescer.go resolveAndEnqueue` | `coalescer.HashActivity` | hash check replaces TODO(08-02) comment | WIRED | `HashActivity(a) == c.lastSentHash.Load()` at line 167; TODO(08-02) count == 0 |
| `coalescer/coalescer.go flushPending` | `c.lastSentHash.Store` | hash stored after successful SetActivity | WIRED | `c.lastSentHash.Store(HashActivity(a))` at line 218; occurs after err==nil check |
| `coalescer/coalescer.go flushPending` | `slog.Info("presence updated", ...)` | success-path INFO log for T3 smoke harness | WIRED | line 224; after lastSentHash.Store, before c.sent.Add(1); confirmed by test output |
| `server/server.go Handler()` | `HookDedupMiddleware.Wrap` | return s.hookDedup.Wrap(mux) replaces bare return mux | WIRED | `s.hookDedup.Wrap(mux)` at server.go:355 |
| `main.go presenceCoalescer` | `srv.HookDedupedCount` | getDedupCount injected after server.NewServer | WIRED | `srv.HookDedupedCount` at main.go:286; srv at line 160 before presenceCoalescer at line 275 |
| `Coalescer.Shutdown` | order before `discordClient.SetActivity(Activity{})` | D-23 shutdown order | WIRED | presenceCoalescer.Shutdown() at line 369; SetActivity clear at line 372 (Shutdown first) |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|---------------|--------|--------------------|--------|
| `coalescer/coalescer.go flushPending` | `*discord.Activity` from pending.Swap | resolver.ResolvePresence called in resolveAndEnqueue | Yes — resolver pulls from session registry + preset | FLOWING |
| `coalescer/coalescer.go emitSummary` | sent/skipRate/skipHash counters + getDedupCount() | atomic.Int64.Swap(0) + srv.HookDedupedCount | Yes — real atomic increments from flushPending/schedule/resolveAndEnqueue/HookDedupMiddleware | FLOWING |
| `server/hook_dedup.go Wrap` | FNV key from URL.Path + probe.SessionID + probe.ToolName + bodyBytes | io.ReadAll(r.Body) + json.Unmarshal probe | Yes — real HTTP request body from Claude Code hooks | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| coalescer package builds | `go build ./coalescer` | exit 0 | PASS |
| server package builds | `go build ./server` | exit 0 | PASS |
| "presence updated" log line in flushPending | `grep -n '"presence updated"' coalescer/coalescer.go` | line 224 in flushPending() body | PASS |
| Log line is INFO (not Debug) | coalescer.go:224 `slog.Info(...)` | slog.Info confirmed | PASS |
| Log line position: after lastSentHash.Store, before c.sent.Add | coalescer.go lines 218, 224, 225 | correct ordering confirmed | PASS |
| TestCoalescer_TokenBucketRate emits "presence updated" | `go test -v ./coalescer/... -run TestCoalescer_TokenBucketRate` | `INFO presence updated details=... state=...` in output | PASS |
| TestCoalescer_FlushSchedule emits "presence updated" | `go test -v ./coalescer/... -run TestCoalescer_FlushSchedule` | `INFO presence updated details=... state=...` in output | PASS |
| All 10 coalescer tests pass | `go test -v ./coalescer/...` | 10/10 PASS (0.473s) | PASS |
| HashActivity nil guard | `coalescer/hash.go: if a == nil { return 0 }` — static inspection | first line of body is nil guard | PASS |
| No sync.Mutex in Coalescer hot path | `grep -c 'sync.Mutex' coalescer/coalescer.go` | 0 | PASS |
| presenceDebouncer removed | `grep -c 'presenceDebouncer' main.go` | 0 | PASS |
| discordRateLimit removed | `grep -c 'discordRateLimit' main.go` | 0 | PASS |
| TODO(08-02) cleared | `grep -c 'TODO(08-02)' coalescer/coalescer.go` | 0 | PASS |
| 0x1F separator both files | `grep 'const sep byte = 0x1F' coalescer/hash.go` + `grep 'dedupSep byte = 0x1F' server/hook_dedup.go` | both present | PASS |
| verify.sh has 6 PASS paths | `grep -c 'pass "T[1-6]"' scripts/phase-08/verify.sh` | 6 | PASS |
| verify.ps1 has 6 Invoke-Test blocks | `grep -c "Invoke-Test -Id 'T[1-6]'" scripts/phase-08/verify.ps1` | 6 | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| RLC-01 | 08-01 | Token bucket allows 2 instant + 1 every 4s | SATISFIED | rate.NewLimiter(rate.Every(4s), 2); TestCoalescer_TokenBucketRate |
| RLC-02 | 08-01 | atomic.Pointer.Store replaces latest; Swap(nil) clears | SATISFIED | atomic.Pointer[discord.Activity] field; pending.Swap(nil) in flushPending; TestCoalescer_PendingSlot |
| RLC-03 | 08-01 | AfterFunc-scheduled flush fires exactly once after Delay() | SATISFIED | time.AfterFunc(d, c.flushPending) in schedule(); TestCoalescer_FlushSchedule |
| RLC-04 | 08-02 | FNV-64a hash deterministic across process restarts | SATISFIED | fnv.New64a() + field-ordered bytes; TestHashActivity_Deterministic |
| RLC-05 | 08-02 | Same Activity content -> same hash (StartTime excluded) | SATISFIED | StartTime absent from hash.go code; TestHashActivity_ExcludesStartTime |
| RLC-06 | 08-02 | atomic.Uint64 lastSentHash load/store correct | SATISFIED | lastSentHash.Load() in resolveAndEnqueue; lastSentHash.Store() in flushPending post-success; TestCoalescer_HashSkip |
| RLC-07 | 08-03 | Dedup middleware wraps /hooks/* only; other routes pass through | SATISFIED | strings.HasPrefix(r.URL.Path, "/hooks/") path filter; TestHookDedup_PathFilter |
| RLC-08 | 08-03 | Dedup key = FNV-64a(route + sep + session + sep + tool + sep + body) | SATISFIED | FNV-64a composition in Wrap(); TestHookDedup_KeyComposition |
| RLC-09 | 08-03 | TTL 500ms; 60s ticker evicts stale entries | SATISFIED | hookDedupTTL=500ms; hookDedupCleanupPeriod=60s; evictOlderThan(); TestHookDedup_TTLCleanup |
| RLC-10 | 08-03 | Middleware preserves request body for downstream JSON decode | SATISFIED | io.NopCloser(bytes.NewReader(bodyBytes)) re-wrap; TestHookDedup_BodyPreserved |
| RLC-11 | 08-01 | 60s summary log fires when counters > 0; skipped when idle | SATISFIED | emitSummary() zero-guard; SummaryInterval=60s; TestCoalescer_SummaryIdleSkip |
| RLC-12 | 08-01 | Shutdown() is idempotent: clears pending + stops timers | SATISFIED | pending.Store(nil) + nil checks on timers; TestCoalescer_ShutdownIdempotent |
| RLC-13 | 08-01 | IPC disconnect discards pending; reconnect triggers fresh signal | SATISFIED | SetActivity error -> return (no hash store, no sent counter); TestCoalescer_ReconnectDiscard |
| RLC-14 | 08-03 | /preview bypasses Coalescer (direct SetActivity) | SATISFIED | /preview is not /hooks/* prefix; TestPreview_BypassesCoalescer |
| RLC-15 | 08-01 | testing/synctest + rate.Limiter interop probe | SATISFIED | TestSynctest_RateLimiterProbe passes; A4 fallback not triggered |
| RLC-16 | 08-04 | verify.sh/verify.ps1 T1..T6 cross-platform smoke | SATISFIED | Scripts exist and structurally correct; CR-01 resolved — slog.Info("presence updated") at coalescer.go:224; T3 can now observe real flush events |
| RLC-17 | 08-04 | CHANGELOG v4.2.0 entry + Keep-a-Changelog 1.1.0 compliant | SATISFIED | [4.2.0] section present; Fixed/Changed/Added subsections; 9 RLC citations; bold-lead-phrase style |

### Anti-Patterns Found

The CR-01 blocker is resolved. Remaining items are warnings/info carried forward from the initial review for tracking; none block the phase goal.

| File | Pattern | Severity | Impact |
|------|---------|----------|--------|
| `coalescer/coalescer.go:73-74` | `debounce` and `flushTimer` fields mutated from multiple goroutines (Run goroutine, AfterFunc callback goroutine, main goroutine via Shutdown) without synchronization | Warning | WR-01 from code review: race window exists between Shutdown() and concurrent schedule() callback — currently undetected by tests; not a regression in dc61b43 |
| `server/hook_dedup.go:131-141` | Body-cap fail-open path rewraps truncated bytes; downstream handler sees partial JSON, returns 400 instead of 413 | Warning | WR-03 from code review: a body over 64 KiB gets a misleading 400 error instead of 413; not a regression in dc61b43 |

### Human Verification Required

These are deliberate UAT steps per CLAUDE.md §Releasing to be run by the user before/after `git tag v4.2.0`. They are not gaps in automated verification — all automated checks pass at 17/17.

#### 1. Live T3 smoke (coalescing invariant — CR-01 now resolved in source)

**Test:** Run `bash scripts/phase-08/verify.sh` (or `powershell -File scripts/phase-08/verify.ps1`) against a running v4.2.0 daemon.
**Expected:** T3 now reports a non-zero `SetActivity` count (> 0 and <= 3) confirming the burst-coalesce invariant I1 is observable from a live daemon. T1-T6 all PASS.
**Why human:** External-infrastructure UAT. Requires a live Discord-connected daemon. The T3 false-green is resolved in source (commit dc61b43 confirmed; test suite emits the log line); this step confirms the fix carries through to the production binary and real Discord IPC.

#### 2. End-to-end presence update behavior (70% skip reduction claim)

**Test:** Trigger 10+ rapid tool calls (e.g., Grep loop) in a real Claude Code session while dsrcode v4.2.0 daemon is running and Discord is open.
**Expected:** Presence updates smoothly ~every 4 seconds; no "stuck" periods; `grep "presence update skipped" reason=content_hash ~/.claude/dsrcode.log` shows hash skips for redundant resolves; `grep "coalescer status" ~/.claude/dsrcode.log` shows a non-zero `sent` counter in the 60s summary.
**Why human:** External-infrastructure UAT. The headline metric (70% skip rate eliminated) requires a live production session. Automated tests use a mock Discord client and synthetic sessions.

### Gaps Summary

No gaps. All 17 RLC truths are fully verified after commit dc61b43.

The one gap from the initial verification (CR-01: T3 false-green due to missing `"presence updated"` log line) has been closed. The fix adds `slog.Info("presence updated", "details", a.Details, "state", a.State)` to `coalescer/coalescer.go flushPending()` at line 224, immediately after `c.lastSentHash.Store(HashActivity(a))` and before `c.sent.Add(1)`. The log line is INFO level (matching the grep in verify.sh/ps1 T3), fires only on the success path (not on IPC error), and is confirmed emitted by the test suite during `TestCoalescer_TokenBucketRate` and `TestCoalescer_FlushSchedule`.

---

_Initial verification: 2026-04-16T23:59:00Z_
_Re-verification: 2026-04-16T23:59:00Z (post-fix commit dc61b43)_
_Verifier: Claude (gsd-verifier)_
