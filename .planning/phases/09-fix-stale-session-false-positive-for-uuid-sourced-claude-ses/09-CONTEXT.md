# Phase 9: Fix stale-session false-positive for UUID-sourced Claude sessions - Context

**Gathered:** 2026-04-18
**Status:** Ready for planning
**Target release:** v4.2.1 hotfix
**Trigger:** Live incident `~/.claude/dsrcode.log` 2026-04-18 11:37–11:44. UUID session `2fe1b32a-ea1d-464f-8cac-375a4fe709c9` removed 2m25s after last hook despite Claude Code still active, triggering auto-exit. Phase-7 Bug-#1 guard incomplete for non-`http-*` session IDs.

**MCP evidence per user mandate:** PRE+POST rounds (`sequential-thinking`, `exa`, `context7`, `crawl4ai`) were run before every decision gate in this discussion. Key external validations captured inline below.

<domain>
## Phase Boundary

**In scope:**
1. Fix the false-positive stale-session removal for UUID-sourced (`SourceClaude`) sessions that arrive via HTTP hooks with short-lived wrapper PIDs.
2. Adjust existing stale-detection tests whose Phase-7 assertion reflects a theoretical path that doesn't exist in production.
3. Add regression tests covering the real-world reproduction case + backstop.
4. Ship as v4.2.1 hotfix with the full release artefact set (CHANGELOG + bump-version + verify harness).

**Out of scope (deferred):**
- Classification refactor (Option A: server-side source override) — breaks `registry_dedup_test.go` upgrade chain (20+ tests). Source enum semantics stay.
- HTTP-bind-retry + shutdown-race bug surfaced in live log (11:58:17 port collision → fatal shutdown). Separate root cause, deferred to v4.2.2 / quick-task.
- PID-recycling edge case (container PID reuse) — industry-standard fix `(pid, starttime)` comparison is substantial; deferred.
- Refcount replacement, `/metrics` endpoint, UI surfaces — unchanged from Phase 7 deferred list.

</domain>

<decisions>
## Implementation Decisions

### D-01 — Fix Option (E-refined, Guard Expansion)
Expand the Phase-7 guard in `session/stale.go:41` from single-source to two-source skip:

```go
// VORHER (Phase 7):
if s.PID > 0 && s.Source != SourceHTTP && !IsPidAlive(s.PID) {

// NACHHER (Phase 9):
if s.PID > 0 && s.Source != SourceHTTP && s.Source != SourceClaude && !IsPidAlive(s.PID) {
```

**Rationale:** In production, UUID session IDs arrive via HTTP hooks (handleHook/handlePostToolUse) carrying the wrapper-launcher PID (start.sh/start.ps1), not a direct Claude Code spawn. The PID is therefore not authoritative for *either* `SourceHTTP` or `SourceClaude`. `SourceJSONL` has `PID=0` by design (the `s.PID > 0` gate already excludes it).

**Industry validation (Exa PRE-MCP):** stacklok/toolhive PR #4325 (`ResetWorkloadPIDIfMatch`) demonstrates the same *guard-tightening* pattern — compare-or-skip under lock, never unconditional mutation.

**Rejected alternatives:**
- Option A (server-side `StartSessionWithSource(..., SourceHTTP)` override): breaks `registry_dedup_test.go` upgrade-chain invariant `JSONL(0) < HTTP(1) < Claude(2)` and the `HasHigherRankSessions(SourceHTTP)` JSONL-skip semantic.
- Option C (runtime.GOOS gate): Unix wrappers exhibit the same weakness (start.sh exits while Claude runs); a GOOS branch addresses only the visible Windows symptom.
- Option D (new `PIDIsEphemeral` flag or `SourceHTTPWrapperPID` enum): over-engineered; achieves no behavior not covered by E-refined.

### D-02 — No Platform Gate (`runtime.GOOS` stays out)
The fix is source-based, not platform-based. On Unix, `start.sh → bash` is the parent; the bash process exits before Claude Code, leaving `IsPidAlive(startShPID)` false. On Windows, orphans are not reparented to PID 1 (system structural), so `IsPidAlive(wrapperPID)` is permanently false within seconds. Both platforms benefit from the source-based skip; no `runtime.GOOS` branch needed.

