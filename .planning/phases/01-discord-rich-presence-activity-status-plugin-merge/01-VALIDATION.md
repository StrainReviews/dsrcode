---
phase: 01
slug: discord-rich-presence-activity-status-plugin-merge
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-02
---

# Phase 1 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go testing (stdlib) + `go test` |
| **Config file** | none — stdlib, no config needed |
| **Quick run command** | `cd C:/Users/ktown/Projects/cc-discord-presence && go test ./...` |
| **Full suite command** | `cd C:/Users/ktown/Projects/cc-discord-presence && go test -v -race ./...` |
| **Estimated runtime** | ~5 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test ./... -count=1`
- **After every plan wave:** Run `go test -v -race ./...`
- **Before `/gsd:verify-work`:** Full suite must be green + `go vet ./...`
- **Max feedback latency:** 5 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 01-01-01 | 01 | 1 | D-05 | integration | `go test ./server/ -run TestHookEndpoints` | ❌ W0 | ⬜ pending |
| 01-01-02 | 01 | 1 | D-07 | unit | `go test ./server/ -run TestActivityMapping` | ❌ W0 | ⬜ pending |
| 01-01-03 | 01 | 1 | D-22 | unit | `go test ./preset/ -run TestLoadPreset` | ❌ W0 | ⬜ pending |
| 01-01-04 | 01 | 1 | D-23 | unit | `go test ./resolver/ -run TestStablePick` | ❌ W0 | ⬜ pending |
| 01-01-05 | 01 | 1 | D-25 | unit | `go test ./resolver/ -run TestFormatStatsLine` | ❌ W0 | ⬜ pending |
| 01-01-06 | 01 | 1 | D-26 | unit | `go test ./session/ -run TestRegistry -race` | ❌ W0 | ⬜ pending |
| 01-01-07 | 01 | 1 | D-29 | unit | `go test ./session/ -run TestStaleCheck` | ❌ W0 | ⬜ pending |
| 01-01-08 | 01 | 1 | D-33 | unit | `go test ./config/ -run TestConfigPriority` | ❌ W0 | ⬜ pending |
| 01-01-09 | 01 | 1 | D-42 | unit | `go test ./discord/ -run TestBackoff` | ❌ W0 | ⬜ pending |
| 01-01-10 | 01 | 1 | D-49 | integration | `go test ./config/ -run TestConfigWatch` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `server/server_test.go` — HTTP endpoint + activity mapping tests
- [ ] `session/registry_test.go` — concurrent session management tests
- [ ] `resolver/resolver_test.go` — presence resolution + stablePick + stats tests
- [ ] `preset/preset_test.go` — preset loading/validation tests
- [ ] `config/config_test.go` — config priority chain + hot-reload tests
- [ ] `discord/backoff_test.go` — exponential backoff timing tests

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Discord presence visible | D-08 to D-15 | Requires running Discord client | Start binary, check Discord profile shows activity |
| Icon rendering | D-45 to D-47 | Visual verification | Upload icons to Developer Portal, check rendering in Discord |
| Button visibility | D-32 | Only visible to OTHER users | Have second Discord account check presence |
| Plugin installation | D-19 | Requires Claude Code marketplace | Run `claude plugin marketplace add DSR-Labs/cc-discord-presence` |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 5s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
