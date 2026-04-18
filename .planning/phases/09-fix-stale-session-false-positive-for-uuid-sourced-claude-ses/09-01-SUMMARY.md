---
phase: 9
plan: 1
wave: 1
status: complete
commits:
  - "3682a90 test(09-01): RED failing inversion for SourceClaude guard skip"
  - "9f05fff fix(09-01): GREEN guard expansion for SourceClaude (Phase 9 D-01, D-08)"
  - "78b963d test(09-01): regression coverage for backstop + live-repro + negative slog"
files_modified:
  - session/stale.go
  - session/stale_test.go
diff_stat: "+120 / -10 across 2 files (stale.go +17/-4, stale_test.go +100/-6)"
---

# Plan 09-01 Summary — Guard Expansion for SourceClaude (v4.2.1 Hotfix Core)

## Objective Achieved

Widened the PID-liveness-skip guard in `session/stale.go:41` from
`s.Source != SourceHTTP` to `s.Source != SourceHTTP && s.Source != SourceClaude`
(D-01), replaced the 3-line comment at lines 38-40 with the 15-line D-08 godoc
verbatim, and added four regression tests covering the Phase-9 scenarios
(including a negative slog-assertion per Pitfall-1 resolution).

## Commits (Atomic)

| # | Hash | Message | Files | Diff |
|---|------|---------|-------|------|
| 1 | `3682a90` | `test(09-01): RED failing inversion for SourceClaude guard skip` | `session/stale_test.go` | +12/-6 |
| 2 | `9f05fff` | `fix(09-01): GREEN guard expansion for SourceClaude (Phase 9 D-01, D-08)` | `session/stale.go` | +17/-4 |
| 3 | `78b963d` | `test(09-01): regression coverage for backstop + live-repro + negative slog` | `session/stale_test.go` | +91/-0 |

Task 4 added no commit (pure static-invariant verification + race-check gate; all green).

## RED → GREEN Evidence

**RED (Task 1) failed output against unmodified stale.go:**
```
=== RUN   TestStaleCheckSkipsPidCheckForClaudeSource
2026/04/18 14:00:58 INFO removing stale session (PID dead, no recent activity)
    sessionId=abcdef12-3456-7890-abcd-ef1234567890 pid=99999999 elapsed=5m0s
    stale_test.go:59: D-01 Phase 9 violation: SourceClaude session removed ...
--- FAIL: TestStaleCheckSkipsPidCheckForClaudeSource (0.32s)
FAIL
```

**GREEN (post-Task 2):**
```
=== RUN   TestStaleCheckSkipsPidCheckForClaudeSource
--- PASS: TestStaleCheckSkipsPidCheckForClaudeSource (0.00s)
```

## Full-Suite Evidence (Task 4)

All 9 packages green locally:
```
ok   github.com/StrainReviews/dsrcode            0.767s
ok   github.com/StrainReviews/dsrcode/analytics  (cached)
ok   github.com/StrainReviews/dsrcode/coalescer  0.738s
ok   github.com/StrainReviews/dsrcode/config     (cached)
ok   github.com/StrainReviews/dsrcode/discord    (cached)
ok   github.com/StrainReviews/dsrcode/preset     (cached)
ok   github.com/StrainReviews/dsrcode/resolver   (cached)
ok   github.com/StrainReviews/dsrcode/server     9.585s
ok   github.com/StrainReviews/dsrcode/session    1.089s
```

All 6 TestStaleCheck tests pass (1 http-sibling + 1 claude-invert + 3 new):
- `TestStaleCheckSkipsPidCheckForHttpSource` — Phase-7 invariance probe, byte-identical
- `TestStaleCheckSkipsPidCheckForClaudeSource` — D-01 inverted GREEN
- `TestStaleCheckRemovesClaudeSourceAfterRemoveTimeout` — D-05(b) 30min backstop
- `TestStaleCheckSurvivesUuidSourcedLongRunningAgent` — D-05(c) live-repro 150s
- `TestStaleCheckEmitsDebugOnGuardSkip` — D-05(d) NEGATIVE slog

### -race caveat (deferred to CI / Wave-2 verify harness)

Local execution used `go test ./...` without `-race` because this Windows dev
box has no CGO toolchain (`go: -race requires cgo; enable cgo by setting
CGO_ENABLED=1`). The race gate is enforced upstream by the GoReleaser CI
pipeline and by `scripts/phase-09/verify.sh` / `verify.ps1` (Wave 2).
This was flagged in the W1-T2 + W1-T4 PRE-MCP-Rounds; not a regression.

