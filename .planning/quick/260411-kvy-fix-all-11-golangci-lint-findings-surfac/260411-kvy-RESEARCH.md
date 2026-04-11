---
name: 260411-kvy-RESEARCH
description: Codebase verification for fixing all 11 golangci-lint findings
type: research
---

# Quick Task 260411-kvy: Fix all 11 golangci-lint findings — Research

**Researched:** 2026-04-11
**Mode:** quick-task (code-level verification only — all best-practices decisions LOCKED in CONTEXT.md)
**Confidence:** HIGH (every claim below traced to line numbers via Read/Grep in this session)

## Summary

All 11 CONTEXT.md line numbers verified in current tree — **zero drift**. The 11 LOCKED decisions are implementable as-written with one caveat, one minor amendment, and one scope-creep question for the planner.

**TL;DR for the planner:**
1. **Line numbers are stable.** Proceed with fixes exactly as CONTEXT.md specifies.
2. **Dead-code grep is clean.** `currentFile` has zero readers outside logger.go; `getGitBranch` in main.go has zero callers inside main.go.
3. **DISCOVERY — 2 additional errcheck hits likely missed in CONTEXT.md:** `discord/client_test.go:391-392` (inside `TestClient_receive/"Read error on payload"` sub-test) contains two more unchecked `binary.Write(&mock.readBuffer, ...)` calls structurally identical to line 43. If errcheck flagged line 43, it **must have also flagged these two** — either CONTEXT.md under-counted (likely) or they are somehow excluded. **Planner must treat 391+392 as a 12th and 13th fix or confirm via fresh lint run that they're out of scope.** Apply the same inline `_ =` pattern.
4. **DISCOVERY — 4 additional `json.NewEncoder(w).Encode(...)` call sites exist in server.go** at lines 1265, 1349, 1358, 1472. These are unchecked too but were not flagged in CONTEXT.md's Category C. **Recommend:** either (a) leave them alone to keep scope tight and add a follow-up task, or (b) include them in the `writeJSON` refactor for consistency. See §3 below for the detailed call-by-call analysis. My recommendation: **include them** — the helper exists anyway, adding 4 more call-site replacements is ~4 lines of churn with zero risk and produces a fully consistent codebase. If this changes the task scope materially, the planner should surface to user before execute.
5. **`handlePostPreview` implicit-200 normalization is safe.** Existing test (`TestPostPreview` line 774) asserts `w.Code == http.StatusOK`, which is what `httptest.ResponseRecorder` returns even without an explicit `WriteHeader`. Making it explicit in the helper keeps all tests green.
6. **Import surface clean.** `log/slog` and `encoding/json` already imported in server.go. No new imports needed.
7. **Zero test rewrites needed.** No existing test asserts on the exact sequence of `WriteHeader`/`Encode` — they all use `httptest.ResponseRecorder` + `w.Code` + body decoding.

## 1. Line-Number Verification (all 11 findings)

| # | CONTEXT.md Location | Verified? | Current content (trimmed) |
|---|---------------------|-----------|---------------------------|
| A1 | `discord/client.go:171` | ✓ EXACT | `binary.Write(buf, binary.LittleEndian, int32(opcode))` |
| A2 | `discord/client.go:172` | ✓ EXACT | `binary.Write(buf, binary.LittleEndian, int32(len(payload)))` |
| A3 | `discord/client_test.go:43` | ✓ EXACT | `binary.Write(&m.readBuffer, binary.LittleEndian, opcode)` |
| B1 | `discord/client_test.go:418` | ✓ EXACT | `client.send(opFrame, testData)` (inside `TestFrameFormat(t *testing.T)` — `t.Fatalf` available) |
| C1 | `server/server.go:1123` | ✓ EXACT | `json.NewEncoder(w).Encode(map[string]bool{"ok": true})` (handlePostConfig) |
| C2 | `server/server.go:1177` | ✓ EXACT | `json.NewEncoder(w).Encode(map[string]interface{}{...})` (handlePostPreview) |
| C3 | `server/server.go:1193` | ✓ EXACT | `json.NewEncoder(w).Encode(resp)` (handleHealth) |
| D1 | `analytics/persist_test.go:199` | ✓ EXACT | `if data.Tokens != nil && len(data.Tokens) > 0 {` |
| D2 | `discord/conn_unix.go:19` | ✓ EXACT | `return nil, fmt.Errorf("Discord IPC socket not found. Make sure Discord is running")` |
| D3 | `logger/logger.go:20` + `:45` | ✓ EXACT | Declaration `currentFile string` line 20, assignment `currentFile = logFile` line 45 |
| D4 | `main.go:558` | ✓ EXACT | `func getGitBranch(projectPath string) string {` |

