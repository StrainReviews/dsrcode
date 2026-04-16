# Phase 8: Presence Rate-Limit Coalescer — Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in `08-CONTEXT.md` — this log preserves the alternatives considered.

**Date:** 2026-04-16
**Phase:** 08-presence-rate-limit-coalescer-stop-drop-on-skip-token-bucket
**Areas discussed:** D (Scope), A (Rate-Limit-Werte), B (Content-Hash), C (Hook-Dedup), E (Flusher-Timer), F (Pending-buffer-Storage), G (Discord-Disconnect), H (Debouncer-Migration), I (Observability), J (Test-Strategie), K (Error-Handling), L (Dedup-Cleanup), M (Preset-Hot-Reload), N (Race detector), O (CHANGELOG), P (Preview-Mode), Q (Shutdown), S (Initial-Burst), T (Clear-on-SIGTERM), U (Cold-Start)
**MCPs used throughout:** sequential-thinking, exa, context7, crawl4ai (per user mandate; post-hoc batches added for completeness)

---

## Area D — Scope für v4.2.0

| Option | Description | Selected |
|---|---|---|
| Alle 5 Fixes, keine Metriken | pending-buffer + token-bucket + content-hash + hook-dedup + race-free state. Observability strictly deferred. | ✓ |
| Alle 5 + Counter in /status | 5 Fixes + 5 atomic counters rendered in existing GET /status. Null neue Route, null neue Dep. | |
| Alle 5 + /metrics (Prometheus) | + prometheus/client_golang + promhttp.Handler(). ~2-3 MB binary bloat, industry standard. | |
| Alle 5 + /metrics (hand-rolled JSON) | New GET /metrics with hand-rolled JSON, no Prometheus dep. | |
| Alle 5 Fixes (initial empfehlung) | Baseline presented first round. | |
| 3 Kern-Fixes, Hook-Dedup später | Split release; hook-dedup as Phase 9. | |

**User's choice (after explanation):** "Alle 5 Fixes, keine Metriken"
**Rationale:** `/metrics`-Endpoint was explained in detail (Variants A/B/C with tradeoffs). User chose to remain consistent with Phase 7 §deferred ("fixes first, observability later") and preserve lean single-binary (Phase 5 DIST-01).

### Plan count
| Option | Description | Selected |
|---|---|---|
| 4 Plans | Coalescer-core / content-hash / hook-dedup / release. | ✓ |
| 5 Plans | Per-fix decomposition. | |
| Claude's Discretion | Planner decides. | |

**User's choice:** 4 Plans

---

## Area A — Rate-Limit-Werte

### How to manage cadence + burst
| Option | Description | Selected |
|---|---|---|
| Hart kodiert | `const discordRateInterval = 4s`, `const discordRateBurst = 2`. Consistent with Phase 7 D-03 "no-toggle" pattern. | ✓ |
| Konfigurierbar via config | Keys `discord_rate_cadence: "4s"`, `discord_rate_burst: 2` with hot-reload. | |
| Halb-hart (Env-Overrides) | Constants + `CC_DISCORD_RATE_*` env vars override. | |

### Which `rate.Limiter` method
| Option | Description | Selected |
|---|---|---|
| `Reserve().Delay()` — non-blocking | Canonical for coalescer scheduling via `time.AfterFunc(delay, flush)`. | ✓ |
| `Wait(ctx)` — blocking in dedicated goroutine | Simpler loop but blocks Coalescer goroutine. | |
| `Allow()` + own timer | Non-blocking bool check + manual 1s retry timer. | |

**User's choice:** Hart kodiert + `Reserve().Delay()`

---

## Area B — Content-Hash-Scope

### Hash fields
| Option | Description | Selected |
|---|---|---|
| Alle sichtbar, StartTime exclude | Details/State/Images/Texts/Buttons hashed; StartTime deterministic-excluded. | ✓ |
| Nur Details + State | Narrow. Risk: SmallImage changes (coding→reading) skipped → stale icon. | |
| Alle 8 Felder inkl. StartTime | Safe but risks hash-never-equals if StartTime fluctuates. | |

