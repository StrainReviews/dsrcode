---
phase: 05
slug: binary-distribution
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-04-06
---

# Phase 5 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (built-in) |
| **Config file** | go.mod (Go 1.25) |
| **Quick run command** | `cd ~/Projects/cc-discord-presence && go test ./... -count=1 -short` |
| **Full suite command** | `cd ~/Projects/cc-discord-presence && go test ./... -count=1 -race -v` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `cd ~/Projects/cc-discord-presence && go test ./... -count=1 -short`
- **After every plan wave:** Run `cd ~/Projects/cc-discord-presence && go test ./... -count=1 -race -v` + `goreleaser check`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 05-01-01 | 01 | 1 | DIST-08 | — | N/A | unit | `go test ./... -count=1 -short` | main_test.go exists | ⬜ pending |
| 05-01-02 | 01 | 1 | DIST-01 | — | N/A | config-check | `goreleaser check` | N/A (config file) | ⬜ pending |
| 05-02-01 | 02 | 1 | DIST-04 | T-05-01 | HTTPS download, verify checksum | integration | Manual test of start.sh | manual-only | ⬜ pending |
| 05-02-02 | 02 | 1 | DIST-05 | T-05-02 | SHA256 checksum match before use | unit | Manual shellcheck + test | manual-only | ⬜ pending |
| 05-02-03 | 02 | 1 | DIST-06 | — | N/A | integration | `echo $CLAUDE_PLUGIN_DATA && ls ${CLAUDE_PLUGIN_DATA}/bin/` | manual-only | ⬜ pending |
| 05-03-01 | 03 | 2 | DIST-02 | — | N/A | manual | Push test tag, verify release | manual-only | ⬜ pending |
| 05-03-02 | 03 | 2 | DIST-03 | — | N/A | smoke | `goreleaser build --snapshot --clean` | N/A (CI) | ⬜ pending |
| 05-04-01 | 04 | 2 | DIST-07 | — | N/A | smoke | `./scripts/bump-version.sh 99.99.99 && git diff` | manual-only | ⬜ pending |
| 05-04-02 | 04 | 2 | DIST-09 | — | N/A | integration | Start twice, verify no download on second | manual-only | ⬜ pending |
| 05-04-03 | 04 | 2 | DIST-10 | — | N/A | grep | `grep -r 'StrainReviews/dsrcode' scripts/` | manual-only | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `goreleaser` installed locally via `go install github.com/goreleaser/goreleaser/v2@latest` or `brew install goreleaser`
- [ ] Existing go test infrastructure covers DIST-08 (version variable rename)

*Existing infrastructure covers most automated requirements. Shell script testing is manual.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Download + fallback flow | DIST-04 | Shell script integration requires network + filesystem | Run start.sh with/without network, verify binary appears in PLUGIN_DATA/bin/ |
| SHA256 verification | DIST-05 | Requires downloaded binary + checksums.txt | Download binary, run checksum verification, tamper with binary, verify rejection |
| Binary location | DIST-06 | Environment variable dependent | `echo $CLAUDE_PLUGIN_DATA && ls ${CLAUDE_PLUGIN_DATA}/bin/cc-discord-presence` |
| Workflow triggers | DIST-02 | Requires GitHub Actions CI | Push v99.99.99-test tag, verify workflow runs, delete tag |
| 5 platform builds | DIST-03 | Requires goreleaser snapshot | `goreleaser build --snapshot --clean` produces 5 binaries in dist/ |
| Bump script | DIST-07 | Multi-file sed replacement | Run bump, verify main.go + plugin.json + marketplace.json + start.sh + start.ps1 updated |
| Version-check skip | DIST-09 | Timing-dependent | Start binary twice, verify --version is <100ms, no download on second start |
| Repo URL consistency | DIST-10 | grep-based | `grep -r 'StrainReviews/dsrcode' scripts/ .goreleaser.yml .github/` |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 15s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
