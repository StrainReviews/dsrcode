# Phase 9: Fix stale-session false-positive for UUID-sourced Claude sessions — Research

**Researched:** 2026-04-18
**Domain:** Go stdlib session-lifecycle guard expansion + hotfix release artefacts (v4.2.1)
**Confidence:** HIGH

## Summary

Phase 9 is an extremely narrow hotfix: one symbol insertion at `session/stale.go:41`, one test rename + assertion inversion, three new regression tests, and the standard v4.2.1 release artefact set (CHANGELOG + `bump-version.sh` + `scripts/phase-09/verify.{sh,ps1}`). Every decision D-01..D-09 is pre-locked in `09-CONTEXT.md`; the planner does not need to re-debate alternatives.

Research confirmed all five focus areas the orchestrator flagged: (1) the `log/slog` `bytes.Buffer` test pattern is already used in this repo at `coalescer/coalescer_test.go:267-282` — no new library call to validate, drop-in reusable; (2) the Phase-7 test-inversion pattern is a simple rename + assertion flip while preserving the file-ownership split that made Wave 1 parallel-safe; (3) `bump-version.sh` touches exactly the five canonical files; (4) `scripts/phase-08/verify.{sh,ps1}` templates are structurally correct and can be copied near-verbatim with only T-assertion content swapped; (5) Keep-a-Changelog 1.1.0 section ordering for `[4.2.1]` follows the exact `[4.1.2]` precedent (Fixed → Changed → Added). (6) Nyquist validation maps cleanly to unit + manual-repro evidence.

**Primary recommendation:** Execute as two plans — `09-01` (stale.go one-line guard + godoc comment + four regression tests) and `09-02` (v4.2.1 release harness). Git tag + push remains user-owned per `CLAUDE.md §Releasing` and the `3-tag-push-limit` memory.

<user_constraints>
## User Constraints (from 09-CONTEXT.md)

### Locked Decisions (D-01..D-09 — treat as authoritative, do NOT re-debate)

- **D-01 — Guard expansion:** `session/stale.go:41` from `s.PID > 0 && s.Source != SourceHTTP && !IsPidAlive(s.PID)` to `s.PID > 0 && s.Source != SourceHTTP && s.Source != SourceClaude && !IsPidAlive(s.PID)`. One-line diff. Rationale: UUID `SourceClaude` sessions in production arrive via HTTP hooks with wrapper PID (start.sh/start.ps1) that exits within seconds — not authoritative for Claude-process liveness. `SourceJSONL` has `PID=0` (already excluded by `s.PID > 0` gate).
- **D-02 — No platform gate:** No `runtime.GOOS` branch. Source-based fix is correct for both Windows (orphans not reparented to PID 1) and Unix (start.sh exits before Claude does).
- **D-03 — Test inversion:** Rename `TestStaleCheckPreservesPidCheckForPidSource` → `TestStaleCheckSkipsPidCheckForClaudeSource`. Invert assertion: SourceClaude + dead PID + 5min activity now *survives*. Add backstop test `TestStaleCheckRemovesClaudeSourceAfterRemoveTimeout` (31min → removed via `removeTimeout`, not PID liveness).
- **D-04 — Test framework:** Keep `SetLastActivityForTest` helper (Phase-7 style). NOT `testing/synctest` — `CheckOnce` is synchronous without goroutines/timers to advance against.
- **D-05 — Four regression tests:**
  - (a) `TestStaleCheckSkipsPidCheckForClaudeSource` — inverted Phase-7 test
  - (b) `TestStaleCheckRemovesClaudeSourceAfterRemoveTimeout` — 30min backstop
  - (c) `TestStaleCheckSurvivesUuidSourcedLongRunningAgent` — live-repro mirror with incident-anchored header comment
  - (d) `TestStaleCheckEmitsDebugOnGuardSkip` — custom `bytes.Buffer` slog handler asserts `"PID dead but session has recent activity, skipping removal"` fires with correct attributes
- **D-06 — v4.2.1 full release set:** `CHANGELOG.md [4.2.1]`, `scripts/bump-version.sh 4.2.1`, `scripts/phase-09/verify.sh` + `verify.ps1` (structural mirror of Phase-8 harness).
- **D-07 — slog line UNCHANGED:** `stale.go:47` `slog.Debug("PID dead but session has recent activity, skipping removal", ...)` stays exactly as-is. After D-01 it also fires for `SourceClaude` — confirmed live in production logs (session `8273c952-…`, pid=25316, 8x firings).
- **D-08 — 15-line godoc comment:** Replace 3-line comment at `stale.go:38-42` with the extended godoc spelling out why both sources are covered and noting platform rationale. Exact wording in `09-CONTEXT.md §D-08`.
- **D-09 — CHANGELOG forensic anchor:** `[4.2.1]` entry quotes live-incident log excerpt (`dsrcode.log` 2026-04-18 11:37-11:44, session `2fe1b32a-ea1d-464f-8cac-375a4fe709c9`, PID 5692, 8x skipping-removal debug lines before removal at 2m25s) as the reproducible trail.

### Claude's Discretion

- Exact slog attribute ordering and key names in regression tests — mirror existing Phase-7 test style.
- Bash vs PowerShell idiom details in verify scripts — mirror Phase 8 patterns verbatim.
- Test-file layout: new file vs append to existing `session/stale_test.go` — planner decides based on test count (recommendation: keep one file `session/stale_test.go`, grows from 55 lines to ~200 lines — still under the 800-line soft cap).

### Deferred Ideas (OUT OF SCOPE — do NOT plan)

- HTTP-bind-retry + shutdown-race (v4.2.2 candidate — separate root cause surfaced in live log 11:58:17 port collision)
- PID-recycling via `(pid, starttime)` — container edge case, substantial scope
- Classification refactor (Option A: server-side source override) — breaks `registry_dedup_test.go` upgrade-chain invariants
- Refcount replacement, `/metrics` endpoint, UI surfaces — unchanged from Phase 7 deferred list
</user_constraints>

<phase_requirements>
## Phase Requirements

Phase 9 maps `D-01..D-09` (locked in `09-CONTEXT.md`) to research support. No new `REQUIREMENTS.md` IDs are introduced — this is a narrow hotfix extension of Phase-7 D-01/D-02.

