# Phase 8: Presence Rate-Limit Coalescer — Stop Drop-on-Skip - Context

**Gathered:** 2026-04-16
**Status:** Ready for planning
**Trigger:** Live log evidence: ~70% "presence update skipped (rate limit)" rate during active MCP-heavy Claude Code session. Root cause `main.go:501-555 presenceDebouncer` — updates inside the 15s cooldown are silently DISCARDED (no pending buffer, no flush).

<domain>
## Phase Boundary

Replace the drop-on-skip `presenceDebouncer` with a **coalescing** rate-limiter so presence updates queued during a cooldown are flushed exactly once when the limiter permits, not discarded. Target v4.2.0 (minor, new internal behavior). Five concrete fixes, all inside `main.go`, `discord/`, and `server/` — no new top-level features, no new UI surfaces, no new hook types.

**In scope (the 5 fixes from ROADMAP):**
1. **Pending-state buffer + flusher** — latest pending Activity kept; flushed on next token
2. **Token bucket via `golang.org/x/time/rate`** — replaces hard 15s check (cadence 4s, burst 2)
3. **FNV-64 content-hash change detection** — skip SetActivity if hash identical to last successful push
4. **Hook-dedup middleware** — wraps mux, drops duplicate POST `/hooks/*` requests (logs show `pre-tool-use` fires 2x / 30-130ms)
5. **Race-free shared state via `atomic.Pointer[Activity]` + `atomic.Uint64`** — no mutex contention on hot path

**Out of scope (deferred):**
- Prometheus `/metrics` endpoint — observability phase (see Phase 7 §deferred precedent)
- Adaptive rate-limit (backoff on IPC errors)
- Config-driven per-session rate-limits
- Alternative IPC transports
- Cross-process coordination (single-daemon design stays)
- Migration away from `channel-of-cap-1 updateChan` signal bus

</domain>

<decisions>
## Implementation Decisions

### Scope & Release (Area D)
- **D-01:** Target tag **v4.2.0** — minor (token bucket + pending buffer are new behaviors, not hotfix). Follows Phase 7 precedent ("no new features in hotfix → v4.1.2"); this phase has new features → bump minor.
- **D-02:** All 5 fixes bundled in v4.2.0. No split. Fixes address one symptom (drop-on-skip + race + hook-spam) holistically.
- **D-03:** **No `/metrics` endpoint** in this phase. Observability strictly deferred like Phase 7 §deferred ("fixes first, observability later"). Keeps binary size clean (Phase 5 DIST-01 constraint).
- **D-04:** **4 plans** — 08-01 Coalescer core (pending-buffer + token-bucket + atomic state), 08-02 Content-hash change detection, 08-03 Hook-dedup middleware, 08-04 Release (version bump + CHANGELOG + manual verify script).

### Rate-Limit Configuration (Area A)
- **D-05:** **Hard-coded constants** in `main.go`, NOT config keys:
  ```go
  const (
      discordRateInterval = 4 * time.Second  // matches empirical Discord RPC ~5/20s
      discordRateBurst    = 2                // allows brief initial burst for first updates
  )
  ```
  Follows Phase 7 D-03 "no toggle when no benefit to tuning" pattern. Bump via hotfix if Discord Protocol changes.
- **D-06:** Use `golang.org/x/time/rate.NewLimiter(rate.Every(discordRateInterval), discordRateBurst)`. New dep `golang.org/x/time` added to `go.mod` (transitive dep `golang.org/x/sys` already present).
- **D-07:** **`Reserve().Delay()` pattern** for non-blocking token acquisition — NOT `Wait(ctx)` (would block goroutine) and NOT `Allow()` (non-deterministic for flusher scheduling). Flow: `r := limiter.Reserve()` → if `r.Delay() == 0`, fire immediately → else `time.AfterFunc(r.Delay(), flushPending)`.

