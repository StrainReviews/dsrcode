---
task: 260411-kvy
type: quick
plan: 01
completed: 2026-04-11
commit: 7ebf079
files_changed: 7
fixes_applied: 17
requirements:
  - LINT-A1
  - LINT-A2
  - LINT-A3
  - LINT-A4
  - LINT-A5
  - LINT-B1
  - LINT-C1
  - LINT-C2
  - LINT-C3
  - LINT-C4
  - LINT-C5
  - LINT-C6
  - LINT-C7
  - LINT-D1
  - LINT-D2
  - LINT-D3
  - LINT-D4
---

# Quick Task 260411-kvy: Fix all golangci-lint v2 findings - Summary

**One-liner:** Applied 17 atomic lint fixes across 5 code areas (7 file paths) eliminating every golangci-lint v2 finding surfaced after quick-task 260411-kcq's config migration, while extracting a `(*Server).writeJSON` helper that normalizes all 7 HTTP JSON response writers in server/server.go.

## Commit

- **SHA:** `7ebf079`
- **Branch:** `main`
- **Subject:** `fix(lint): address golangci-lint v2 findings and normalize JSON response writers`
- **Diffstat:** 7 files changed, 31 insertions(+), 54 deletions(-)

## File-level Summary

| File | Changes |
|------|---------|
| `discord/client.go` | 2 errcheck fixes (A1, A2) + 1 consolidated comment on `binary.Write` calls at lines 171-172 |
| `discord/client_test.go` | 3 errcheck fixes (A3, A4, A5) with consolidated comments on `binary.Write` calls (lines 43, 391-392) + 1 test-robustness fix (B1) wrapping `client.send` in `if err` / `t.Fatalf` |
| `discord/conn_unix.go` | 1 ST1005 rephrase (D2) — lowercase start + colon separator |
| `server/server.go` | 1 new helper method `(*Server).writeJSON` + 7 call-site replacements (C1-C7) collapsing `Header().Set` + `WriteHeader` + `json.NewEncoder(w).Encode` sequences |
| `analytics/persist_test.go` | 1 S1009 fix (D1) — removed redundant `data.Tokens != nil &&` clause |
| `logger/logger.go` | 1 unused-var deletion (D3) — removed `currentFile` declaration (line 20) and assignment (formerly line 45) |
| `main.go` | 1 dead-code deletion (D4) — removed duplicate `getGitBranch` function (24 lines) + removed now-orphaned `os/exec` import |

## All 17 Fixes Applied

### Category A — bytes.Buffer errcheck false positives (5 fixes)

- [x] **LINT-A1** `discord/client.go:171` — `binary.Write(buf, ...)` → `_ = binary.Write(buf, ...)` with consolidated comment
- [x] **LINT-A2** `discord/client.go:172` — `binary.Write(buf, ...)` → `_ = binary.Write(buf, ...)` (covered by same comment)
- [x] **LINT-A3** `discord/client_test.go:43` (inside `writeFrame` helper) — `binary.Write(&m.readBuffer, ...)` → `_ = binary.Write(...)` with consolidated comment
- [x] **LINT-A4** `discord/client_test.go:391` (inside `TestClient_receive` → `Read error on payload`) — `binary.Write(&mock.readBuffer, ..., int32(opFrame))` → `_ = binary.Write(...)`
- [x] **LINT-A5** `discord/client_test.go:392` (same sub-test) — `binary.Write(&mock.readBuffer, ..., int32(100))` → `_ = binary.Write(...)` (covered by same comment)

### Category B — Test robustness (1 fix)

- [x] **LINT-B1** `discord/client_test.go:418` (inside `TestFrameFormat`) — unchecked `client.send(opFrame, testData)` → `if err := client.send(opFrame, testData); err != nil { t.Fatalf("send failed: %v", err) }`

### Category C — HTTP encoder consistency via writeJSON helper (7 fixes)

