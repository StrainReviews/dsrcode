---
phase: 8
slug: presence-rate-limit-coalescer-stop-drop-on-skip-token-bucket
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-16
---

# Phase 8 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from `08-RESEARCH.md §Validation Architecture` (17 RLC-IDs, 8 invariants).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` + Go 1.25 `testing/synctest` + `net/http/httptest` |
| **Config file** | None — idiomatic `_test.go` files co-located with source (Go convention) |
| **Quick run command** | `go test ./coalescer/... ./server/... -race -count=1` |
| **Full suite command** | `go test ./... -race -count=1 -v` |
| **Race verification** | `-race` mandatory (already in `.github/workflows/test.yml:28`) |
| **Estimated runtime** | ~20 seconds full suite; ~5 seconds per-task quick |

---

## Sampling Rate

- **After every task commit:** `go test ./coalescer ./server -race -count=1` (~5 s)
- **After every plan wave:** `go test ./... -race -count=1 -v` (~20 s)
- **Before `/gsd-verify-work`:** Full suite green + `scripts/phase-08/verify.sh` T1..T6
- **Max feedback latency:** 20 seconds (full suite)

---

## Per-Task Verification Map

| Req ID | Plan | Wave | Behavior | Invariant | Test Type | Automated Command | File Exists | Status |
|--------|------|------|----------|-----------|-----------|-------------------|-------------|--------|
| RLC-01 | 08-01 | 1 | Token bucket allows 2 instant + 1 every 4s | I1 | unit (synctest) | `go test ./coalescer -run TestCoalescer_TokenBucketRate -race` | ❌ W0 | ⬜ pending |
| RLC-02 | 08-01 | 1 | `atomic.Pointer.Store` replaces latest; `Swap(nil)` clears | I1 | unit | `go test ./coalescer -run TestCoalescer_PendingSlot -race` | ❌ W0 | ⬜ pending |
| RLC-03 | 08-01 | 1 | `AfterFunc`-scheduled flush fires exactly once after `Delay()` | I1 | unit (synctest) | `go test ./coalescer -run TestCoalescer_FlushSchedule -race` | ❌ W0 | ⬜ pending |
| RLC-04 | 08-02 | 2 | FNV-64a hash deterministic across process restarts | I2 | unit | `go test ./coalescer -run TestHashActivity_Deterministic -race` | ❌ W0 | ⬜ pending |
| RLC-05 | 08-02 | 2 | Same Activity content → same hash (StartTime excluded) | I2 | unit | `go test ./coalescer -run TestHashActivity_ExcludesStartTime -race` | ❌ W0 | ⬜ pending |
| RLC-06 | 08-02 | 2 | `atomic.Uint64 lastSentHash` load/store correct | I2, I7 | unit | `go test ./coalescer -run TestCoalescer_HashSkip -race` | ❌ W0 | ⬜ pending |
| RLC-07 | 08-03 | 2 | Dedup middleware wraps `/hooks/*` only; other routes pass through | I3, I8 | integration | `go test ./server -run TestHookDedup_PathFilter -race` | ❌ W0 | ⬜ pending |
| RLC-08 | 08-03 | 2 | Dedup key = FNV-64a(route + sep + session + sep + tool + sep + body) | I3 | integration | `go test ./server -run TestHookDedup_KeyComposition -race` | ❌ W0 | ⬜ pending |
| RLC-09 | 08-03 | 2 | TTL 500 ms; 60 s ticker evicts stale entries | I3 | integration (synctest) | `go test ./server -run TestHookDedup_TTLCleanup -race` | ❌ W0 | ⬜ pending |
| RLC-10 | 08-03 | 2 | Middleware preserves request body for downstream JSON decode | I8 | integration | `go test ./server -run TestHookDedup_BodyPreserved -race` | ❌ W0 | ⬜ pending |
| RLC-11 | 08-01 | 1 | 60 s summary log fires when counters > 0; skipped when idle | — | unit (synctest) | `go test ./coalescer -run TestCoalescer_SummaryIdleSkip -race` | ❌ W0 | ⬜ pending |
| RLC-12 | 08-01 | 1 | `Shutdown()` is idempotent: clears pending + stops timers | I5 | unit | `go test ./coalescer -run TestCoalescer_ShutdownIdempotent -race` | ❌ W0 | ⬜ pending |
| RLC-13 | 08-01 | 1 | IPC disconnect discards pending; reconnect triggers fresh signal | I4 | unit | `go test ./coalescer -run TestCoalescer_ReconnectDiscard -race` | ❌ W0 | ⬜ pending |
| RLC-14 | 08-03 | 2 | `/preview` bypasses Coalescer (direct `SetActivity`) | — | integration | `go test ./server -run TestPreview_BypassesCoalescer` | ❌ W0 | ⬜ pending |
| RLC-15 | 08-01 | 1 | `testing/synctest` + `rate.Limiter` interop probe | — | probe | `go test ./coalescer -run TestSynctest_RateLimiterProbe -race` | ❌ W0 | ⬜ pending |
| RLC-16 | 08-04 | 3 | `verify.sh` / `verify.ps1` T1..T6 cross-platform smoke | I1..I4 | manual / smoke | `bash scripts/phase-08/verify.sh` / `.\scripts\phase-08\verify.ps1` | ❌ W0 | ⬜ pending |
| RLC-17 | 08-04 | 3 | CHANGELOG v4.2.0 entry present + Keep-a-Changelog 1.1.0 compliant | — | doc-lint | `grep '## \[4\.2\.0\]' CHANGELOG.md` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Observable Invariants (Nyquist Dimension 8)

