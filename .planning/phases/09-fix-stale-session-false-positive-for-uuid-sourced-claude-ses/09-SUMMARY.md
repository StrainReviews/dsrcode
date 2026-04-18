---
phase: 9
status: complete
release: v4.2.1
release_date: 2026-04-18
plans_complete: 2
commits_total: 8
---

# Phase 9 Summary — Fix Stale-Session False Positive for UUID-sourced Claude Sessions (v4.2.1 Hotfix)

## Phase Outcome

Closed the v4.2.0 live-incident reproduction: session
`2fe1b32a-ea1d-464f-8cac-375a4fe709c9` (wrapper PID 5692) was incorrectly
removed 2m25s after its last hook while Claude Code was still active, causing
daemon auto-exit ~6 s later. Root cause was the Phase-7 guard at
`session/stale.go:41` only covering `SourceHTTP` — UUID-shaped session IDs
(sourced as `SourceClaude`) fell through to the PID-liveness branch and were
removed because the wrapper-launcher PID was long dead.

**Fix (v4.2.1):** guard expanded to
`s.PID > 0 && s.Source != SourceHTTP && s.Source != SourceClaude && !IsPidAlive(s.PID)`
(D-01). UUID-sourced Claude Code sessions now survive arbitrarily long
MCP-heavy silences up to the 30-minute `removeTimeout` backstop, on both
Windows (orphan-reparenting absence) and Unix (start.sh wrapper exit).

## Waves Executed

| Wave | Plan | Purpose | Commits |
|------|------|---------|---------|
| 1 | 09-01 | Core hotfix: guard expansion + godoc + 4 regression tests | `3682a90`, `9f05fff`, `78b963d`, `d2f8b09` |
| 2 | 09-02 | v4.2.1 release harness: bump-version, CHANGELOG, verify.sh/ps1, human-verify checkpoint | `535f035`, `847c965`, `7325629`, `2c2719d` |

Full per-plan breakdowns in `09-01-SUMMARY.md` and `09-02-SUMMARY.md`.

## Commit Hashes (Atomic, per Task)

```
3682a90 test(09-01): RED failing inversion for SourceClaude guard skip        (W1-T1)
9f05fff fix(09-01): GREEN guard expansion for SourceClaude (Phase 9 D-01, D-08) (W1-T2)
78b963d test(09-01): regression coverage for backstop + live-repro + negative slog (W1-T3)
d2f8b09 docs(09-01): add plan 09-01 summary (Wave 1 complete)                 (W1-T4 evidence)
535f035 chore(09-02): bump version to v4.2.1                                  (W2-T1)
847c965 docs(09-02): changelog v4.2.1 with live-incident forensic anchor      (W2-T2)
7325629 chore(09-02): add scripts/phase-09/verify.sh (bash T1-T3 harness)     (W2-T3)
2c2719d chore(09-02): add scripts/phase-09/verify.ps1 (PowerShell parity)     (W2-T4)
```

Tasks W2-T5 (live verify) and W2-T6 (human-verify) did not produce git
commits — evidence captured in `09-02-SUMMARY.md §Live Verify Evidence` and
`§Human-Verify Checkpoint`.

## Decisions Locked (D-01..D-09)

| ID | Decision | Landed |
|----|----------|--------|
| D-01 | Guard expansion `s.Source != SourceClaude` at stale.go:41 | `9f05fff` |
| D-02 | No `runtime.GOOS` gate — source-based solves Windows + Unix | grep-verified |
| D-03 | Phase-7 test inverted + renamed, 1 backstop test added | `3682a90` + `78b963d` |
| D-04 | `SetLastActivityForTest` helper used; NOT `testing/synctest` | grep-verified |
| D-05 | 4 regression tests (a/b/c/d) — (d) NEGATIVE assertion | `3682a90` + `78b963d` |
| D-06 | v4.2.1 full release set (CHANGELOG + bump-version + verify.sh/ps1) | `535f035`, `847c965`, `7325629`, `2c2719d` |
| D-07 | slog.Debug line at stale.go:47 UNCHANGED | grep-verified (count == 1) |
| D-08 | 15-line godoc at stale.go:38-52 verbatim from CONTEXT | `9f05fff` |
| D-09 | CHANGELOG [4.2.1] cites live-incident log excerpt | `847c965` |

## §Manual-Verify-Evidence

**Task 5 — Live verify harness run against v4.2.1 daemon:**

```
PASS: T1 dsrcode --version contains 4.2.1
PASS: T2 /health returned HTTP 200
T3 waiting 150s for stale-check ticker to sweep (mirrors incident elapsed)...
PASS: T3 SourceClaude UUID session survived PID-liveness sweep (0 removals observed)

All Phase 9 verify tests passed (T1..T3).
```