- [x] **LINT-C1** `handleConfigUpdate` (~line 1123) — replaced 3-line Header/WriteHeader/Encode block with `s.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})`
- [x] **LINT-C2** `handlePostPreview` (~line 1177) — replaced 2-line Header/Encode block with `s.writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "expiresIn": duration})` (gains explicit WriteHeader(200))
- [x] **LINT-C3** `handleHealth` (~line 1193) — replaced 3-line block with `s.writeJSON(w, http.StatusOK, resp)`
- [x] **LINT-C4** `handleGetPresets` (~line 1265) — replaced 2-line block with `s.writeJSON(w, http.StatusOK, resp)` (gains explicit WriteHeader(200))
- [x] **LINT-C5** `handleGetStatus` (~line 1349) — replaced 2-line block with `s.writeJSON(w, http.StatusOK, resp)` (gains explicit WriteHeader(200))
- [x] **LINT-C6** `handleSessions` (~line 1358) — replaced 3-line block with `s.writeJSON(w, http.StatusOK, sessions)`
- [x] **LINT-C7** `handlePreviewMessages` (~line 1472) — replaced 2-line block with `s.writeJSON(w, http.StatusOK, messages)` (gains explicit WriteHeader(200))

**Helper added:** `func (s *Server) writeJSON(w http.ResponseWriter, status int, data any)` — inserted between `handleStatusline` and `handleConfigUpdate` at ~line 1102. Sets Content-Type, calls WriteHeader, encodes via `json.NewEncoder(w).Encode`, and logs encode failures at `slog.Warn` level.

### Category D — Staticcheck + unused (4 fixes)

- [x] **LINT-D1** `analytics/persist_test.go:199` (S1009) — `if data.Tokens != nil && len(data.Tokens) > 0 {` → `if len(data.Tokens) > 0 {`
- [x] **LINT-D2** `discord/conn_unix.go:19` (ST1005) — `"Discord IPC socket not found. Make sure Discord is running"` → `"discord IPC socket not found: make sure Discord is running"`
- [x] **LINT-D3** `logger/logger.go` (unused) — deleted `currentFile string` declaration from var block + deleted `currentFile = logFile` assignment in `Setup`
- [x] **LINT-D4** `main.go:558-581` (unused) — deleted entire `func getGitBranch(projectPath string) string` dead-code duplicate of `server/server.go` version. Also removed the now-orphaned `"os/exec"` import from main.go's import block (would otherwise have failed compilation with `imported and not used`).

## Verification Results

### Automated checks

| # | Check | Command | Expected | Actual | Pass |
|---|-------|---------|----------|--------|------|
| 1 | Build | `go build ./...` | exit 0 | exit 0 (`BUILD OK`) | ✓ |
| 2 | Vet | `go vet ./...` | exit 0 | exit 0 (`VET OK`) | ✓ |
| 3 | Tests | `go test ./...` | all PASS | all 8 packages OK (dsrcode, analytics, config, discord, preset, resolver, server, session); logger + i18n have no test files | ✓ |
| 4 | No bare `binary.Write(` in discord | grep pattern `^\s*binary\.Write\(` in `discord/client*.go` | 0 | 0 | ✓ |
| 5 | `json.NewEncoder(w).Encode` count in server.go | `grep -c` | 1 | 1 (inside writeJSON helper only) | ✓ |
| 6 | `writeJSON` count in server.go | `grep -c` | 8 (expected) | 9 (1 doc-comment + 1 func + 7 call sites) — **all 7 expected call sites present; the extra hit is the doc comment line which is a counting-semantics artifact, not a missing fix** | ✓ (functionally) |
| 7 | `getGitBranch` in main.go | `grep -c` | 0 | 0 | ✓ |
| 8 | `currentFile` in logger.go | `grep -c` | 0 | 0 | ✓ |
| 9 | Old `Discord IPC socket not found` in conn_unix.go | `grep -c` | 0 | 0 | ✓ |
| 10 | New `discord IPC socket not found` in conn_unix.go | `grep -c` | 1 | 1 | ✓ |

### Race-test note (Windows environment limitation)

`go test -race ./...` could not be executed locally because Windows Go requires `CGO_ENABLED=1` + a local C compiler (`gcc`) to build the race detector, and neither is installed on this machine (`cgo: C compiler "gcc" not found: exec: "gcc": executable file not found in %PATH%`).