| ID | Invariant | How Verified |
|----|-----------|--------------|
| **I1** | Burst of N>2 `updateChan` signals in 4 s → ≤2 `SetActivity` calls | RLC-01/02/03 unit tests + `verify.sh` T3 |
| **I2** | Identical-content burst of N signals → exactly 1 `SetActivity` call | RLC-04/05/06 unit tests + `verify.sh` T4 |
| **I3** | Duplicate POST `/hooks/pre-tool-use` within 500 ms → 1 handler invocation | RLC-07/08/09 integration tests + `verify.sh` T5 |
| **I4** | IPC close → pending cleared; reconnect → fresh Activity pushed | RLC-13 unit test + simulated mock IPC close |
| **I5** | After `cancel()` + `Shutdown()` no coalescer goroutines remain | RLC-12 unit test + `runtime.NumGoroutine()` before/after |
| **I6** | `go test -race ./...` passes, including under synctest bubbles | CI enforced at `.github/workflows/test.yml:28` |
| **I7** | No `sync.Mutex.Lock` in Coalescer flush path (Reserve→Delay→Swap→SetActivity) | Code inspection + grep `sync.Mutex` in `coalescer/` |
| **I8** | Handler downstream of `HookDedupMiddleware` decodes body identically to bypass | RLC-10 integration test |

---

## Wave 0 Requirements

- [ ] `coalescer/coalescer.go` — new package
- [ ] `coalescer/hash.go` — FNV-64 helper (could inline if planner prefers)
- [ ] `coalescer/coalescer_test.go` — stubs for RLC-01..RLC-06, RLC-11..RLC-13, RLC-15
- [ ] `server/hook_dedup.go` — `HookDedupMiddleware`
- [ ] `server/hook_dedup_test.go` — stubs for RLC-07..RLC-10, RLC-14
- [ ] `main.go` — swap `go presenceDebouncer(...)` → `go coalescer.Run(ctx)` + wrap `s.Handler()` via middleware
- [ ] `scripts/phase-08/verify.sh` + `verify.ps1` — smoke harness for RLC-16
- [ ] `CHANGELOG.md` v4.2.0 entry — RLC-17
- [ ] `go.mod` — add `require golang.org/x/time v0.15.0` (transitive `x/sys` already present)

**Framework install:** None needed — stdlib `testing` + stdlib `testing/synctest` (Go 1.25 GA) + one dep. `go mod tidy` handles the require line.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Live Discord RPC presence updates under MCP-heavy session | I1, I2 (end-to-end) | Requires running Discord client + active Claude Code MCP session | 1. Launch Discord. 2. Start Claude Code in a project. 3. Trigger ≥10 rapid tool calls (e.g. `Grep` loop). 4. Observe Discord presence updates ≤4 s after final signal. 5. `grep 'presence updated' dsrcode.log | wc -l` shows ≤N/3 entries for N signals. |
| 70 % → <5 % skip-rate improvement (headline metric) | Phase goal | Requires side-by-side before/after profiling | Compare `grep 'presence update skipped' dsrcode.log | wc -l / total signals` between v4.1.2 and v4.2.0 under identical workload. |

---

## Validation Sign-Off

- [ ] All 17 RLC tasks have `<automated>` verify OR Wave 0 test stub
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING file references above
- [ ] No watch-mode flags (Go tests are single-shot)
- [ ] Feedback latency < 20 s (full suite)
- [ ] `nyquist_compliant: true` set in frontmatter after Wave 0 complete
- [ ] All 8 invariants map to at least one RLC-NN requirement

**Approval:** pending