### Content-Hash Change Detection (Area B)
- **D-08:** Hash algorithm: **`hash/fnv` `New64a()`** (FNV-1a, 64-bit). Deterministic across process restarts (unlike `hash/maphash` which has per-process seed). Stdlib, zero new deps.
- **D-09:** **Hash scope — all user-visible fields, StartTime excluded**:
  - INCLUDE: `Details`, `State`, `LargeImage`, `LargeText`, `SmallImage`, `SmallText`, `Buttons[].Label`, `Buttons[].URL`
  - EXCLUDE: `StartTime` — verified deterministic via `resolver_test.go:333-339` ("StartTime should be from earliest session"). Timer runs client-side in Discord; we do NOT want to repush identical payloads just because StartTime is re-computed equal. Code comment MUST explain this.
- **D-10:** **Storage: `atomic.Uint64 lastSentHash`**. Lock-free compare-and-skip on hot path. Solves the "race on lastUpdate" symptom directly — the entire state-transition is expressible via `atomic.Uint64.Load` + `Store` without mutex.
- **D-11:** Serialization MUST be deterministic. Approach: write each field's bytes to the hasher in a fixed order via `hasher.Write([]byte(activity.Details))` etc. — NO `json.Marshal` (map-field ordering could shift between Go versions).

### Hook Deduplication (Area C)
- **D-12:** **Dedup key = FNV-64a of `route | "\x00" | session_id | "\x00" | tool_name | "\x00" | body`**. Null-byte separator prevents injection ambiguity.
- **D-13:** **TTL 500ms**. Empirical 30-130ms duplicate spacing (Phase 8 trigger evidence) × 4 safety margin. No legitimate human rapid-retry produces an identical session+tool+body tuple within 500ms.
- **D-14:** **Placement = middleware wrapping `s.Handler()`**. Path filter `strings.HasPrefix(r.URL.Path, "/hooks/")` — applies to all 15 registered hook routes. Handler code stays dedup-agnostic. Non-hook routes (`/status`, `/preview`, `/config`, `/sessions`, `/health`, `/statusline`, `/presets`) pass through untouched.
- **D-15:** **Response on duplicate = HTTP 200 + `slog.Debug` "hook deduped"**. Silent success; Claude Code sees no difference. No `X-Dsrcode-Deduped` header (response not consumed anyway). No `204 No Content` (break consistency with existing hook-200s).
- **D-16:** **Storage: `sync.Map[string]time.Time`** (key → last-seen timestamp) + **background `time.Ticker(60s)` cleanup** that walks and deletes entries where `now - seen > ttl`. Pattern matches OneUptime in-memory TTL cache + matches Phase 6 `lastTranscriptRead sync.Map` precedent.

### Coalescer Architecture (Areas E, F, H)
- **D-17:** **Pending buffer: `atomic.Pointer[discord.Activity]`** (Go 1.19+). `Store(a)` replaces latest. `Swap(nil)` atomically flushes-and-clears. Lock-free hot path. Zero-value nil = no pending. This single atomic eliminates the race on `lastUpdate` from the current code.
- **D-18:** **Flusher timer: `time.AfterFunc(delay, flushPending)` single-shot**, replaced on each new pending-store. Flow: new pending → `timer.Stop()` (cancel prior) → `reservation := limiter.Reserve()` → `timer = time.AfterFunc(reservation.Delay(), flushPending)`. If `delay == 0`, flush inline (no timer). Minimal goroutines, exact latency.
- **D-19:** **100ms debounce LAYER BEFORE Coalescer** — keep current `debounceDelay = 100 * time.Millisecond` front-aggregator. Signal flow: `updateChan` → 100ms `AfterFunc` aggregation → resolver+hash → Coalescer (Reserve / AfterFunc / SetActivity). 100ms reduces rapid-burst load on Coalescer (5 signals in 50ms collapse to 1). Minimal-invasive change.