| ID | Description | Research Support |
|----|-------------|------------------|
| D-01 | Guard expansion: `s.Source != SourceClaude` added to PID-liveness skip at stale.go:41 | `session/stale.go:38-48` read verbatim — current guard at line 41 matches CONTEXT exactly; `session/source.go:13-17` confirms `SourceClaude = 2` enum value in same package (no import needed). |
| D-02 | Source-based fix, no `runtime.GOOS` gate | `session/stale_unix.go` / `session/stale_windows.go` exist but contain only `IsPidAlive` platform impl — NOT the guard. Guard lives in shared `stale.go`, confirmed by `ls session/` + file inspection. |
| D-03 | Phase-7 test inversion + rename | `session/stale_test.go:38-54` read verbatim — the exact `TestStaleCheckPreservesPidCheckForPidSource` body to invert is present at the documented line numbers. |
| D-04 | `SetLastActivityForTest` test helper | `session/registry.go:451-461` read verbatim — helper exists under `r.mu.Lock()` and takes `sessionID, t time.Time`. Matches CONTEXT D-04 description. |
| D-05 (a,b,c) | 3 positive-path regression tests | All reachable via existing `session.NewRegistry`, `reg.StartSession`, `reg.SetLastActivityForTest`, `session.CheckOnce`, `reg.GetSession` — confirmed API surface. |
| D-05 (d) | slog `bytes.Buffer` custom handler | In-repo precedent at `coalescer/coalescer_test.go:267-282` — exact pattern works with `slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))` + `defer slog.SetDefault(origLogger)`. Drop-in reusable. [VERIFIED: repo grep] |
| D-06 | v4.2.1 full release set | `scripts/bump-version.sh` read verbatim — touches the 5 canonical files listed in CONTEXT §D-06 in one pass; `scripts/phase-08/verify.{sh,ps1}` read verbatim — structural template drop-in usable. |
| D-07 | slog line unchanged | `session/stale.go:47` read verbatim — existing line format matches CONTEXT `PID dead but session has recent activity, skipping removal` with `sessionId`/`pid`/`elapsed` attributes. No code change needed. |
| D-08 | 15-line godoc comment | Exact wording provided in `09-CONTEXT.md §D-08`. Current 3-line comment at `stale.go:38-40` confirmed by read. |
| D-09 | CHANGELOG forensic anchor | `CHANGELOG.md` read verbatim — format `## [4.1.2] - 2026-04-13` with bold-lead-phrase bullets + D-NN citations is the exact template to mirror for `## [4.2.1] - 2026-04-18`. |
</phase_requirements>

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|--------------|----------------|-----------|
| Session stale-detection logic | Daemon / Go stdlib | — | `CheckOnce` runs in-process in the daemon every `interval` seconds; no network or storage tier involved. [VERIFIED: `session/stale.go:15-63`] |
| PID-liveness probe | OS / Platform | Daemon | `IsPidAlive` implementation is platform-specific (`stale_unix.go` vs `stale_windows.go`); called by the daemon-owned guard but depends on OS process table. Phase 9 does NOT touch this layer — only the guard around it. [VERIFIED: `ls session/` shows both files] |
| Session registry mutation | Daemon (in-memory) | — | `EndSession` is called only from `CheckOnce` after the guard; Phase 9's effect is to call it *less often* (skip for SourceClaude). No new mutation path. [VERIFIED: `session/stale.go:44,52-53`] |
| Test assertions | Test tier (external `session_test` package) | — | External test package convention (`stale_test.go:1 package session_test`) unchanged. New regression tests slot into the same file. [VERIFIED: `session/stale_test.go:1`] |
| Log emission | Observability (slog) | — | `slog.Debug` and `slog.Info` from `session/stale.go` — Phase 9 keeps the lines unchanged; test (d) asserts the Debug line fires. [VERIFIED: `session/stale.go:43,47,52,59`] |
| Release artefact propagation | Build / CI tier | — | `scripts/bump-version.sh` + GoReleaser on tag push. No Claude-side execution of `git tag` (user-owned per CLAUDE.md §Releasing). [VERIFIED: `scripts/bump-version.sh` + CLAUDE.md] |

**Why this matters:** the capability map confirms the 1-line fix and its tests live entirely within a single package (`session/`), with no cross-tier coupling. This is why 2 plans is the right scope — not 4+.

## Project Constraints (from CLAUDE.md)

Extracted directives that MUST be honored during planning:

1. **GSD workflow enforcement** — all edits must flow through a GSD command; Phase 9 satisfies this via `/gsd-plan-phase 9` → `/gsd-execute-phase 9`.
2. **Releasing procedure** — `git tag` + `git push --tags` is the USER's step, never Claude's. Claude runs `bump-version.sh 4.2.1` and stages the files; user executes the tag + push. Reinforced by the `3-tag-push-limit` memory (never push >3 new tags at once; GitHub silently drops tag webhooks).
3. **Testing gate** — `go test -v ./...` must pass before the user commits. Phase 9 execution plan must run this as a verification step.
4. **Model pricing table** — not relevant to Phase 9 (no pricing code touched).
5. **MCP mandate (user directive)** — PRE+POST MCP rounds (sequential-thinking, exa, context7, crawl4ai) mandatory at every task boundary. This is an execution-time constraint, not a planning-time one, but the plan must document the expectation.

## Standard Stack

### Core (all already in use; Phase 9 adds nothing new)

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go stdlib `log/slog` | Go 1.26 | Structured logging; Debug line + test assertion | Already the project's logging backbone; no alternatives considered. [VERIFIED: `go version go1.26.0 windows/amd64`] |
| Go stdlib `testing` | Go 1.26 | Test framework for regression tests (a), (b), (c), (d) | Idiomatic Go; external test package `session_test` already established. [VERIFIED: `session/stale_test.go:1`] |
| Go stdlib `bytes` | Go 1.26 | `bytes.Buffer` as slog handler sink for test (d) | Exact pattern already in repo at `coalescer/coalescer_test.go:267-282`. [VERIFIED: in-repo grep] |
| Go stdlib `time` | Go 1.26 | `time.Now().Add(-150*time.Second)` for backdating test timestamps | Phase-7 convention; matches existing `stale_test.go` style. [VERIFIED: `session/stale_test.go:26,48`] |

### Supporting (no new dependencies)

No new dependencies are added in Phase 9. `go.mod` MUST NOT change. Verify via `git diff go.mod` returns empty after Phase 9 lands.

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `bytes.Buffer` + `slog.NewTextHandler` for D-05(d) | `testing/slogtest.TestHandler` | `slogtest.TestHandler` validates handler *implementations*, not log *content* — not what we need. `bytes.Buffer` + grep on `buf.String()` is the correct tool. [CITED: go.dev/pkg/testing/slogtest] |
| Custom `SetLastActivityForTest` | `testing/synctest.Test` + virtual clock | Phase-8 uses `synctest` for rate.Limiter virtual time; Phase-9's `CheckOnce` has no goroutines/timers to advance against (locked via D-04). [VERIFIED: `session/stale.go:31-63` is linear sync code] |
| New file `session/stale_regression_test.go` | Append to existing `session/stale_test.go` | Existing file is 55 lines; adding 3 tests brings it to ~200 lines. Still well under the 800-line soft cap from `common/coding-style.md`. Single-file keeps the blame trail compact. [Discretion per CONTEXT] |

**Installation:** No `npm install` / `go get` needed. Zero dependency changes. [VERIFIED: `09-CONTEXT.md §out-of-scope`]

**Version verification:** The `golang.org/x/time/rate v0.15.0` added in Phase 8 (per 08-04 CHANGELOG) is untouched. No version bumps needed.

## Architecture Patterns

### System Architecture Diagram (Phase 9 change surface)

