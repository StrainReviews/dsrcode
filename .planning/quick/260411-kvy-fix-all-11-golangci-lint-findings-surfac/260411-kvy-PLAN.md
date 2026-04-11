---
task: 260411-kvy
type: quick
plan: 01
wave: 1
depends_on: []
files_modified:
  - discord/client.go
  - discord/client_test.go
  - discord/conn_unix.go
  - server/server.go
  - analytics/persist_test.go
  - logger/logger.go
  - main.go
autonomous: true
requirements:
  - LINT-A1  # bytes.Buffer binary.Write errcheck (discord/client.go:171)
  - LINT-A2  # bytes.Buffer binary.Write errcheck (discord/client.go:172)
  - LINT-A3  # bytes.Buffer binary.Write errcheck (discord/client_test.go:43)
  - LINT-A4  # bytes.Buffer binary.Write errcheck (discord/client_test.go:391) [researcher discovery]
  - LINT-A5  # bytes.Buffer binary.Write errcheck (discord/client_test.go:392) [researcher discovery]
  - LINT-B1  # test-robustness errcheck on client.send (discord/client_test.go:418)
  - LINT-C1  # json.NewEncoder errcheck (server/server.go:1123 handlePostConfig)
  - LINT-C2  # json.NewEncoder errcheck (server/server.go:1177 handlePostPreview)
  - LINT-C3  # json.NewEncoder errcheck (server/server.go:1193 handleHealth)
  - LINT-C4  # json.NewEncoder consistency (server/server.go:1265 handleGetPresets) [researcher discovery]
  - LINT-C5  # json.NewEncoder consistency (server/server.go:1349 handleGetStatus) [researcher discovery]
  - LINT-C6  # json.NewEncoder consistency (server/server.go:1358 handleSessions) [researcher discovery]
  - LINT-C7  # json.NewEncoder consistency (server/server.go:1472 handlePreviewMessages) [researcher discovery]
  - LINT-D1  # staticcheck S1009 redundant nil check (analytics/persist_test.go:199)
  - LINT-D2  # staticcheck ST1005 error string style (discord/conn_unix.go:19)
  - LINT-D3  # unused var currentFile (logger/logger.go:20,45)
  - LINT-D4  # unused func getGitBranch (main.go:558-581)
must_haves:
  truths:
    - "CI Lint job runs cleanly on the next push (all 17 flagged + discovered findings eliminated)"
    - "go build ./... succeeds"
    - "go vet ./... succeeds"
    - "go test -race ./... passes with zero regressions"
    - "Only one definition of json.NewEncoder(w).Encode exists in server.go — inside writeJSON helper"
    - "getGitBranch exists only in server/server.go (main.go copy deleted)"
    - "currentFile symbol is absent from logger/logger.go"
    - "discord/conn_unix.go error string starts lowercase and uses colon separator"
  artifacts:
    - path: "discord/client.go"
      provides: "client.go with _ = binary.Write lines 171-172 + consolidated comment"
    - path: "discord/client_test.go"
      provides: "client_test.go with _ = binary.Write lines 43, 391, 392 + t.Fatalf-wrapped send call near line 418"
    - path: "discord/conn_unix.go"
      provides: "conn_unix.go with rephrased lowercase error string"
    - path: "server/server.go"
      provides: "writeJSON helper method + 7 call sites using s.writeJSON (1123, 1177, 1193, 1265, 1349, 1358, 1472)"
      contains: "func (s *Server) writeJSON"
    - path: "analytics/persist_test.go"
      provides: "persist_test.go with redundant nil clause removed at line 199"
    - path: "logger/logger.go"
      provides: "logger.go with currentFile declaration and assignment deleted"
    - path: "main.go"
      provides: "main.go with getGitBranch function (lines 558-581) deleted"
  key_links:
    - from: "server/server.go handlers (1121, 1176, 1191, 1263, 1347, 1356, 1470)"
      to: "server/server.go writeJSON helper"
      via: "s.writeJSON(w, http.StatusOK, payload) method call"
      pattern: "s\\.writeJSON\\(w,"
    - from: "discord/client_test.go TestFrameFormat (around line 418)"
      to: "discord/client.send"
      via: "if err := client.send(opFrame, testData); err != nil { t.Fatalf(...) }"
      pattern: "if err := client\\.send"
---