**No line drift.** Safe to apply surgical Edits by exact line/string.

## 2. Dead-Code Grep Verification

### 2.1 `currentFile` (for D-3 deletion)
Grep for `currentFile` across entire repo (excluding `.planning/*` meta-docs):
- `logger/logger.go:20` — declaration ✓
- `logger/logger.go:45` — assignment ✓
- **Zero other hits in any .go file.** Safe to delete both lines.

### 2.2 `getGitBranch` (for D-4 deletion)
Grep for `getGitBranch` across all `.go` files:
- `main.go:558` — definition (dead)
- `server/server.go:1087` — caller (calls server-package version)
- `server/server.go:1555` — comment
- `server/server.go:1557` — active definition
- **Zero other .go hits.** Critically, **`main.go` has NO internal call to `getGitBranch`** — the only caller anywhere is `server.go:1087`, which resolves to the server-package function. Safe to delete `main.go:558-581`.

The function body in main.go (lines 558-581) is self-contained: no receiver, no closure captures, no shared helpers. Plain delete.

## 3. DISCOVERY — Additional `json.NewEncoder(w).Encode(...)` Sites

Grep in `server/server.go` found **7 total** `json.NewEncoder(w).Encode(...)` calls. CONTEXT.md flags 3. The other 4:

| Line | Handler | Context | Recommendation |
|------|---------|---------|----------------|
| 1265 | `handleGetPresets` | After `w.Header().Set(Content-Type)` — **no WriteHeader** (implicit 200, same pattern as handlePostPreview) | **Include in writeJSON refactor** |
| 1349 | `handleGetStatus` | After `w.Header().Set(Content-Type)` — **no WriteHeader** (implicit 200) | **Include in writeJSON refactor** |
| 1358 | `handleSessions` | After `w.Header().Set(...)` + **explicit `w.WriteHeader(http.StatusOK)`** | **Include in writeJSON refactor** |
| 1472 | `handlePreviewMessages` | After `w.Header().Set(Content-Type)` — **no WriteHeader** (implicit 200) | **Include in writeJSON refactor** |

**Why errcheck did not flag these 4:** likely because the `exclusions.presets` in .golangci.yml v2 includes `std-error-handling` — which explicitly excludes common stdlib error-return patterns. But `std-error-handling` is documented to exclude `fmt.Print*`, `os.Close`, NOT `Encoder.Encode`. So these 4 **should have been flagged** unless there's some path-based exclusion I'm not seeing. The 3 lines that DID get flagged are all in request handlers that call `w.WriteHeader(...)` FIRST then Encode — perhaps errcheck applies location heuristics.

**Regardless of why errcheck missed them, the planner has a choice:**

- **Option A (minimum scope):** Fix only the 3 flagged lines. Leaves inconsistency: half the server uses `writeJSON`, half uses inline `json.NewEncoder`. Future additions may not notice the helper exists. Lint may catch the other 4 on a future toolchain version, requiring a follow-up task.
- **Option B (recommended):** Fix all 7 sites. +4 line replacements, zero risk, consistent codebase, helper becomes the established pattern. Commit message can still be `fix(lint): address all 11 golangci-lint v2 findings` with a body note: "Also normalized 4 additional `json.NewEncoder` sites in server.go for consistency with the new `writeJSON` helper."