- Wall-clock: ~155 s (150 s sleep + overhead)
- Exit code: 0
- Post-run log sanity: `grep -c 'removing stale.*verify09-' ~/.claude/dsrcode.log` = 0
- Daemon `/health` uptime during checkpoint: 5m51s (steady-state after rebind)

**Task 6 — Human-verify checkpoint:** User typed `approved` at 2026-04-18
14:26:00 local time after reviewing the `<what-built>` / `<how-to-verify>`
block. Claude did NOT execute `git tag v4.2.1` or `git push origin main
--tags` — confirmed via `git tag --list 'v4.2.1'` returning empty from
Claude's execution context.

## §MCP-Rounds

Per the zwingende MCP-Regel, **every Wave-1 and Wave-2 task** executed a
PRE-round (sequential-thinking + context7 + exa + crawl4ai) and a POST-round
(sequential-thinking + follow-up-specific MCPs as needed) before/after any
file modification, commit, or user interaction. Detailed per-task round
tables are in `09-01-SUMMARY.md §MCP-Rounds` (Wave-1 tasks T1-T4) and
`09-02-SUMMARY.md §MCP-Rounds` (Wave-2 tasks T1-T6). Summary of the mandate
compliance:

- **Total tasks with PRE+POST:** 10 (W1-T1 through W2-T6)
- **Tools used in PRE rounds:** `mcp__sequential-thinking__sequentialthinking`
  (10/10), `mcp__context7__resolve-library-id + query-docs` (10/10),
  `mcp__exa__web_search_exa` (10/10), `mcp__crawl4ai__ask` (10/10 — all 10
  documented as Python-only / external-doc-not-relevant skips per mandate
  allowance for explicit documented no-op)
- **Tools used in POST rounds:** `mcp__sequential-thinking__sequentialthinking`
  (10/10); follow-up exa/context7/crawl4ai runs triggered only when a
  surprise surfaced — none needed across Phase 9 (the 1-symbol-insertion
  discipline + exact-wording godoc + Phase-8-mirrored verify harness kept
  the surprise surface small)
- **MCP-driven course corrections:** zero. All PRE-round analyses matched
  the plan's locked decisions; all POST-rounds verified the expected invariants.

## §Deferred

No new deferred items surfaced during Phase 9 execution. Carry-over from
Phase-9 CONTEXT §Deferred remains on the backlog:

1. **HTTP-server bind-retry + shutdown-race** — `dsrcode.log` 11:58:17
   port-collision → fatal shutdown. Candidate v4.2.2 / `/gsd-quick`.
2. **PID-recycling via `(pid, starttime)`** — openclaw PR #26443 pattern.
   Not needed for v4.2.1 (guard-skip-by-source is sufficient).
3. **Option-A classification refactor** (server-side Source override) —
   breaks `registry_dedup_test.go` upgrade-chain; explicitly rejected
   for Phase 9.
4. **Refcount replacement, /metrics endpoint, UI surfaces** — unchanged
   from Phase-7-deferred.

Log analysis between each commit (W1-T1 through W2-T6) surfaced zero new
anomalies. The currently-running daemon was observed rebinding cleanly
post-restart; no port-collision on this run.

## Diff Scope

```
$ git diff --stat HEAD~8 HEAD -- 'session/*.go' 'CHANGELOG.md' 'main.go' \
       '.claude-plugin/*.json' 'scripts/start.*' 'scripts/phase-09/*'
 .claude-plugin/marketplace.json     |   2 +-
 .claude-plugin/plugin.json          |   2 +-
 CHANGELOG.md                        |  12 ++
 main.go                             |   2 +-
 scripts/phase-09/verify.ps1         | 163 ++++++++++++++++++++++++++++++++
 scripts/phase-09/verify.sh          | 106 +++++++++++++++++++++
 scripts/start.ps1                   |   2 +-
 scripts/start.sh                    |   2 +-
 session/stale.go                    |  21 ++++-
 session/stale_test.go               | 109 +++++++++++++++++++++-
 10 files changed, 410 insertions(+), 11 deletions(-)
```

Untouched: `session/source.go`, `session/registry.go`, `server/server.go`,
`coalescer/*`, `registry_dedup_test.go`, `go.mod`, `go.sum` — all per-plan
scope discipline.

## Next User Action (Releasing)

Per `CLAUDE.md §Releasing` and the `3-tag-push-limit` memory, the next steps
belong exclusively to the user:

```bash
git tag v4.2.1
git push origin main --tags
```

The tag push triggers GoReleaser CI (`.github/workflows/release.yml`) which
builds the 5-platform binaries (darwin-amd64, darwin-arm64, linux-amd64,
linux-arm64, windows-amd64) with SHA256 checksums and cuts the GitHub
Release. No further Claude action required; Phase 9 is complete pending
that user-owned follow-up.