### D-03 — Phase-7 Test Inversion + New Backstop Test
`session/stale_test.go:38-54` `TestStaleCheckPreservesPidCheckForPidSource` asserted: *"UUID + dead PID + 5min old activity → remove"* as the Phase-7 spec. That spec reflected a theoretical direct-Claude-spawn path; production has never taken it. After E-refined, the test FAILS (session survives). Action:

1. **Invert + rename** → `TestStaleCheckSkipsPidCheckForClaudeSource` — SourceClaude + dead PID + 5min old activity → **survives** (guard skip).
2. **Add backstop** → `TestStaleCheckRemovesClaudeSourceAfterRemoveTimeout` — SourceClaude + dead PID + 31min old activity → **removed** (via `removeTimeout` backstop, not PID-liveness).

Prior art: Keith-CY/carrier #584 — test-only PR to cover stale-process recovery regressions is branchenüblich.

### D-04 — Test Framework: `SetLastActivityForTest`
Keep Phase-7-style deterministic test helper `registry.SetLastActivityForTest(sessionID, t)` (registry.go:453). `CheckOnce` is synchronous with no timers/goroutines — `testing/synctest.Test` (Go 1.25 graduated) brings no value here and would couple unrelated tests to the bubble isolation model used in Phase 8 for `rate.Limiter`.

**Context7 evidence:** `testing/synctest` is designed for goroutine-coordinated time-testing (`synctest.Test(t, ...)` + `synctest.Wait()`). Linear synchronous stale-detection has no durably-blocked goroutines to advance against.

### D-05 — Regression Test Scope (all four scenarios)
1. **(a)** `TestStaleCheckSkipsPidCheckForClaudeSource` — inverted Phase-7 test. Validates E-refined guard-skip for `SourceClaude`.
2. **(b)** `TestStaleCheckRemovesClaudeSourceAfterRemoveTimeout` — 30-min backstop still removes zombie sessions.
3. **(c)** `TestStaleCheckSurvivesUuidSourcedLongRunningAgent` — live-reproduction mirror. Header comment references `dsrcode.log` 2026-04-18 11:37–11:44, session `2fe1b32a-…`, PID 5692, elapsed 150s.
4. **(d)** slog-debug assertion — custom `bytes.Buffer` slog handler (or `testing/slogtest.TestHandler`) confirms the `"PID dead but session has recent activity, skipping removal"` line fires with the correct attributes. Observability guard.

### D-06 — v4.2.1 Release Artefacts (full set analogous to Phase 7 + Phase 8)
1. `CHANGELOG.md` — new `## [4.2.1] - 2026-04-18` section above `[4.2.0]`, with `### Fixed`, `### Changed`, `### Added` subsections per Keep-a-Changelog 1.1.0 (Exa validation).
2. `scripts/bump-version.sh 4.2.1` — propagates across the 5 canonical files (`main.go`, `.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json`, `scripts/start.sh`, `scripts/start.ps1`).
3. `scripts/phase-09/verify.sh` (bash) — T1-build, T2-test (all 4 regression tests), T3-test-race, T4-live-daemon-smoke (optional, documented), T5-version-propagation-check. `set -euo pipefail` + log-offset pattern from Phase 8.
4. `scripts/phase-09/verify.ps1` (PowerShell) — parity.
5. Git tag + push **remains user's follow-up** per CLAUDE.md §Releasing and `3-tag-push-limit` memory.

### D-07 — slog Line Stays Unchanged
Keep the existing `slog.Debug("PID dead but session has recent activity, skipping removal", ...)` line unchanged. After E-refined it fires for `SourceClaude` as well (confirmed already firing in the live log for session `8273c952-…`, pid=25316, 8 times in the current session). Debug level is correct (per-tick 30s cadence, handler drops by default at LevelInfo — `Enabled()` early-drop per Go-Blog slog performance notes).