### Lifecycle (Areas G, M, P, Q, T, U)
- **D-20: Discord disconnect behavior** — On IPC error or disconnect: **discard pending** via `atomic.Pointer.Store(nil)`. `connectionLoop` at `main.go:489` on reconnect triggers `updateChan <- struct{}{}` so Coalescer recomputes a fresh Activity. No stale data pushed post-reconnect.
- **D-21: Preset hot-reload** — When `config/watcher.go` reloads preset and signals `updateChan`: Coalescer treats it identically to any other signal. Before new resolver run, **discard pending** (same as disconnect) so old-preset-message doesn't push. Consistent rule: "any state-source change → drop pending, recompute fresh".
- **D-22: Preview mode coexistence** — `/preview` endpoint continues to call `discordClient.SetActivity(...)` **directly, BYPASSING Coalescer**. Preview is time-bounded + user-initiated; must not be throttled. After preview ends, normal `onPreviewEnd` → `updateChan` signal → Coalescer resumes.
- **D-23: Shutdown behavior** — New `Coalescer.Shutdown()` method: (1) `atomic.Pointer.Store(nil)` clears pending, (2) `timer.Stop()` cancels flusher, (3) `cleanupTicker.Stop()` halts dedup GC. Called via `ctx.Done()` in the coalescer goroutine. `main.go` D-07 sequence (Phase 6.04) — `SetActivity(clearActivity)` BEFORE `discord.Close()` — remains **direct-to-client**, NOT through Coalescer. No rate-limit wait on shutdown path.
- **D-24: Cold-start** — No special first-signal path. 100ms debounce (D-19) applies universally; initial latency ~100ms is imperceptible. Token bucket starts with burst=2 full tokens (see D-25) so first two coalescer flushes are instant.
- **D-25: Initial token state** — `rate.NewLimiter` default: bucket **initially full** (2 tokens). First & second coalesced updates after daemon-start flush immediately. Important UX: "daemon just started, presence already on Discord".

### Observability (Area I)
- **D-26:** **DEBUG per-event** kept: `slog.Debug("presence update skipped", "reason", "rate_limit"|"content_hash")` and `slog.Debug("hook deduped", "route", r, "key", k)`.
- **D-27:** **Periodic INFO summary every 60s**:
  ```
  slog.Info("coalescer status",
      "sent", sentCount, "skipped_rate", rateSkip, "skipped_hash", hashSkip,
      "deduped", dedupCount, "pending_age_s", pendingAge)
  ```
  Counters are `atomic.Int64`. Summary ticker lives in the Coalescer goroutine; reset counters after each log. Gives ops-level visibility without spam.
- **D-28:** All counter increments and ticker live inside the Coalescer struct — no global state, no new package-level vars.

### Testing (Areas J, N)
- **D-29:** **Unit tests** (`coalescer_test.go`): fake time via injected clock, mock `discord.Client` (interface), cover: (a) pending coalescing, (b) content-hash skip, (c) token-bucket wait, (d) disconnect discards pending, (e) shutdown clears state.
- **D-30:** **Integration tests** in `server/server_test.go`: verify hook-dedup middleware drops exact-repeat POST within TTL, passes distinct bodies, cleans up after 60s.
- **D-31:** **Live verify script** `scripts/phase-08/verify.sh` + `verify.ps1` matching Phase 7 `verify.ps1` pattern. Minimum checks: T1 binary version, T2 /health, T3 burst-of-10 rapid updateChan signals arrive as ≤3 SetActivity calls, T4 identical-content burst arrives as 1 call, T5 duplicate POST `/hooks/pre-tool-use` within 500ms gets deduped (check log), T6 60s summary log appears.
- **D-32:** **CI workflow** `.github/workflows/test.yml` MUST add `-race` flag: `go test -race ./...`. Current workflow lacks `-race` (verified via grep). Mandatory — `atomic.Pointer` / `atomic.Uint64` / `sync.Map` correctness depends on race-free verification.

### Error Handling (Area K)
- **D-33:** `SetActivity` error → `slog.Debug("discord SetActivity failed", "error", err)` + discard pending + return. No retry, no explicit disconnect. Discord IPC is binary-state: either working (transient errors rare) or `connectionLoop` detects disconnect and reconnects. Matches current code behavior at `main.go:544-547`.