```
   Claude Code (UUID session)
            |
            v
   HTTP POST /hooks/pre-tool-use
            |  (carries wrapper-launcher PID via X-Claude-PID or os.Getppid)
            v
   server.handleHook  ---------------->  registry.StartSession(req, wrapperPID)
                                              |
                                              v
                                         sourceFromID("uuid") => SourceClaude
                                         stores Session{PID: wrapperPID, Source: SourceClaude}
                                              |
   (wrapper process exits seconds later; PID is now orphaned)
                                              |
   every ~interval ticks:                     v
   CheckOnce ----reads----> registry.GetAllSessions()
       |
       v
   for each s:
     elapsed = now - s.LastActivityAt
     if s.PID > 0 && s.Source != SourceHTTP && s.Source != SourceClaude  <== D-01 GUARD
                                    && !IsPidAlive(s.PID) {
        // BEFORE Phase 9: this branch was entered for UUID sessions and called EndSession
        // AFTER Phase 9: guard skips the branch; removeTimeout backstop still reaches zombies
     }
     if elapsed > removeTimeout { EndSession }   // 30min backstop (unchanged)
     if elapsed > idleTimeout  { TransitionToIdle } // 10min idle (unchanged)
```

The arrow from `if s.PID > 0 && s.Source != SourceHTTP && s.Source != SourceClaude` is the *only* logic change. Everything upstream and downstream is preserved.

### Recommended Project Structure (no change)

```
session/
├── stale.go          # MODIFIED: 1-line guard + 15-line godoc (D-01, D-08)
├── stale_test.go     # MODIFIED: rename 1 test + invert + add 3 regression tests (D-03, D-05)
├── stale_unix.go     # UNCHANGED: IsPidAlive Unix impl
├── stale_windows.go  # UNCHANGED: IsPidAlive Windows impl (tasklist)
├── source.go         # UNCHANGED: SessionSource enum, sourceFromID
├── registry.go       # UNCHANGED: SetLastActivityForTest helper used by tests
└── registry_dedup_test.go  # UNCHANGED: 20+ upgrade-chain tests — MUST stay green

scripts/phase-09/     # NEW: verify harness directory (mirror scripts/phase-08/)
├── verify.sh         # NEW: T1..Tn bash smoke
└── verify.ps1        # NEW: T1..Tn PowerShell smoke
```

### Pattern 1: In-package enum guard (D-01)

**What:** Extend a boolean guard with another enum-equality check in the same condition expression.
**When to use:** Hotfix tightening a previously-under-constrained condition; source and target enum values live in the same package so no new imports are needed.
**Example:**
```go
// Source: session/stale.go:41 (post-Phase-9)
if s.PID > 0 && s.Source != SourceHTTP && s.Source != SourceClaude && !IsPidAlive(s.PID) {
    // only SourceJSONL reaches this branch — and SourceJSONL has PID=0 by design,
    // so the s.PID > 0 gate already excludes it. Net effect: the branch is now dead
    // for JSONL and for both HTTP-hook-delivered source kinds (HTTP synthetic + UUID).
    // The removeTimeout backstop below is the only remaining termination path for
    // sessions whose PID is not authoritative.
}
```

### Pattern 2: bytes.Buffer slog capture for log-content assertion (D-05 d)

