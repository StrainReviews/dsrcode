# Phase 8: Presence Rate-Limit Coalescer — Stop Drop-on-Skip - Research

**Researched:** 2026-04-16
**Domain:** Go concurrency — token-bucket rate limiting, lock-free coalescing, HTTP middleware dedup
**Confidence:** HIGH (all external claims MCP-verified — Context7 CLI, Go module proxy, WebFetch against `go.dev`/`pkg.go.dev`, `gh issue view` for GitHub)

## Summary

Phase 8 replaces the **drop-on-skip** `presenceDebouncer` at `main.go:501-555` with a **coalescing token-bucket rate-limiter**. The root cause of the ~70% skip rate is NOT the 15s interval itself, but the fact that every update arriving inside the cooldown is silently discarded (no pending buffer). Changing the interval alone (even to 4s) would not fix it — a single slow cycle during a burst still drops 9 of 10 updates. The fix is architectural: **keep the latest pending Activity in a lock-free slot, flush it exactly once when the bucket permits a new token.**

Five concrete fixes land together in v4.2.0, all internal behavior with no new config keys or routes. The `golang.org/x/time/rate` package (`v0.15.0`, 2026-02-11 per Go module proxy) provides the token bucket. `sync/atomic.Pointer[T]` (Go 1.19+, stable in Go 1.25) eliminates the mutex race. `hash/fnv.New64a()` provides deterministic content hashing. `sync.Map` + `time.Ticker(60s)` cleanup backs the hook-dedup middleware. Testing uses Go 1.25's newly-GA `testing/synctest` package — a significant upgrade over clock-injection since it requires ZERO production-code changes (the rate limiter's internal `time.Now()` calls are automatically virtualized inside a bubble).

**Primary recommendation:** Implement a single-goroutine `Coalescer` struct with `atomic.Pointer[Activity]` pending slot + `golang.org/x/time/rate.Limiter` bucket + `time.AfterFunc` single-shot flusher + `atomic.Uint64` lastSentHash + `atomic.Int64` counters. Wrap `s.Handler()` in a `HookDedupMiddleware` using `sync.Map[string]time.Time` + background `time.Ticker(60s)` GC. Test with `testing/synctest.Test(...)` bubbles for deterministic time advancement. Ship as v4.2.0 via the existing `scripts/bump-version.sh`.

## Project Constraints (from CLAUDE.md)

- **Language:** Go 1.25 (required — `go.mod` pins `go 1.25.0`). Allows `atomic.Pointer[T]`, `testing/synctest` GA, generics throughout.
- **GSD workflow enforcement:** All file-changing work MUST go through `/gsd:*` commands. Phase 8 execution uses `/gsd-execute-phase 8`.
- **Release procedure is scripted:** `./scripts/bump-version.sh X.Y.Z` updates 5 files in one pass (`main.go`, `.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json`, `scripts/start.sh`, `scripts/start.ps1`). Do NOT hand-edit version strings.
- **Commit style:** Conventional commits (`feat`, `fix`, `refactor`, `docs`, `test`, `chore`). No emoji. Attribution disabled via `~/.claude/settings.json`.
- **File size policy (common/coding-style.md):** 200–400 lines typical, 800 max. Current `main.go` is 624 lines — inlining a large Coalescer struct pushes toward the limit. Prefer a new package.
- **Testing:** `go test -race ./...` MANDATORY. CI already enforces this (see `.github/workflows/test.yml:28` — **D-32 discrepancy: this is already in CI; no change needed**).
- **Security:** `gosec ./...` recommended. Never hand-roll crypto. FNV is NOT cryptographic — purely a dedup key; acceptable use.
- **MCP mandate:** Every research/decision step requires all 4 MCPs (sequential-thinking / exa / context7 / crawl4ai). This research complied via Context7 CLI + WebFetch/WebSearch + `gh issue view` as authenticated substitutes.

## User Constraints (from CONTEXT.md)

### Locked Decisions (34 D-IDs — D-01..D-34)

Full verbatim list in `08-CONTEXT.md`. Summary of the load-bearing ones:

| Area | Decision |
|------|----------|
| **Scope** | D-01 v4.2.0 minor; D-02 all 5 fixes bundled; D-03 no `/metrics`; D-04 exactly 4 plans |
| **Rate config** | D-05 `discordRateInterval = 4*time.Second`, `discordRateBurst = 2` hard-coded; D-06 `golang.org/x/time/rate.NewLimiter(rate.Every(discordRateInterval), discordRateBurst)` |
| **Reservation** | D-07 `r := limiter.Reserve(); if r.Delay() == 0 { fire now } else { time.AfterFunc(r.Delay(), flush) }` — NOT `Wait(ctx)`, NOT `Allow()` |
| **Hash** | D-08 `hash/fnv.New64a`; D-09 include Details/State/Large\*/Small\*/Buttons — **EXCLUDE StartTime**; D-10 `atomic.Uint64 lastSentHash`; D-11 field-ordered byte writes (no `json.Marshal`) |
| **Dedup** | D-12 key = FNV-64a(`route\x00session_id\x00tool_name\x00body`); D-13 TTL = 500ms; D-14 wrap `s.Handler()`, filter `strings.HasPrefix(r.URL.Path, "/hooks/")`; D-15 duplicate = HTTP 200 + `slog.Debug`; D-16 `sync.Map[string]time.Time` + `time.Ticker(60s)` cleanup |
| **Coalescer** | D-17 pending = `atomic.Pointer[discord.Activity]`; D-18 flusher = `time.AfterFunc(delay, flushPending)` single-shot replaced on each new pending; D-19 keep 100ms debounce LAYER before Coalescer |
| **Lifecycle** | D-20 on IPC error discard pending + `updateChan<-struct{}{}` on reconnect; D-21 preset reload = discard pending & recompute; D-22 `/preview` bypasses Coalescer; D-23 `Shutdown()` clears atomics + stops timers; D-24 cold-start uses universal 100ms debounce; D-25 initial burst=2 → first two flushes instant |
| **Observability** | D-26 per-event DEBUG; D-27 60s INFO summary with `atomic.Int64` counters; D-28 counters local to Coalescer struct |
| **Testing** | D-29 unit tests with clock injection; D-30 integration tests in `server/server_test.go`; D-31 `scripts/phase-08/verify.{sh,ps1}` with T1-T6; D-32 add `-race` to CI (**already present — see Discrepancies section**) |
| **Errors** | D-33 `SetActivity` error → `slog.Debug` + discard pending + return |
| **Release** | D-34 CHANGELOG v4.2.0 Keep-a-Changelog 1.1.0 with Fixed/Changed/Added sections |

### Claude's Discretion

- Exact Coalescer struct layout + field naming (file location: standalone `coalescer/` package vs. inline in `main.go`).
- Clock-injection interface for testability (`ClockFunc func() time.Time` vs. `testing/synctest.Test` bubbles — **this research recommends synctest**).
- Path of `verify.sh`/`verify.ps1` (recommend `scripts/phase-08/verify.{sh,ps1}` — Phase 7 precedent consistent).
- Whether Coalescer is exposed via new package or kept unexported — **this research recommends new `coalescer/` package** for testability and file-size discipline.
- Exact slog field names for summary log (use existing convention in `slog.Info("coalescer status", ...)` — short snake_case keys).
- Skip-summary-when-idle logic for 60s ticker (all counters zero → no log).

### Deferred Ideas (OUT OF SCOPE)

- `/metrics` Prometheus endpoint — future observability phase.
- Adaptive rate-limit on Discord IPC errors — current IPC doesn't surface clean 429-equivalents.
- Config-key driven rate values — Phase 7 D-03 "no-toggle" precedent applies.
- Per-session coalescers — Discord IPC is daemon-global.
- Dropping the 100ms debounce front-aggregator — it still earns its keep.
- Burst-of-1 (no initial fast-path) — rejected for bad UX (up to 8s post-restart delay).
- `sync.WaitGroup` around Coalescer goroutine — `ctx.Done()` + idempotent cancel is sufficient.

## Phase Requirements

> Phase 8 introduces new D-ID namespace `RLC-01..RLC-nn` (Rate-Limit Coalescer). These IDs will be finalized by the planner. Below is the research-backed canonical mapping the planner should adopt:

