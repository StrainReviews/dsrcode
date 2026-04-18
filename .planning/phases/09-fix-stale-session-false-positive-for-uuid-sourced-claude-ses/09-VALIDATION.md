---
phase: 9
slug: fix-stale-session-false-positive-for-uuid-sourced-claude-ses
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-18
---

# Phase 9 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Derived from `09-RESEARCH.md §Validation Architecture`. D-05(d) refined to negative assertion per user decision (2026-04-18).

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` 1.26 (standard `go test`) |
| **Config file** | none — Go convention |
| **Quick run command** | `go test -v ./session/... -run TestStaleCheck` |
| **Full suite command** | `go test -race -v ./...` |
| **Estimated runtime** | ~3s quick · ~30–60s full suite on this repo |
| **Slog capture pattern** | `coalescer/coalescer_test.go:267-282` (in-repo reference) |
| **Activity backdate helper** | `session.SetLastActivityForTest` (registry.go:451-461) |

---

## Sampling Rate

- **After every task commit:** Run `go test -v ./session/... -run TestStaleCheck` (<3s)
- **After every plan wave:** Run `go test -race -v ./...` (full suite)
- **Before `/gsd-verify-work`:** Full suite green + `scripts/phase-09/verify.{sh,ps1}` green
- **Max feedback latency:** 3 seconds (quick) · 60 seconds (full)

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 09-01-01 | 01 | 1 | D-03 | — | RED: Phase-7 test fails post-rename | unit | `go test -run TestStaleCheckSkipsPidCheckForClaudeSource -v ./session/...` (must FAIL) | ❌ W0 | ⬜ pending |
| 09-01-02 | 01 | 1 | D-01, D-08 | — | GREEN: guard expansion + godoc | unit | `go test -run TestStaleCheckSkipsPidCheckForClaudeSource -v ./session/...` (must PASS) | ❌ W0 | ⬜ pending |
| 09-01-03 | 01 | 1 | D-05(b) | — | Backstop removeTimeout test | unit | `go test -run TestStaleCheckRemovesClaudeSourceAfterRemoveTimeout -v ./session/...` | ❌ W0 | ⬜ pending |
| 09-01-04 | 01 | 1 | D-05(c) | — | Live-incident regression mirror | unit | `go test -run TestStaleCheckSurvivesUuidSourcedLongRunningAgent -v ./session/...` | ❌ W0 | ⬜ pending |
| 09-01-05 | 01 | 1 | D-05(d) | — | Negative slog assertion (Debug MUST NOT fire for SourceClaude) | unit (slog) | `go test -run TestStaleCheckEmitsDebugOnGuardSkip -v ./session/...` | ❌ W0 | ⬜ pending |
| 09-01-06 | 01 | 1 | D-02, D-04, D-07 | — | Static invariants: no GOOS branch, no synctest, slog line unchanged | static | `! grep -n 'runtime.GOOS' session/stale.go && ! grep -n 'synctest' session/stale_test.go && grep -c 'PID dead but session has recent activity, skipping removal' session/stale.go` | ✅ / ❌ W0 | ⬜ pending |
| 09-01-07 | 01 | 1 | D-01..D-08 | — | Full-suite race-check green | integration | `go test -race -v ./...` | ✅ | ⬜ pending |
| 09-02-01 | 02 | 2 | D-06 | — | `bump-version.sh 4.2.1` propagates across 5 files | integration | `for f in main.go .claude-plugin/plugin.json .claude-plugin/marketplace.json scripts/start.sh scripts/start.ps1; do grep -c '4\.2\.1' "$f"; done` (each ≥1) | ❌ W0 | ⬜ pending |
| 09-02-02 | 02 | 2 | D-06 | — | `scripts/phase-09/verify.sh` + `verify.ps1` exist + executable | integration | `test -x scripts/phase-09/verify.sh && test -f scripts/phase-09/verify.ps1` | ❌ W0 | ⬜ pending |
| 09-02-03 | 02 | 2 | D-09 | — | CHANGELOG [4.2.1] section with live-log forensic anchor | static | `grep -c '## \[4.2.1\]' CHANGELOG.md == 1 && grep -c '2fe1b32a-ea1d-464f-8cac-375a4fe709c9' CHANGELOG.md >= 1` | ❌ W0 | ⬜ pending |
| 09-02-04 | 02 | 2 | D-09 | — | CHANGELOG section ordering `[Unreleased] < [4.2.1] < [4.2.0]` | static | `awk '/## \[/{print NR": "$0}' CHANGELOG.md \| head -3` — check line numbers ascending | ❌ W0 | ⬜ pending |
| 09-02-05 | 02 | 2 | D-06 | — | Verify harness green on live daemon | integration (live) | `bash scripts/phase-09/verify.sh` (all PASS) or `pwsh -File scripts/phase-09/verify.ps1` (all PASS) | ❌ W0 | ⬜ pending |
| 09-02-06 | 02 | 2 | v4.2.1 release gate | — | Manual MCP-heavy >2min silence, daemon stays up | manual E2E | User runs Claude Code with MCP-heavy flow, observes `dsrcode.log` absent of `"removing stale session"` for >5min | manual-only | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `session/stale_test.go` — rename + invert 1 test (D-03), add 3 new tests (D-05 b/c/d). Import `bytes`, `log/slog`, `strings`.
- [ ] `session/stale.go` — 1-line guard insert (D-01), 15-line godoc replace (D-08).
- [ ] `CHANGELOG.md` — new `## [4.2.1] - 2026-04-18` section at line 10, Fixed / Changed / Added subsections, live-incident excerpt (D-09).
- [ ] 5 version files propagated via `./scripts/bump-version.sh 4.2.1`.
- [ ] `scripts/phase-09/verify.sh` — NEW, mirror `scripts/phase-08/verify.sh`.
- [ ] `scripts/phase-09/verify.ps1` — NEW, PowerShell parity.
- [ ] Framework install: NONE needed — `go test` already available; `log/slog`, `bytes`, `strings`, `time` all stdlib.