### Release Docs (Area O)
- **D-34: CHANGELOG v4.2.0 entry** follows Keep a Changelog 1.1.0 + Phase 6/7 structure:
  ```
  ## [4.2.0] - 2026-MM-DD
  ### Fixed
  - Presence updates no longer dropped during Discord rate-limit cooldown — latest update is coalesced and flushed when the limiter permits.
  - Race condition on presence-update shared state eliminated via atomic.Pointer.
  - Duplicate hook events (observed on PreToolUse firing 2x in 30-130ms) are now deduped server-side.
  ### Changed
  - Presence debouncer replaced with a token-bucket coalescer (4s cadence, burst 2) matching Discord RPC empirical limits.
  - FNV-64 content-hash now gates SetActivity calls — identical payloads are never resent.
  ### Added
  - Hook-dedup middleware wrapping /hooks/* routes, 500ms TTL.
  - Periodic 60s INFO summary with coalescer counters.
  ```
  Headline angle: **"Fix presence stalling during long MCP-heavy sessions"** — user-visible benefit front-loaded.

### Folded Todos
(No pending todos matched Phase 8 scope — `gsd-tools todo match-phase 08` returned 0.)

### Claude's Discretion
- Exact Coalescer struct layout + field naming (`coalescer.go` or inline in `main.go`).
- Clock-injection interface for testability (mockable now()).
- Path of `verify.sh`/`verify.ps1` (likely `scripts/phase-08/verify.*` per Phase 7 pattern).
- Whether Coalescer is exposed via new package or kept unexported in `main.go` (lean on existing single-file layout).
- Exact slog field names for the summary log (follow existing slog conventions in codebase).
- Ticker interval for the 60s-summary if session-idle (skip log when all counters zero to avoid noise).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Core code files (where changes happen)
- `main.go:40-46` — Current `discordRateLimit = 15 * time.Second` + `debounceDelay = 100 * time.Millisecond` constants to update/augment
- `main.go:132` — `updateChan = make(chan struct{}, 1)` signal bus (keep, see D-19)
- `main.go:144, 245` — current `case updateChan <- struct{}{}:` send sites (keep)
- `main.go:160` — `go presenceDebouncer(ctx, updateChan, registry, &presetMu, &currentPreset, discordClient, ...)` wiring (replace body, keep signature spirit)
- `main.go:501-555` — `presenceDebouncer` function body — drop-on-skip root cause at line 530 `if time.Since(lastUpdate) < discordRateLimit { return }`; full rewrite into Coalescer
- `discord/client.go:24-34` — `Activity` struct — hash-source of D-09
- `discord/client.go:82-153` — `SetActivity` — call site of flush
- `server/server.go:320-347` — `Handler()` method and mux registration — middleware inserted around this return
- `server/server.go:349-472` — `handleHook` — pre-tool-use route that fires 2x (verified via Phase 8 trigger logs)
- `resolver/resolver.go:34, 171, 382` — `ResolvePresence` + `StartTime` assignment — verifies D-09 "StartTime deterministic"
- `config/watcher.go:12, 79` — config watcher's separate `debounceDelay` constant (not touched; separate pattern)

### Prior phase contracts (carry forward)
- **Phase 1 D-30:** Channel-based debounce pattern (buffered channel of 1) — KEEP for `updateChan` signal bus.
- **Phase 1 D-31:** Discord 15s rate limit enforcement in debouncer — REPLACED by token bucket (D-05 supersedes).
- **Phase 1 D-34:** Exponential backoff for Discord reconnection — untouched; interacts with D-20.
- **Phase 6.04 D-07:** "Discord Activity cleared BEFORE IPC Close" — REMAINS; clear path bypasses Coalescer (D-23).
- **Phase 6.04 D-10/D-11:** `context.WithCancel` + `CancelFunc` idempotency → Coalescer goroutine honors `ctx.Done()` same pattern.
- **Phase 6 D-09:** HTTP 200 <10ms + defer-recover background goroutines — hook-dedup middleware MUST preserve this (return 200 immediately on dedup hit).
- **Phase 7 D-03:** "No config toggle when no benefit to tuning" — guides D-05 (hard-coded rate-limit values).
- **Phase 7 D-04/D-05:** `registry.Touch()` does NOT fire `notifyChange` — so PostToolUse-floods of the past are already eliminated; Coalescer only sees "real" presence changes via `UpdateActivity → notifyChange → updateChan`.