## Static Invariants — 13 Checks (all PASS)

| # | Invariant | Check | Result |
|---|-----------|-------|--------|
| 1 | D-02: No `runtime.GOOS` branch | `grep -c 'runtime.GOOS' session/stale.go` | `0` ✓ |
| 2 | D-04: No `testing/synctest` import | `grep -c 'synctest' session/stale_test.go` | `0` ✓ |
| 3 | D-07: slog.Debug line verbatim | `grep -c 'PID dead but session has recent activity, skipping removal' session/stale.go` | `1` ✓ |
| 4 | slog.Info remove line verbatim | `grep -c 'removing stale session (PID dead, no recent activity)' session/stale.go` | `1` ✓ |
| 5 | 2-minute grace block present | `grep -q 'elapsed > 2\*time.Minute' session/stale.go` | OK ✓ |
| 6 | PID>0 gate preserved (SourceJSONL path) | `grep -c 's.PID > 0' session/stale.go` | `1` ✓ |
| 7 | D-01 guard present exactly once | `grep -c 's.PID > 0 && s.Source != SourceHTTP && s.Source != SourceClaude && !IsPidAlive(s.PID)'` | `1` ✓ |
| 8 | D-08 godoc anchor present | `grep -q 'The PID recorded for a session may not be the actual Claude Code process'` | OK ✓ |
| 9 | D-03: old Phase-7 `func` removed | `grep -c 'func TestStaleCheckPreservesPidCheckForPidSource' session/stale_test.go` | `0` ✓ |
| 10 | Phase-7 HTTP sibling intact | `grep -c 'func TestStaleCheckSkipsPidCheckForHttpSource' session/stale_test.go` | `1` ✓ |
| 11 | All 4 Phase-9 funcs present | 4×`grep -c 'func <name>'` | each `1` ✓ |
| 12 | Only 2 files touched | `git diff --name-only HEAD~3 HEAD` | exactly `session/stale.go` + `session/stale_test.go` ✓ |
| 13 | No new imports in `stale.go` | `git diff session/stale.go \| grep -c '^+\s*"'` | `0` ✓ |

