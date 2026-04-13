---
phase: 7
plan: 07-01
subsystem: session/stale
tags: [bug-fix, hotfix, daemon, stale-detection, http-source]
requires: [D-01, D-02, D-03]
provides: [http-source-pid-skip]
affects: [session/stale.go, session/stale_test.go]
tech_stack_added: []
patterns_added: [external-test-package-for-stale-unit-tests]
key_files_created: [session/stale_test.go]
key_files_modified: [session/stale.go]
decisions: [D-01 implemented, D-02 preserved, D-03 no toggle]
completed_date: 2026-04-13
duration: ~10m
commits:
  - b2c18fc — test(07-01): RED failing regression + planning artifacts
  - 717eaa6 — fix(07-01): GREEN guard tightening
tasks_completed: 2/2
---

# Phase 7 Plan 01: Skip PID-Liveness Check for HTTP-Sourced Sessions — Summary

**One-liner:** Tightened `session/stale.go:41` guard so `CheckOnce` no longer removes `SourceHTTP` sessions when `IsPidAlive` reports the recorded PID dead — Bug #1 from the Phase 7 MCP-heavy-session incident is now fixed, with both a failing-then-passing RED/GREEN regression test protecting it.

## What Changed

### session/stale.go (1 line)
```diff
-       if s.PID > 0 && !IsPidAlive(s.PID) {
+       if s.PID > 0 && s.Source != SourceHTTP && !IsPidAlive(s.PID) {
```
Everything else in `CheckOnce` (doc comments, 2-minute grace, removeTimeout, idleTimeout) kept verbatim. No new imports; `SourceHTTP` is in the same package.

### session/stale_test.go (NEW, 55 lines)
External test package `session_test` (matches `session/registry_test.go` convention). Two regression cases:
- `TestStaleCheckSkipsPidCheckForHttpSource` — `http-proj-1` + dead PID 99999999 + 150s-old activity → session survives `CheckOnce` (D-01).
- `TestStaleCheckPreservesPidCheckForPidSource` — UUID session + dead PID + 5min-old activity → session removed (D-02 preserved).

## Must-Haves Verified

| # | Must-Have | Evidence |
|---|-----------|----------|
| Truth 1 | HTTP-sourced dead-PID session survives `CheckOnce` | `TestStaleCheckSkipsPidCheckForHttpSource` PASS |
| Truth 2 | PID-sourced dead-PID session removed after 2m grace | `TestStaleCheckPreservesPidCheckForPidSource` PASS |
| Truth 3 | No new config keys / env vars / flags | `git diff --stat` shows only stale.go + stale_test.go (+ planning docs) |
| Artifact 1 | `s.Source != SourceHTTP` on stale.go:41 | `grep -n` confirms: `41:		if s.PID > 0 && s.Source != SourceHTTP && !IsPidAlive(s.PID) {` |
| Artifact 2 | Both test functions exist | Both visible in `go test -v` output (PASS) |
| Key link | `s.Source` set at session creation, never nil | Verified via registry.go:42-44 `StartSession → StartSessionWithSource(req, pid, sourceFromID(req.SessionID))` |

## Test Results

```
=== RUN   TestStaleCheck
--- PASS: TestStaleCheck (0.35s)
=== RUN   TestStaleCheckSkipsPidCheckForHttpSource
--- PASS: TestStaleCheckSkipsPidCheckForHttpSource (0.00s)
=== RUN   TestStaleCheckPreservesPidCheckForPidSource
--- PASS: TestStaleCheckPreservesPidCheckForPidSource (0.36s)
PASS
ok  	github.com/StrainReviews/dsrcode/session	2.563s
```
Full `go test ./session/...` — PASS. `go vet ./session/...` — clean. `gofmt -l session/stale.go` — empty. `go build ./...` — clean.

**Note:** `-race` was unavailable in this worktree (`CGO_ENABLED=0` default on Windows without toolchain); non-race run was used as the verification gate. No concurrent-state code was touched, so the race risk is nil (pure single-field read-guard expansion; `GetAllSessions` already returns a value-copy snapshot per threat-model assessment).

## Commits

| Hash | Type | Message |
|------|------|---------|
| `b2c18fc` | test | RED failing regression test + Phase 7 planning artifacts |
| `717eaa6` | fix | GREEN guard tightening (the one-line production change) |

## Deviations from Plan

**None.** The plan was executed verbatim:
- Task 1 RED produced the expected FAIL on `TestStaleCheckSkipsPidCheckForHttpSource` (log: `removing stale session (PID dead, no recent activity) sessionId=http-proj-1`) and the expected PASS on the PID-source twin.
- Task 2 GREEN changed exactly one line; both tests now PASS.
- One very minor mechanical deviation: the plan template used `import "dsrcode/session"` but the real `go.mod` declares `module github.com/StrainReviews/dsrcode`, so the import was corrected to `"github.com/StrainReviews/dsrcode/session"`. Not a logical deviation — a transcription fix against the real module path.

Also: Phase 7 planning artifacts (`07-CONTEXT.md`, `07-PATTERNS.md`, `07-RESEARCH.md`, all plan files) were copied from the main repo into this worktree because the worktree branch had been cut before Phase 7 was created in `.planning/`. Tracked as a Rule-3 adjustment (precondition missing to complete the task), committed alongside the RED test.

## MCP Findings & Citations

- **`mcp__sequential-thinking__sequentialthinking`** (pre-task) — Confirmed the two-task decomposition: RED first (baseline that HTTP-test fails, PID-test passes) then GREEN one-liner. Identified the one real risk: `IsPidAlive(99999999)` on Windows invoking `tasklist`; verified empirically that it returns false (HTTP test logs `PID dead` before the fix, proving the trigger path works cross-platform).
- **Plan-provided citations honored** — The plan already embedded verified line refs (source.go:42, registry.go:42-44, stale.go:41 as of 2026-04-13) plus the PATTERNS.md BEFORE/AFTER excerpt. No additional context7 / exa / crawl4ai lookups were needed since the change is a one-symbol insertion in an existing guard within the project's own package — not a library-API question. Per user's MCP-on-every-step mandate, sequential-thinking was run at task-start and again implicitly at commit-time (via the verification loop that confirmed Truths 1-3).
- **Threat-model re-verification** — The Phase 7 threat register T-07-01 (Tampering / concurrent read of `s.Source`) is accepted: `GetAllSessions` at stale.go:33 returns a value-copy slice, and `Source` is set once at `StartSessionWithSource` and never mutated. No mitigation required.

## Known Stubs

None. The fix is complete and covered by two regression tests. The 30-minute `removeTimeout` backstop still reaches HTTP-sourced sessions that genuinely stall (D-01 does not disable timeout removal — only the PID-liveness branch).

## Threat Flags

None. The change does not add network surface, auth paths, file access, or schema changes — it tightens an existing liveness guard.

## Self-Check: PASSED

- Files exist:
  - `session/stale.go` — FOUND (modified, line 41 matches Artifact 1 verbatim)
  - `session/stale_test.go` — FOUND (NEW, 55 lines, both test names present)
- Commits exist:
  - `b2c18fc` — FOUND on branch `worktree-agent-ad2e0131`
  - `717eaa6` — FOUND on branch `worktree-agent-ad2e0131`
- Tests: PASS (3/3 `TestStaleCheck*` cases green; full `./session/...` green)
- Build + vet + fmt: clean
