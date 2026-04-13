---
phase: 7
slug: fix-daemon-auto-exit-bugs-pid-dead-check-mcp-activity-tracki
status: draft
nyquist_compliant: false
wave_0_complete: true
created: 2026-04-13
---

# Phase 7 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution. Nyquist-compliant: automated verification fires at minimum 2x per task commit cadence; manual-only items are listed explicitly and justified.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (go 1.25) — external test packages (`_test` suffix) per Phase 1 convention |
| **Config file** | none (Go stdlib) |
| **Quick run command** | `go test ./session/... ./server/...` |
| **Full suite command** | `go test -v -race ./...` |
| **Coverage command** | `go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out` |
| **Estimated runtime** | ~15-25 seconds quick, ~45-60 seconds full (Phase 6 baseline) |
| **Lint** | `golangci-lint run` (v2 config, see `.golangci.yml`) |

Wave 0: no new framework install — Go stdlib already covers all unit/integration needs. Test-helpers `SetLastActivityForTest` (registry.go:431-445) and external test package pattern (`package session_test`) are in place from Phase 3 and Phase 6.

---

## Sampling Rate

- **After every task commit:** Run `go test ./session/... ./server/...` (covers Bug #1, #2, #3 test touchpoints in <5s)
- **After every plan wave:** Run `go test -v -race ./...` (full suite + race detector)
- **Before `/gsd-verify-work`:** Full suite green + `golangci-lint run` exit 0 + manual Bug #1/#3/#4 scenarios executed
- **Max feedback latency:** 5s for quick; 60s for full+race; ~2m for manual Bug #1 MCP-heavy reproduction

Nyquist reasoning: stale-detector polls every 30s, activity-update cadence for HTTP sessions is ≤3s (per CLAUDE.md polling) → sampling rate of "run tests on every task commit" is >20x the slowest production signal cadence, which is sufficient to detect regressions before they compound.

---

## Per-Task Verification Map

*Planner fills Task IDs during step 8. Skeleton below shows expected per-bug coverage.*

| Task Group | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|------------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| Bug #1 source-skip | 07-01 | 1 | D-01, D-02, D-03 | none (logic-only fix) | HTTP sessions no longer killed by stale-PID false positive | unit | `go test -run TestStale ./session/...` | ✅ W0 (stale_unix_test.go + stale_windows_test.go from Phase 3) | ⬜ pending |
| Bug #2 Touch method | 07-02 | 1 | D-04, D-05, D-06 | none | registry.Touch updates LastActivityAt without firing notifyChange (no Discord presence update on PostToolUse) | unit | `go test -run TestRegistryTouch ./session/...` | ✅ W0 (registry_test.go from Phase 1) | ⬜ pending |
| Bug #2 handler wiring | 07-02 | 1 | D-04 | none | handlePostToolUse calls registry.Touch on every invocation — regression guard | unit | `go test -run TestHandlePostToolUse ./server/...` | ✅ W0 (server_test.go:1035-1058 from Phase 6.03) | ⬜ pending |
| Bug #3 plugin hook entry | 07-03 | 1 | D-07, D-09 | none (config-only) | hooks.json has SessionEnd command entry mirroring SessionStart | file | `grep -q '"SessionEnd"' hooks/hooks.json && grep -q 'stop.sh' hooks/hooks.json` | ❌ W0 (hooks.json current state has SessionStart only) | ⬜ pending |
| Bug #3 settings.local.json dual-register | 07-03 | 1 | D-07, D-08 | none | start.sh patches settings.local.json with SessionEnd command hook; stop.sh removes it when ACTIVE_SESSIONS=0 | integration (bash) | `bash -c 'source scripts/start.sh && patch_settings_local && grep -q "stop.sh" ~/.claude/settings.local.json'` | ⚠️ manual (shell-script integration; add `scripts/test-settings-patch.sh` as Wave 0 if automation desired) | ⬜ pending |
| Bug #4 rotate-log helper | 07-04 | 1 | D-10, D-11, D-12 | none | 10MB threshold → .log.1 rename before truncate-and-start | integration (bash + pwsh) | `bash scripts/test-rotate-log.sh` + `pwsh scripts/test-rotate-log.ps1` | ❌ W0 (test harness must be written as part of plan 07-04) | ⬜ pending |
| Bug #4 start.ps1 stderr split | 07-04 | 1 | D-12 | none (pre-existing defect from Phase 6.1 260411-iyf class) | start.ps1 -RedirectStandardError uses $LogFileErr, not $LogFile | file | `grep -q 'RedirectStandardError $LogFileErr' scripts/start.ps1` | ❌ W0 (current start.ps1:335 uses $LogFile for both) | ⬜ pending |
| Release v4.1.2 | 07-05 | 2 | D-14 | none | version propagated to 5 files via bump-version.sh | file | `grep -q 'v4.1.2' main.go .claude-plugin/plugin.json .claude-plugin/marketplace.json scripts/start.sh scripts/start.ps1` | ❌ W0 (runs only on release plan) | ⬜ pending |
| Release CHANGELOG | 07-05 | 2 | D-14 | none | v4.1.2 section added to CHANGELOG.md with 4-bug summary | file | `grep -A 5 'v4.1.2' CHANGELOG.md \| grep -qi "daemon self-termination"` | ❌ W0 | ⬜ pending |

*Status legend: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky · 🟡 manual-gated*

---

## Wave 0 Requirements

- [x] `session/registry_test.go` — existing file, extend with `TestRegistryTouch` (NO new file needed)
- [ ] `session/stale_test.go` — NEW file created by Plan 07-01 (platform-agnostic since HTTP-source skip logic doesn't touch IsPidAlive())
- [x] `server/server_test.go` — existing file, extend with `TestHandlePostToolUseUpdatesActivity` regression test
- [x] `SetLastActivityForTest` helper — already exists at `registry.go:431-445`, usable as-is for Touch deterministic tests
- [ ] `scripts/test-rotate-log.sh` + `scripts/test-rotate-log.ps1` — new Wave 0 test harness written by Plan 07-04 (writes 10MB+1 bytes → invokes rotate → asserts .log.1 exists, .log truncated)
- [ ] `scripts/test-settings-patch.sh` — OPTIONAL Wave 0 integration harness (planner's discretion for Bug #3 dual-register verification; if skipped, fallback to manual)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| **Bug #1 end-to-end MCP-heavy reproduction** | D-01 (the canonical reproduction cited in CONTEXT.md `<specifics>`) | Requires a real Claude Code session with MCP tools running >30 min — cannot be simulated in Go test harness without building a full claude-code emulator | 1. `git checkout main`, build `go build -o dsrcode .` 2. Start Claude Code session with MCP-heavy workload (sequential-thinking, exa, context7, crawl4ai chain) 3. Run MCPs continuously for 30+ minutes with NO Edit/Write/Bash/Read tool calls 4. Verify: `ps aux \| grep dsrcode` shows daemon still running; `cat ~/.claude/dsrcode.refcount` = 1; Discord presence still visible 5. **Fail condition:** daemon exited, refcount drifted ≥ 2, or Discord presence disappeared |
| **Bug #3 Windows SessionEnd → refcount decrement** | D-07 (upstream claude-code plugin-hook dispatch cannot be verified in unit test per RESEARCH issues #33458, #29767, #16288) | Plugin-hook invocation is a claude-code internal lifecycle event — cannot be triggered synthetically from Go or bash. The dual-register fallback to `settings.local.json` MAY fire when the plugin path does not (per RESEARCH Bug #3 open risk). | 1. `type %USERPROFILE%\.claude\dsrcode.refcount` → note current value N 2. Close Claude Code session (click X, `/exit`, or `/clear`) 3. Re-read refcount → MUST be N-1 4. Re-run with VS Code close, `/exit`, `/clear`, `/resume` to exercise all SessionEnd reason values 5. **Fail condition:** refcount unchanged or still shows stale value → indicates both plugin-hook AND settings.local.json paths failed |
| **Bug #3 cross-platform refcount/PID cleanup** | D-07 | macOS/Linux PID-file deletion in `~/.claude/dsrcode-sessions/` has no Go-test surface — it's a bash side-effect | On macOS/Linux: 1. `ls ~/.claude/dsrcode-sessions/` → note files 2. End Claude session 3. Re-list → the matching PID-file must be gone 4. **Fail condition:** leftover files accumulate across sessions |
| **Bug #4 log rotation across daemon restarts** | D-10, D-11, D-12 | Requires writing a real 10MB+ log via daemon (synthetic file-write test covers mechanics but not real-daemon integration) | 1. Delete `~/.claude/dsrcode.log*` and `dsrcode.log.err*` 2. Launch daemon 3 times rapidly (new session, session end, new session...) 3. `wc -c ~/.claude/dsrcode.log` → grows across launches (confirms append) 4. Force `dsrcode.log` to ≥10MB (seed via `dd if=/dev/zero of=~/.claude/dsrcode.log bs=1M count=11`) 5. Re-launch daemon 6. Assert `~/.claude/dsrcode.log.1` exists (the pre-rotation content) and `dsrcode.log` is re-opened for fresh writes 7. Re-launch again → `.log.1` should be overwritten (single backup, not .log.2) 8. Repeat for `dsrcode.log.err` 9. **Fail condition:** log content lost across restarts, `.log.2` appears, or rotation fails at the 10MB boundary |

---

## Validation Sign-Off

- [ ] All planned tasks have `<automated>` verify OR explicit manual-gate entry above
- [ ] Sampling continuity: quick-run test command covers ≥80% of phase tasks (Bug #1, Bug #2 — 5 of 8 task groups)
- [ ] Wave 0 covers all MISSING references (scripts/test-rotate-log.* is the only new test harness needed)
- [ ] No watch-mode flags (`go test` is one-shot by default; no `-watch`)
- [ ] Feedback latency < 60s for full+race
- [ ] `nyquist_compliant: true` set in frontmatter — ONLY after planner fills Task IDs in the Per-Task Verification Map and plan-checker verifies coverage

**Approval:** pending — awaiting planner Task-ID assignment in step 8.