### Hash storage
| Option | Description | Selected |
|---|---|---|
| `atomic.Uint64 lastSentHash` | Lock-free compare-and-skip. Pairs with D-02 race-free requirement. | ✓ |
| `sync.Mutex + uint64` field | Mutex covers pending+hash+lastUpdate. | |
| Claude's Discretion | Planner decides based on Coalescer struct design. | |

**User's choice:** Alle sichtbar (StartTime exclude) + `atomic.Uint64`

---

## Area C — Hook-Deduplication-Strategy

### Dedup key
| Option | Description | Selected |
|---|---|---|
| route + session + tool + body-hash | FNV-64(route\|session\_id\|tool\_name\|body). Precise. | ✓ |
| route + session + tool (no body) | Risk: legitimate back-to-back same-tool events erroneously dropped. | |
| route + body-hash only | Cross-session collisions. | |

### Dedup TTL
| Option | Description | Selected |
|---|---|---|
| 500 ms | 4× headroom over observed 30-130ms duplicate spacing. | ✓ |
| 250 ms | Tight; fails on 130ms jitter. | |
| 1 s | Defensive; risks dropping legitimate rapid retries. | |

### Dedup placement
| Option | Description | Selected |
|---|---|---|
| Middleware um gesamten mux, Filter auf /hooks/* | Single wrapper. Handler code stays dedup-agnostic. | ✓ |
| Per-handler check | Inline at start of each hook-handler. | |
| Nur um /hooks/pre-tool-use (schmal) | Risk: other duplicate bugs stay invisible. | |

### Dedup response
| Option | Description | Selected |
|---|---|---|
| HTTP 200 + debug-log silent | Claude Code sees no difference. | ✓ |
| HTTP 200 + `X-Dsrcode-Deduped: 1` | Debug header; response not consumed. | |
| HTTP 204 No Content | Semantic but breaks consistency with existing hook-200s. | |

**User's choice:** All 4 "empfohlen" options

---

## Area E — Flusher-Timer-Architektur

| Option | Description | Selected |
|---|---|---|
| `time.AfterFunc` single-shot | Replaced on each new pending. Minimal goroutines, exact latency. | ✓ |
| Dedicated Ticker (jede 1s) | Simpler loop but up-to-1s extra latency. | |
| Long-lived Wait-Goroutine | Elegant but more coordination overhead. | |

**User's choice:** `time.AfterFunc` single-shot

---

## Area F — Pending-buffer Storage-Form

| Option | Description | Selected |
|---|---|---|
| `atomic.Pointer[Activity]` | Go 1.19+ lock-free pointer. Store(a) replaces, Swap(nil) flush-and-clear. | ✓ |
| `sync.Mutex + *Activity field` | Classical. Mutex contention on hot path. | |
| `chan *Activity cap 1` | Select-send-with-drop. Nonblocking but less idiomatic for latest-value-wins. | |

**User's choice:** `atomic.Pointer[Activity]`

---

## Area G — Discord-Disconnect Behavior

| Option | Description | Selected |
|---|---|---|
| Verwerfen + re-trigger bei reconnect | `atomic.Pointer.Store(nil)` + connectionLoop signals updateChan on reconnect. | ✓ |
| Pending halten bis reconnect | Push stale state on reconnect. | |
| Pending verwerfen, kein re-trigger | User sees stale presence until next hook. | |

**User's choice:** Verwerfen + re-trigger

---

## Area H — Debouncer-Migration

| Option | Description | Selected |
|---|---|---|
| 100ms debounce beibehalten + Coalescer dahinter | Layer: updateChan → 100ms AfterFunc → resolver+hash → Reserve() → flush. | ✓ |
| Komplett ersetzen (Coalescer = presenceDebouncer) | Cleaner diff; no pre-aggregation. | |
| Claude's Discretion | Planner decides. | |

**User's choice:** 100ms beibehalten + Coalescer dahinter

---

## Area I — Observability-Logs

| Option | Description | Selected |
|---|---|---|
| DEBUG per-event + INFO Summary 60s | Forensic debug + ops-level 60s summary. | ✓ |
| Nur per-event DEBUG (wie heute) | No INFO summary. | |
| Nur INFO bei state-changes | Minimal noise, no live feedback. | |

**User's choice:** DEBUG per-event + INFO Summary 60s

---

## Area J — Test-Strategie

| Option | Description | Selected |
|---|---|---|
| Unit + integration + verify-script | Coalescer-unit (fake time) + server-integration + live bash+ps1 script (Phase 7 pattern). | ✓ |
| Unit + integration (no manual script) | Automated tests only. | |
| Unit only | Middleware would be undertested. | |

**User's choice:** Unit + integration + verify-script

---

## Area K — SetActivity-Error-Handling

| Option | Description | Selected |
|---|---|---|
| Log + verwerfen, kein retry | Keep today's behavior; connectionLoop handles reconnect. | ✓ |
| 1 retry nach 500ms | Adds code-path for transient errors. | |
| Trigger explizitem disconnect | SetActivity error → discord.Close → reconnect. | |

**User's choice:** Log + verwerfen, kein retry

---

## Area L — Hook-Dedup-Map Cleanup

| Option | Description | Selected |
|---|---|---|
| Ticker 60s + delete expired | Background goroutine walks map, deletes old entries. | ✓ |
| Opportunistic inline prune | Inline N-entry prune during IsDuplicate check. | |
| LRU mit fixed cap 1000 | External lru lib, natural eviction. | |

**User's choice:** Ticker 60s + delete expired

---

## Area M — Preset-Hot-Reload während pending update

| Option | Description | Selected |
|---|---|---|
| Verwerfen + re-trigger | Consistent with G3 disconnect-behavior. | ✓ |
| Pending as-is pushen | User sees one stale-preset update. | |
| Re-compute pending mit neuem preset | Resolver called again, more coupling. | |

**User's choice:** Verwerfen + re-trigger

---

## Area N — Race detector im CI-Workflow

| Option | Description | Selected |
|---|---|---|
| Mandatory in test.yml: `go test -race ./...` | Current CI has NO -race (verified grep). Fix adds it. | ✓ |
| Nur lokal, CI bleibt go test | Race conditions can slip through PR merge. | |
| Race in separatem CI-job (non-blocking) | Visible but not blocking. | |

**User's choice:** Mandatory in test.yml

---

## Area O — CHANGELOG v4.2.0 entry-Struktur

| Option | Description | Selected |
|---|---|---|
| User-benefit front-loaded | Headline: "Fix presence stalling during long sessions". Added/Changed/Fixed sections per Keep a Changelog 1.1.0. | ✓ |
| Nur Changed-section (alles intern) | Internal refactor framing. | |
| Minimal bullet list | One-liner per fix. | |

**User's choice:** User-benefit front-loaded

---

## Area P — Preview-Mode Coexistence

| Option | Description | Selected |
|---|---|---|
| Preview bypasst Coalescer | Direct discordClient.SetActivity() as today. | ✓ |
| Preview geht auch durch Coalescer | Rate-limits apply to preview. | |
| Coalescer pausieren während Preview | More state-machine. | |

**User's choice:** Preview bypasst Coalescer

---

## Area Q — Shutdown-Behavior des Coalescers

| Option | Description | Selected |
|---|---|---|
| `Coalescer.Shutdown()` discard'd pending | Stops flusher-timer + cleanup-ticker. Main.go D-07 sequence unberührt. | ✓ |
| Flush pending letzten update | Up-to-4s shutdown latency. | |
| Nur timer stop, pending bleibt (GC) | Minimal cleanup. | |

**User's choice:** `Coalescer.Shutdown()` discard'd pending

---

## Area S — Initial Token-Bucket State

| Option | Description | Selected |
|---|---|---|
| 2 volle tokens beim Start | rate.NewLimiter default. First 2 updates instant. | ✓ |
| 0 tokens, drain 2x | 4-8s delay on first presence (bad UX). | |
| 1 token (half-burst) | Compromise without clear rationale. | |

**User's choice:** 2 volle tokens

---

## Area T — Presence-Clear-on-SIGTERM

| Option | Description | Selected |
|---|---|---|
| Bypasst Coalescer, pending discard'd | Direct SetActivity(clear) → Close. No rate-limit wait on shutdown. | ✓ |
| Clear geht durch Coalescer | Up-to-4s delay pointless. | |
| Clear via Coalescer mit high-priority bypass | More state-machine. | |

**User's choice:** Bypasst Coalescer

---

## Area U — Cold-Start der ersten presence

| Option | Description | Selected |
|---|---|---|
| 100ms debounce wie heute, dann instant | No special first-signal path. | ✓ |
| Skip 100ms debounce für ersten signal | <10ms first presence via isFirstSignal flag. | |
| Claude's Discretion | Planner decides. | |

**User's choice:** 100ms debounce wie heute

---

## Claude's Discretion (documented in CONTEXT.md §decisions)
- Exact Coalescer struct layout + field naming
- Clock-injection interface for testability
- Path of verify.sh/verify.ps1
- Whether Coalescer is a new package or lives in main.go
- Exact slog field names for summary log
- Whether 60s-summary suppresses output when all counters zero

## Deferred Ideas (captured in CONTEXT.md §deferred)
- /metrics Prometheus endpoint
- Adaptive rate-limit with Discord error backoff
- Config-key driven rate values
- Per-session coalescers
- Debouncer full replacement (drop 100ms front-aggregator)
- Burst of 1 (no initial fast-path)
- sync.WaitGroup around Coalescer goroutine

---

## MCP usage note

Per user directive ("nutze zwingend in allen Schritten und Zwischenschritten die MCPs sequential-thinking, exa, context7, crawl4ai"), all 20 gray-area decisions were research-grounded:
- `sequential-thinking` — used for every pre-decision analysis (thoughts 1-14)
- `exa` — Discord RPC rate-limit validation, token-bucket patterns, FNV change-detection, hook-dedup idioms, graceful-shutdown Go patterns, slog observability, Prometheus/client_golang evaluation, idempotency middleware
- `context7` — golang.org/x/time/rate resolution, prometheus/client_golang docs, fsnotify resolution, keep-a-changelog resolution, mennanov/limiters (rejected as distributed-overkill)
- `crawl4ai` — rate-limiter patterns, sync.Map TTL cleanup, Discord IPC graceful-close

Initial rounds on Areas B/E/F/G/H/I/J/K/L/M/N/O/P/Q/S/T/U used primarily sequential-thinking; post-hoc batches (after user correction) added exa/context7/crawl4ai coverage per mandate. All decision claims verified against live search results and standard-library source before write-up.

### MCP-Revalidation pass (post-hoc)

After the user pointed out incomplete MCP coverage, all 20 Areas were re-presented with the MCP-Fakten embedded directly in the option descriptions:
- Round 1/5 (D, A, A.2): Scope + Rate-Limit-Werte + Limiter-API
- Round 2/5 (B, B.2, C, D.2): Hash-Scope + Storage + Hook-Dedup + Plan-count
- Round 3/5 (E, F, G, H): Coalescer-Architektur
- Round 4/5 (I, J, K, L): Observability / Testing / Errors / Cleanup
- Round 5a/5 (M, N, O, P): Hot-Reload / CI / CHANGELOG / Preview
- Round 5b/5 (Q, S, T, U): Shutdown / Initial State / Clear-on-Exit / Cold-Start

**Result:** All 20 decisions remained identical between initial round and MCP-revalidation round. The empfohlene option won every time, now explicitly research-grounded via live MCP facts (Discord Doku vs empirical limits, Go stdlib atomic.Pointer semantics, fnv.New64a determinism, keepachangelog 1.1.0 sections, slog best practices, fsnotify watch pattern, OneUptime sync.Map+ticker cleanup).