*Existing infrastructure covers all phase requirements — no new fixtures.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Daemon survives real MCP-heavy session >2min silence | v4.2.1 release gate | Requires live Claude Code + real HTTP hooks + wall-clock time — cannot be faked in unit tests | 1. Build daemon: `go build -o dsrcode .` 2. Start via `scripts/start.sh` (Unix) or `start.ps1` (Windows). 3. Open Claude Code, initiate MCP-heavy flow (e.g. long sequential-thinking + multiple tool calls). 4. Leave Claude idle for >5min. 5. Tail `~/.claude/dsrcode.log`. 6. Assert: NO `"removing stale session"` lines emitted for the UUID session within 5min elapsed. 7. Assert: `dsrcode-sessions/` still contains the session file. |
| Git tag + push triggers GoReleaser | D-06 release gate | User-owned step per CLAUDE.md §Releasing + `3-tag-push-limit` memory — Claude must not automate | 1. After all plans land + full suite green, user runs `git tag v4.2.1` 2. `git push origin main --tags` 3. Observe GitHub Actions — GoReleaser workflow starts on tag push. 4. Wait for 5-platform binaries + SHA256 checksums published to Releases. 5. Confirm `/dsrcode:update` slash command can fetch v4.2.1. |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify (static + unit + integration all present)
- [ ] Wave 0 covers all MISSING references (stale.go, stale_test.go, CHANGELOG, 5 version files, verify scripts)
- [ ] No watch-mode flags (Go test is one-shot)
- [ ] Feedback latency < 3s (quick) / < 60s (full)
- [ ] `nyquist_compliant: true` set in frontmatter after pattern-mapper + planner review

**Nyquist dimensions covered (from 09-RESEARCH §Nyquist Dimensions Covered):**

| Dimension | Evidence |
|-----------|----------|
| Positive path (D-01 works for SourceClaude) | Test (a) `SkipsPidCheckForClaudeSource` |
| Backstop path (removeTimeout still works) | Test (b) `RemovesClaudeSourceAfterRemoveTimeout` |
| Regression anchor (live incident 2fe1b32a / PID 5692 / 150s) | Test (c) `SurvivesUuidSourcedLongRunningAgent` |
| Observability path (negative slog assertion) | Test (d) `EmitsDebugOnGuardSkip` — NEGATIVE form per Pitfall-1 resolution |
| Cross-source invariance (Phase-7 HTTP guard green) | Existing `TestStaleCheckSkipsPidCheckForHttpSource` stays green |
| No downgrade to SourceJSONL (PID=0 excluded) | Existing `s.PID > 0` gate preserved; acceptance: `grep -c 's.PID > 0' session/stale.go == 1` |
| Release artefact integrity | verify.sh T1 (version), T2+ (live daemon smoke), CHANGELOG grep |
| Manual end-user validation | MCP-heavy >2min silence scenario (09-02 checklist) |

**Approval:** pending