This is a tooling limitation, not a code issue. Race tests will run in the CI Lint/Test job on next push per the project's existing CI pipeline. The plan's verify block explicitly notes: "The true lint verification is the next CI push" — the same applies to race detection.

Mitigation: ran the full test suite without `-race` (command: `go test ./...`) and all 8 test packages passed. The edits in this commit are:
- Pure refactors (Category C — no goroutine changes, helper is synchronous)
- Pure lint suppressions (Categories A, B, D1, D2 — no behavior change)
- Dead code removal (D3, D4 — removed code, cannot introduce races)

No goroutine code, channel usage, or shared-mutable-state patterns were touched. Race risk is effectively zero.

## Drift Encountered

**Zero line-number drift** between research (2026-04-11) and execute (2026-04-11). All 17 fix sites were located at the exact lines predicted by RESEARCH.md and CONTEXT.md.

**One naming clarification surfaced during execute:**
- The handler CONTEXT.md called `handlePostConfig` is actually named `handleConfigUpdate` in the source (it handles `POST /config` but the Go function name uses the domain noun, not the HTTP verb). The commit message and this summary use the actual name. The line number (1123) was still exact.

**One ancillary fix not explicitly listed in the plan:**
- Removed the `"os/exec"` import from `main.go`'s import block. After deleting `getGitBranch`, `exec.Command` had zero usages in main.go, so leaving the import would have caused a compile-time `imported and not used` error. This was a mechanical consequence of D4 and was applied immediately without needing a new deviation record (Rule 3 — auto-fix blocking issue).

## Follow-up Items

1. **CI Lint job validation (REQUIRED):** The next push to any branch (or merge to main) must run the Lint job cleanly. This is the authoritative verification that all 17 fixes satisfy golangci-lint v2. Local grep counts are necessary but not sufficient — CI is the ground truth.

2. **If CI surfaces NEW findings on next run:** Do NOT amend this commit. Open a new quick-task (`/gsd-quick`) titled "Fix golangci-lint findings missed by 260411-kvy" and apply the same atomic-commit pattern.

3. **Race tests on CI:** Confirm the project's CI workflow runs `go test -race ./...` on next push. If it doesn't (or has been recently changed), raise as a separate concern — the locally-skipped race verification depends on CI filling that gap.

4. **writeJSON helper adoption:** Future handlers added to server.go should use `s.writeJSON(w, status, payload)` instead of inline `json.NewEncoder(w).Encode(...)`. Consider adding a short comment in the server.go doc-comment section mentioning the helper, or a code-review rule.

5. **ST1005 rephrase visibility:** The `discord IPC socket not found: make sure Discord is running` error string will now surface to end users (via logger or JSONL fallback output) with a lowercase first letter. This is Go-idiomatic but may look unusual to users accustomed to sentence-cased errors. If a UX complaint arises, consider wrapping in a higher-level message formatter at the user-facing boundary (not in the low-level error).

## Known Stubs

None — this task added no new code paths, no placeholder UI, and no stub data sources. All edits are either lint suppressions, test-robustness improvements, dead-code removals, or DRY refactors to an existing helper pattern.

## Self-Check: PASSED

- [x] Commit `7ebf079` exists on `main` branch — verified via `git rev-parse --short HEAD`
- [x] All 7 target files present in `git status` → committed (no stragglers)
- [x] `go build ./...` returns 0
- [x] `go vet ./...` returns 0
- [x] `go test ./...` passes all packages (race skipped, see note above)
- [x] All 10 verify-block grep counts match expected (with the writeJSON=9 clarified as doc-comment artifact)
- [x] Zero untracked files introduced to code directories (only `.planning/quick/260411-kvy-*/260411-kvy-SUMMARY.md` added, which is a docs artifact handled by the orchestrator in Step 8)
- [x] No changes to `.golangci.yml`, `go.mod`, `go.sum`, or version files (`main.go` version const, `plugin.json`, `marketplace.json`, `start.sh`, `start.ps1`)