<objective>
Eliminate every golangci-lint v2 finding surfaced after quick-task 260411-kcq's config migration and normalize JSON response writers in server/server.go as one atomic unit.

Purpose: Make the CI Lint job green on the next push AND produce a fully consistent server.go JSON-encoding surface. The 11 original findings, the 2 researcher-discovered errcheck twins, and the 4 researcher-discovered json.NewEncoder consistency sites all share the same root cause story ("v1 config-parse error masked them") and the same commit window. Splitting would fragment the narrative and leave the codebase half-migrated between two patterns.

Output:
- 5 files edited (discord/client.go, discord/client_test.go, discord/conn_unix.go, server/server.go, analytics/persist_test.go, logger/logger.go, main.go — 7 paths, 5 "code areas")
- 1 new private method `(*Server).writeJSON` in server/server.go
- 2 deletions (logger.go `currentFile`, main.go `getGitBranch`)
- 1 atomic commit: `fix(lint): address golangci-lint v2 findings and normalize JSON response writers`
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/quick/260411-kvy-fix-all-11-golangci-lint-findings-surfac/260411-kvy-CONTEXT.md
@.planning/quick/260411-kvy-fix-all-11-golangci-lint-findings-surfac/260411-kvy-RESEARCH.md
@CLAUDE.md
@.planning/STATE.md

# Target files (executor will Read these at the start of Task 1, before any Edit)
@discord/client.go
@discord/client_test.go
@discord/conn_unix.go
@server/server.go
@analytics/persist_test.go
@logger/logger.go
@main.go

<interfaces>
<!-- Key contracts the executor needs. All stdlib; no new imports required. -->

Already imported in server/server.go (verified lines 5-28 of that file):
- "encoding/json"   // for json.NewEncoder
- "log/slog"        // for slog.Warn
- "net/http"        // for http.ResponseWriter, http.StatusOK

New helper signature to add in server/server.go (place near other private helpers,
e.g., after the last existing unexported method, not inside any handler):

```go
// writeJSON writes a JSON response with the given status and logs encode
// failures at Warn level (typically client disconnect after WriteHeader).
func (s *Server) writeJSON(w http.ResponseWriter, status int, data any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    if err := json.NewEncoder(w).Encode(data); err != nil {
        slog.Warn("failed to encode response", "error", err)
    }
}
```

slog call style (matches existing server.go pattern — positional k/v, not slog.Attr):
    slog.Warn("failed to encode response", "error", err)

Discord send signature (for Category B fix in client_test.go):
    func (c *Client) send(op int32, payload []byte) error
Enclosing test: func TestFrameFormat(t *testing.T) — t.Fatalf is in scope.
</interfaces>
</context>

<tasks>

<task type="auto" tdd="false">
  <name>Task 1: Apply all 17 lint fixes atomically across 7 files</name>
  <files>
    discord/client.go,
    discord/client_test.go,
    discord/conn_unix.go,
    server/server.go,
    analytics/persist_test.go,
    logger/logger.go,
    main.go
  </files>
  <action>
This task applies every fix in CONTEXT.md plus the researcher-discovered additional sites. **Read each target file fully before editing it** — line numbers in this plan are verified-current at plan time but may drift between plan and execute if any intermediate edit happens. If a grep no longer finds the expected string, re-read the file and locate the equivalent code before editing; do NOT force a match.

Order of edits (follow this sequence; it minimizes re-reads and keeps verify-green):

### Step 1 — discord/client.go (Category A1 + A2)

Replace the current 2-line block at lines 171-172:

```
binary.Write(buf, binary.LittleEndian, int32(opcode))
binary.Write(buf, binary.LittleEndian, int32(len(payload)))
```

With a 3-line block (single consolidated comment above both lines):

```
// bytes.Buffer.Write never returns an error; binary.Write inherits that guarantee
_ = binary.Write(buf, binary.LittleEndian, int32(opcode))
_ = binary.Write(buf, binary.LittleEndian, int32(len(payload)))
```

Preserve surrounding whitespace/indentation exactly as the original file.

### Step 2 — discord/client_test.go (Categories A3, A4, A5, B1)

**A3 — line 43 area:** In the `writeFrame` test helper, replace the existing `binary.Write(&m.readBuffer, binary.LittleEndian, opcode)` call with:

```
// bytes.Buffer.Write never returns an error; binary.Write inherits that guarantee
_ = binary.Write(&m.readBuffer, binary.LittleEndian, opcode)
```