### External specs / stdlib
- `golang.org/x/time/rate` — https://pkg.go.dev/golang.org/x/time/rate — `Limiter.Reserve()`, `Reservation.Delay()`, `NewLimiter(rate.Every(d), burst)`. Main stdlib extension (add to `go.mod`).
- `hash/fnv` — https://pkg.go.dev/hash/fnv — `New64a()` for deterministic cross-process FNV-1a. Stdlib, zero new dep.
- `sync/atomic` `Pointer[T]` / `Uint64` — https://pkg.go.dev/sync/atomic — Go 1.19+ generic atomic types. Stdlib.
- Discord RPC docs — https://discord.com/developers/docs/topics/rpc — offizielle SET_ACTIVITY command spec (doc-stated 1/15s limit; empirical ~5/20s per Phase 8 trigger evidence).
- Discord rate-limit issue `discord/discord-api-docs#668` — confirmation that "5/60" regression period happened historically; informs our 4s cadence as conservative vs. empirical.
- Keep a Changelog 1.1.0 — https://keepachangelog.com/en/1.1.0/ — CHANGELOG v4.2.0 entry structure (Added/Changed/Fixed sections — D-34).

### Project files (metadata + docs)
- `.planning/REQUIREMENTS.md` — new D-IDs for this phase MUST NOT collide with D-01..D-56 (Phase 1), DSR-01..DSR-42 (Phase 2), D-01..D-25 (Phase 3), DPA-01..DPA-30 (Phase 4), DIST-01..DIST-10 (Phase 5). New namespace suggested: `RLC-01..RLC-nn` (Rate-Limit Coalescer).
- `.planning/ROADMAP.md` — Phase 8 entry at line 122; goal string to remain unchanged post-plan.
- `CHANGELOG.md` — v4.1.2 entry pattern at top; v4.2.0 inserted above (D-34).
- `scripts/bump-version.sh` — touches 5 files per `CLAUDE.md §Releasing`; reuse as-is in plan 08-04.
- `.github/workflows/test.yml` — CI update target for `-race` flag (D-32).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`updateChan` signal bus** (`main.go:132`) — stays. Coalescer consumes it the same way the current `presenceDebouncer` does.
- **`resolver.ResolvePresence`** — stays. Called inside Coalescer post-debounce to get the Activity. Deterministic w.r.t. StartTime (verified via resolver_test.go:333).
- **`discord.Client.SetActivity`** — stays. Called by Coalescer flush. Thread-safe assumption holds since only one goroutine (Coalescer) calls it.
- **`config.DisplayDetail` getter closure** — stays; passed to Coalescer like today.
- **`slog` logger** — stays; project-wide convention.
- **`sync.Map` precedent** — `server.go:592 s.lastTranscriptRead` uses sync.Map for per-session throttle; hook-dedup reuses the idiom.

### Established Patterns
- **Immutable copy-before-modify** (`session/registry.go:149` etc.) — applicable to Activity; `atomic.Pointer.Store` pushes a whole new value.
- **Defer-recover in background goroutines** (Phase 6 D-09) — Coalescer flusher goroutine MUST have `defer func() { recover() ... slog.Error }()`.
- **External test packages** (Phase 1-7 pattern) — coalescer_test.go uses `package main_test` or a dedicated `package coalescer_test`.
- **Constructor-function injection** for testability — Coalescer takes a `ClockFunc func() time.Time` param defaulting to `time.Now` so tests can inject fake time.
- **Context flow** — Coalescer.Start(ctx) honors ctx.Done() for shutdown (D-23).

### Integration Points
- **main.go:160** — `go presenceDebouncer(...)` call site — replaced with `coalescer := NewCoalescer(...); go coalescer.Run(ctx)`.
- **server/server.go Handler()** — wrap returned `http.Handler` in hook-dedup middleware.
- **Shutdown sequence in main.go** — add `coalescer.Shutdown()` call BEFORE `SetActivity(clearActivity)` + `discord.Close()`.
- **go.mod** — add `require golang.org/x/time v0.15.0` (latest as of 2026-02-11 per exa search).

### Tests affected
- NEW: `coalescer_test.go` (package TBD by planner)
- NEW: `server/hook_dedup_test.go` (package `server_test`)
- UPDATE: `.github/workflows/test.yml` — add `-race` to `go test` invocation
- NEW: `scripts/phase-08/verify.sh` + `scripts/phase-08/verify.ps1` (Phase 7 pattern)