**What:** Swap `slog.Default()` with a JSON/Text handler that writes to a `bytes.Buffer`, exercise the code under test, then grep the buffer's string content.
**When to use:** Asserting that a specific log line was emitted with the right attributes; *not* for handler-contract conformance testing (that's what `slogtest.TestHandler` is for).
**Example:**
```go
// Source: coalescer/coalescer_test.go:267-282 (verified in-repo pattern)
func TestStaleCheckEmitsDebugOnGuardSkip(t *testing.T) {
    var buf bytes.Buffer
    origLogger := slog.Default()
    slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
    defer slog.SetDefault(origLogger)

    reg := session.NewRegistry(func() {})
    req := session.ActivityRequest{SessionID: "uuid-abcdef12-3456-7890-abcd-ef1234567890", Cwd: "/tmp/p"}
    reg.StartSession(req, 99999999) // dead PID
    reg.SetLastActivityForTest(req.SessionID, time.Now().Add(-30*time.Second)) // inside grace

    session.CheckOnce(reg, 10*time.Minute, 30*time.Minute)

    out := buf.String()
    if !strings.Contains(out, "PID dead but session has recent activity, skipping removal") {
        t.Errorf("D-05(d) violation: expected debug line not emitted; got: %q", out)
    }
    if !strings.Contains(out, "sessionId=") || !strings.Contains(out, "pid=") || !strings.Contains(out, "elapsed=") {
        t.Errorf("D-05(d) violation: expected attributes missing; got: %q", out)
    }
}
```

Note the 30-second activity backdate keeps `elapsed < 2*time.Minute` so the Debug branch fires rather than the Info-remove branch. The existing slog line at `stale.go:47` is in the `else` path of the `elapsed > 2*time.Minute` check inside the now-unreachable-for-SourceClaude block — **but** after D-01 the outer guard skips the entire block for `SourceClaude`, so the Debug line never fires via stale.go for SourceClaude.

**CRITICAL DISCOVERY (corrects CONTEXT D-07 assumption):** Reading `session/stale.go:41-48` carefully, the slog.Debug line at :47 is *inside* the `if s.PID > 0 && s.Source != SourceHTTP && !IsPidAlive(s.PID)` block. After D-01 widens that guard to also exclude `SourceClaude`, the block is entered *only for SourceJSONL* (which has PID=0 and can't enter anyway) or for any future non-enumerated source. **For `SourceClaude`, the Debug line DOES NOT fire after the fix.**

This means CONTEXT §D-05(d) wording "slog-debug assertion confirms the line fires for the guard skip" is **semantically ambiguous**:
- Option α: Test asserts the line fires for an OLD-style HTTP session that's *not* yet skipped (but HTTP is already in the guard, so this is also skipped) — contradicts D-01.
- Option β: Test asserts the line DOES NOT fire for SourceClaude after D-01 (negative assertion on the log).
- Option γ: Test uses a hypothetical "source that is gated but where the inner grace path is reachable" — impossible after D-01.

**Resolution for planner:** The intent of D-05(d) is "observability guard — prove the slog line is structurally correct and fires on the code path that still exists." Three options for the planner:
1. **Remove test (d)** and rely on the live-log evidence already documented in D-07 (the Debug line fires 8x in production for session `8273c952-…`). Downside: no automated regression.
2. **Adapt test (d)** to assert the Info-remove line (`"removing stale session (PID dead, no recent activity)"`) fires when a *hypothetical future SourceX* without a guard entry is added. Downside: requires exposing a test hook or a new test-only source. Over-engineered.
3. **Adapt test (d)** as a *negative* assertion: with `SourceClaude` + dead PID + elapsed < 2min, the Debug line from stale.go:47 MUST NOT fire (because the outer guard skips the whole block). This is a useful regression — it proves the guard *actually* skips the block, not just the `EndSession` call.

**Planner recommendation:** Go with option 3 (negative assertion). It's the strongest automated evidence D-01 is implemented correctly. Flag this to the user at discuss/plan time because it's a semantic refinement of D-05(d) not a full re-debate.

Confidence: HIGH (code inspection confirms the block structure; there's no way for the Debug line at :47 to fire for SourceClaude after D-01).

### Pattern 3: Test rename + assertion invert (D-03)

**What:** Rename a test function, flip the assertion from removed → survives (or vice versa), keep the setup identical.
**When to use:** The previous test codified a theoretical path that production doesn't take; the real production path has the opposite outcome.
**Example:**
```go
// BEFORE (session/stale_test.go:38-54):
func TestStaleCheckPreservesPidCheckForPidSource(t *testing.T) {
    // ... setup with UUID session, dead PID, 5min activity ...
    session.CheckOnce(reg, 10*time.Minute, 30*time.Minute)
    if s := reg.GetSession("abcdef12-..."); s != nil {
        t.Error("D-02 violation: PID-sourced session with dead PID and old activity NOT removed")
    }
}

// AFTER (Phase 9):
func TestStaleCheckSkipsPidCheckForClaudeSource(t *testing.T) {
    // ... identical setup ...
    session.CheckOnce(reg, 10*time.Minute, 30*time.Minute)
    if s := reg.GetSession("abcdef12-..."); s == nil {
        t.Error("D-01 Phase 9 violation: SourceClaude session removed by PID-liveness check despite the SourceClaude guard")
    }
}
```

Git blame stays useful because the surrounding setup block is byte-identical; only the function name and the `!= nil` → `== nil` flip change.

### Anti-Patterns to Avoid

- **Anti-pattern 1:** Adding a `runtime.GOOS == "windows"` branch around the guard. CONTEXT D-02 forbids this. Both platforms benefit from the source-based skip.
- **Anti-pattern 2:** Introducing a new `PIDIsEphemeral bool` field on `Session` to flag the case. Over-engineered; E-refined achieves the same with zero surface change.
- **Anti-pattern 3:** Removing the 2-minute grace block. D-02 keeps it — it only affects the now-dead path and has zero observable impact after D-01 widens the outer guard.
- **Anti-pattern 4:** Adding a config toggle for the guard. No config key added (matches Phase-7 D-03 precedent).
- **Anti-pattern 5:** Running `git tag v4.2.1 && git push --tags` from Claude's execution. Forbidden by CLAUDE.md §Releasing and `3-tag-push-limit` memory.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Version propagation across 5 files | Custom sed loops in execute-plan tasks | `scripts/bump-version.sh 4.2.1` | Already idempotent, tested by Phases 7 + 8, touches the correct 5 files in one pass. [VERIFIED: `scripts/bump-version.sh:29-51`] |
| Live-daemon verification harness | Ad-hoc bash snippets | `scripts/phase-08/verify.{sh,ps1}` as template | Proven structure with log-offset idiom, cleanup-on-exit trap, pass/fail counters. Mirror verbatim, swap T-assertions. [VERIFIED: `scripts/phase-08/verify.sh:36-63`] |
| CHANGELOG formatting | Free-form prose | Keep-a-Changelog 1.1.0 `Fixed / Changed / Added` | Matches all prior entries; `[4.1.2]` is the exact voice template. [CITED: keepachangelog.com/en/1.1.0 — confirmed against `CHANGELOG.md:29-37`] |
| Slog log-line capture in tests | Custom writer wrapping `os.Stderr` | `slog.SetDefault(slog.New(slog.NewTextHandler(&buf, ...)))` + defer restore | Already the in-repo idiom. [VERIFIED: `coalescer/coalescer_test.go:267-282`] |
| Test-deterministic activity backdating | `time.Sleep` in tests | `SetLastActivityForTest(id, time.Now().Add(-Δ))` | Already the Phase-7 idiom in `session/stale_test.go`. [VERIFIED: `session/stale_test.go:26,48`] |

**Key insight:** This phase has zero net-new infrastructure. Every tool it needs is in the repo from prior phases. The planner's job is to wire them together in the Phase-7/Phase-8 style.

## Runtime State Inventory

Phase 9 is a pure code-and-tests fix with a follow-on release artefact set. There is NO runtime-state migration category with surprising items, but per the researcher protocol every category must be answered explicitly:

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | **None** — verified by `09-CONTEXT.md §Integration Points` and by reading `session/registry.go` (all session data is in-memory only; persistence is via `~/.claude/dsrcode.log` append-only log). Phase 9 does not change the Session struct shape. | — |
| Live service config | **None** — no external services reference the guard logic; Discord RPC, n8n, Cloudflare, etc. are unaware of the session-lifecycle semantics. | — |
| OS-registered state | **None** — `IsPidAlive` queries the OS process table but is read-only; no OS registration exists referencing the specific guard. The `dsrcode.pid` file at `~/.claude/dsrcode.pid` records only the daemon PID, unaffected by session guards. | — |
| Secrets / env vars | **None** — no env var controls the guard. Client ID `1455326944060248250` is hardcoded and unchanged. [VERIFIED: CLAUDE.md §Configuration] | — |
| Build artifacts / installed packages | **Binary rebuild required.** The shipped `dsrcode` binary from v4.2.0 has the old 2-source guard. After v4.2.1 lands, users must update via `/dsrcode:update` or re-download. The `go.sum` / `go.mod` are unchanged. CI via GoReleaser (triggered by `git push --tags`) builds fresh binaries for 5 platforms. | User runs `/dsrcode:update` (existing slash command DSR-30) after v4.2.1 GitHub Release is published. |

**The canonical question answered:** After every file in the repo is updated and v4.2.1 is tagged, the only remaining runtime item is the user's locally-running daemon binary — which auto-updates via the existing update path. No data migration.

## Common Pitfalls

### Pitfall 1: Forgetting the Debug line becomes unreachable for SourceClaude

**What goes wrong:** Planner writes test (d) assuming the Debug line still fires for `SourceClaude` after D-01. It does not — the outer guard skips the whole block.
**Why it happens:** CONTEXT §D-07 says "fires for `SourceClaude` as well (confirmed already firing in the live log)" — but that live-log evidence is from BEFORE D-01 is applied. After D-01 widens the guard, the line fires only for hypothetical future sources that aren't guarded.
**How to avoid:** Planner codifies test (d) as a *negative* assertion (Debug line MUST NOT fire for SourceClaude after D-01) or drops test (d) entirely in favor of live-log evidence. Recommendation: negative assertion (Option 3 in Pattern 2 above).
**Warning signs:** Test (d) written with `strings.Contains(buf.String(), "PID dead but session has recent activity, skipping removal")` as a positive assertion against a `SourceClaude` session — this will ALWAYS fail after D-01 lands.

### Pitfall 2: Accidentally removing the 2-minute grace block

**What goes wrong:** Planner misreads the godoc change and deletes the `if elapsed > 2*time.Minute` inner block, thinking it's now dead code.
**Why it happens:** After D-01 the block is unreachable for SourceHTTP and SourceClaude. The planner may assume this makes the whole block dead and remove it.
**How to avoid:** Leave the inner block verbatim. Any future source added that is NOT guarded (e.g. a direct-spawn path if ever implemented) will need the 2-minute grace. Removing it now would be a latent regression.
**Warning signs:** Diff of `session/stale.go` shows more than the 1-line guard change + godoc replacement. Target diff size: `+15 lines (godoc) / −3 lines (old godoc) / +1 symbol (`&& s.Source != SourceClaude`)` = net +13 lines. Anything larger is over-modification.

### Pitfall 3: `dsrcode` binary not on PATH during verify.sh T1

**What goes wrong:** `verify.sh` T1 runs `dsrcode --version` and fails because the binary is in `./dsrcode` (repo root) not on PATH.
**Why it happens:** Phase 8 verify.sh assumes the binary is installed globally; for local `go build -o dsrcode .` development it's only in the repo root.
**How to avoid:** Mirror Phase-8 verify.sh verbatim and note the precondition ("binary must be on PATH or invoked as `./dsrcode`"). Let the user install it before running T1, or use `${DSRCODE_BIN:-dsrcode}` env-var fallback.
**Warning signs:** T1 FAIL with `dsrcode: command not found` — this is a test-environment issue, not a bug.

### Pitfall 4: `CHANGELOG.md [4.2.1]` inserted in wrong position

**What goes wrong:** Entry placed below `[4.1.2]` or at the bottom of the file.
**Why it happens:** Markdown append is easier than insert; planner task doesn't specify the exact insert line.
**How to avoid:** Insert at line 10 (immediately after `## [Unreleased]` on line 8, immediately before `## [4.2.0]` on line 10). Acceptance criterion: `grep -n '## \[4.2.1\]' CHANGELOG.md` returns a line number lower than `grep -n '## \[4.2.0\]'`.
**Warning signs:** Running `head -20 CHANGELOG.md` after the edit does not show `[4.2.1]` at the top.

### Pitfall 5: User's manual `git tag` skipped

**What goes wrong:** Plan marked complete without user running `git tag v4.2.1 && git push --tags`; GoReleaser never triggers; no GitHub Release is cut.
**Why it happens:** The tag+push is explicitly a user step (CLAUDE.md §Releasing + `3-tag-push-limit` memory). Claude must not automate it.
**How to avoid:** Plan 09-02 ends with a `checkpoint:human-verify` gate task (Phase-8 pattern), instructing the user on the exact command. Do not mark SUMMARY until the user types "approved" and confirms the tag is pushed.
**Warning signs:** No new tag visible in `git tag --list 'v4.2.*'` after plan completion.

## Code Examples

### Example 1: D-01 guard expansion (stale.go:41)

```go
// Source: session/stale.go (verified current state at line 41)

// BEFORE Phase 9:
if s.PID > 0 && s.Source != SourceHTTP && !IsPidAlive(s.PID) {

// AFTER Phase 9 (one symbol insertion):
if s.PID > 0 && s.Source != SourceHTTP && s.Source != SourceClaude && !IsPidAlive(s.PID) {
```

### Example 2: D-08 15-line godoc comment (stale.go:38-42)

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

### Example 3: D-05(a) `TestStaleCheckSkipsPidCheckForClaudeSource` (inverted test)

```go
// Source: derived from session/stale_test.go:38-54 (inverted per D-03)
// TestStaleCheckSkipsPidCheckForClaudeSource verifies D-01 Phase 9: SourceClaude
// (UUID) sessions with a dead PID are NOT removed by the PID-liveness check in
// production. Their PID is the short-lived wrapper-launcher (start.sh /
// start.ps1), not the Claude Code process itself. Only the removeTimeout
// backstop (30min default) should remove them.
func TestStaleCheckSkipsPidCheckForClaudeSource(t *testing.T) {
    reg := session.NewRegistry(func() {})

    req := session.ActivityRequest{
        SessionID: "abcdef12-3456-7890-abcd-ef1234567890", // no prefix => SourceClaude
        Cwd:       "/home/user/project",
    }
    reg.StartSession(req, 99999999) // dead PID

    // Activity 5 minutes ago: well past the 2-minute grace, before removeTimeout.
    reg.SetLastActivityForTest(req.SessionID, time.Now().Add(-5*time.Minute))

    session.CheckOnce(reg, 10*time.Minute, 30*time.Minute)

    if s := reg.GetSession(req.SessionID); s == nil {
        t.Fatal("D-01 Phase 9 violation: SourceClaude session removed by PID-liveness check despite the SourceClaude guard")
    }
}
```

### Example 4: D-05(b) `TestStaleCheckRemovesClaudeSourceAfterRemoveTimeout` (backstop)

```go
// TestStaleCheckRemovesClaudeSourceAfterRemoveTimeout verifies that the 30-min
// removeTimeout backstop still reaches truly stale SourceClaude sessions —
// zombies that have stopped sending hooks entirely are removed even though
// the PID-liveness check is skipped for them.
func TestStaleCheckRemovesClaudeSourceAfterRemoveTimeout(t *testing.T) {
    reg := session.NewRegistry(func() {})

    req := session.ActivityRequest{
        SessionID: "fedcba98-7654-3210-fedc-ba9876543210", // UUID => SourceClaude
        Cwd:       "/home/user/zombie-project",
    }
    reg.StartSession(req, 99999999) // dead PID (but guard skips the liveness check)

    // Activity 31 minutes ago: past the 30-min removeTimeout backstop.
    reg.SetLastActivityForTest(req.SessionID, time.Now().Add(-31*time.Minute))

    session.CheckOnce(reg, 10*time.Minute, 30*time.Minute)

    if s := reg.GetSession(req.SessionID); s != nil {
        t.Error("D-05(b) violation: SourceClaude session with 31min-old activity NOT removed by removeTimeout backstop")
    }
}
```

### Example 5: D-05(c) `TestStaleCheckSurvivesUuidSourcedLongRunningAgent` (live-repro)

```go
// TestStaleCheckSurvivesUuidSourcedLongRunningAgent mirrors the live incident
// captured in ~/.claude/dsrcode.log 2026-04-18 11:37-11:44: session
// 2fe1b32a-ea1d-464f-8cac-375a4fe709c9, wrapper PID 5692, removed at elapsed
// 2m25s (150s) while Claude Code was still active. The v4.2.0 daemon auto-exited
// 6 seconds later. This test reproduces the exact elapsed interval (150s) from
// the incident and confirms the v4.2.1 fix keeps the session alive.
func TestStaleCheckSurvivesUuidSourcedLongRunningAgent(t *testing.T) {
    reg := session.NewRegistry(func() {})

    req := session.ActivityRequest{
        SessionID: "2fe1b32a-ea1d-464f-8cac-375a4fe709c9", // live-incident UUID
        Cwd:       "/home/user/long-running-project",
    }
    reg.StartSession(req, 5692) // the actual wrapper PID from the incident log

    // 150 seconds: the exact elapsed from the incident before auto-removal triggered in v4.2.0.
    reg.SetLastActivityForTest(req.SessionID, time.Now().Add(-150*time.Second))

    session.CheckOnce(reg, 10*time.Minute, 30*time.Minute)

    if s := reg.GetSession(req.SessionID); s == nil {
        t.Fatal("D-05(c) live-repro violation: incident session 2fe1b32a-... would have been removed again at 150s elapsed — v4.2.1 fix did not hold")
    }
}
```

### Example 6: D-05(d) `TestStaleCheckEmitsDebugOnGuardSkip` (slog buffer) — REVISED per Pitfall 1

```go
// TestStaleCheckEmitsDebugOnGuardSkip verifies observability: after D-01 widens
// the outer guard to skip SourceClaude, the inner slog.Debug line at
// stale.go:47 MUST NOT fire for SourceClaude + elapsed < 2min. (If it did
// fire, the guard skip would only be partial — the test proves the guard
// skips the entire block, not just the EndSession call.)
//
// This is the NEGATIVE form of the CONTEXT D-05(d) assertion, per the
// RESEARCH Pitfall-1 resolution. Positive-form "Debug fires for guard skip"
// is semantically impossible after D-01 because the Debug line is INSIDE
// the block the guard now skips.
func TestStaleCheckEmitsDebugOnGuardSkip(t *testing.T) {
    var buf bytes.Buffer
    origLogger := slog.Default()
    slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
    defer slog.SetDefault(origLogger)

    reg := session.NewRegistry(func() {})
    req := session.ActivityRequest{SessionID: "uuid-guard-skip-test-abcd", Cwd: "/tmp/p"}
    reg.StartSession(req, 99999999) // dead PID
    reg.SetLastActivityForTest(req.SessionID, time.Now().Add(-30*time.Second)) // inside 2min grace

    session.CheckOnce(reg, 10*time.Minute, 30*time.Minute)

    // Negative assertion: the "PID dead but session has recent activity"
    // debug line lives INSIDE the guarded block. After D-01, SourceClaude
    // sessions do not enter that block at all — so the line MUST NOT appear.
    if strings.Contains(buf.String(), "PID dead but session has recent activity") {
        t.Errorf("D-05(d) violation: inner Debug line fired for SourceClaude, proving guard skip is partial; got: %q", buf.String())
    }

    // Positive assertion: the session IS preserved (guard did its job).
    if s := reg.GetSession(req.SessionID); s == nil {
        t.Errorf("D-05(d) consistency violation: SourceClaude session removed despite guard skip expectation")
    }
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Phase-7 single-source guard `s.Source != SourceHTTP` | Phase-9 two-source guard `s.Source != SourceHTTP && s.Source != SourceClaude` | 2026-04-18 (this phase) | UUID-sourced Claude sessions no longer false-positively removed during long MCP-heavy silences |
| Phase-7 test `TestStaleCheckPreservesPidCheckForPidSource` (asserts removal) | Phase-9 test `TestStaleCheckSkipsPidCheckForClaudeSource` (asserts survival) | 2026-04-18 (this phase) | Spec now matches the only production path — UUID sessions delivered via HTTP hooks with wrapper PID |
| `scripts/phase-08/verify.{sh,ps1}` for v4.2.0 release validation | `scripts/phase-09/verify.{sh,ps1}` for v4.2.1 hotfix validation | 2026-04-18 (this phase) | Mirror of the Phase-8 pattern; no structural innovation |

**Deprecated/outdated:**
- The comment at `session/stale.go:38-40` is deprecated by D-08 (replaced with 15-line godoc).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The slog.Debug line at `stale.go:47` becomes unreachable for SourceClaude after D-01 | Pattern 2, Pitfall 1, Example 6 | If wrong, D-05(d) positive assertion would be valid and the negative-assertion refinement is unnecessary. Mitigation: I re-verified via code inspection — line 47 is inside the `if s.PID > 0 && s.Source != SourceHTTP && !IsPidAlive(s.PID)` block; after adding `&& s.Source != SourceClaude` the block skips entirely for SourceClaude. Confidence HIGH via direct source read at stale.go:38-48. [VERIFIED: code read] |
| A2 | `SourceClaude` is an in-package const reachable from `stale.go` without import | D-01 example | If wrong, plan would need `session` import in stale.go which would be a cycle. Mitigation: `source.go:1 // Package session` + `source.go:13-17 const SourceClaude` confirms both files are in `package session`. [VERIFIED: file read] |
| A3 | User's manual `git tag v4.2.1 && git push --tags` will trigger GoReleaser CI | Pitfall 5, Release artefacts | If wrong, release binaries won't be auto-built. Mitigation: CLAUDE.md §Releasing step 3 explicitly states "GitHub Actions runs GoReleaser automatically on tag push"; Phase-7 + Phase-8 both used this path successfully. [CITED: CLAUDE.md §Releasing] |

**Only 3 assumed claims in this research.** All have mitigations or verification paths. Assumption A1 is the one the planner should flag to the user at discuss-time — it refines D-05(d) from positive to negative assertion.

## Open Questions

1. **D-05(d) test form — positive or negative slog assertion?**
   - What we know: The Debug line at stale.go:47 lives INSIDE the block the D-01 guard now skips. For SourceClaude the line cannot fire via stale.go after D-01.
   - What's unclear: CONTEXT §D-05(d) wording suggests positive ("the line fires") which is impossible post-D-01.
   - Recommendation: Planner adopts the NEGATIVE-assertion refinement (Example 6) and flags this to the user during `/gsd-plan-phase` as a 1-line semantic clarification, not a re-debate. If the user prefers Option 1 (drop test (d) entirely), the phase still meets D-01..D-04 + D-05(a,b,c) + D-06..D-09.

2. **Test file layout — single file or split?**
   - What we know: `stale_test.go` currently 55 lines; adding 3 tests + renaming 1 brings it to ~180 lines. Under 800-line cap.
   - What's unclear: Whether separating regression tests into `stale_regression_test.go` improves readability.
   - Recommendation: Keep single file `stale_test.go`. Easier code review, smaller git history surface, still under cap. Formally Claude's discretion per CONTEXT.

3. **`dsrcode --version` T1 precondition in verify.sh**
   - What we know: Phase-8 verify.sh assumes binary on PATH.
   - What's unclear: Whether local dev (binary at `./dsrcode` only) should get a T1 skip-with-warning.
   - Recommendation: Mirror Phase-8 verbatim (fail T1 if `dsrcode` not on PATH). Document the precondition in the script header comment. User can export `PATH=.:$PATH` before running.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|-------------|-----------|---------|----------|
| Go toolchain | Build + tests | ✓ | 1.26.0 windows/amd64 | — [VERIFIED: `go version`] |
| `bash` | `scripts/bump-version.sh`, `scripts/phase-09/verify.sh` | ✓ (Git Bash on Windows) | — | PowerShell equivalents exist |
| PowerShell 5.1+ | `scripts/phase-09/verify.ps1` | ✓ (Windows built-in) | 5.1+ | — |
| `curl` | verify.sh T2-T5 HTTP calls | ✓ | — (Git Bash ships it) | `Invoke-WebRequest` in .ps1 branch |
| `dsrcode` binary on PATH | verify.{sh,ps1} T1 | depends | v4.2.1 post-build | User builds locally via `go build -o dsrcode .` |
| Running dsrcode daemon | verify.{sh,ps1} T2-T6 | depends | v4.2.1 post-restart | User runs `scripts/start.sh` or `start.ps1` before running verify |
| `git` | Commit + tag (user step) | ✓ | — [VERIFIED: git status in init context] | — |
| `sed` | `bump-version.sh` | ✓ (Git Bash) | — | — |
| `grep`, `wc`, `dd` | verify.sh log-offset idiom | ✓ (Git Bash) | — | — |

**Missing dependencies with no fallback:** None.
**Missing dependencies with fallback:** The daemon + binary must be freshly built for verify.{sh,ps1} — this is a user step documented in Plan 09-02.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` 1.26 (standard `go test`) |
| Config file | None (Go convention — no config) |
| Quick run command | `go test -v ./session/... -run TestStaleCheck` |
| Full suite command | `go test -race -v ./...` |
| slog capture pattern | `coalescer/coalescer_test.go:267-282` (reference implementation) |
| Activity backdate helper | `session.SetLastActivityForTest` (registry.go:453) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| D-01 | Guard skips PID-liveness for SourceClaude | unit | `go test -run TestStaleCheckSkipsPidCheckForClaudeSource -v ./session/...` | ❌ Wave 0 (inversion of existing test) |
| D-02 | No `runtime.GOOS` branch introduced | static | `! grep -n 'runtime.GOOS' session/stale.go` (must return empty) | ✅ (code inspection) |
| D-03 | Rename + invert existing Phase-7 test | unit | `go test -run TestStaleCheckSkipsPidCheckForClaudeSource -v ./session/...` AND `! grep -n 'TestStaleCheckPreservesPidCheckForPidSource' session/stale_test.go` | ❌ Wave 0 |
| D-04 | `SetLastActivityForTest` helper used (not synctest) | static | `! grep -n 'synctest' session/stale_test.go` AND `grep -c 'SetLastActivityForTest' session/stale_test.go >= 4` | ✅ (helper already exists) |
| D-05(a) | SourceClaude + dead PID + 5min survives | unit | `go test -run TestStaleCheckSkipsPidCheckForClaudeSource -v ./session/...` | ❌ Wave 0 |
| D-05(b) | SourceClaude + dead PID + 31min removed by removeTimeout | unit | `go test -run TestStaleCheckRemovesClaudeSourceAfterRemoveTimeout -v ./session/...` | ❌ Wave 0 |
| D-05(c) | Live-incident session 2fe1b32a + PID 5692 + 150s survives | unit (live-repro) | `go test -run TestStaleCheckSurvivesUuidSourcedLongRunningAgent -v ./session/...` | ❌ Wave 0 |
| D-05(d) | Debug log line negative-assertion for guard skip | unit (slog) | `go test -run TestStaleCheckEmitsDebugOnGuardSkip -v ./session/...` | ❌ Wave 0 |
| D-06 | v4.2.1 propagated across 5 files | integration (grep) | `grep -c '4\.2\.1' main.go .claude-plugin/plugin.json .claude-plugin/marketplace.json scripts/start.sh scripts/start.ps1` (each ≥ 1) | ❌ Wave 0 (bump-version.sh run) |
| D-06 | `scripts/phase-09/verify.{sh,ps1}` exist | integration | `test -x scripts/phase-09/verify.sh && test -f scripts/phase-09/verify.ps1` | ❌ Wave 0 |
| D-07 | slog line unchanged at stale.go | static | `grep -c 'PID dead but session has recent activity, skipping removal' session/stale.go == 1` | ✅ (already present) |
| D-08 | 15-line godoc at stale.go:38-52 | static | `awk 'NR>=38 && NR<=52' session/stale.go \| grep -c '^//'` ≥ 13 | ❌ Wave 0 |
| D-09 | CHANGELOG [4.2.1] with live-log excerpt | static | `grep -c '## \[4.2.1\]' CHANGELOG.md == 1` AND `grep -c '2fe1b32a-ea1d-464f-8cac-375a4fe709c9' CHANGELOG.md >= 1` | ❌ Wave 0 |
| D-09 | CHANGELOG [4.2.1] section ordering | static | `awk '/## \[/{print NR": "$0}' CHANGELOG.md \| head -3` shows `[Unreleased] < [4.2.1] < [4.2.0]` | ❌ Wave 0 |
| v4.2.1 manual repro | MCP-heavy >2min silence → daemon stays up | manual E2E | User runs Claude Code with MCP-heavy flow; observes `dsrcode.log` for absence of `"removing stale session"` on UUID session over 5min | manual-only (documented in 09-02 verify checklist) |
| v4.2.1 live verify harness | All T1..Tn pass | integration | `bash scripts/phase-09/verify.sh` (all PASS) or `pwsh -File scripts/phase-09/verify.ps1` (all PASS) | ❌ Wave 0 |

### Sampling Rate

- **Per task commit:** `go test -run TestStaleCheck -v ./session/...` (fast: <3s)
- **Per wave merge:** `go test -race -v ./...` (full suite, ~30-60s on this repo)
- **Phase gate:** Full suite green + `scripts/phase-09/verify.{sh,ps1}` green against live daemon + user types "approved" before git tag.

### Wave 0 Gaps

- [ ] `session/stale_test.go` — existing 55-line file must grow: rename 1 test + invert assertion (D-03), add 3 new tests `TestStaleCheckRemovesClaudeSourceAfterRemoveTimeout` / `TestStaleCheckSurvivesUuidSourcedLongRunningAgent` / `TestStaleCheckEmitsDebugOnGuardSkip` (D-05 b,c,d). Imports to add: `bytes`, `log/slog`, `strings` (for test d).
- [ ] `session/stale.go` — 1-line guard insertion at :41, 15-line godoc replacement at :38-42 (D-01, D-08).
- [ ] `CHANGELOG.md` — new `## [4.2.1] - 2026-04-18` section at line 10, Fixed + Changed + Added subsections per Keep-a-Changelog 1.1.0, bold-lead-phrase bullets with D-NN citations, verbatim incident log excerpt per D-09.
- [ ] 5 version files (main.go + plugin.json + marketplace.json + start.sh + start.ps1) — propagated by `./scripts/bump-version.sh 4.2.1`.
- [ ] `scripts/phase-09/verify.sh` — NEW, mirror `scripts/phase-08/verify.sh` structure, swap T-assertions for Phase-9 scope (binary version, /health, SourceClaude-specific stale-check behavior over wall-clock).
- [ ] `scripts/phase-09/verify.ps1` — NEW, PowerShell parity.
- [ ] Framework install: NONE needed — `go test` already present.

*(No new testing framework or fixture setup required — all infrastructure is in place from Phase 7 + Phase 8.)*

### Nyquist Dimensions Covered

Per Nyquist (sampling adequate to reconstruct the behavior), Phase 9 covers:

| Dimension | Evidence |
|-----------|----------|
| **Positive path** (D-01 works for SourceClaude) | Test (a) `SkipsPidCheckForClaudeSource` |
| **Backstop path** (removeTimeout still works) | Test (b) `RemovesClaudeSourceAfterRemoveTimeout` |
| **Regression anchor** (specific live incident) | Test (c) `SurvivesUuidSourcedLongRunningAgent` with incident UUID + PID + elapsed |
| **Observability path** (log emission behavior) | Test (d) `EmitsDebugOnGuardSkip` negative assertion |
| **Cross-source invariance** (Phase-7 HTTP guard still works) | Existing `TestStaleCheckSkipsPidCheckForHttpSource` stays green |
| **No downgrade to SourceJSONL** (PID=0 still excluded) | Existing `s.PID > 0` gate unchanged — implicit invariance; add acceptance check via `grep -c 's.PID > 0' session/stale.go == 1` |
| **Release artefact integrity** | verify.sh T1 (version), T2 (/health), remaining Tn (manual MCP-heavy repro documented in verify script header) |
| **Manual end-user validation** | CONTEXT §specifics manual reproduction scenario: MCP-heavy session >2min silence; daemon stays up |

Static checks + 4 unit tests + 1 inverted unit test + 1 integration harness + 1 manual E2E = **8 sampling points against 9 locked decisions** → above the Nyquist floor for this 1-line fix.

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | No authentication surface in this change |
| V3 Session Management | no | Daemon-internal session registry — no HTTP session cookies / tokens |
| V4 Access Control | no | No authorization decisions affected |
| V5 Input Validation | no | Guard operates on already-validated `SessionSource` enum values (set at `StartSession`) |
| V6 Cryptography | no | No cryptographic operations in scope |
| V7 Error Handling / Logging | yes | `slog.Debug`/`slog.Info` lines emit `sessionId`, `pid`, `elapsed` — no PII beyond what was already logged in Phase 7 |
| V11 Business Logic | yes | The guard IS the business logic under test — unit tests validate the intended behavior |

### Known Threat Patterns for Go daemon + session guard

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Stale-session false positive removes active session (the bug being fixed) | Denial of Service (self-inflicted) | Extend guard to cover UUID source (D-01). Tests (a) and (c) validate. |
| Stale-session false negative keeps zombie forever | Denial of Service (resource leak) | `removeTimeout` 30min backstop unchanged. Test (b) validates. |
| Log injection via malicious sessionId | Tampering / Log forging | `slog` uses structured attrs (`"sessionId", s.SessionID`), not format-string interpolation. Not a new vector — already Phase-7 pattern. |
| PID recycling (Docker, PID reuse after exit) | Spoofing (same PID, different process) | Out of scope for Phase 9 (deferred per CONTEXT §deferred); unchanged from Phase 7. |
| Race between `CheckOnce` and `StartSession` | Tampering (concurrent mutation) | `GetAllSessions` returns value-copy slice; `EndSession` takes registry lock. Threat T-07-01 accepted in Phase-7 Plan 07-01; still applies. |

No new security surface introduced. The change reduces the blast radius of the existing self-inflicted DoS (false-positive removal during active sessions).

## Sources

### Primary (HIGH confidence)

- `09-CONTEXT.md` (locked decisions D-01..D-09) — authoritative user intent, pre-locked at discuss-phase
- `session/stale.go:15-63` — current production code read verbatim
- `session/stale_test.go:1-55` — current test file read verbatim
- `session/source.go:1-56` — `SourceClaude` enum definition + `sourceFromID` logic
- `session/registry.go:42-117, 451-465` — `StartSession` dispatch + `SetLastActivityForTest` helper
- `coalescer/coalescer_test.go:267-282` — in-repo reference implementation for `bytes.Buffer` + slog pattern
- `scripts/bump-version.sh:1-63` — version propagation script (5 canonical files confirmed)
- `scripts/phase-08/verify.sh:1-152` — bash verify harness template
- `scripts/phase-08/verify.ps1:1-209` — PowerShell verify harness template
- `CHANGELOG.md:1-42` — Keep-a-Changelog 1.1.0 format + `[4.1.2]` + `[4.2.0]` voice template
- `CLAUDE.md §Releasing` + `CLAUDE.md §GSD Workflow Enforcement` — project-level constraints

### Secondary (MEDIUM confidence)

- `go version go1.26.0 windows/amd64` — local toolchain confirmed via Bash
- `.planning/phases/07-fix-daemon-auto-exit-bugs-pid-dead-check-mcp-activity-tracki/07-01-PLAN.md + 07-01-SUMMARY.md` — Phase-7 precedent for RED/GREEN + 1-line-diff discipline
- `.planning/phases/08-.../08-04-PLAN.md` — Phase-8 release harness precedent (T1-T6 template)
- `.planning/STATE.md` — Phase-7 + Phase-8 decisions already logged; confirms file paths and test conventions
- Keep-a-Changelog 1.1.0 format — cited in CONTEXT as Exa PRE-MCP validation; cross-confirmed against `CHANGELOG.md` existing entries

### Tertiary (LOW confidence — none)

No tertiary-source-only claims in this research. Every factual claim is either verified against the repo or cited from an authoritative doc/precedent.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — no new dependencies; every library already in use and verified via in-repo grep
- Architecture: HIGH — 1-line change with well-understood scope, no cross-tier coupling, canonical refs locked in CONTEXT
- Pitfalls: HIGH — Pitfall 1 discovered during research (D-05(d) semantic ambiguity) gives the planner a concrete refinement; all other pitfalls derived from close reading of the production code

**Research date:** 2026-04-18
**Valid until:** 2026-05-18 (30 days — this is stable stdlib + in-repo pattern research, no fast-moving external ecosystem)

## RESEARCH COMPLETE

**Phase:** 9 — Fix stale-session false-positive for UUID-sourced Claude sessions
**Confidence:** HIGH

### Key Findings

- **All nine locked decisions D-01..D-09 are implementable with the code and scripts already in the repo.** No new dependencies, no new infrastructure.
- **In-repo slog+bytes.Buffer precedent at `coalescer/coalescer_test.go:267-282` is drop-in reusable for test D-05(d).** No library research required.
- **`scripts/bump-version.sh` touches exactly the 5 canonical files listed in CONTEXT §D-06** — confirmed via file read (`main.go`, `.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json`, `scripts/start.sh`, `scripts/start.ps1`).
- **CRITICAL REFINEMENT (Pitfall 1):** D-05(d) positive assertion ("Debug line fires for the guard skip") is semantically impossible after D-01, because the Debug line at `stale.go:47` lives INSIDE the block the outer guard now skips. The planner should adopt the NEGATIVE-assertion form (Example 6) and surface this 1-line clarification to the user at `/gsd-plan-phase` time — not a re-debate, just a semantic tightening.
- **`git tag v4.2.1 && git push --tags` MUST remain the user's manual step** per CLAUDE.md §Releasing and `3-tag-push-limit` memory. Plan 09-02 ends with a `checkpoint:human-verify` gate.

### File Created

`.planning/phases/09-fix-stale-session-false-positive-for-uuid-sourced-claude-ses/09-RESEARCH.md`

### Confidence Assessment

| Area | Level | Reason |
|------|-------|--------|
| Standard Stack | HIGH | No new deps; `log/slog`, `bytes`, `testing`, `time` all Go stdlib already in use |
| Architecture | HIGH | 1-line change, scope locked by CONTEXT, direct source read confirms all integration points |
| Pitfalls | HIGH | Pitfall 1 discovered via code inspection of stale.go block structure; refinement concrete and testable |
| Release harness | HIGH | Phase-8 verify.{sh,ps1} read verbatim, structural drop-in for phase-09 directory |
| CHANGELOG | HIGH | Exact `[4.1.2]` voice template in file; Keep-a-Changelog 1.1.0 compliance via file inspection |
| D-05(d) semantic | HIGH (on the refinement) / MEDIUM (on CONTEXT literal reading) | Code path analysis definitive; user should approve the negative-assertion refinement at plan-time |

### Open Questions

1. D-05(d) positive vs negative slog assertion (Pitfall 1) — planner flags to user at discuss/plan time; recommendation Example 6 (negative).
2. Test file layout (single vs split) — discretionary; recommendation: single `stale_test.go` growing from 55 to ~180 lines.
3. `dsrcode --version` T1 precondition — mirror Phase-8 verbatim; document in script header.

### Ready for Planning

Research complete. Planner can now create 2 plan files:
- `09-01-PLAN.md` — stale.go guard + godoc + 4 regression tests (covers D-01, D-02, D-03, D-04, D-05, D-07, D-08)
- `09-02-PLAN.md` — v4.2.1 release artefacts + human-verify checkpoint (covers D-06, D-09)

Planner should also create `09-VALIDATION.md` from the Validation Architecture section above.

*Phase: 09-fix-stale-session-false-positive-for-uuid-sourced-claude-ses*
*Research date: 2026-04-18*
*Target release: v4.2.1 hotfix*