**A4 + A5 — lines 391-392 area:** Inside `TestClient_receive` → `t.Run("Read error on payload", ...)`, there are two structurally-identical `binary.Write(&mock.readBuffer, ...)` calls. Replace the 2-line block with:

```
// bytes.Buffer.Write never returns an error; binary.Write inherits that guarantee
_ = binary.Write(&mock.readBuffer, binary.LittleEndian, int32(opFrame))
_ = binary.Write(&mock.readBuffer, binary.LittleEndian, int32(5))
```

(Use the actual arguments present in the current file — Read it first and substitute whatever the second arg of each call currently is. The comment + `_ =` prefix are what matters.)

**B1 — line 418 area:** The call `client.send(opFrame, testData)` currently discards its error. Replace with:

```
if err := client.send(opFrame, testData); err != nil {
    t.Fatalf("send failed: %v", err)
}
```

Keep the following line (`frame := mock.writeBuffer.Bytes()`) unchanged. The enclosing function is `TestFrameFormat(t *testing.T)`, so `t.Fatalf` is in scope.

### Step 3 — discord/conn_unix.go (Category D2)

Line 19 replacement (single-line surgical edit):

Before:
```
return nil, fmt.Errorf("Discord IPC socket not found. Make sure Discord is running")
```

After:
```
return nil, fmt.Errorf("discord IPC socket not found: make sure Discord is running")
```

### Step 4 — server/server.go (Categories C1-C7)

**4a — Add the writeJSON helper.** Insert the helper method shown in the `<interfaces>` block above. Choose an insertion point near other private `(s *Server)` helper methods (e.g., after the last existing unexported Server method, before the handler section, or immediately above the first handler that will use it — whichever produces the smallest diff with the existing file structure). Add it exactly once.

**4b — Replace all 7 call sites.** For each of the 7 json.NewEncoder sites, collapse the `w.Header().Set(...) + (optional) w.WriteHeader(...) + json.NewEncoder(w).Encode(...)` sequence into a single `s.writeJSON(w, http.StatusOK, <payload>)` call.

Call site transformations (CONTEXT.md line numbers; verify by grep before each Edit):

- **handlePostConfig (~1121-1123):**
  `s.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})`

- **handlePostPreview (~1176-1180):**
  `s.writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "expiresIn": duration})`
  (The `duration` variable name must match the local variable in the current handler — Read the file and use the exact name.)

- **handleHealth (~1191-1193):**
  `s.writeJSON(w, http.StatusOK, resp)`

- **handleGetPresets (~1263-1265):**
  `s.writeJSON(w, http.StatusOK, resp)`

- **handleGetStatus (~1347-1349):**
  `s.writeJSON(w, http.StatusOK, resp)`

- **handleSessions (~1356-1358):**
  `s.writeJSON(w, http.StatusOK, sessions)`
  (This site currently has an explicit `w.WriteHeader(http.StatusOK)` — delete it along with the Header().Set line and Encode line; replace all three with the single helper call.)

- **handlePreviewMessages (~1470-1472):**
  `s.writeJSON(w, http.StatusOK, messages)`

**CRITICAL:** At each site, remove the preceding `w.Header().Set("Content-Type", "application/json")` line AND (where present) the preceding `w.WriteHeader(...)` line, because the helper performs both operations. Do NOT leave stale `w.Header().Set` or `w.WriteHeader` lines above any of the 7 replaced sites.

**CRITICAL:** Do NOT touch any other `json.NewEncoder(...)` that encodes to a non-`http.ResponseWriter` target (e.g., encoding to a file, to `os.Stdout`, to a bytes.Buffer). `writeJSON` is specifically for HTTP response writes. A grep for `json.NewEncoder(w)` should narrow this to the right sites.

### Step 5 — analytics/persist_test.go (Category D1)

Line 199 replacement:

Before:
```
if data.Tokens != nil && len(data.Tokens) > 0 {
```

After:
```
if len(data.Tokens) > 0 {
```

### Step 6 — logger/logger.go (Category D3)

Delete **two lines**:
- Line 20: `var currentFile string` (or whatever the exact declaration is — Read first to confirm exact syntax, including any leading/trailing whitespace or tab indent)
- Line 45: the `currentFile = logFile` assignment (exact text — Read first to confirm)