</code_context>

<specifics>
## Specific Ideas

- **Reproduction scenario the fix must pass:** During an MCP-heavy Claude Code session with 10+ `updateChan` signals inside a 15s window, the Discord presence MUST reflect the latest state (hash differs) at most ~4s after the signal was sent. The current bug: only the first of those 10 signals reaches Discord; the other 9 are silently dropped.
- **Empirical cadence source:** The "4s / burst 2" values come from live log correlation (STATE.md line 115 Phase 8 trigger). NOT a guess; not documented by Discord officially (their doc still claims 1/15s, last validated in 2018).
- **No magic numbers in code:** The "500ms dedup TTL" and "60s summary interval" MUST be named constants at the top of the coalescer / middleware file — never inline literals.
- **Version string source:** `main.go` version var already set by `scripts/bump-version.sh`; verify script checks `dsrcode --version == v4.2.0`.
- **CHANGELOG voice:** Past tense, user-benefit-first, following v4.1.2 CHANGELOG entry style as immediate precedent.

</specifics>

<deferred>
## Deferred Ideas

- **`/metrics` Prometheus endpoint** — surfaced during Area D discussion (Variante A + B). Deferred to future observability phase consistent with Phase 7 §deferred precedent. Counter-in-`/status` (Variante C) also rejected to keep this phase focused. Revisit when multi-daemon or multi-user deployments emerge.
- **Adaptive rate-limit with Discord error backoff** — Option A3 in Area A discussion. Discord IPC does not surface clean 429-equivalents; disconnect events are the only signal. Not worth the complexity now that empirical 4s/2 is known to work.
- **Config-key driven rate values** — Option A2 in Area A. Phase 7 D-03 "no-toggle" precedent; binary hotfix suffices if Discord changes its protocol.
- **Per-session coalescers** — only applicable if Discord ever adds per-activity-type limits; today the IPC channel is daemon-global.
- **Debouncer full replacement (drop 100ms front-aggregator)** — Option H1 in Area H. The 100ms debounce still earns its keep by absorbing rapid bursts before they reach the Coalescer. Revisit only if profiling shows the 100ms as a bottleneck.
- **Burst of 1 (no initial fast-path)** — Option S2 in Area S. Rejected for bad UX — daemon restart would see up-to-8s delay to first presence.
- **sync.WaitGroup around Coalescer goroutine** — Phase 6.04 `CancelFunc` idempotency + ctx.Done path is sufficient. WaitGroup is over-engineering here.

### Reviewed Todos (not folded)
(No pending todos matched Phase 8 scope at discuss-phase time.)

</deferred>

<success_hints>
## Hints for Researcher & Planner

- **Do NOT touch** existing `config/watcher.go debounceDelay` (separate 100ms for file-watcher); `registry.Touch()` semantics (Phase 7 D-04/D-05); `connectionLoop` reconnect logic (Phase 1 D-34).
- **Do NOT introduce** new config fields, new env vars, new CLI flags, new plugin hooks, new routes beyond `/hooks/*` middleware coverage.
- **Do verify** by running `scripts/phase-08/verify.sh` end-to-end after the final plan; the 70-% → <5-% skip-rate improvement must be observable in live logs.
- **Plan count sanity check:** 4 plans target (D-04). Fewer means you're bundling a plan's worth of work elsewhere; more means you're over-decomposing a middleware.
- **Atomicity guarantee:** Every plan commit independently compiles + passes `go test -race ./...`. The token-bucket and pending-buffer can be in one plan because they have to land together to avoid a regression window.
- **Release target:** v4.2.0 via `./scripts/bump-version.sh 4.2.0`. 5-file version propagation is automated; just run the script in plan 08-04.
- **MCP-Mandate reminder:** This phase was discussed under strict MCP requirement (sequential-thinking / exa / context7 / crawl4ai) per user instruction. Researcher and planner MUST continue that pattern: pre- and post-MCP rounds per task as in Phases 6 and 7.

</success_hints>

---

*Phase: 08-presence-rate-limit-coalescer-stop-drop-on-skip-token-bucket*
*Context gathered: 2026-04-16*