**My research recommendation: Option B.** Include the 4 additional sites. Scope impact is minimal (each site becomes a 1-line call to `s.writeJSON`), all 4 sites have existing test coverage via `server_test.go` (tests for `/presets`, `/status`, `/sessions`, `/preview/messages`), and all 4 assert only on `w.Code` + decoded body (no WriteHeader-sequence assertions). **The planner should surface this choice to the user** because "fix all 11" literally means 11, but "fix all lint issues and leave server.go consistent" is a tighter product.

## 4. DISCOVERY — Possibly Additional errcheck Hits in client_test.go

Grep for `binary.Write` across `discord/` found **5 unchecked sites**, not 3:

| Line | Target | Listed in CONTEXT.md? |
|------|--------|------------------------|
| `client.go:171` | `buf` (bytes.Buffer) | ✓ A1 |
| `client.go:172` | `buf` (bytes.Buffer) | ✓ A2 |
| `client_test.go:43` | `&m.readBuffer` (bytes.Buffer) | ✓ A3 |
| `client_test.go:391` | `&mock.readBuffer` (bytes.Buffer) | **✗ NOT LISTED** |
| `client_test.go:392` | `&mock.readBuffer` (bytes.Buffer) | **✗ NOT LISTED** |

Lines 391-392 are inside `TestClient_receive` / `t.Run("Read error on payload", ...)`. They are structurally **identical** to line 43 (same function signature, same target type `bytes.Buffer`). If errcheck flagged line 43, it **must** also flag 391 and 392.

**Likely explanation:** the CI errcheck output contained more than 11 findings and CONTEXT.md's author counted the 3 Category A hits from the first errcheck section only, missing the duplicate hits in `TestClient_receive`. Worth checking the raw CI output.

**Planner action:**
1. **Preferred:** Treat lines 391-392 as A4 and A5 — apply the same inline `_ =` pattern with comment. Zero risk, 2-line churn, prevents the next CI run from re-failing.
2. **Minimum:** Leave them. If they reappear in the next CI run, open a tiny follow-up.

**My research recommendation: fix them.** They're the same pattern, same file, same commit can include them. The task title says "fix all 11 golangci-lint findings" — if CI reveals 13, the scope adjusts. Better to over-fix than leave obvious same-pattern twins.

## 5. Cat C Behavior Normalization — Verified Safe

CONTEXT.md notes `handlePostPreview` (line 1177) currently sets Content-Type but does NOT call `WriteHeader` — `Encode` triggers implicit 200. The new `writeJSON` helper makes `WriteHeader(200)` explicit.

**Verification that this is test-safe:**
- `server_test.go:774` (`TestPostPreview`): asserts `w.Code != http.StatusOK` → fails.
- `server_test.go:817` (`TestPostPreviewDefaults`): same pattern.
- `server_test.go:853` + `server_preview_test.go:55,107,153`: same pattern.

All tests use `httptest.NewRecorder()`, whose `.Code` field **defaults to 200** when no `WriteHeader` is called. Adding an explicit `WriteHeader(200)` cannot change the observed value — it just makes the server's wire behavior match the test's already-assumed value. **Zero test breakage.**

Same analysis for any of the 4 Option-B sites (1265, 1349, 1472 all have no explicit WriteHeader currently; 1358 already has `WriteHeader(http.StatusOK)`).

## 6. Import Surface Check

`server/server.go` already imports (lines 5-28):
- `"encoding/json"` ✓ (line 7)
- `"log/slog"` ✓ (line 10)
- `"net/http"` ✓ (line 12)

**No new imports needed** for the `writeJSON` helper. The helper signature `func (s *Server) writeJSON(w http.ResponseWriter, status int, data any)` uses only stdlib types already in scope.

For `discord/client_test.go:418` fix (`t.Fatalf("send failed: %v", err)`): the enclosing function is `func TestFrameFormat(t *testing.T)` (line 401). `t` is in scope, `t.Fatalf` is available, no imports needed.

## 7. Test-Rewrite Risk — None Found