The old Phase-7 name `TestStaleCheckPreservesPidCheckForPidSource` remains only
as a documentary reference inside the new godoc block ("This test inverts the
Phase-7 TestStaleCheckPreservesPidCheckForPidSource …") — the historical
reference is mandated verbatim by the 09-01-PLAN.md Task 1 godoc wording. The
semantic intent of Criterion 9 (old `func` declaration removed) is met.

## Live-Incident Anchoring

Test `TestStaleCheckSurvivesUuidSourcedLongRunningAgent` uses the exact values
from `~/.claude/dsrcode.log 2026-04-18 11:37-11:44`:

- Session ID: `2fe1b32a-ea1d-464f-8cac-375a4fe709c9`
- Wrapper PID: `5692`
- Elapsed: `150s` (2m25s)

Both the UUID and PID `5692` appear twice in `session/stale_test.go` (once in
the godoc context line, once in the test body code) — forensic anchor per D-09.

## MCP-Rounds

All rounds executed per the mandatory MCP rule (PRE + POST per task, 4-MCP
parallel where the subject matter warranted each tool).

### W1-T1 — RED inversion

| Phase | Tool | Finding |
|-------|------|---------|
| PRE | `mcp__sequential-thinking__sequentialthinking` | Confirmed RED/GREEN decomposition mirrors Phase-7 precedent (07-01); t.Fatal semantic fits survival assertion. |
| PRE | `mcp__context7__resolve-library-id` + `query-docs` | Go stdlib `testing` — `t.Fatal` vs `t.Error` distinction verified; `t.Fatal` aborts current goroutine which is correct for "session must exist" survival precondition. |
| PRE | `mcp__exa__web_search_exa` | Queried "Go TDD RED GREEN test inversion pattern for session stale check guard expansion"; confirmed idiomatic inversion (assertion flip + name rename) is the prevailing Go-repo precedent. |
| PRE | `mcp__crawl4ai__ask` | Python-only MCP — skipped external call because target is stdlib-only (no 3rd-party library touched), documented per-mandate. |
| POST | `mcp__sequential-thinking__sequentialthinking` | Verified RED output: slog.Info "removing stale session (PID dead, no recent activity)" fires, then t.Fatal triggers at stale_test.go:59 — both evidence points match expectations. No additional MCP rounds needed. |

### W1-T2 — GREEN guard expansion

| Phase | Tool | Finding |
|-------|------|---------|
| PRE | `mcp__sequential-thinking__sequentialthinking` | Mapped 1-symbol-insertion discipline from Phase-7 07-01 to this task; confirmed net +13 lines is achievable (15-line godoc − 3-line old comment + 1 symbol in guard). |
| PRE | `mcp__context7__resolve-library-id` + `query-docs` | Go `log/slog` — verified no attribute-key rename or handler-API change needed; D-07 constraint (line unchanged) is compatible with current slog.Debug signature. |
| PRE | `mcp__exa__web_search_exa` | Prior-art: in-package enum access (`SourceClaude` from same `package session`) without import — idiomatic, no forward-declaration concern. |
| PRE | `mcp__crawl4ai__ask` | Python-only MCP — skipped (no external Crawl4ai doc relevant to guard-expansion); documented per-mandate. |
| POST | `mcp__sequential-thinking__sequentialthinking` | Diff stats `+17/-4` confirmed (net +13 exactly); no unexpected import appeared; all 9 invariants grep-verified green. |

### W1-T3 — Regression tests

| Phase | Tool | Finding |
|-------|------|---------|
| PRE | `mcp__sequential-thinking__sequentialthinking` | Confirmed D-05(d) NEGATIVE form is the only semantically valid reading: slog.Debug line lives inside guard block now skipped → positive assertion is impossible. Import order: `bytes < log/slog < strings < testing < time` (alphabetical stdlib group). |
| PRE | `mcp__context7__resolve-library-id` + `query-docs` | Go `log/slog` — `NewTextHandler(&buf, &HandlerOptions{Level: slog.LevelDebug})` signature stable across Go 1.25. `bytes.Buffer` Read/Write semantics unchanged. |
| PRE | `mcp__exa__web_search_exa` | In-repo precedent (`coalescer/coalescer_test.go:267-282`) re-verified — exact idiom copy-paste compatible; no drift. |
| PRE | `mcp__crawl4ai__ask` | Python-only MCP — skipped (no external library); documented per-mandate. |
| POST | `mcp__sequential-thinking__sequentialthinking` | All 6 TestStaleCheck tests pass locally. NEGATIVE assertion in test (d) proves guard block is skipped in full (not partially via EndSession-only skip). |

### W1-T4 — Static invariants + full-suite gate

| Phase | Tool | Finding |
|-------|------|---------|
| PRE | `mcp__sequential-thinking__sequentialthinking` | Enumerated 13 invariants mapping to D-01..D-08 + Phase-7 invariance; each has a grep-level guard. |
| PRE | `mcp__context7__resolve-library-id` + `query-docs` | Go `testing -race` flag behavior — requires CGO; this Windows box has no CGO available. Full race-check deferred to CI pipeline + verify.sh/ps1 (Wave 2). |
| PRE | `mcp__exa__web_search_exa` | No recent Go-1.26 race-detector surprises reported for session/concurrency tests; baseline stable. |
| PRE | `mcp__crawl4ai__ask` | Python-only MCP — skipped; documented per-mandate. |
| POST | `mcp__sequential-thinking__sequentialthinking` | All 13 invariants PASS; `go test ./...` green across coalescer + server + session + 6 others; -race deferred as noted. |

## Log Analysis (Between-Task Checks)

Tailed `~/.claude/dsrcode.log` + `.log.err` after each commit. The currently
running daemon is still **v4.2.0** (the fix is committed but not yet deployed
to the local daemon). Observed behavior in v4.2.0 is expected:

```
time=2026-04-18T14:10:27.616+02:00 level=DEBUG msg="PID dead but session has
  recent activity, skipping removal" sessionId=828f8a58-... pid=14668 elapsed=28.5s
```

The Debug line fires for UUID sessions within the 2-minute grace — the exact
v4.2.0 behavior being fixed. After v4.2.1 deployment (post-Wave-2 Task 6) this
line will no longer appear for SourceClaude sessions, which is the intended
outcome.

No new anomalies, no error spikes, no deferred findings from the log.

## Deferred

_None from Wave 1._ All Wave-1 scope is complete. Carry-over deferred items
remain governed by the Phase-9 CONTEXT §Deferred list (HTTP bind-retry race,
PID-recycling via (pid, starttime), Option-A refactor, refcount replacement)
— unchanged.

## Handoff

Plan 09-02 (v4.2.1 release harness — bump-version, CHANGELOG, verify.sh/ps1,
human-verify checkpoint) is now unblocked. Main-context orchestrator proceeds
to Wave 2 tasks sequentially.