### D-08 — Extended Code Comment in `stale.go:38-42`
Replace the current 3-line comment with a 15-line godoc explaining why the guard covers both sources and noting the platform-specific rationale:

```go
// PID liveness check — skip for sessions whose PID is not authoritative.
//
// The PID recorded for a session may not be the actual Claude Code process:
//   - SourceHTTP synthetic IDs ("http-*") always use a wrapper-process PID
//     from start.sh/start.ps1 — the wrapper exits within seconds.
//   - SourceClaude UUID IDs also arrive via the HTTP hook path in production
//     (Claude Code does not spawn dsrcode directly), carrying the same
//     short-lived wrapper PID via X-Claude-PID header or os.Getppid().
//
// On Windows, orphan processes are not reparented to PID 1 — once the
// wrapper exits, IsPidAlive(wrapperPID) is permanently false even though
// Claude Code itself is active. On Unix the wrapper parent chain has the
// same weakness (start.sh exits before Claude does).
//
// The removeTimeout backstop (30min default) still reaches truly stale
// sessions that have stopped sending hooks entirely.
```

### D-09 — CHANGELOG Framing with Live-Reproduction Evidence
The `## [4.2.1]` entry quotes the exact incident log excerpt (`dsrcode.log` 2026-04-18 11:37–11:44, session `2fe1b32a-ea1d-464f-8cac-375a4fe709c9`, PID 5692, 8× skipping-removal debug lines before removal at 2m25s) as the forensic anchor. Future regressions can be diffed against this reproducible trail.

### Claude's Discretion
- Exact slog attribute ordering and key names in regression tests — planner picks the idiom closest to existing Phase-7 test style.
- Bash vs PowerShell idiom details in verify scripts — mirror Phase 8 patterns.
- Test-file layout (new file vs append to `stale_test.go`) — planner decides based on test count.

