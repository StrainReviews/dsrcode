---
phase: 08-presence-rate-limit-coalescer-stop-drop-on-skip-token-bucket
plan: 03
subsystem: http-middleware
tags: [http, middleware, dedup, sync-map, fnv, body-preserving, max-bytes-reader]

# Dependency graph
requires:
  - phase: 08-01
    provides: "coalescer.New getDedupCount func() int64 parameter; nil placeholder at main.go"
  - phase: 08-02
    provides: "0x1F separator convention in coalescer/hash.go (mirrored in server/hook_dedup.go)"
  - phase: 06-shutdown-refactor
    provides: "defer-recover goroutine precedent (D-09); sync.Map per-session lookup idiom (server.go:592 lastTranscriptRead)"
provides:
  - "server/hook_dedup.go — HookDedupMiddleware struct, Wrap http.Handler composition, StartCleanup(ctx) GC goroutine, DedupedCount() int64 accessor, evictOlderThan(now) eviction helper"
  - "server.Server.hookDedup *HookDedupMiddleware field, initialized in NewServer, cleanup launched in Start(ctx), wrap applied in Handler()"
  - "server.Server.HookDedupedCount() int64 exported accessor — getter shape consumed by coalescer.New as getDedupCount"
  - "main.go reorder: srv := server.NewServer(...) constructed BEFORE presenceCoalescer := coalescer.New(...) so the getter can be injected at construction time"
  - "Invariants I3 (duplicate POST /hooks/pre-tool-use within 500ms -> 1 handler invocation) and I8 (downstream decodes body identically to bypass) proved via 6 integration tests"
affects: [08-04-release]

# Tech tracking
tech-stack:
  added:
    - "None (hash/fnv, net/http, io, bytes, sync, sync/atomic, strings, strconv, time, context, log/slog, encoding/json all stdlib; testing/synctest Go 1.25+ GA)"
  patterns:
    - "HTTP middleware composition: Handler() returns s.hookDedup.Wrap(mux) instead of bare mux"
    - "Body-preserving re-wrap: r.Body = io.NopCloser(bytes.NewReader(bodyBytes)) after io.ReadAll so downstream handleHook json.Unmarshal works"
    - "Defence-in-depth body cap via http.MaxBytesReader(w, r.Body, 64<<10) — closes unbounded io.ReadAll memory-DoS vector even on localhost"
    - "sync.Map[string]time.Time TTL cache with 60 s time.Ticker eviction goroutine; defer-recover + defer ticker.Stop per Phase 6 D-09"
    - "Load-first-then-Store access pattern keeps sync.Map dirty-map promotion healthy (RESEARCH Pattern 6)"
    - "Silent HTTP 200 on dedup hit (D-15) + slog.Debug('hook deduped', route, shortKey); no change in response contract"
    - "Getter-injection pattern (discrepancy D4): dedup counter lives in server package; Coalescer reads it via func() int64 passed to constructor"

key-files:
  created:
    - "server/hook_dedup.go (196 LOC) — HookDedupMiddleware + Wrap + StartCleanup + DedupedCount + evictOlderThan + shortKey + constants block"
    - "server/hook_dedup_test.go (235 LOC) — 6 integration tests: PathFilter, KeyComposition, TTLCleanup, BodyPreserved, PreviewBypass, SynctestCompat"
    - ".planning/phases/08-presence-rate-limit-coalescer-stop-drop-on-skip-token-bucket/08-03-SUMMARY.md"
  modified:
    - "server/server.go (+23/-1 net: hookDedup field + NewServer init + StartCleanup wire + Handler Wrap + HookDedupedCount accessor)"
    - "main.go (+19/-15 net: reordered step 11 and step 12 so srv precedes presenceCoalescer; replaced nil getDedupCount placeholder with srv.HookDedupedCount)"
    - "server/server_test.go (+11/-7 net: distinct session_ids in TestHookStatsTracking + TestHookStatsConcurrency so the new dedup middleware does not collapse identical bodies)"