After deletion, `currentFile` must appear zero times in logger/logger.go. If there is any third reference (shouldn't be — research confirmed only declaration + assignment), stop and report it rather than deleting it blindly.

### Step 7 — main.go (Category D4)

Delete the entire `func getGitBranch(projectPath string) string { ... }` function starting at line 558. The function is self-contained (24 lines, lines 558-581) and has zero callers in main.go. Delete from the leading comment (if any immediately precedes it) through the closing `}` on line 581. If the function has a preceding comment block that is specifically about it (e.g., `// getGitBranch returns ...`), delete that too. Do NOT delete blank lines that separate the function from adjacent functions.

After deletion, `getGitBranch` must appear zero times in main.go.

### Global rules for this task

- Use the Edit tool with exact old_string matches for all changes except the two full deletions in Step 6 and Step 7, which can also use Edit with the full block as old_string.
- Do NOT run `gofmt` or `goimports` as separate steps — Go editor tooling via Claude's Edit should preserve formatting. If `go build` flags a formatting issue after edits, then run `gofmt -w <file>` on the offending file.
- Do NOT add any new imports. All required packages (`log/slog`, `encoding/json`, `net/http` for server.go; `binary`, `bytes` for discord files; `fmt` for conn_unix) are already imported in their respective files (verified in research).
- Do NOT touch .golangci.yml — the config is already correct post-260411-kcq; this task's job is to make the code match the config, not the reverse.
- Do NOT install golangci-lint v2 locally. The true lint verification is the next CI push; local verification is build + vet + race-test.
- Do NOT bump any version. This is a lint/cleanup fix, not a feature or user-visible release.
- Do NOT create new test files. All 17 fixes are either surface-only, equivalent-behavior refactors, or dead-code deletions — existing test coverage is sufficient.

### Commit (after all verify steps pass)

One atomic commit:

```
fix(lint): address golangci-lint v2 findings and normalize JSON response writers

Fixes all 17 issues surfaced after quick-task 260411-kcq migrated .golangci.yml
to the v2 format:

- Category A (5x errcheck, bytes.Buffer false positives): inline _ = prefix on
  binary.Write calls in discord/client.go (lines 171-172) and
  discord/client_test.go (lines 43, 391-392), with a consolidated comment
  documenting the stdlib nil-error guarantee.

- Category B (1x errcheck, test robustness): wrap client.send call in
  discord/client_test.go in an if-err block with t.Fatalf.

- Category C (7x errcheck + consistency): extract (*Server).writeJSON helper
  in server/server.go and replace all 7 json.NewEncoder(w).Encode call sites
  (handlePostConfig, handlePostPreview, handleHealth, handleGetPresets,
  handleGetStatus, handleSessions, handlePreviewMessages) with s.writeJSON.
  The helper centralizes slog.Warn logging on encode failure (typically client
  disconnect after WriteHeader).

- Category D (4x staticcheck/unused): remove redundant nil-check in
  analytics/persist_test.go:199, rephrase error string in
  discord/conn_unix.go:19 to satisfy ST1005 (lowercase start + colon
  separator), delete unused var currentFile in logger/logger.go, delete
  unused duplicate func getGitBranch in main.go (active version remains in
  server/server.go:1557).

Total: 17 fixes across 5 code areas (7 file paths). 11 were flagged by CI
golangci-lint v2; 6 additional structurally-identical sites were discovered
during research and fixed in the same commit for consistency.

Zero behavior change for end users. handlePostPreview, handleGetPresets,
handleGetStatus, and handlePreviewMessages now send an explicit
WriteHeader(200) via writeJSON — all existing tests use httptest.NewRecorder
which defaults .Code to 200, so this normalization is test-safe
(verified in RESEARCH.md section 5).
```

The commit message body is long-form; use a HEREDOC to preserve newlines.
  </action>
  <verify>
Run all of the following **in order**. Every step must succeed before proceeding to the commit. If any step fails, stop and debug — do NOT skip ahead.

<automated>
# 1. Compile check
go build ./...

# 2. Vet check
go vet ./...

# 3. Full test suite with race detector
go test -race ./...

# 4. No unprefixed binary.Write calls remain in discord (expect 0)
#    (grep for lines starting with `binary.Write` — without the `_ =` prefix)
grep -En '^\s*binary\.Write\(' discord/client.go discord/client_test.go | wc -l
# Expected output: 0

# 5. Only one definition of json.NewEncoder(w).Encode in server.go (expect 1)
grep -c 'json\.NewEncoder(w)\.Encode' server/server.go
# Expected output: 1

# 6. writeJSON appears 8 times in server.go (1 definition + 7 call sites)
grep -c 'writeJSON' server/server.go
# Expected output: 8

# 7. getGitBranch is gone from main.go (expect 0)
grep -c 'getGitBranch' main.go
# Expected output: 0

# 8. currentFile is gone from logger.go (expect 0)
grep -c 'currentFile' logger/logger.go
# Expected output: 0

# 9. Old capitalized conn_unix error string gone (expect 0)
grep -c 'Discord IPC socket not found' discord/conn_unix.go
# Expected output: 0

# 10. New lowercase conn_unix error string present (expect 1)
grep -c 'discord IPC socket not found' discord/conn_unix.go
# Expected output: 1
</automated>

**Interpreting the greps:** Steps 4-10 are substring counts. If any count differs from the expected value, a fix was either missed, applied to the wrong line, or left a stale copy of the old string. Re-Read the affected file and correct. Do not commit until all 10 checks pass.

**Post-commit verification (manual, optional):** Push the branch and watch the next CI run. The "Lint" job should now succeed. If it surfaces new findings, open a follow-up quick task — do NOT amend this commit.
  </verify>
  <done>
All 10 verify steps pass AND the atomic commit `fix(lint): address golangci-lint v2 findings and normalize JSON response writers` has been created on the current branch.

Specifically:
- [ ] `go build ./...` returns exit 0
- [ ] `go vet ./...` returns exit 0
- [ ] `go test -race ./...` reports PASS for every package (zero FAIL, zero DATA RACE)
- [ ] No bare `binary.Write(...)` calls in discord/client.go or discord/client_test.go (all prefixed with `_ =`)
- [ ] Exactly one `json.NewEncoder(w).Encode` line remains in server/server.go — inside the writeJSON helper body
- [ ] `writeJSON` appears exactly 8 times in server/server.go (1 method definition + 7 call sites)
- [ ] `getGitBranch` is absent from main.go
- [ ] `currentFile` is absent from logger/logger.go
- [ ] Old "Discord IPC socket not found. Make sure..." string is absent from discord/conn_unix.go
- [ ] New "discord IPC socket not found: make sure..." string is present in discord/conn_unix.go
- [ ] Single atomic commit exists on branch with the full multi-paragraph message body
- [ ] No changes to .golangci.yml, go.mod, go.sum, or version files (main.go version const, plugin.json, marketplace.json, start.sh, start.ps1)
  </done>
</task>

</tasks>

<verification>
## Overall Phase Checks

1. **Build:** `go build ./...` compiles all packages cleanly.
2. **Vet:** `go vet ./...` passes.
3. **Tests:** `go test -race ./...` passes with race detector.
4. **Substring invariants:** The 7 grep counts in the task verify block match their expected values.
5. **Commit:** Exactly one new commit on the branch, conventional-commits `fix(lint): ...` subject, multi-paragraph body documenting all 4 categories and 17 total fixes.
6. **CI (post-push):** The Lint job on the next GitHub Actions run succeeds. This is the authoritative verification — local grep counts are necessary but not sufficient.
</verification>

<success_criteria>
## Measurable Completion

- All 17 fixes applied exactly as specified (11 CONTEXT.md + 6 researcher discoveries).
- Zero regressions: full test suite passes with `-race`.
- Zero behavior change observable to end users or existing tests.
- Zero new imports.
- Zero touched config files (.golangci.yml, go.mod, go.sum untouched).
- Zero version bumps.
- One atomic commit on the current branch.
- Next CI push produces a green Lint job.
</success_criteria>

<output>
After completion, create `.planning/quick/260411-kvy-fix-all-11-golangci-lint-findings-surfac/260411-kvy-SUMMARY.md` with:

- Commit SHA of the single atomic commit
- Final file-level summary (7 paths, per-file change count)
- List of all 17 fixes as applied (confirm each one landed)
- Grep verification results (the 10 verify steps, with actual output)
- Notes on any drift encountered (line numbers that shifted between plan and execute, if any)
- Link to the CI run that validates the Lint job is green
- Any follow-up items (e.g., if golangci-lint surfaces NEW findings on the next run that this plan didn't anticipate)
</output>