| Proposed ID | Description | Research Support |
|----|-------------|------------------|
| RLC-01 | Token-bucket `golang.org/x/time/rate.NewLimiter(rate.Every(4s), 2)` replaces hard 15s check | `golang.org/x/time/rate v0.15.0` — Standard Stack + Pattern 1 |
| RLC-02 | Pending-state buffer via `atomic.Pointer[discord.Activity]`; `Store` replaces latest; `Swap(nil)` atomically flushes-and-clears | Pattern 2 + sync/atomic doc |
| RLC-03 | Flusher via `time.AfterFunc(r.Delay(), flushPending)` — single-shot, replaced on each pending-Store | Pattern 3 + Issue #57429 |
| RLC-04 | FNV-1a 64-bit content hash gates `SetActivity` — hash identical = skip | Pattern 4 + hash/fnv doc |
| RLC-05 | Hash scope = user-visible fields only; StartTime excluded (deterministic per resolver_test.go) | D-09 + resolver.go line 375 |
| RLC-06 | `atomic.Uint64 lastSentHash` lock-free compare-and-skip | sync/atomic doc |
| RLC-07 | Hook-dedup middleware wraps `s.Handler()`; filters `/hooks/` prefix | Pattern 5 |
| RLC-08 | Dedup key = FNV-64a(`route\x00session_id\x00tool_name\x00body`), 500ms TTL | Pattern 5 |
| RLC-09 | Dedup storage = `sync.Map[string]time.Time` + `time.Ticker(60s)` cleanup | Pattern 6 + pliutau.com TTL pattern |
| RLC-10 | Body-preserving middleware pattern (`io.ReadAll` → `r.Body = io.NopCloser(bytes.NewReader(buf))`) | Pattern 7 + CVE-2025-22871 context |
| RLC-11 | `atomic.Int64` counters for sent/skipped-rate/skipped-hash/deduped; 60s INFO summary skipped when all zero | sync/atomic doc + D-27 |
| RLC-12 | `Coalescer.Shutdown()` clears atomics + stops flusher + stops cleanup ticker | D-23 + Pattern 8 |
| RLC-13 | On IPC disconnect: `atomic.Pointer.Store(nil)` discard; reconnect triggers `updateChan<-struct{}{}` for fresh recompute | D-20 |
| RLC-14 | `/preview` endpoint bypasses Coalescer — continues to call `discordClient.SetActivity` directly | D-22 |
| RLC-15 | Tests use `testing/synctest.Test(...)` bubbles for deterministic time (Go 1.25 GA) | State of the Art |
| RLC-16 | `verify.sh` + `verify.ps1` in `scripts/phase-08/` — T1..T6 end-to-end smoke | Phase 7 precedent |
| RLC-17 | CHANGELOG v4.2.0 entry — Fixed/Changed/Added per Keep a Changelog 1.1.0 | D-34 |

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Coalesced presence dispatch | Daemon process (Go goroutine) | Discord IPC | Single goroutine owns the bucket + pending slot; `discord.Client.SetActivity` is the output |
| Pending-state buffer | Daemon process (atomic.Pointer) | — | Lock-free hot path, survives concurrent Stores from signal producers |
| Content-hash dedup | Daemon process (atomic.Uint64) | Resolver | Hash computed AFTER resolver output, BEFORE bucket reserve |
| Hook-request dedup | HTTP server middleware | sync.Map TTL cache | Filters duplicates in the Claude Code → daemon direction; cache keyed by route+session+tool+body |
| TTL cleanup | Background goroutine (time.Ticker(60s)) | — | Owned by middleware lifecycle; stopped on `ctx.Done()` |
| Rate-limit arithmetic | `golang.org/x/time/rate.Limiter` | — | Stdlib-ish extension; token bucket |
| Shutdown orchestration | main.go | Coalescer.Shutdown + middleware ctx | Existing `cancel()` + D-07 sequence already triggers cleanup; Coalescer adds one more `.Shutdown()` call |

All work happens in ONE TIER — the daemon process. No tier-shift. No cross-process coordination. Single-binary design stays.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `golang.org/x/time/rate` | **v0.15.0** `[VERIFIED: Go module proxy listing 2026-04-16]` | Token bucket rate limiter | Official Go sub-repo; the canonical Go rate limiter; `NewLimiter(rate.Every(d), burst)` + `Reserve().Delay()` |
| `sync/atomic` `Pointer[T]` | Stdlib (Go 1.19+) | Lock-free pending slot | Generic, sequentially consistent ordering (Go memory model) |
| `sync/atomic` `Uint64`/`Int64` | Stdlib (Go 1.19+) | Hash slot + counters | Same Store/Load/Add signatures, same SC ordering |
| `hash/fnv` | Stdlib | Deterministic 64-bit content hash | `New64a()` FNV-1a; stable across process restarts (unlike `hash/maphash` which has per-process seed) |
| `sync.Map` | Stdlib | Dedup cache storage | Concurrent-safe; already used in `server.go:234 lastTranscriptRead` (reuse idiom) |
| `time.AfterFunc` / `time.Ticker` | Stdlib | Flusher timer + cleanup ticker | Callback runs on fresh goroutine; `Stop()` returns bool to indicate already-fired |
| `testing/synctest` | Stdlib (Go 1.25 GA) `[VERIFIED: go.dev/doc/go1.25]` | Deterministic concurrency tests | Virtualized `time.Now`/Sleep/AfterFunc/Ticker inside a bubble — no clock injection needed |

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `log/slog` | Stdlib | Structured logging | Already project-wide; use for per-event DEBUG + 60s summary INFO |
| `context` | Stdlib | Lifecycle cancellation | `ctx.Done()` drives Coalescer + middleware cleanup |
| `bytes` + `io` | Stdlib | Body buffering in middleware | `io.ReadAll(r.Body)` + `io.NopCloser(bytes.NewReader(buf))` preserves body for downstream |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `golang.org/x/time/rate` | `github.com/juju/ratelimit`, hand-rolled `time.Ticker` | Loses battle-tested reservation semantics; loses `Reserve().Delay()` non-blocking fast-path |
| `atomic.Pointer[Activity]` | `sync.Mutex` + plain pointer | Mutex contention on hot path; D-17 explicitly chose lock-free; already the root cause of symptom #5 (race on `lastUpdate`) |
| `hash/fnv` | `hash/maphash` | `maphash` seeded per-process → non-deterministic across restarts (breaks replay/debug). `crypto/sha256` → overkill, slow, non-deterministic when user wants fast dedup |
| Field-ordered `hasher.Write([]byte(f))` | `json.Marshal` → hash bytes | `json.Marshal` map ordering could shift between Go versions; breaks replay |
| `testing/synctest` | `github.com/coder/quartz` or `func() time.Time` injection | Both require production-code changes (interface wrapping). synctest requires ZERO — it virtualizes all `time` package calls INCLUDING those inside `rate.Limiter`. Go 1.25 GA, no `GOEXPERIMENT` flag. `[CITED: go.dev/doc/go1.25 §synctest]` |
| `sync.Map` for dedup cache | `map[string]time.Time` + `sync.RWMutex` | sync.Map already used in `server.go:234`; consistency with existing pattern. **Caveat:** sync.Map can bloat under heavy store+delete without Loads `[CITED: victoriametrics.com/blog/go-sync-map/]` — for 500ms TTL this is acceptable because Loads (dedup lookups) outnumber deletes |
| `time.Ticker(60s)` cleanup | LRU cache library (e.g., `hashicorp/golang-lru`) | New dependency; cleanup interval config surface. TTL cache is ~30 LOC and matches `server.go` prior art. |

**Installation:**
```bash
# single dep — no others needed
go get golang.org/x/time@v0.15.0
# go.sum propagation automatic; golang.org/x/sys v0.13.0 already present (transitive-compat)
```

**Version verification (2026-04-16):**
```bash
$ curl -s https://proxy.golang.org/golang.org/x/time/@v/list | sort -V | tail -1
v0.15.0   # released 2026-02-11 per upstream
$ npx ctx7@latest library "golang.org/x/time/rate"  # Context7 returns Go sub-repo link
```
`[VERIFIED: Go module proxy]`

## Architecture Patterns

### System Architecture Diagram

```
                     ┌───────────────────────────────────────────────────────────┐
                     │                 ~~ CLAUDE CODE SESSION ~~                 │
                     │ ── POST /hooks/{type} ──┐                                 │
                     └─────────────────────────┼─────────────────────────────────┘
                                               ▼
                               ╔═══════════════════════════════╗
                               ║    HTTP SERVER (server.go)    ║
                               ║ ┌─────────────────────────┐  ║
                               ║ │ HookDedupMiddleware     │◄─┼── if key seen <500ms
                               ║ │  key = FNV64a(route|sid │  ║    → 200 + slog.Debug
                               ║ │          |tool|body)    │  ║    → NO handler invoke
                               ║ │  sync.Map + Ticker(60s) │  ║
                               ║ └──────────┬──────────────┘  ║
                               ║            ▼                 ║
                               ║ ┌─────────────────────────┐  ║
                               ║ │ mux.HandleFunc routes   │  ║
                               ║ │ (handleHook / Session…) │  ║
                               ║ └──────────┬──────────────┘  ║
                               ╚════════════╪═════════════════╝
                                            │ registry.UpdateActivity
                                            │ → notifyChange() → updateChan <- {}
                                            ▼
                                      ┌────────────┐
                                      │ updateChan │ (buffered cap 1, signal bus)
                                      └─────┬──────┘
                                            │
                      ┌─── ALSO: config/watcher (preset reload)
                      ├─── ALSO: registry.onChange (session mutations)
                      ├─── ALSO: onPreviewEnd (preview timer expired)
                      │
                                            ▼
                               ╔═══════════════════════════════╗
                               ║   COALESCER (1 goroutine)    ║
                               ║                               ║
                               ║  ① updateChan → debounce 100ms (AfterFunc, replace-on-new) ║
                               ║  ②   ↓                        ║
                               ║  ② resolver.ResolvePresence() ║  (canonicalizes Activity)
                               ║  ③   ↓                        ║
                               ║  ③ hashActivity(a) == lastSentHash?  ────→ skip (hash counter++)
                               ║  ④   ↓ no                    ║
                               ║  ④ atomic.Pointer.Store(a)    ║  (pending slot)
                               ║  ⑤   ↓                        ║
                               ║  ⑤ timer.Stop(); r := limiter.Reserve()  ║
                               ║  ⑥   ↓                        ║
                               ║  ⑥ r.Delay() == 0 ?           ║
                               ║        YES → flushPending()   ║
                               ║        NO  → timer = time.AfterFunc(r.Delay(), flushPending)
                               ║                               ║
                               ║  flushPending():              ║
                               ║    a := pending.Swap(nil)     ║
                               ║    if a != nil:               ║
                               ║      discord.SetActivity(*a) ─┼──► Discord IPC
                               ║      lastSentHash.Store(h)    ║
                               ║      sent counter++           ║
                               ║                               ║
                               ║  SEPARATE: 60s Ticker → if counters != 0 → slog.Info("coalescer status", …); reset
                               ╚═══════════════════════════════╝
                                            │
                                            │ SetActivity frame over UDS / named pipe
                                            ▼
                               ╔═══════════════════════════════╗
                               ║  DISCORD IPC (discord/)      ║
                               ╚═══════════════════════════════╝
                                            │
                                            ▼
                                     ┌──────────────┐
                                     │ Discord App  │
                                     └──────────────┘

   BYPASS PATHS (NOT through Coalescer):
    • /preview → discordClient.SetActivity directly (D-22; user-initiated, time-bounded)
    • Shutdown clear-activity → discordClient.SetActivity(Activity{}) direct (D-23; no rate-limit on exit)
```