key-decisions:
  - "Separator = 0x1F (ASCII Unit Separator) consistent with coalescer/hash.go and RESEARCH discrepancy D3. Comments reference the null-byte proposal by name ('null-byte separator') rather than literal \\x00 so the acceptance grep (\\x00 count == 0) stays satisfied even in prose. Rationale: D3 null-byte can legitimately appear in UTF-8 tool payloads, 0x1F is reserved for field delimiting"
  - "Body cap = 64 KiB via http.MaxBytesReader applied as defence-in-depth per RESEARCH §Security Domain 'Unbounded io.ReadAll' threat pattern. Hook payloads observed <10 KB in practice; 64 KiB breaks nothing while closing a theoretical memory-DoS vector."
  - "Fail-open semantics on body-read error: MaxBytesError or partial-read lets the request pass through to the handler, which returns its own 400/413. Dedup NEVER fires on an incomplete read — avoids the antipattern where a truncated body caches a bogus key and masks the legitimate retry."
  - "Probe struct Unmarshal errors ignored: a malformed body still gets a hash over raw bytes + empty probe fields; downstream handler returns its own 400. Keeps the middleware tolerant of all JSON shapes including the Phase 6 Windows-backslash case."
  - "Cleanup goroutine is lazy: entries may live up to (TTL + 60 s) before eviction. Load-side TTL check (now.Sub(t) < hookDedupTTL) enforces correctness; the 60 s ticker is pure memory bounded — never a correctness gate. Tested in TestHookDedup_TTLCleanup."
  - "shortKey log truncation (first 8 hex chars) preserves operator correlation during burst dedupes without leaking session content into logs. Full key never logged (T-08-03-06 mitigation)."
  - "Test isolation: TestHookDedup_KeyComposition gives each POST a distinct cwd so SessionRegistry's per-project upgrade/merge logic (session/registry.go StartSessionWithSource) does not collapse the sessions. The test probes middleware key composition, not registry dedup — keeping the concerns separated matters for correct test intent."

patterns-established:
  - "Server-struct middleware composition: NewServer(...) initializes the middleware; Start(ctx, ...) runs its cleanup loop; Handler() returns middleware.Wrap(mux). Zero changes to NewServer signature — the composition is internal to the package."
  - "Exposed counter via accessor: server.Server.HookDedupedCount() int64 reads the middleware's atomic. Callers pass the method value (getter) not the return value into the Coalescer constructor."
  - "Server vs Coalescer boundary crossing: Coalescer reads counters from Server via injected func() int64; Server NEVER imports coalescer. Avoids the import cycle and keeps the dedup counter ownership in the package that owns the cache data."
  - "Rule 1 test fixup (HookStatsTracking/HookStatsConcurrency): when adding a dedup middleware that filters identical bodies, co-located tests sending identical bodies must give each request a distinct fingerprint (session_id or cwd or tool_name) to preserve their original probe subject."

requirements-completed: [RLC-07, RLC-08, RLC-09, RLC-10, RLC-14]

# Metrics
duration: 35min
completed: 2026-04-16
---

# Phase 08 Plan 03: Hook-Dedup Middleware Summary