### Folded Todos
None — no pending todos matched Phase 9 scope at discuss-phase time.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Core code files (change site + references)
- `session/stale.go:41` — the guard expansion (D-01). 1-line diff + extended godoc comment (D-08).
- `session/stale_test.go:38-54` — Phase-7 test to invert + rename (D-03). New tests added in same file or sibling.
- `session/source.go:14-17` — SessionSource enum definition (unchanged; E-refined preserves semantics).
- `session/registry.go:42-44` — `StartSession → StartSessionWithSource(req, pid, sourceFromID(req.SessionID))` (unchanged; dispatch path preserved).
- `session/registry.go:451-465` — `SetLastActivityForTest` test helper used in D-04.
- `session/registry_dedup_test.go:1-260` — 20+ upgrade-chain tests. MUST stay green (E-refined doesn't touch registry).
- `server/server.go:410-445` (handleHook) — HTTP hook handler calling `StartSession` (unchanged).

### Hook evidence / live reproduction
- `~/.claude/dsrcode.log` lines 1-34 (original incident, 2026-04-18 11:37–11:44, session `2fe1b32a-…`, PID 5692 — logs were later zeroed on daemon restart 11:52:24; authoritative transcript preserved in this CONTEXT.md and CHANGELOG).
- `~/.claude/dsrcode.log` lines 17-454 (post-restart session `8273c952-…`, PID 25316, 8× skipping-removal live demonstrations).

### Prior phase contracts (carry forward)
- Phase 7 `07-CONTEXT.md` D-01/D-02/D-03 (Bug #1 guard introduction) — Phase 9 extends, never removes.
- Phase 7 `07-01-SUMMARY.md` — one-line diff precedent (same surgical style).
- Phase 3 D-05/D-06 — Windows `IsPidAlive` via `tasklist`, PID-based tracking on macOS/Linux. Phase 9 does not touch `IsPidAlive` implementation, only the guard around it.
- Phase 1 D-41/D-42 — X-Claude-PID header + `os.Getppid()` fallback. Accepted as unreliable for HTTP-hook context; E-refined handles it at the guard level.
- Phase 8 `scripts/phase-08/verify.sh` / `verify.ps1` — structural template for D-06 verify harness.

### Release artefacts
- `CHANGELOG.md` — [4.2.1] entry format per Keep-a-Changelog 1.1.0 (Exa PRE-MCP validation: keepachangelog.com/en/1.1.0, micro/go-micro PR #2878 Go-project example).
- `scripts/bump-version.sh` — 5-file version propagation. Phase 7/8 idempotent usage.
- `main.go` — version string.
- `.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json` — manifest version.
- `scripts/start.sh`, `scripts/start.ps1` — daemon-version reference strings.

### Specs / framework docs
- `.planning/REQUIREMENTS.md` — D-ID namespace. Phase 9 D-IDs defined above don't collide with Phase 1-7 D-NN.
- `.planning/ROADMAP.md` Phase 9 entry — goal line finalized during planning from this CONTEXT.md.
- Go stdlib `log/slog` (go1.25.3) — Debug/Info level taxonomy (context7 PRE-MCP).
- Go stdlib `testing` (go1.25.3) — test framework; `testing/synctest` NOT used (see D-04).

### External prior-art citations
- stacklok/toolhive PR #4325 — guard-tightening pattern validates D-01 direction.
- openclaw/openclaw PR #26443 — PID-recycling via `(pid, starttime)` comparison (deferred, not applied in Phase 9).
- Keith-CY/carrier #584 — stale-process regression test pattern.
- ruvnet/claude-flow #1171 — inverse bug (orphan accumulation) for CHANGELOG-framing contrast.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `session.SessionSource` enum (`source.go:11-23`) — JSONL=0, HTTP=1, Claude=2. Unchanged.
- `registry.SetLastActivityForTest(sessionID, t)` — deterministic activity clock for D-04.
- `registry.GetSession(sessionID)` — read-after-check validation in tests.
- `slog.Debug` line at `stale.go:47` — already structurally correct (key=sessionId + pid + elapsed). Just extend comment (D-08).
- `scripts/bump-version.sh` + `scripts/phase-NN/verify.{sh,ps1}` — Phase 7/8 templates drop-in usable.

### Established Patterns
- Immutable copy-before-modify in registry mutation (registry.go:149, :201, :232). Stale.go does not mutate — no change here.
- External test package `session_test` (stale_test.go:1). Phase 9 tests stay in same external package.
- go:embed-free stale.go — no config coupling. Guard change is pure logic.
- Phase 7/8 commit style: `test(NN-MM): RED failing` → `fix(NN-MM): GREEN`. Phase 9 planner reproduces.

### Integration Points
- D-01 fix is local to `session/stale.go` `CheckOnce` function. No API surface change, no registry change, no server change.
- D-03 test inversion is local to `session/stale_test.go`. No cross-file coupling.
- D-06 release artefacts touch `CHANGELOG.md`, `main.go`, `plugin.json`, `marketplace.json`, `start.{sh,ps1}`, `scripts/phase-09/verify.{sh,ps1}` — standard Phase 7/8 footprint.

### Tests affected
- `session/stale_test.go` — invert D-03 test + add 3 new regression tests (D-05 scenarios a/b/c/d).
- `session/registry_dedup_test.go` — MUST stay green; Phase 9 does not touch registry.
- `session/*_test.go` generally — MUST stay green; only stale_test.go changes.

</code_context>

<specifics>
## Specific Ideas

- **Reproduction scenario the fix must pass (MANUAL):** Start Claude Code, let dsrcode register a UUID-sourced session, trigger a chain of MCP/long-thinking calls with ≥2min between hook events. Daemon must remain up, session must remain in registry, no auto-exit.
- **Live-log anchor:** `~/.claude/dsrcode.log` 2026-04-18 11:37–11:44, session `2fe1b32a-ea1d-464f-8cac-375a4fe709c9`, PID 5692. Include this exact excerpt in CHANGELOG [4.2.1] and regression-test header comment.
- **Release note angle:** "Daemon no longer auto-exits during long-running UUID-sourced Claude Code sessions" — user-visible, matches reported symptom, credits the live incident.
- **Test naming convention (Phase-7 parity):** `TestStaleCheckSkipsPidCheckForClaudeSource`, `TestStaleCheckRemovesClaudeSourceAfterRemoveTimeout`, `TestStaleCheckSurvivesUuidSourcedLongRunningAgent`, `TestStaleCheckEmitsDebugOnGuardSkip`.

</specifics>

<deferred>
## Deferred Ideas

### HTTP-Server-Bind-Retry + Shutdown-Race (separate bug, surfaced during Phase-9 log analysis)
**Evidence:** `~/.claude/dsrcode.log` 2026-04-18 11:58:17 — daemon startup hit `bind: [port in use]`, triggered fatal shutdown (coalescer + Discord cleanup ran). 6 seconds later a hook arrived → "new session during grace period, cancelling shutdown" → daemon continued somehow. Log trail is inconsistent — HTTP server state unclear.
**Root cause hypothesis:** start.ps1/stop.ps1 timing race — old daemon not fully killed before new daemon spawns, or OS TIME_WAIT on port 19460. Fatal error handling in main.go auto-exit path may be incomplete.
**Standard fix (Exa evidence):** StackOverflow/captncraig pattern — detect `syscall.EADDRINUSE`, retry with backoff. golang/go #36819 (graceful-shutdown-race, CL 409537 patched in Go 1.19+) may also apply.
**Scope:** stop.ps1 port-release wait + server.go bind-retry + shutdown-sequence hardening. Separate root cause from D-01.
**Target:** v4.2.2 or `/gsd-quick` after v4.2.1 ships.

### PID-Recycling Edge Case (openclaw PR #26443)
Container environments (Docker, Podman, Kubernetes) can recycle PIDs. The industry-standard detection is `(pid, starttime)` comparison reading `/proc/pid/stat` field 22 (psutil pattern). For dsrcode this would further harden `IsPidAlive` semantics in containerized dev setups. Out-of-scope for Phase 9 hotfix; the E-refined guard already skips the PID check entirely for the sources we care about. Revisit if container-PID-recycling reports arrive.

### Classification Refactor (Option A — Server-side source override)
Moving source classification from `sourceFromID(id)` heuristic to call-site explicit (HTTP handler passes `SourceHTTP`, direct spawn — if ever implemented — passes `SourceClaude`) would be semantically cleaner. Blocked by `registry_dedup_test.go` upgrade-chain invariants. Revisit if a direct-spawn path is ever added or the upgrade chain is redesigned.

### Refcount replacement, `/metrics` endpoint, X-Claude-PID enforcement upstream
Unchanged from Phase 7 deferred list.

### Reviewed Todos (not folded)
(No pending todos reviewed and deferred — none matched Phase 9 scope.)

</deferred>

<success_hints>
## Hints for Researcher & Planner

- **Plan count sanity:** 2-3 plans is the right size. E.g. `09-01` (fix + tests) + `09-02` (release artefacts). 4+ plans is over-decomposition for this 1-line fix.
- **Do NOT touch:** `registry.go`, `source.go`, `server/server.go`, any Phase-1/2/3/6/7/8 file outside the listed canonical refs. Scope is extremely narrow.
- **Do verify:** after GREEN, run the manual reproduction scenario (MCP-heavy session >2min silence) end-to-end before marking Phase 9 done.
- **Release target:** `v4.2.1`. `bump-version.sh 4.2.1` handles the 5-file propagation. User ships the git tag + push per CLAUDE.md §Releasing (not Claude's step — `3-tag-push-limit` memory).
- **MCP mandate:** per user directive, PRE+POST MCP rounds (`sequential-thinking`, `exa`, `context7`, `crawl4ai`) are mandatory at every task boundary during execution, same as discussion.

</success_hints>

---

*Phase: 09-fix-stale-session-false-positive-for-uuid-sourced-claude-ses*
*Context gathered: 2026-04-18*
*Target release: v4.2.1 hotfix*