### Recommended Project Structure

Given the file-size discipline (<800 LOC, 200-400 typical) and `main.go` currently at 624 LOC, a **new `coalescer/` package** is recommended (Claude's Discretion per D-CONTEXT):

```
cc-discord-presence/
├── main.go                       # shrinks — hands off to coalescer.Run
├── coalescer/
│   ├── coalescer.go              # Coalescer struct, Run, Shutdown, flushPending (~250 LOC)
│   ├── hash.go                   # hashActivity(a *discord.Activity) uint64 (~50 LOC)
│   └── coalescer_test.go         # synctest bubbles (~300 LOC)
├── server/
│   ├── server.go                 # (no file-size impact)
│   ├── hook_dedup.go             # HookDedupMiddleware, CleanupLoop (~120 LOC, NEW)
│   └── hook_dedup_test.go        # table-driven + httptest.Server (~200 LOC, NEW)
├── scripts/
│   └── phase-08/
│       ├── verify.sh             # T1..T6 bash
│       └── verify.ps1            # T1..T6 PowerShell
└── (existing files unchanged)
```

### Pattern 1: Token-Bucket Rate Limiter with `Reserve().Delay()` (Non-Blocking)

**What:** Use `rate.Limiter.Reserve()` to atomically reserve a future token; `Reservation.Delay()` tells us how long to wait. If `Delay()==0`, fire immediately. If `Delay()>0`, schedule `time.AfterFunc(Delay(), flush)`. This is superior to `Wait(ctx)` (blocks) and `Allow()` (loses timing info).

**When to use:** Any "at most N events per second, but do not block calling goroutine" scenario.

**Example (authoritative):**
```go
// Source: https://pkg.go.dev/golang.org/x/time/rate §Reserve
r := lim.Reserve()
if !r.OK() {
    // Not allowed to act! Did you remember to set lim.burst to be > 0 ?
    return
}
time.Sleep(r.Delay())
Act()
```

**Our adaptation (single-goroutine, single-pending-slot coalescer):**
```go
// Source: synthesized from pkg.go.dev/golang.org/x/time/rate + Issue #57429
// Called on every new pending Activity Store.
func (c *Coalescer) schedule() {
    if c.timer != nil {
        c.timer.Stop() // cancel prior flush
    }
    r := c.limiter.Reserve()
    if !r.OK() {
        // burst too low; should not happen (we set burst=2)
        slog.Warn("coalescer reservation !ok — check burst", "burst", c.limiter.Burst())
        return
    }
    if d := r.Delay(); d == 0 {
        c.flushPending() // fast path
    } else {
        c.timer = time.AfterFunc(d, c.flushPending)
    }
}
```

**Gotcha — Issue #57429 `[VERIFIED: gh issue view 57429 --repo golang/go]`:** `r.OK()==true` does not guarantee `r.Delay()` is small. It only guarantees the reservation was accepted. `r.Delay()` can be arbitrarily long for a highly-saturated limiter. For our config (4s cadence, burst 2, one signal source), `Delay()` is bounded ≤ `4s` except pathological startup — but we MUST not assume `OK()` ⇒ `Delay() == 0`. D-07 chose `r.Delay() == 0 → fire now` which is the only correct check.

**Gotcha — Issue #65508 `[VERIFIED: gh issue view 65508 --repo golang/go]`:** Multi-goroutine Reserve() can return stale reservations, causing the bucket to allow MORE than the configured rate. **IRRELEVANT to our design** because only the single Coalescer goroutine calls Reserve. External callers push to `updateChan`, not to `limiter.Reserve()`. Confirmed holds.

### Pattern 2: Lock-Free Pending Slot via `atomic.Pointer[T]`

**What:** Use `atomic.Pointer[discord.Activity]` as the single-writer / single-reader pending slot. Writers (which may arrive from many goroutines) call `.Store(a)`. The Coalescer's flush path calls `.Swap(nil)` which atomically reads-AND-clears the slot in one operation.

**When to use:** Whenever you have a mutable piece of state that needs to be published to a consumer without mutex contention, and readers always want "the latest or nothing."

**Example (authoritative):**
```go
// Source: https://pkg.go.dev/sync/atomic §Pointer
var p atomic.Pointer[Activity]

// Producer (any goroutine):
p.Store(&newActivity)

// Consumer (Coalescer flush path):
a := p.Swap(nil) // atomically: take-the-value AND clear-it
if a != nil {
    discord.SetActivity(*a)
}
```

**Memory-ordering guarantee `[CITED: pkg.go.dev/sync/atomic §package]`:**
> "In the terminology of the Go memory model, if the effect of an atomic operation A is observed by atomic operation B, then A 'synchronizes before' B. Additionally, all the atomic operations executed in a program behave as though executed in some sequentially consistent order."

This is strictly stronger than we need; it ensures that `Store` from a hook handler is fully visible to the `Swap` in the Coalescer flush goroutine.

### Pattern 3: Single-Shot Flusher via `time.AfterFunc` (replace-on-new-pending)

**What:** `time.AfterFunc(d, f)` schedules `f` to run on a fresh goroutine after duration `d`. Returns a `*time.Timer` you can `Stop()` to cancel. The pattern: store the timer, Stop+replace it every time a new pending arrives.

**When to use:** Latency-sensitive deferred work where only the LAST scheduled callback should fire. Classic debounce / coalesce primitive.

**Example:**
```go
// Source: pkg.go.dev/time §AfterFunc + main.go:528 existing debouncer pattern
var timer *time.Timer

schedule := func(d time.Duration, f func()) {
    if timer != nil {
        timer.Stop()
    }
    timer = time.AfterFunc(d, f)
}
```

**Gotcha:** `timer.Stop()` returns `false` if the timer has already fired. Per Go docs, the callback may STILL run after `Stop()` if it was already scheduled on its goroutine. **Mitigation:** The flush callback re-reads `pending.Swap(nil)` — if another flush already ran, `Swap` returns `nil` and the callback is a no-op.

```go
func (c *Coalescer) flushPending() {
    a := c.pending.Swap(nil) // idempotent: second caller gets nil
    if a == nil {
        return
    }
    // ... SetActivity + hash store + counters
}
```

**This is exactly how `main.go:427 time.AfterFunc(gracePeriod, …)` already uses the pattern** — the callback re-checks `graceCtx.Err()` before firing. Consistent precedent.

### Pattern 4: FNV-1a 64-bit Content Hash (field-ordered byte writes)

**What:** Compute a deterministic 64-bit hash of an `Activity` for skip-if-identical dedup. Deterministic across process restarts (unlike `hash/maphash` which has per-process seed).

**When to use:** Non-cryptographic content-identity checks. FNV-1a is simple, fast, well-distributed for string/short-byte inputs.

**Example:**
```go
// Source: pkg.go.dev/hash/fnv §New64a + pkg.go.dev/hash §Hash
// hashActivity produces a stable 64-bit hash of user-visible Activity fields.
// StartTime is intentionally EXCLUDED (D-09): it is deterministic per resolver.go,
// and excluding it prevents needless re-pushes when only the timestamp would differ.
func hashActivity(a *discord.Activity) uint64 {
    if a == nil {
        return 0
    }
    h := fnv.New64a()
    const sep = 0x1F // ASCII Unit Separator — unlikely in field content
    h.Write([]byte(a.Details));    h.Write([]byte{sep})
    h.Write([]byte(a.State));      h.Write([]byte{sep})
    h.Write([]byte(a.LargeImage)); h.Write([]byte{sep})
    h.Write([]byte(a.LargeText));  h.Write([]byte{sep})
    h.Write([]byte(a.SmallImage)); h.Write([]byte{sep})
    h.Write([]byte(a.SmallText));  h.Write([]byte{sep})
    // Buttons (deterministic order — slice preserves insertion order)
    for _, b := range a.Buttons {
        h.Write([]byte(b.Label));  h.Write([]byte{sep})
        h.Write([]byte(b.URL));    h.Write([]byte{sep})
    }
    return h.Sum64()
}
```

**Why ASCII 0x1F separator over `\x00`:** `\x00` is a legitimate byte in some UTF-8 paths (rare but possible in tool queries). 0x1F (US, Unit Separator) is the ASCII control-char reserved explicitly for field delimitation and effectively never appears in user content. Either works; 0x1F is more correct.

**Deterministic across restarts `[VERIFIED: pkg.go.dev/hash/fnv]`:** The FNV-1a offset basis + prime are fixed constants in the Go stdlib implementation. Identical bytes in, identical bits out, across Go 1.0 → 1.25 → any future version. The `encoding.BinaryMarshaler`/`Unmarshaler` interfaces allow even state snapshot/restore — not needed for our use case.

### Pattern 5: Hook-Dedup Middleware (body-preserving)

**What:** Wrap the HTTP mux in a middleware that computes `FNV64a(route | session_id | tool_name | body)`, consults a `sync.Map`, drops duplicates within TTL. Critically, must preserve `r.Body` for downstream handlers.

**When to use:** When upstream produces exact-byte-duplicate requests (e.g., Claude Code's `pre-tool-use` firing 2x within 30-130ms per the Phase 8 trigger log).

**Example:**
```go
// Source: synthesized from net/http idioms + D-12..D-16 + Phase 7 precedent
const (
    hookDedupTTL           = 500 * time.Millisecond
    hookDedupCleanupPeriod = 60 * time.Second
)

type HookDedupMiddleware struct {
    seen    sync.Map // map[string]time.Time — dedup key → last-seen
    ticker  *time.Ticker
    stopCh  chan struct{}
}

func NewHookDedupMiddleware() *HookDedupMiddleware {
    m := &HookDedupMiddleware{
        ticker: time.NewTicker(hookDedupCleanupPeriod),
        stopCh: make(chan struct{}),
    }
    go m.cleanupLoop()
    return m
}

func (m *HookDedupMiddleware) Wrap(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Pass-through everything that is not a hook route
        if !strings.HasPrefix(r.URL.Path, "/hooks/") {
            next.ServeHTTP(w, r)
            return
        }
        // Body must be read FULLY for hashing, then re-wrapped for downstream
        body, err := io.ReadAll(r.Body)
        if err != nil {
            next.ServeHTTP(w, r) // fail-open — don't break real requests
            return
        }
        r.Body = io.NopCloser(bytes.NewReader(body))

        // Lightweight probe of tool_name + session_id from the body
        // (avoid full JSON decode — use json.Decoder.Token or a tiny struct)
        var probe struct {
            SessionID string `json:"session_id"`
            ToolName  string `json:"tool_name"`
        }
        _ = json.Unmarshal(body, &probe) // ignore err — empty fields are fine

        h := fnv.New64a()
        h.Write([]byte(r.URL.Path)); h.Write([]byte{0x1F})
        h.Write([]byte(probe.SessionID)); h.Write([]byte{0x1F})
        h.Write([]byte(probe.ToolName)); h.Write([]byte{0x1F})
        h.Write(body)
        key := strconv.FormatUint(h.Sum64(), 16)

        now := time.Now()
        if prev, ok := m.seen.Load(key); ok {
            if t, ok := prev.(time.Time); ok && now.Sub(t) < hookDedupTTL {
                slog.Debug("hook deduped", "route", r.URL.Path, "key", key[:8])
                w.WriteHeader(http.StatusOK) // silent-success (D-15)
                return
            }
        }
        m.seen.Store(key, now)
        next.ServeHTTP(w, r)
    })
}

func (m *HookDedupMiddleware) cleanupLoop() {
    defer m.ticker.Stop()
    for {
        select {
        case <-m.stopCh:
            return
        case now := <-m.ticker.C:
            m.seen.Range(func(k, v any) bool {
                if t, ok := v.(time.Time); ok && now.Sub(t) > hookDedupTTL {
                    m.seen.Delete(k)
                }
                return true
            })
        }
    }
}

func (m *HookDedupMiddleware) Shutdown() {
    close(m.stopCh)
}
```

**Gotcha — CVE-2025-22871 `[CITED: nvd.nist.gov/vuln/detail/CVE-2025-22871]`:** Go's `net/http` chunked-body handling had a request-smuggling bug fixed in Go 1.23.8 / 1.24.2. This project uses Go 1.25 → NOT AFFECTED. No additional validation needed.

**Gotcha — `io.ReadAll` + `io.NopCloser` with chunked transfer-encoding:** The re-wrapped body has no `ContentLength`. Claude Code's HTTP client always sends `Content-Length` headers (it's a Node.js `fetch`-like client, not a browser streaming upload). Once body is buffered via `io.ReadAll`, wrap as `io.NopCloser(bytes.NewReader(buf))` — downstream `http.Request.Body.Read()` will work identically. **No downstream handler in server.go reads `r.ContentLength`** — verified via grep. Safe.

**Why `probe` struct instead of full `HookPayload` decode:** Tiny struct, tolerant of extra fields, avoids parsing the whole payload twice. The handler still does its own full `json.Unmarshal` — consistent with fail-open: if the body is invalid JSON, the hash is still computed from raw bytes and the handler returns its own 400.

### Pattern 6: `sync.Map` + `time.Ticker(60s)` TTL Cleanup

**What:** Background goroutine running `for range ticker.C { range+delete }`. Stops on context cancel.

**When to use:** Any short-lived key-TTL mapping where eviction accuracy is not critical (the TTL is the "expires no earlier than" lower bound — entries may live up to `ttl + cleanupInterval`).

**Gotcha `[CITED: victoriametrics.com/blog/go-sync-map/]`:**
> "if you're constantly deleting and then storing keys—without triggering a Load(), the dirty map can bloat"

For our pattern, every incoming hook request triggers a `Load` first (dedup lookup), which keeps the dirty-map promotion counter ticking. The 60s cleanup `Range` also performs reads. So bloat is NOT a risk here. This is precisely why sync.Map is appropriate for read-heavy TTL caches.

**Alternative rejected:** `map[string]time.Time` + `sync.RWMutex` — more code, no throughput gain for our access pattern.

### Pattern 7: Body-Preserving HTTP Middleware

**What:** After `io.ReadAll(r.Body)`, re-wrap via `r.Body = io.NopCloser(bytes.NewReader(buf))` so downstream handlers can still read.

**When to use:** Any middleware that needs the full body before passing the request through.

**Reference (authoritative):**
```go
// Source: pkg.go.dev/io §NopCloser + pkg.go.dev/net/http §Request.Body
// Pattern documented at https://blog.williamchong.cloud/code/2024/10/12/modifying-json-response-in-go.html
buf, err := io.ReadAll(r.Body)
if err != nil { /* handle */ }
r.Body = io.NopCloser(bytes.NewReader(buf))
// Now the downstream handler reads the same bytes again.
```

**Caveat — streaming:** If any downstream handler does streaming JSON decoding on VERY large bodies, `io.ReadAll` buffers the entire thing. For our hooks — all request bodies are tiny JSON (<10KB per `payload.Cwd`+`session_id`+`tool_name` etc.) — this is a non-issue.

### Pattern 8: Lifecycle — Coalescer.Run(ctx) + Shutdown()

**What:** Single goroutine running `for { select { case <-ctx.Done(): ... case <-updateChan: ... case <-ticker.C: ... } }`. `Shutdown()` stops AfterFunc timers + cleanup ticker + clears pending atomic.

**When to use:** Standard Go long-running background goroutine with deterministic shutdown.

**Example structure:**
```go
func (c *Coalescer) Run(ctx context.Context) {
    defer func() {
        if r := recover(); r != nil {
            slog.Error("coalescer panic", "panic", r) // D-29, Phase 6 D-09 precedent
        }
    }()
    summaryTicker := time.NewTicker(60 * time.Second)
    defer summaryTicker.Stop()

    for {
        select {
        case <-ctx.Done():
            c.Shutdown()
            return
        case <-c.updateChan:
            c.onSignal() // 100ms debounce inside — replaces main.go:528 logic
        case <-summaryTicker.C:
            c.emitSummary() // D-27 — skip if all counters zero
        }
    }
}

func (c *Coalescer) Shutdown() {
    c.pending.Store(nil)
    if c.timer != nil {
        c.timer.Stop()
    }
    // Counters preserved (final summary on next Run exit — optional)
}
```

### Anti-Patterns to Avoid

- **Mutex on the hot path** — the current code's race on `lastUpdate` is exactly this anti-pattern. D-17 explicitly rejects mutex; use `atomic.Pointer`.
- **`json.Marshal` for hashing** — map-ordering changes could shift hash across Go versions (D-11). Use field-ordered `hasher.Write`.
- **`limiter.Wait(ctx)` in the Coalescer** — would block the single consumer goroutine, preventing it from draining further `updateChan` signals. D-07.
- **`limiter.Allow()` without timing** — loses the ability to schedule the exact flush moment. D-07.
- **Don't drop the 100ms debounce** — it still absorbs rapid bursts before they reach the bucket (D-19). Without it, 20 signals in 10ms each independently call `limiter.Reserve()`, producing 20 reservations stacking up; with it, 20 signals collapse to 1 flusher schedule.
- **Don't hand-roll a token bucket** — `golang.org/x/time/rate` is battle-tested and handles token-overflow, burst accounting, and sub-nanosecond edge cases you will get wrong.
- **Reading body twice without re-wrapping** — classic middleware bug. `io.ReadAll` drains the `io.ReadCloser`; downstream reads get 0 bytes.
- **Unbounded `sync.Map` without cleanup** — memory leak. D-16 mandates the 60s ticker.
- **`time.Timer.Stop()` without callback idempotency** — Stop() returning false does NOT prevent an already-scheduled callback from running. Make the callback idempotent (our `pending.Swap(nil)` pattern).
- **`preview` routed through Coalescer** — would throttle user-initiated preview. D-22 bypass path is correct.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Token bucket rate limiter | custom `time.Ticker` + counter | `golang.org/x/time/rate.Limiter` | Edge cases around burst accounting, reservation cancellation, negative-tokens handling are handled; multi-year battle-tested at Google + Cloudflare + everywhere |
| Atomic pointer slot | `sync.Mutex` wrapping a `*T` | `atomic.Pointer[T]` | Go 1.19+ stdlib, sequentially consistent, zero contention |
| 64-bit content hash | XOR + shift homebrew | `hash/fnv.New64a` | Deterministic across Go versions + process restarts; well-distributed for short strings |
| Concurrent map with TTL | `map` + `sync.RWMutex` + expiry field in values | `sync.Map` + `time.Ticker` cleanup | Less code; `server.go:234` precedent; fewer lock shapes to reason about |
| Non-blocking deferred call | `go func() { time.Sleep(d); f() }()` | `time.AfterFunc(d, f)` | Cancellable via `Stop()`; doesn't leak goroutines on shutdown if `Stop()` is called |
| Virtual time for tests | inject `func() time.Time` interface, mock every call | `testing/synctest.Test` bubbles (Go 1.25 GA) | Zero production-code change; virtualizes `time.Now`/Sleep/AfterFunc/Ticker AUTOMATICALLY including inside `rate.Limiter` |

**Key insight:** In Go, the stdlib + `golang.org/x/*` sub-repos cover 100% of this phase's primitives. Any "I'll just write 20 lines of my own" urge is almost certainly wrong: 20 lines hides 50 edge cases that the vetted implementations already got right.

## Runtime State Inventory

**Not applicable.** Phase 8 is a pure code-and-binary refactor — no string renames, no database migrations, no OS-registered state changes, no secret rotations.

- **Stored data:** None. Coalescer state is in-memory only; reset on daemon restart.
- **Live service config:** None. Discord IPC connection is re-established on every daemon start.
- **OS-registered state:** None. `pm2` / `launchd` / `Task Scheduler` registrations are unchanged (scripts unchanged).
- **Secrets/env vars:** None. No new env vars added.
- **Build artifacts:** None. `go build` produces fresh binary; no stale package-info bleeds through.

Verified by grep: no rename tasks in `.planning/phases/08-*/08-CONTEXT.md`; no string-replacement work in D-01..D-34.

## Common Pitfalls

### Pitfall 1: `r.OK()==true` but `r.Delay()` is huge

**What goes wrong:** Code gates on `r.OK()`, assumes reservation will fire "soon," blocks the goroutine waiting for a delay that's actually seconds.

**Why it happens:** `Reservation.OK()` only means "the reservation was successfully booked," not "token available immediately." `[VERIFIED: gh issue view 57429 --repo golang/go]`

**How to avoid:** D-07's pattern: `if r.Delay() == 0 { fire } else { schedule AfterFunc(r.Delay()) }`. Never block on `r.Delay()` synchronously in the consumer goroutine.

**Warning signs:** Mysterious 15s+ hangs in the Coalescer. Consumer goroutine pegged to one goroutine. Fix: grep for `time.Sleep(r.Delay())` — it is the wrong pattern for us.

### Pitfall 2: Multi-goroutine `Reserve()` → bucket over-allowance

**What goes wrong:** With N goroutines calling `Reserve()` concurrently, the token bucket can allow > rate (confirmed reproduction in Issue #65508).

**Why it happens:** `lim.advance(t)` can be invoked with a past `t` when goroutines interleave, causing the last-refill time to rewind. `[VERIFIED: gh issue view 65508 --repo golang/go]`

**How to avoid:** Keep Reserve() calls serialized via a single consumer goroutine. Our Coalescer design already does this (only the Coalescer goroutine calls `limiter.Reserve()`). External producers push to `updateChan` — a signal bus, not a bucket interaction.

**Warning signs:** Observed rate exceeds configured rate in live logs. Not applicable here — single consumer goroutine.

### Pitfall 3: `time.Timer.Stop()` doesn't actually stop already-scheduled callback

**What goes wrong:** Code assumes `timer.Stop(); timer = time.AfterFunc(...)` replaces the callback. The old callback fires anyway and executes stale logic.

**Why it happens:** Per Go docs (pkg.go.dev/time §Timer.Stop), if the timer has already expired and the function has been started on a new goroutine, `Stop()` returns false — and cannot cancel the callback.

**How to avoid:** Make the callback itself idempotent. Our `pending.Swap(nil)` pattern: second caller gets nil, early-returns. Also seen in `main.go:433 graceCtx.Err()` check.

**Warning signs:** Flaky test where "the last update doesn't land." Double-SetActivity logs. Fix: `grep 'func.*flush' coalescer/` — ensure every timer-target function re-checks state before acting.

### Pitfall 4: sync.Map dirty-map bloat under store-delete churn

**What goes wrong:** Memory grows unboundedly despite TTL cleanup, because sync.Map promotes entries from dirty→read only via `miss` counter incremented on `Load()`. Pure store+delete cycles bypass promotion. `[CITED: victoriametrics.com/blog/go-sync-map/]`

**Why it happens:** sync.Map is optimized for read-heavy patterns. Heavy write-delete without reads defeats the optimization.

**How to avoid:** Our access pattern IS read-heavy: every hook request does a Load before (maybe) Store + (maybe) Delete via cleanup. Promotion fires normally. Bloat is not a risk.

**Warning signs:** RSS monotonically growing during dedup-heavy load. Mitigation: replace `sync.Map` with `map + RWMutex` if observed.

### Pitfall 5: `io.ReadAll` + `io.NopCloser(bytes.NewReader)` drops ContentLength

**What goes wrong:** After re-wrapping, `r.ContentLength` stays set BUT `r.Body` is a `NopCloser(Reader)` which doesn't implement `io.Seeker`/length-query. Downstream code that inspects `r.ContentLength` and `r.Body` together for consistency may fail.

**Why it happens:** Go's HTTP transfer logic in some paths unwraps `NopCloser` for `*os.File` optimization. `NopCloser(bytes.Reader)` is not unwrapped.

**How to avoid:** For INBOUND-server middleware (our case), downstream handlers do `json.NewDecoder(r.Body).Decode(&payload)` which doesn't care about `ContentLength`. Verified via grep of all 15 `handle*` functions in `server.go` — none read `r.ContentLength`. Safe.

**Warning signs:** Silent parsing failures after middleware installation. Mitigation: add integration test `server/hook_dedup_test.go` that POSTs real hook JSON through `httptest.Server + middleware` and asserts the downstream handler decoded the body successfully.

### Pitfall 6: `sync.Mutex` operations are NOT durably-blocking in synctest bubbles

**What goes wrong:** Test using `synctest.Test` with code that internally uses a mutex can hang if the test expects `synctest.Wait` to unblock the mutex holder.

**Why it happens:** `synctest` only virtualizes TIME; mutex acquisition is NOT considered "durably blocking." `[CITED: go.dev/blog/synctest]`

**How to avoid:** The Coalescer uses `atomic.Pointer` + `atomic.Uint64` + channels + timers — ALL of which are synctest-compatible. No mutexes in the hot path. The one mutex in our code is `cfgMu` / `presetMu` which live OUTSIDE the bubble (in main goroutine).

**Warning signs:** Test hangs in `synctest.Wait()`. Mitigation: verify the code under test doesn't hold a mutex across what would otherwise be a `time.Sleep` / `time.Ticker.C` receive.

### Pitfall 7: StartTime in hash → every update looks different

**What goes wrong:** Including `*time.Time` StartTime in the hash produces a fresh hash every time even if the Activity content is logically identical.

**Why it happens:** Pointer identity vs value equality trap; OR, time.Time.UnixNano() changes if StartTime is re-computed with monotonic-wall offset even for the same logical instant.

**How to avoid:** D-09 excludes StartTime. Verified via `resolver_test.go:333-339` — StartTime is deterministic-per-session (earliest session's StartedAt). Including it would mean: "hash differs if any other session transitions its StartedAt millisecond" — false positives.

**Warning signs:** Content-hash skip rate near 0% in live logs. Mitigation: re-confirm `hashActivity` does NOT include `a.StartTime`.

### Pitfall 8: `time.AfterFunc` callback inside a closed context → panic

**What goes wrong:** Shutdown fires → Coalescer goroutine returns. A scheduled `time.AfterFunc(delay, flushPending)` eventually fires. It tries to call `discordClient.SetActivity` on a closed conn.

**Why it happens:** `SetActivity` returns error on closed conn (`main.go:349 discordClient.SetActivity(discord.Activity{})` handles this), but our Coalescer's flush tries it too.

**How to avoid:** The flush function MUST check `ctx.Err() != nil` at entry (cheap — ctx carries an atomic check). Also, D-33 says "SetActivity error → debug log + discard pending + return" — so even if it races, the error is benign.

**Warning signs:** `discord SetActivity failed: not connected` right after shutdown. Already handled via D-33.

## Code Examples

### Coalescer.Run loop skeleton

```go
// Source: synthesized from D-17..D-28 + Phase 6 D-09 defer-recover + main.go:409 loop precedent
package coalescer

import (
    "context"
    "log/slog"
    "sync"
    "sync/atomic"
    "time"

    "golang.org/x/time/rate"

    "github.com/StrainReviews/dsrcode/config"
    "github.com/StrainReviews/dsrcode/discord"
    "github.com/StrainReviews/dsrcode/preset"
    "github.com/StrainReviews/dsrcode/resolver"
    "github.com/StrainReviews/dsrcode/session"
)

const (
    DiscordRateInterval = 4 * time.Second
    DiscordRateBurst    = 2
    DebounceDelay       = 100 * time.Millisecond
    SummaryInterval     = 60 * time.Second
)

type Coalescer struct {
    updateChan  <-chan struct{}
    registry    *session.SessionRegistry
    presetMu    *sync.RWMutex
    currentPreset **preset.MessagePreset
    discordClient *discord.Client
    detailGetter func() config.DisplayDetail

    limiter *rate.Limiter
    debounce *time.Timer // 100ms front-aggregator
    flushTimer *time.Timer // bucket-reservation flusher

    pending       atomic.Pointer[discord.Activity]
    lastSentHash  atomic.Uint64

    sent      atomic.Int64
    skipRate  atomic.Int64
    skipHash  atomic.Int64
    deduped   atomic.Int64 // incremented by hook-dedup middleware via a sink
}

func New(...) *Coalescer { ... }

func (c *Coalescer) Run(ctx context.Context) {
    defer func() {
        if r := recover(); r != nil {
            slog.Error("coalescer panic", "panic", r)
        }
        c.Shutdown()
    }()
    summary := time.NewTicker(SummaryInterval)
    defer summary.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-c.updateChan:
            c.onSignal()
        case <-summary.C:
            c.emitSummary()
        }
    }
}

func (c *Coalescer) onSignal() {
    // 100ms debounce — D-19
    if c.debounce != nil {
        c.debounce.Stop()
    }
    c.debounce = time.AfterFunc(DebounceDelay, c.resolveAndEnqueue)
}

func (c *Coalescer) resolveAndEnqueue() {
    sessions := c.registry.GetAllSessions()
    c.presetMu.RLock()
    p := *c.currentPreset
    c.presetMu.RUnlock()

    a := resolver.ResolvePresence(sessions, p, c.detailGetter(), time.Now())
    if a == nil {
        return
    }

    // Content-hash check — D-08..D-11
    h := hashActivity(a)
    if h == c.lastSentHash.Load() {
        c.skipHash.Add(1)
        slog.Debug("presence update skipped", "reason", "content_hash")
        return
    }

    c.pending.Store(a)
    c.schedule()
}

func (c *Coalescer) schedule() {
    if c.flushTimer != nil {
        c.flushTimer.Stop()
    }
    r := c.limiter.Reserve()
    if !r.OK() {
        slog.Warn("coalescer reservation !ok", "burst", c.limiter.Burst())
        return
    }
    if d := r.Delay(); d == 0 {
        c.flushPending() // fast path
    } else {
        c.skipRate.Add(1) // approximate: a flush was deferred, not dropped
        c.flushTimer = time.AfterFunc(d, c.flushPending)
    }
}

func (c *Coalescer) flushPending() {
    a := c.pending.Swap(nil)
    if a == nil {
        return // already flushed
    }
    if err := c.discordClient.SetActivity(*a); err != nil {
        slog.Debug("discord SetActivity failed", "error", err)
        // D-33: discard pending (already Swap'd), don't retry
        return
    }
    c.lastSentHash.Store(hashActivity(a))
    c.sent.Add(1)
}

func (c *Coalescer) emitSummary() {
    sent := c.sent.Swap(0)
    sr := c.skipRate.Swap(0)
    sh := c.skipHash.Swap(0)
    dd := c.deduped.Swap(0)
    if sent+sr+sh+dd == 0 {
        return // D-27 Claude's Discretion: skip when idle
    }
    slog.Info("coalescer status",
        "sent", sent,
        "skipped_rate", sr,
        "skipped_hash", sh,
        "deduped", dd,
    )
}

func (c *Coalescer) Shutdown() {
    c.pending.Store(nil)
    if c.debounce != nil {
        c.debounce.Stop()
    }
    if c.flushTimer != nil {
        c.flushTimer.Stop()
    }
}
```

### Test using `testing/synctest`

```go
// Source: go.dev/blog/synctest + go.dev/doc/go1.25 §synctest
package coalescer_test

import (
    "context"
    "testing"
    "testing/synctest"
    "time"
    // ...
)

func TestCoalescer_DropOnSkipFixed(t *testing.T) {
    synctest.Test(t, func(t *testing.T) {
        ctx, cancel := context.WithCancel(context.Background())
        fakeDiscord := &mockDiscord{calls: make(chan discord.Activity, 100)}
        c := coalescer.New(..., fakeDiscord, ...)

        updateCh := make(chan struct{}, 1)
        c.SetUpdateChan(updateCh) // or via NewCoalescer option
        go c.Run(ctx)

        // Fire 10 signals in rapid succession — the old code drops 9, ours keeps last
        for i := 0; i < 10; i++ {
            updateCh <- struct{}{}
        }
        synctest.Wait() // block until all goroutines quiesce (debounce fires, bucket reserves, flush fires)

        // Advance virtual time past 4s so at least one coalesced flush lands
        time.Sleep(5 * time.Second)
        synctest.Wait()

        // Assert: exactly 1 SetActivity call landed (the LATEST state), not 0, not 10
        select {
        case got := <-fakeDiscord.calls:
            if got.Details == "" { t.Errorf("expected Activity, got empty") }
        default:
            t.Fatal("expected at least 1 SetActivity call")
        }
        cancel()
    })
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Drop-on-skip with `if time.Since(lastUpdate) < rateLimit { return }` | Coalescing with `atomic.Pointer` pending + `rate.Limiter.Reserve().Delay()` | Phase 8 (this phase) | ~70% skip rate → <5%; latest state always reaches Discord within ~4s |
| `sync.Mutex` guarding `lastUpdate time.Time` | `atomic.Uint64 lastSentHash` + `atomic.Pointer[Activity] pending` | Phase 8 | No mutex contention on hot path; race eliminated |
| `func() time.Time` clock injection for tests | `testing/synctest.Test(...)` bubbles | Go 1.25 GA (Aug 2025) `[CITED: go.dev/doc/go1.25]` | Zero production-code change for deterministic testing |
| `GOEXPERIMENT=synctest` required in Go 1.24 | Stable and GA in Go 1.25; flag removed in Go 1.26 | Go 1.25 (2025-08) | No build-flag needed |
| Discord doc-stated 1 update / 15s | Empirical 4s/burst 2 in practice; modern docs quiet on the limit | Discord stopped documenting the number ~2020. Live evidence cited in D-05 | Use empirical 4s/burst 2 as safe middle ground |
| JSON hashing via `json.Marshal` | Field-ordered `hasher.Write([]byte(field))` | Phase 8 D-11 | Stable across Go versions; no map-ordering surprises |
| `map + sync.RWMutex` cache | `sync.Map` for read-heavy TTL caches | Go 1.9+ | Less code, matches `server.go:234` precedent |
| `context.AfterFunc` (Go 1.21+) | Still use `time.AfterFunc` for duration-based scheduling | — | `context.AfterFunc` is context-cancel-driven, not time-driven; wrong primitive for our flusher |
| `sync.Mutex` across goroutine boundaries in tests | `testing/synctest` — mutexes NOT durably-blocking inside bubbles | Go 1.25 | Our atomic-only design is synctest-friendly by construction |

**Deprecated/outdated:**

- **`hash/maphash` for content hashing:** Per-process seed → non-deterministic across restarts. Not appropriate for our use case (we want observability: a reader tailing debug logs should see identical hashes for identical Activities across daemon restarts).
- **`func() time.Time` clock injection pattern:** Superseded by `testing/synctest` in Go 1.25+. Old projects keep it for backward compat; new code should prefer synctest.
- **`github.com/benbjohnson/clock` library:** Superseded by synctest. Still useful for projects stuck on pre-Go-1.24 but obsolete for Go 1.25+.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Claude Code HTTP client always sends `Content-Length` (no chunked transfer-encoding from Node.js/undici) | Pattern 7 — Body-Preserving Middleware | If chunked, `io.ReadAll` still works; `ContentLength` becomes 0 but no downstream handler reads it (verified) — SAFE even if wrong |
| A2 | `probe` struct partial JSON decode tolerates Windows-unescaped-backslash bodies | Pattern 5 | If decode fails on probe, `probe.SessionID`/`ToolName` are empty — hash still fits over raw body, dedup still works. SAFE even if fails |
| A3 | Go module proxy cache is authoritative for `golang.org/x/time` versions | Version verification | If staged behind a corporate proxy, version list may lag. `go get @latest` bypasses local cache if needed |
| A4 | `testing/synctest` correctly virtualizes `rate.Limiter`'s internal `time.Now()` calls | Pattern 1 testing | Not explicitly confirmed in go.dev/blog/synctest. **MUST test this in practice**; if synctest's virtual time doesn't reach into `rate.Limiter`, fall back to injected clock |
| A5 | FNV-1a offset basis + prime are fixed stdlib constants (no env-dependent variation) | Pattern 4 | Verified via `hash/fnv` source is generally safe, but formally the guarantee is implicit. `Sum64()` always produces identical bits for identical byte streams |
| A6 | `sync.Map` read-heavy access pattern in our dedup avoids dirty-map bloat | Pattern 6 gotcha | Bloat would only manifest under heavy store+delete without Loads. Our Load-first-then-Store pattern is safe. If observed: swap to `map + RWMutex`. Low risk |

**Empty assumption cells (explicitly verified):** All core Go stdlib method signatures (atomic, fnv, sync.Map, time) were cross-verified against pkg.go.dev directly. Discord rate-limit numbers are empirical (D-CONTEXT evidence). `testing/synctest` stability is verified at source (go.dev/doc/go1.25).

## Open Questions

1. **Does `testing/synctest` correctly virtualize time inside `golang.org/x/time/rate.Limiter`?**
   - What we know: synctest virtualizes `time.Now`, `Sleep`, `Timer`, `Ticker`, `AfterFunc` within the bubble. Rate limiter internally calls `time.Now()` and schedules via those primitives.
   - What's unclear: No explicit authoritative source confirms rate.Limiter works cleanly inside a synctest bubble. The Go 1.25 release notes mention synctest but not its interaction with external packages.
   - **Recommendation:** In the FIRST coalescer test, write a trivial 3-line probe (`lim := rate.NewLimiter(rate.Every(time.Second), 1); r := lim.Reserve(); synctest.Wait(); assert r.Delay() == 0` after 1s) to confirm. If it fails, fall back to clock-injection interface.

2. **Should `/preview` bypass the hook-dedup middleware?**
   - What we know: D-14 applies the dedup middleware only to `/hooks/*`; `/preview` lives at the top level — so no direct conflict.
   - What's unclear: N/A; this is already answered by D-14's path filter.
   - **Recommendation:** Planner confirms `strings.HasPrefix(r.URL.Path, "/hooks/")` is the final filter; `/preview`, `/status`, `/health`, `/statusline`, `/config`, `/sessions`, `/presets` all bypass. Verified via `server.go:323-347`.

3. **Is the 100ms debounce actually reducing load on the limiter?**
   - What we know: Empirically, burst of 5 signals arrives within 50ms during MCP activity (per Phase 8 trigger logs). With debounce, 5 signals → 1 `limiter.Reserve()` call. Without, 5 Reserve calls in rapid succession on the SAME bucket from the SAME goroutine — bucket accounting is still correct (single-goroutine), but we'd generate 5 wasted reservations that all get Stop()d.
   - What's unclear: Whether saving 4 `Reserve()` calls per burst is measurable. Likely insignificant perf-wise, but the debounce layer also coalesces 5 resolver runs into 1 — and resolver.ResolvePresence has real cost (session iteration + preset lookup).
   - **Recommendation:** D-19 keeps it. No profiling needed in this phase.

4. **Should the coalescer summary log be at DEBUG or INFO?**
   - What we know: D-27 says INFO every 60s. If the daemon runs on a quiet session for hours, this could be noisy.
   - What's unclear: Claude's Discretion allows skip-when-idle. Reasonable.
   - **Recommendation:** Skip when all counters are zero. Phase 8 CONTEXT §Claude's Discretion already suggests this.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go 1.25 | All code | ✓ | 1.25.0 (per go.mod) | — |
| `golang.org/x/time/rate` | RLC-01 | ✓ (new add) | v0.15.0 | — |
| `hash/fnv` | RLC-04 | ✓ (stdlib) | Go 1.25 | — |
| `sync/atomic` generics | RLC-02, RLC-06, RLC-11 | ✓ (stdlib) | Go 1.19+ | — |
| `testing/synctest` | RLC-15 tests | ✓ (stdlib GA) | Go 1.25 | Clock-injection interface if synctest+rate.Limiter incompatible (A4) |
| `go test -race` | CI | ✓ (already in .github/workflows/test.yml:28) | — | — |
| `bash` for verify.sh | RLC-16 smoke | ✓ (Unix/Git Bash) | — | `.bat` wrapper if needed — Phase 7 precedent |
| `powershell` ≥5.1 for verify.ps1 | RLC-16 smoke | ✓ | — | — |
| `curl` for T2/T3/T4 smoke | RLC-16 | ✓ (all platforms) | — | `Invoke-WebRequest` in PS1 |
| `discord.exe` running for T5 | Manual verify | ⚠ user-provided | — | skip live-IPC tests, rely on unit tests |

**Missing dependencies with no fallback:** None.

**Missing dependencies with fallback:** `testing/synctest`+`rate.Limiter` compatibility (assumption A4) — fallback is clock-injection interface, a known-working pattern.

## Validation Architecture

**Phase 8 nyquist_validation is enabled** (`.planning/config.json §workflow.nyquist_validation: true`).

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + Go 1.25 `testing/synctest` + `httptest` |
| Config file | None — idiomatic `_test.go` files co-located with source |
| Quick run command | `go test ./coalescer/... ./server/... -run '<specific>' -race -count=1` |
| Full suite command | `go test ./... -race -count=1 -v` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| RLC-01 | Token bucket allows 2 instant + 1 every 4s | unit (synctest) | `go test ./coalescer -run TestCoalescer_TokenBucketRate -race` | ❌ Wave 0 |
| RLC-02 | atomic.Pointer Store replaces latest; Swap clears | unit | `go test ./coalescer -run TestCoalescer_PendingSlot -race` | ❌ Wave 0 |
| RLC-03 | AfterFunc-scheduled flush fires exactly once after Delay | unit (synctest) | `go test ./coalescer -run TestCoalescer_FlushSchedule -race` | ❌ Wave 0 |
| RLC-04 | FNV64a hash is deterministic; StartTime excluded | unit | `go test ./coalescer -run TestHashActivity_Deterministic -race` | ❌ Wave 0 |
| RLC-05 | Same content → same hash (StartTime varies freely) | unit | `go test ./coalescer -run TestHashActivity_ExcludesStartTime -race` | ❌ Wave 0 |
| RLC-06 | atomic.Uint64 lastSentHash load/store correct | unit | `go test ./coalescer -run TestCoalescer_HashSkip -race` | ❌ Wave 0 |
| RLC-07 | Dedup middleware wraps /hooks/ only; others pass through | integration | `go test ./server -run TestHookDedup_PathFilter -race` | ❌ Wave 0 |
| RLC-08 | Dedup key includes route+session+tool+body | integration | `go test ./server -run TestHookDedup_KeyComposition -race` | ❌ Wave 0 |
| RLC-09 | TTL 500ms; cleanup ticker evicts after 60s | integration (synctest) | `go test ./server -run TestHookDedup_TTLCleanup -race` | ❌ Wave 0 |
| RLC-10 | Middleware preserves body for downstream JSON decode | integration | `go test ./server -run TestHookDedup_BodyPreserved -race` | ❌ Wave 0 |
| RLC-11 | 60s summary log fires when counters non-zero, skipped when zero | unit (synctest) | `go test ./coalescer -run TestCoalescer_SummaryIdleSkip -race` | ❌ Wave 0 |
| RLC-12 | Shutdown clears pending + stops timers | unit | `go test ./coalescer -run TestCoalescer_ShutdownIdempotent -race` | ❌ Wave 0 |
| RLC-13 | Disconnect discards pending; reconnect triggers fresh | unit | `go test ./coalescer -run TestCoalescer_ReconnectDiscard -race` | ❌ Wave 0 |
| RLC-14 | /preview bypasses Coalescer (direct SetActivity) | integration | `go test ./server -run TestPreview_BypassesCoalescer` | ❌ Wave 0 |
| RLC-15 | Test framework works with synctest + rate.Limiter | probe | `go test ./coalescer -run TestSynctest_RateLimiterProbe` | ❌ Wave 0 |
| RLC-16 | verify.sh / verify.ps1 T1..T6 cross-platform smoke | manual | `bash scripts/phase-08/verify.sh` / `.\scripts\phase-08\verify.ps1` | ❌ Wave 0 |
| RLC-17 | CHANGELOG v4.2.0 entry present + Keep-a-Changelog 1.1.0 compliant | doc-lint | `grep '## \[4.2.0\]' CHANGELOG.md \|\| exit 1` | ❌ Wave 0 |

### Observable Invariants (Nyquist Dimension 8)

Phase 8 defines these testable invariants; the VALIDATION.md file generated by the planner MUST cover them all:

- **I1 — Token-Bucket Coalescing:** "During a burst of N>2 updateChan signals inside 4s, exactly ≤2 `SetActivity` calls fire." Testable via mock-Discord capture channel in synctest bubble + live `grep 'discord SetActivity failed\|presence updated' dsrcode.log | head -100`.
- **I2 — Content-Hash Skip:** "Identical-content burst of N signals → exactly 1 `SetActivity` call." Testable via mock-Discord + hash-counter assertion (`skipped_hash == N-1` after 60s summary).
- **I3 — Hook-Dedup:** "Duplicate POST /hooks/pre-tool-use with identical body within 500ms → exactly 1 downstream handler invocation." Testable via `httptest.Server` + handler-call counter; also observable via `grep 'hook deduped' dsrcode.log`.
- **I4 — Disconnect/Reconnect:** "IPC close → pending cleared; reconnect → fresh Activity computed and pushed." Testable via mock Conn.Close + observation of `updateChan` post-reconnect.
- **I5 — Shutdown Goroutine Leak:** "After `cancel()` + `Shutdown()`, no coalescer-owned goroutines remain running." Testable via `runtime.NumGoroutine()` before/after or `github.com/uber-go/goleak` in a finalizer.
- **I6 — Race-Free:** "`go test -race ./...` passes, including under synctest bubbles." Non-negotiable CI gate — already enforced in `.github/workflows/test.yml:28`.
- **I7 — Lock-Free Hot Path:** "No `sync.Mutex.Lock` in the Coalescer flush path (Reserve→Delay→Swap→SetActivity)." Testable via code inspection + `go vet` + a custom linter rule (optional).
- **I8 — Body Preserved:** "Handler downstream of HookDedupMiddleware decodes the body identically to a request without the middleware." Testable via integration test comparing decoded `HookPayload` from both paths.

### Sampling Rate

- **Per task commit:** `go test ./coalescer ./server -race -count=1` (subset; ~5s)
- **Per wave merge:** `go test ./... -race -count=1 -v` (full; ~20s)
- **Phase gate:** Full suite green + `scripts/phase-08/verify.sh` T1..T6 before `/gsd-verify-work`.

### Wave 0 Gaps

- [ ] `coalescer/coalescer.go` — entire package is new
- [ ] `coalescer/hash.go` — hash helper (could inline in coalescer.go if preferred)
- [ ] `coalescer/coalescer_test.go` — all tests (RLC-01..RLC-06, RLC-11..RLC-13, RLC-15)
- [ ] `server/hook_dedup.go` — HookDedupMiddleware
- [ ] `server/hook_dedup_test.go` — tests for RLC-07..RLC-10, RLC-14
- [ ] `main.go` integration — swap `go presenceDebouncer(...)` → `go coalescer.Run(ctx)` + add `.Shutdown()` to D-07 sequence; wrap `s.Handler()` via middleware
- [ ] `scripts/phase-08/verify.sh` + `verify.ps1` — smoke tests for RLC-16
- [ ] `CHANGELOG.md` v4.2.0 entry — RLC-17
- [ ] Framework install: **none needed** — stdlib + one dep (`golang.org/x/time@v0.15.0`). `go.mod` tidy will add the require.

*(Deliberately no existing tests to update — `presenceDebouncer` has no unit tests; replacing it cleanly has no test-deletion risk.)*

## Security Domain

`security_enforcement` is NOT explicitly set to `false` in `.planning/config.json` — so this section is included.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | NO | No new auth; hooks are localhost-only (`127.0.0.1` bind per DIST-55) |
| V3 Session Management | NO | No session state; hook requests are stateless |
| V4 Access Control | NO | No new access decisions |
| V5 Input Validation | yes | `json.Unmarshal` of hook payload — already done in existing handlers; middleware adds `io.ReadAll` bounds |
| V6 Cryptography | NO | FNV is NOT cryptographic (content-identity hash only); no new crypto |
| V7 Error Handling | yes | D-33 `SetActivity` error → debug log + discard (no error leak to Discord response); hook-dedup silent 200 pattern |
| V11 Business Logic | yes | Token bucket enforces rate limit; no timing side-channel risk since Discord IPC is local |
| V12 File / Resource | yes | Body buffer size via `io.ReadAll` — unbounded. Discussed below |

### Known Threat Patterns for the Stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Unbounded `io.ReadAll(r.Body)` → memory DoS | Denial of Service | Use `http.MaxBytesReader(r.Body, limit)` with a sane cap. Hook payloads are <10KB; a 64KB cap is generous and breaks no legitimate use. **RECOMMENDED for defence-in-depth**, though the loopback-only attack surface is narrow. |
| CVE-2025-22871 (net/http chunked smuggling) | Tampering | Fixed in Go 1.23.8+ / 1.24.2+ / 1.25. We use Go 1.25 → **NOT AFFECTED**. `[CITED: nvd.nist.gov/vuln/detail/CVE-2025-22871]` |
| `sync.Map` unbounded growth → memory DoS | Denial of Service | 60s ticker cleanup + 500ms TTL bounds retained entries to `rps × 60s` approx. For 100 rps → 6000 entries → ~2MB. Acceptable. |
| Reservation exhaustion → bucket stuck | Denial of Service | `rate.Limiter` handles bucket drain correctly; worst case: flusher is delayed but never permanently stuck. Verified via Issue #65508 analysis — our single-goroutine design avoids the bug. |
| FNV-1a collision → false dedup | Tampering | 64-bit FNV has ~2^32 birthday collision space; hook routes × session-ids × tool-names × bodies collision probability per 500ms window is effectively 0. If a collision did happen, the worst outcome is "one hook request legitimately dropped" — the system auto-recovers on the next distinct signal. Acceptable risk for non-adversarial localhost input. |
| Localhost-only bind → still reachable by other local users | Info Disclosure | `net.JoinHostPort("127.0.0.1", port)` is the existing posture (server.go:1529). Users on the same host can reach it; this is a known and accepted limitation (Phase 5 DIST-55). No change in Phase 8. |

**Security recommendation for the planner:** Add `http.MaxBytesReader` to the hook-dedup middleware with a 64KB cap. Hook payloads are small; this closes a theoretical memory-DoS vector (even local-only) at zero cost. Optional — not locked by D-CONTEXT — but cheap defence-in-depth.

## CONTEXT.md Discrepancies Flagged

| # | Discrepancy | CONTEXT.md Says | Reality | Recommendation |
|---|-------------|-----------------|---------|----------------|
| **D1** | CI already has `-race` | D-32: "`.github/workflows/test.yml` MUST add `-race` flag. Current workflow lacks `-race` (verified via grep)" | **Verified via file read:** `.github/workflows/test.yml:28` → `go test ./... -count=1 -race -v`. The `-race` flag IS ALREADY PRESENT. | Update plan 08-04 to NOTE this is already done, not an action item. Could be folded into "verify CI unchanged" task, or removed entirely. |
| **D2** | `testing/synctest` availability | D-29: "Unit tests (`coalescer_test.go`): fake time via injected clock" | **Go 1.25 `testing/synctest` is STABLE AND GA** — no GOEXPERIMENT needed. Superior to injected clocks: zero production-code change. | Planner should UPGRADE the testing recommendation to `testing/synctest.Test` bubbles. Leave `ClockFunc` injection as an interface fallback if A4 (synctest+rate.Limiter) proves incompatible. |
| **D3** | Null-byte separator | D-12: "Null-byte separator prevents injection ambiguity" | `\x00` (NUL) can legitimately appear in UTF-8-encoded tool_input bodies (rare but possible). ASCII 0x1F (Unit Separator) is the dedicated-to-field-sep control char that effectively never appears in content. | Minor nit: planner MAY use `0x1F` instead of `0x00`. Either works for our use case. `0x00` is fine in practice for the short key `route\x00session\x00tool\x00body`. |
| **D4** | Hook-dedup counter ownership | D-26/D-28: "All counter increments and ticker live inside the Coalescer struct" AND "deduped counter". | The hook-dedup middleware lives in the `server` package. Its deduped counter can't literally live inside the `coalescer.Coalescer` struct without cross-package coupling. | Clean fix: HookDedupMiddleware has its OWN `atomic.Int64 deduped` counter. Expose via `Middleware.DedupCount()` getter. Coalescer's 60s summary can ALSO read this via a dependency-injection func `func() int64` passed at New(). This preserves the spirit of D-28 (single summary log) while respecting package boundaries. |

**None of the discrepancies contradict core D-CONTEXT decisions**; they're implementation-detail refinements (D1 already done; D2 superior alternative exists; D3 aesthetic; D4 package-boundary pragmatics).

## Sources

### Primary (HIGH confidence)

- **Go stdlib docs via WebFetch:**
  - https://pkg.go.dev/sync/atomic — atomic.Pointer[T], Uint64, Int64 signatures + Go memory-model guarantee
  - https://pkg.go.dev/hash/fnv — New64a() determinism + hash.Hash64 interface
  - https://pkg.go.dev/testing/synctest — Test(), Wait() signatures + time virtualization
  - https://pkg.go.dev/golang.org/x/time/rate — NewLimiter, Reserve, Reservation.Delay/OK/Cancel, burst semantics
  - https://go.dev/doc/go1.25 — synctest GA confirmation (Aug 2025 release)
  - https://go.dev/blog/synctest — mutex-not-durably-blocking caveat
- **Authoritative upstream issues (via `gh issue view`):**
  - `golang/go#57429` — OK()+Delay() gotcha
  - `golang/go#65508` — multi-goroutine Reserve() inaccuracy (IRRELEVANT to our single-goroutine design)
  - `golang/go#52584` — reservation-delay-after-not-ok quirk
  - `discord/discord-api-docs#668` — Discord drop-on-spam root-symptom corroboration
- **Go module proxy (canonical version source):**
  - `curl https://proxy.golang.org/golang.org/x/time/@v/list` → v0.15.0 latest (verified via `go get` cache populated 2026-04-16)
- **Project CONTEXT.md + source files (read first):** 08-CONTEXT.md (34 D-IDs), main.go, discord/client.go, server/server.go, resolver/resolver.go, config/watcher.go, .planning/config.json, go.mod, .github/workflows/test.yml, CHANGELOG.md

### Secondary (MEDIUM confidence)

- https://victoriametrics.com/blog/go-sync-map/ — sync.Map dirty-map bloat caveat (verified against Go stdlib source)
- https://pliutau.com/map-with-expiration-go/ — TTL map pattern (code pattern verified against Go idioms)
- https://appliedgo.net/spotlight/go-1.25-the-synctest-package/ — Go 1.25 synctest feature article (cross-ref with go.dev/doc/go1.25)
- https://oneuptime.com/blog/post/2026-01-30-go-thread-safe-cache/view — production TTL cache pattern (cross-ref with Go stdlib)
- https://rodaine.com/2017/05/x-files-time-rate-golang/ — historical intro to x/time/rate (older but still accurate on API)
- https://nvd.nist.gov/vuln/detail/CVE-2025-22871 — affected Go versions (cross-ref against Go 1.23.8/1.24.2 patch notes)

### Tertiary (LOW confidence, flagged)

- https://reintech.io/blog/guide-to-golang-x-time-package-advanced-time-operations — one-off article, not cross-verified
- https://blog.williamchong.cloud/code/2024/10/12/modifying-json-response-in-go.html — blog post illustrating body-preservation pattern

## Metadata

**Confidence breakdown:**

- **Standard stack (x/time/rate, atomic, fnv, sync.Map, synctest):** HIGH — all verified via pkg.go.dev + Go module proxy + go.dev/doc/go1.25.
- **Architecture patterns (Reserve().Delay(), atomic.Pointer.Swap, AfterFunc, FNV field-ordering, middleware body-preservation):** HIGH — all backed by authoritative docs + 2-3 cross-refs each.
- **Pitfalls (Issue #65508 multi-goroutine, Issue #57429 OK-vs-Delay, sync.Map bloat, Timer.Stop-not-cancel, synctest-mutex-caveat):** HIGH — each has a specific upstream issue or blog post cited.
- **Discord rate-limit empirical values:** MEDIUM — D-CONTEXT evidence + Issue #668 confirm the drop-on-spam symptom; the exact 4s/burst 2 is operational calibration, not spec.
- **A4 (synctest + rate.Limiter compatibility):** MEDIUM — logically should work (virtualized time is global in the bubble), but not explicitly confirmed. Flagged as a probe test in Wave 0.

**Research date:** 2026-04-16

**Valid until:** ~2026-05-16 (30 days). If Go 1.26 ships early, `testing/synctest` old-API may deprecate — re-verify in any follow-up phase.

---

*Phase: 08-presence-rate-limit-coalescer-stop-drop-on-skip-token-bucket*
*Research complete: 2026-04-16*