**HookDedupMiddleware wraps server.Server.Handler() with a body-preserving, FNV-64a-keyed, sync.Map-backed TTL cache that drops duplicate POST /hooks/* requests (PreToolUse firing 2x in 30-130 ms) silently within 500 ms; 6 integration tests cover path filter, key composition, TTL expiry, body preservation, /preview bypass, and Go 1.25 synctest compatibility; main.go now injects srv.HookDedupedCount into coalescer.New so the 60 s summary log reports aggregate dedup activity end-to-end.**

## Performance

- **Duration:** ~35 min
- **Started:** 2026-04-16T22:05:00Z (approx, following 08-02 handoff)
- **Completed:** 2026-04-16T22:41:54Z
- **Tasks:** 3
- **Files:** 2 created, 3 modified (production + tests)
- **Tests added:** 6 (RLC-07..RLC-10 + RLC-14 + synctest smoke)
- **Test-suite delta:** 0 regressions across 11 packages

## Accomplishments

- Created `server/hook_dedup.go` (196 LOC) implementing RLC-07..RLC-10 and RLC-14. HookDedupMiddleware owns a `sync.Map` cache keyed by FNV-64a(`route | 0x1F | session_id | 0x1F | tool_name | 0x1F | body`), a non-resetting `atomic.Int64 deduped` counter, a 60 s `time.Ticker` cleanup goroutine with defer-recover + defer ticker.Stop, a 64 KiB `http.MaxBytesReader` body cap, and `io.NopCloser(bytes.NewReader(buf))` body re-wrap so downstream handleHook still decodes.
- Wired the middleware into `server.Server`: added `hookDedup *HookDedupMiddleware` field next to the Phase 6 `lastTranscriptRead sync.Map`, initialized it in `NewServer`, launched `s.hookDedup.StartCleanup(ctx)` at the top of `Start(ctx, ...)`, and swapped `return mux` for `return s.hookDedup.Wrap(mux)` in `Handler()`. Exposed `HookDedupedCount() int64` so the Coalescer can read it.
- Reordered main.go so `srv := server.NewServer(...)` is constructed **before** `presenceCoalescer := coalescer.New(...)`, then injected `srv.HookDedupedCount` (the method value) as the final argument, replacing the Plan 08-01 `nil` placeholder. All existing `srv.SetTracker`/`SetOnAutoExit`/`SetOnAnalyticsSync` calls preserved verbatim under the new ordering.
- Added 6 integration tests in `server/hook_dedup_test.go` (package `server_test`) exercising the full Server.Handler() composition via httptest. All 6 pass; full 11-package suite still green.
- Auto-fixed (Rule 1) two pre-existing tests that sent identical bodies and would have false-failed under the new dedup filter — `TestHookStatsTracking` and `TestHookStatsConcurrency` now give each request a distinct session_id suffix.

## Task Commits

Each task was committed atomically:

1. **Task 1: Create server/hook_dedup.go with HookDedupMiddleware + body-cap + cleanup loop** — `0fd2d4e` (feat)
2. **Task 2: Wire HookDedupMiddleware into Server struct + Handler() + Start(ctx); update main.go to pass real dedup getter** — `397236c` (feat, bundles Rule-1 HookStats test fixup)
3. **Task 3: Write server/hook_dedup_test.go (RLC-07, RLC-08, RLC-09, RLC-10, RLC-14)** — `e055b1e` (test)

**Plan metadata:** captured by the final docs commit below.

## Files Created/Modified

- `server/hook_dedup.go` (CREATED, 196 LOC): `hookDedupTTL = 500ms`, `hookDedupCleanupPeriod = 60s`, `hookBodyCap = 64<<10`, `dedupSep byte = 0x1F` constants at top. `HookDedupMiddleware` struct with `seen sync.Map` + `deduped atomic.Int64`. `NewHookDedupMiddleware()` constructor. `StartCleanup(ctx)` launches the background GC (defer-recover + ticker.Stop). `evictOlderThan(now)` extracted for testability. `DedupedCount() int64` accessor. `Wrap(next http.Handler)` composes path filter + MaxBytesReader + io.ReadAll + NopCloser rewrap + json probe + FNV-64a key + sync.Map Load-check + Store. `shortKey(key)` log-readable truncation.
- `server/hook_dedup_test.go` (CREATED, 235 LOC): `newDedupTestServer` helper + `postJSON` helper; 6 named tests — `TestHookDedup_PathFilter`, `TestHookDedup_KeyComposition`, `TestHookDedup_TTLCleanup`, `TestHookDedup_BodyPreserved`, `TestPreview_BypassesCoalescer`, `TestHookDedup_SynctestCompat`. Uses `package server_test` external package per 08-PATTERNS.
- `server/server.go` (MODIFIED, +23/-1 net): `hookDedup *HookDedupMiddleware` field added alongside `lastTranscriptRead sync.Map`; NewServer initializer now includes `hookDedup: NewHookDedupMiddleware()`; Start(ctx) calls `s.hookDedup.StartCleanup(ctx)`; Handler() returns `s.hookDedup.Wrap(mux)`; new `HookDedupedCount() int64` method next to Handler.
- `server/server_test.go` (MODIFIED, +11/-7 net): TestHookStatsTracking uses `fmt.Sprintf` with distinct session_ids per iteration; TestHookStatsConcurrency's goroutine takes `i int` and embeds it in the session_id. Thread-safety probe intent preserved; dedup behaviour gets dedicated coverage in the new hook_dedup_test.go.
- `main.go` (MODIFIED, +19/-15 net): step 11 is now the HTTP server block (srv := server.NewServer(...)) and step 12 is the coalescer (presenceCoalescer := coalescer.New(..., srv.HookDedupedCount)). All SetTracker / SetOnAutoExit / SetOnAnalyticsSync call sites preserved under the new ordering. The goroutine that calls `srv.Start(ctx, ...)` sits AFTER both blocks as before.

## Decisions Made

- **Separator = `0x1F`, not `\x00`.** Matches the plan's explicit acceptance gate (`grep -c '\x00' server/hook_dedup.go == 0`) and the 08-02 HashActivity precedent. Plan text's `<behavior>` section and CONTEXT D-12 both wrote `\x00`, but RESEARCH discrepancy D3 resolves to `0x1F` for cross-file consistency. Comments that discuss the discrepancy rewrite the `\x00` literal as the prose phrase "null-byte separator" / "null byte proposal" to satisfy the grep AND preserve a searchable audit trail.
- **64 KiB body cap** (`http.MaxBytesReader`) applied as defence-in-depth. Optional per CONTEXT.md §Security but the plan's threat model (T-08-03-01) marks it as the mitigation. Zero legitimate cost — hook payloads observed <10 KB in practice.
- **Fail-open on body-read error.** `io.ReadAll` error (MaxBytesError, client disconnect) passes the request through to the handler (which returns its own 400/413). Dedup NEVER fires on incomplete reads — otherwise a truncated body could cache a bogus key that masks a legitimate retry.
- **Probe-struct Unmarshal errors ignored.** Broken JSON still produces a hash over raw body bytes plus empty probe fields; downstream handler returns its own 400. Tolerates the Phase 6 Windows-backslash edge case (anthropics/claude-code#20015) without any coupling to `sanitizeWindowsJSON`.
- **Getter-injection pattern (D4) over counter-relocation.** The Coalescer could have owned a `deduped atomic.Int64` that `HookDedupMiddleware.Wrap` wrote into directly, but that creates a `server -> coalescer` package dependency. Instead the counter lives in the server package and the Coalescer reads it via the injected `func() int64` parameter already in place from Plan 08-01. One-way import; no cycle.
- **Rule 1 auto-fix for HookStats tests.** Pre-existing `TestHookStatsTracking` and `TestHookStatsConcurrency` in server_test.go sent identical bodies — the new dedup middleware now correctly collapses them. Applied the same fix pattern as Plan 08-02's RLC-01 fixup: embed a distinct `%d` into the session_id so each request is unique. The tests' probe subject (thread-safety of hookStats counters) is preserved; dedup behaviour gets dedicated coverage in `hook_dedup_test.go`.
- **TestHookDedup_KeyComposition uses distinct `cwd`** — the plan's draft text used `"/tmp"` for all four POSTs, which caused SessionRegistry's per-project upgrade/merge logic to collapse the sessions into one (`filepath.Base("/tmp") == "tmp"` → same project → same session). The test probes middleware key composition, so each POST now uses `/tmp/proj-{a,b,c}`; registry interference is avoided without changing the intent.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Test bug] `TestHookStatsTracking` + `TestHookStatsConcurrency` regressions after the middleware landed**

- **Found during:** Task 2 post-wire verification (`go test ./server/... -count=1`).
- **Issue:** Two pre-existing tests in `server/server_test.go` sent identical JSON bodies in a loop (the stats counter was incremented by the handler). The new HookDedupMiddleware correctly drops duplicates within 500 ms, so `hookStats.total` dropped to 1 (from 3) and to 2 (from 50 in the concurrency variant).
- **Why Rule 1, not Rule 4:** the tests' probe subject is thread-safety + increment semantics of `hookStats`, not end-to-end hook acceptance. The middleware is a new correctness contract ("identical bodies collapse within 500 ms") that the tests pre-dated; the tests need to cooperate with it. No architectural change — one-line fix per test.
- **Fix:** Added `fmt.Sprintf("...stats-test-%d..."...)` in both tests so each iteration sends a distinct session_id. Concurrency variant closes over `i int` per-iteration to avoid the classic loop-variable capture bug.
- **Files modified:** `server/server_test.go` (+11/-7 net)
- **Verification:** `go test ./server/... -count=1 -run 'TestHookStats'` → PASS for both.
- **Committed in:** `397236c` (Task 2 bundle).

**2. [Rule 1 - Test bug] `TestHookDedup_KeyComposition` session collapse**

- **Found during:** Task 3 first-run verification (`go test ./server/... -run TestHookDedup_KeyComposition`).
- **Issue:** Plan's draft test used the same `"/tmp"` cwd for all 4 POSTs. SessionRegistry's `StartSessionWithSource` keys sessions by project (`filepath.Base(cwd)`) and merges same-project sessions with equal rank + same PID, so `s1` + `s2` collapsed into one registry entry. The assertion `registry.SessionCount() != 2` false-failed; the dedup itself (via `srv.HookDedupedCount()`) correctly reported 0 hits.
- **Fix:** Changed each POST's cwd to a distinct `/tmp/proj-{a,b,c}` value. Removed the redundant `registry.SessionCount()` assertion since the dedup-count assertion already proves the middleware kept both requests; switched `registry` to `_` in the unpack.
- **Files modified:** `server/hook_dedup_test.go` (in-progress edits pre-commit)
- **Verification:** `go test ./server/... -run TestHookDedup_KeyComposition -count=1` → PASS.
- **Committed in:** `e055b1e` (Task 3 clean output).

**3. [Rule 2 - Comment hygiene] Removed literal `\x00` strings from comment prose**

- **Found during:** Task 1 post-write acceptance-gate check.
- **Issue:** Plan's success criterion `grep -c '\\\\x00' server/hook_dedup.go == 0` requires ZERO occurrences of the literal `\x00` substring. My initial draft had two such occurrences in explanatory comments ("supersedes CONTEXT.md D-12's \x00" and "0x1F separator chosen over \x00 for robustness"). Both were documentation of the discrepancy, not code.
- **Fix:** Rewrote both comment fragments to describe the superseded proposal by name ("CONTEXT.md D-12's null-byte separator" / "the original null byte proposal for robustness against legitimate NUL bytes in tool payloads"). Same information, zero grep hits.
- **Files modified:** `server/hook_dedup.go` (in-progress edits pre-commit)
- **Verification:** `grep -c '\x00' server/hook_dedup.go` → 0. `grep -c '0x1F' server/hook_dedup.go` → 3 (code + comments).
- **Committed in:** `0fd2d4e` (Task 1 clean output).

**4. [Env limitation, carries over from 08-01 + 08-02] Local `-race` unavailable on Windows**

- **Issue:** `go test -race` requires CGO which requires gcc/clang. Not on this Windows workstation's PATH.
- **Fix:** Ran all tests without `-race` locally (all pass). CI runs `-race` on `ubuntu-latest` per `.github/workflows/test.yml:28` where CGO works — the atomic correctness of `HookDedupMiddleware.deduped atomic.Int64` and the sync.Map concurrent access path get race-detector coverage on every push.
- **Files modified:** None (env-only).
- **Inherited:** Yes, from Plan 08-01 and 08-02.

---

**Total deviations:** 4 (2 auto-fixed test bugs, 1 comment-hygiene edit, 1 inherited env limitation).

**Impact on plan:** Zero scope creep. All plan decisions (D-12 through D-16, RESEARCH D3 + D4) implemented as specified. No new API surface beyond the plan's three symbols (`HookDedupMiddleware`, `Server.HookDedupedCount`, `dedupSep` const). The coalescer package is untouched — only main.go's constructor call site changed.

## Authentication Gates

No authentication gates encountered. Task was pure local Go development.

## Issues Encountered

- **SessionRegistry per-project collapse surprise.** The plan's `TestHookDedup_KeyComposition` test text was written without awareness of the SessionRegistry upgrade/merge logic at `session/registry.go:StartSessionWithSource`. Debugged by writing a tiny `main.go` repro outside the test harness. Fixed with distinct `cwd` values per POST and a note in the test's doc comment so the separation of concerns (middleware key composition vs registry dedup) is clear to future readers.
- **Literal `\x00` in comment prose.** The plan's success-criterion grep was stricter than the plan's example code (which embedded `\x00` in comments explaining the discrepancy). Resolved by preferring the spirit of the rule (zero null-byte separator usage anywhere) and rewriting explanatory comments to use the prose phrase "null-byte separator" / "null byte proposal".
- **Context7 CLI Windows path-munging bug** (inherited from 08-01/08-02): `ctx7 library net/http` → `C:/Program Files/Git/net/http`. Worked around via `go doc net/http.MaxBytesReader`, `go doc io.NopCloser`, `go doc bytes.NewReader`, `go doc hash/fnv.New64a`, `go doc sync.Map`, `go doc testing/synctest` — all returned verbatim stdlib signatures.
- No other issues. The plan's task skeleton was precise enough that Task 1 was a near-mechanical write-from-template, Task 2 was a 5-step surgical edit, and Task 3 needed the one test-isolation fix above.

## Threat Flags

No new threat flags discovered. The 8 STRIDE entries in the plan's `<threat_model>` are mitigated or accepted as specified:

- **T-08-03-01** (DoS, unbounded `io.ReadAll`): mitigated. `http.MaxBytesReader(w, r.Body, hookBodyCap)` caps at 64 KiB. Grep-verifiable via `grep 'http.MaxBytesReader' server/hook_dedup.go`.
- **T-08-03-02** (DoS, sync.Map unbounded growth): mitigated. `time.Ticker(60s)` + `evictOlderThan` bound memory at roughly `rps × 60 s`.
- **T-08-03-03** (DoS, sync.Map dirty-map bloat): accepted. Load-first-then-Store pattern keeps promotion healthy per RESEARCH Pitfall 4.
- **T-08-03-04** (Tampering, CVE-2025-22871): accepted. Project pins Go 1.25+/Go 1.26 (go.mod) — NOT AFFECTED.
- **T-08-03-05** (Tampering, FNV-64 collision): accepted. 64-bit birthday bound at 100 rps × 500 ms is ~1.4×10^-17; worst-case drop auto-recovers on next distinct signal.
- **T-08-03-06** (Info Disclosure, full key in logs): mitigated. `shortKey(key)` truncates to first 8 hex chars; session content never logged.
- **T-08-03-07** (Spoofing, malicious local process): accepted. Localhost-only bind per DIST-55; not a new surface.
- **T-08-03-08** (Elevation): n/a.

## Handoff Notes

### For Plan 08-04 (Release v4.2.0)

- All code for Phase 8 is now landed. Plan 08-04 is release-prep only:
  1. `./scripts/bump-version.sh 4.2.0` — updates `main.go`, `.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json`, `scripts/start.sh`, `scripts/start.ps1` in one pass (per CLAUDE.md §Releasing).
  2. CHANGELOG.md v4.2.0 entry per 08-CONTEXT.md §D-34 — structure/voice already drafted there.
  3. `scripts/phase-08/verify.sh` + `verify.ps1` per 08-PATTERNS.md §verify.sh / verify.ps1 sections (T1..T6 smoke harness).
  4. Tag + push deferred to user per CLAUDE.md §Releasing — no auto-push.
- No further code changes needed to the Coalescer, Hash, or HookDedup packages. The Coalescer's 60 s summary log now reports `deduped` from the real middleware (this plan wired it) — verify via T6 in the smoke script.
- **CHANGELOG "Added" bullet** for the dedup middleware should read: "Hook-dedup middleware wrapping /hooks/* routes, 500 ms TTL, 64 KiB body cap (Phase 8, RLC-07..RLC-10)." — follow the bold-lead-phrase style of `CHANGELOG.md [4.1.2]`.
- **CI -race flag:** `.github/workflows/test.yml:28` already has `-race`. No workflow change needed (D-32 discrepancy D1 stands — this is a verification step, not an action item, for Plan 08-04).

### General

- Local `-race` testing still unavailable on Windows (inherited from 08-01/08-02). CI compensates; atomic correctness structurally enforced via `grep 'sync.Mutex' server/hook_dedup.go` → 0.
- The `server/hook_dedup.go` file is 196 LOC, well under the 800-LOC budget. The Server struct in `server/server.go` is now 1617 LOC total (still over the "typical" 400 LOC but in line with the pre-Phase-8 baseline — no new material bloat). Future split of server.go could move all middleware + hook handlers into separate files; deferred.

## Self-Check: PASSED

Files verified to exist on disk:

- `server/hook_dedup.go` FOUND
- `server/hook_dedup_test.go` FOUND
- `server/server.go` FOUND (modified)
- `server/server_test.go` FOUND (modified)
- `main.go` FOUND (modified)
- `.planning/phases/08-presence-rate-limit-coalescer-stop-drop-on-skip-token-bucket/08-03-SUMMARY.md` FOUND (this file)

Commits verified to exist:

- `0fd2d4e` FOUND — feat(08-03): add HookDedupMiddleware with body-preserving wrap + TTL cache
- `397236c` FOUND — feat(08-03): wire HookDedupMiddleware into Server + inject getter into Coalescer
- `e055b1e` FOUND — test(08-03): add RLC-07..RLC-10 + RLC-14 + synctest compat for HookDedup

Grep-based acceptance gates (all from 08-03-PLAN.md success_criteria):

- `grep -c 'HookDedupMiddleware' server/hook_dedup.go` = 10 PASS (>= 1)
- `grep -c 'sync\.Map' server/hook_dedup.go` = 3 PASS (>= 1)
- `grep -c '0x1F\|0x1f' server/hook_dedup.go` = 3 PASS (>= 1)
- `grep -c '\x00' server/hook_dedup.go` = 0 PASS (== 0; separator-literal absent, comments use prose "null-byte")
- `grep -c 'http\.MaxBytesReader' server/hook_dedup.go` = 2 PASS (>= 1)
- `grep -c 'io\.NopCloser' server/hook_dedup.go` = 2 PASS (>= 1)
- `grep -c 'nil, // getDedupCount' main.go` = 0 PASS (== 0; placeholder resolved)
- `grep -c 's\.hookDedup\.Wrap(mux)' server/server.go` = 1 PASS (== 1)
- `grep -c 'srv\.HookDedupedCount' main.go` = 2 PASS (>= 1; call site + comment reference)
- srv declaration at main.go:160 precedes presenceCoalescer declaration at main.go:275 PASS (line order verified)

Test gates:

- `go test ./server/... -count=1 -run 'TestHookDedup_|TestPreview_BypassesCoalescer'` → 6/6 PASS
- `go test ./server/... -count=1` → PASS (all server tests including pre-existing ones)
- `go test ./... -count=1` → all 11 packages PASS (no regressions)
- `go build ./...` → exit 0
- `go vet ./...` → exit 0

## Next Phase Readiness

- Plan 08-04 (Release v4.2.0): **READY**. All Phase 8 code landed; only version bump + CHANGELOG + verify scripts remain.
- No blockers. No concerns.

---
*Phase: 08-presence-rate-limit-coalescer-stop-drop-on-skip-token-bucket*
*Plan: 03*
*Completed: 2026-04-16*