Grep across `server/*_test.go` for patterns asserting on `w.Header()` or `WriteHeader` call sequences: **zero hits**. All server tests use `httptest.ResponseRecorder` and assert only on:
- `w.Code` (status code)
- `w.Body` (decoded JSON)
- Side effects (callback invocation, registry state)

The `writeJSON` extraction is behavior-preserving across all observable dimensions these tests check.

## 8. Per-Finding Risk Enumeration

| # | Fix | Risk | Notes |
|---|-----|------|-------|
| A1-A2 | Inline `_ =` on client.go:171-172 | **Risk-free** | Pure lint suppression; stdlib guarantees nil error |
| A3 | Inline `_ =` on client_test.go:43 | **Risk-free** | Test helper, same guarantee |
| A4-A5 | (New) Inline `_ =` on client_test.go:391-392 | **Risk-free** | Same pattern; see §4 |
| B1 | Wrap client_test.go:418 in `if err := ...; err != nil { t.Fatalf(...) }` | **Risk-free** | Test-only; makes existing implicit success explicit |
| C1-C3 | Extract `writeJSON` helper; replace 3 call sites | **Behavior normalization** (1177 gains explicit WriteHeader, verified safe §5) | Option A minimum |
| C4-C7 | (Option B) Also replace lines 1265, 1349, 1358, 1472 | **Behavior normalization** (3 of 4 gain explicit WriteHeader, 1 already has it) | Planner decision §3 |
| D1 | Remove `!= nil` clause in persist_test.go:199 | **Risk-free** | `len(nil map)` is spec-defined as 0 |
| D2 | Rephrase conn_unix.go:19 error string | **Minor behavior change** | Error text visible to user — "Discord IPC socket not found. Make sure Discord is running" → "discord IPC socket not found: make sure Discord is running". Grep confirmed no tests match on this exact string, and no callers parse it |
| D3 | Delete logger.go:20 and :45 | **Risk-free** | Confirmed zero readers §2.1 |
| D4 | Delete main.go:558-581 (entire getGitBranch function) | **Risk-free** | Confirmed zero main.go callers §2.2 |

**Orchestrator's belief:** "only Cat C has any behavior implication" — **refined:** Cat C (Option B adds 3 more implicit-200 normalizations of the same class) and Cat D2 (error string text change). D2 is user-facing but no automated test nor downstream parser consumes the string; treat as cosmetic.

## 9. MUST-FIX Issues Found

1. **Lines 391-392 in client_test.go** — likely additional errcheck hits omitted from CONTEXT.md's count of 11. Treat as A4/A5.
2. **4 additional `json.NewEncoder` sites in server.go** — inconsistency risk if not included in `writeJSON` refactor. Recommended Option B.
3. **(Advisory)** The actual golangci-lint CI output log should be re-examined to confirm items 1 and 2 above — my research is based on static code inspection, not a fresh lint run. If planner has access to the CI run artifacts, cross-check before finalizing scope.

## 10. Planner Handoff Checklist

Before executing:
- [ ] Decide Option A vs Option B for Cat C scope (recommend B)
- [ ] Decide whether to include client_test.go:391-392 as A4/A5 (recommend yes)
- [ ] If Option B + A4/A5 both accepted, final scope becomes **15 fixes**, not 11 — consider amending commit message to `fix(lint): address golangci-lint v2 findings and normalize JSON response writers`
- [ ] Optional: re-run golangci-lint output capture to confirm the hypothetical 391/392 hits before fix

## Sources

- `.planning/quick/260411-kvy-.../260411-kvy-CONTEXT.md` — LOCKED decisions
- `.planning/STATE.md` — project state
- `./CLAUDE.md` — Go 1.25, project conventions
- `.planning/phases/06-hook-system-overhaul-.../06-04-SUMMARY.md` — confirms getGitBranch duplication
- Code files verified via Read/Grep this session: discord/client.go, discord/client_test.go, discord/conn_unix.go, server/server.go, server/server_test.go, server/server_preview_test.go, analytics/persist_test.go, logger/logger.go, main.go, .golangci.yml

**Research date:** 2026-04-11
**Valid until:** until next edit to any of the above files (line numbers will drift)
