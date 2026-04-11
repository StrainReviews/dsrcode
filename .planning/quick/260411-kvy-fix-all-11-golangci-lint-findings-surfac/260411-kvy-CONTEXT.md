---
name: 260411-kvy-CONTEXT
description: Locked implementation decisions for fixing all 11 golangci-lint findings post-v2-migration
type: context
---

# Quick Task 260411-kvy: Fix all 11 golangci-lint findings - Context

**Gathered:** 2026-04-11
**Status:** Ready for planning
**Discussion mode:** --discuss --research (intensive MCP verification)

<domain>
## Task Boundary

Fix all 11 golangci-lint findings surfaced after .golangci.yml v2 migration (quick-task 260411-kcq). Scope: make CI Lint job green on the next push. Findings are pre-existing code issues that were masked by the v1 config-parse error.

**Findings breakdown:**
- **Category A (3× errcheck, bytes.Buffer false positives):**
  - `discord/client.go:171` — `binary.Write(buf, binary.LittleEndian, int32(opcode))`
  - `discord/client.go:172` — `binary.Write(buf, binary.LittleEndian, int32(len(payload)))`
  - `discord/client_test.go:43` — `binary.Write(&m.readBuffer, binary.LittleEndian, opcode)`
- **Category B (1× errcheck, test robustness):**
  - `discord/client_test.go:418` — `client.send(opFrame, testData)` (error ignored in test)
- **Category C (3× errcheck, HTTP encoder after WriteHeader):**
  - `server/server.go:1123` — `handlePostConfig` → `json.NewEncoder(w).Encode(map[string]bool{"ok": true})`
  - `server/server.go:1177` — `handlePostPreview` → `json.NewEncoder(w).Encode(map[string]interface{}{...})`
  - `server/server.go:1193` — `handleHealth` → `json.NewEncoder(w).Encode(resp)`
- **Category D (2× staticcheck + 2× unused):**
  - `analytics/persist_test.go:199` — S1009: `if data.Tokens != nil && len(data.Tokens) > 0` (nil check redundant — len() of nil map is 0)
  - `discord/conn_unix.go:19` — ST1005: `fmt.Errorf("Discord IPC socket not found. Make sure Discord is running")`
  - `logger/logger.go:20` — unused: `var currentFile string` (assigned line 45, never read)
  - `main.go:558` — unused: `func getGitBranch(projectPath string) string` (duplicate of `server/server.go:1557`; main.go copy is dead — Phase 6.04 SUMMARY flagged this as code duplication)

</domain>

<decisions>
## Implementation Decisions

### Category A — bytes.Buffer false positives (errcheck)

**Decision:** Inline `_ =` blank identifier with explanatory comment at each site.

**Rationale:**
- `bytes.Buffer.Write` is documented as "err is always nil" in stdlib. `binary.Write` inherits that guarantee when target is `*bytes.Buffer`.
- Inline form is self-documenting at the call site — reader doesn't need to consult `.golangci.yml` to understand why the error is ignored.
- Only 3 occurrences; centralized exclude-functions would be overkill.
- The comment documents the invariant, not just suppresses the linter.

**Reference fix pattern (client.go:170-173):**
```go
buf := new(bytes.Buffer)
// bytes.Buffer.Write never returns an error; binary.Write inherits that guarantee
_ = binary.Write(buf, binary.LittleEndian, int32(opcode))
_ = binary.Write(buf, binary.LittleEndian, int32(len(payload)))
buf.Write(payload)
```

For `client_test.go:43` (inside `writeFrame` test helper), the same pattern. Single consolidated comment above both `binary.Write` calls.

---

### Category B — Test robustness (errcheck)

**Decision:** Wrap `client.send(...)` in `if err := ...; err != nil { t.Fatalf(...) }` pattern.

**Rationale:**
- Claude's discretion per unambiguous Go test idiom: unchecked errors in test code are a test-smell because failures cascade into misleading downstream assertion failures.
- `t.Fatalf` (not `t.Errorf`) because subsequent assertions depend on `send` having actually written to the mock buffer — continuing on error produces meaningless downstream output.
- Message format follows "Useful Test Failures" from Go Code Review Comments: `t.Fatalf("send failed: %v", err)`.

**Reference fix pattern (client_test.go:418):**
```go
if err := client.send(opFrame, testData); err != nil {
    t.Fatalf("send failed: %v", err)
}
frame := mock.writeBuffer.Bytes()
```

---

### Category C — HTTP encoder error handling (errcheck)

**Decision:** Extract a `writeJSON` helper method on `*Server` with centralized `slog.Warn` logging. Replace all 3 inline `json.NewEncoder(w).Encode(...)` calls with `s.writeJSON(...)`.

**Rationale:**
- Matches OneUptime production-HTTP-server pattern (MCP-verified via exa) — canonical Go idiom for JSON response writers.
- `slog.Warn` (not `Error`) because after `WriteHeader`, the only plausible cause is client disconnect, which is a transient network condition, not a server-side bug. Warn-level preserves observability for pattern detection (e.g., 1000× per minute would indicate a real problem) without alarming on individual disconnects.
- DRY — saves ~6 lines across 3 handlers, future handlers benefit automatically.
- Helper is a method on `*Server` so future changes (e.g., metrics, tracing) have a single insertion point.

**Reference helper (server/server.go, placed near other private helpers):**
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

**Call sites:**
```go
// handlePostConfig (line 1121-1123)
s.writeJSON(w, http.StatusOK, map[string]bool{"ok": true})

// handlePostPreview (line 1176-1180) — note: had no WriteHeader call, Encode implicitly sent 200
s.writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "expiresIn": duration})

// handleHealth (line 1191-1193)
s.writeJSON(w, http.StatusOK, resp)
```

**Note for planner:** `handlePostPreview` currently sets Content-Type but does NOT call `WriteHeader` explicitly — the first Encode triggers implicit 200. The helper makes the `WriteHeader(200)` explicit, which is a behavior normalization (status code is unchanged).

---

### Category D — Staticcheck + unused fixes

#### D-1: analytics/persist_test.go:199 (S1009)
**Decision:** Remove the redundant `!= nil` clause. `len()` of a nil map returns 0, so the nil check is dead code.

**Fix:**
```go
// Before:
if data.Tokens != nil && len(data.Tokens) > 0 {
// After:
if len(data.Tokens) > 0 {
```

#### D-2: discord/conn_unix.go:19 (ST1005)
**Decision:** Lowercase start + colon-form rephrase. Keep "Discord" (proper noun) in the message body.

**Rationale:**
- Go Code Review Comments explicitly says error strings should not start with capital unless proper noun, AND does not apply to logging. But staticcheck's ST1005 heuristic doesn't know about proper noun exemptions and flags any capital start.
- Pragmatic path: lowercase start satisfies the linter cleanly without `//nolint` noise.
- The mid-sentence period (". Make sure") is also stylistically wrong — errors should be single-clause phrases. Replace with colon separator.
- Preserves "Discord" as proper noun in the body (where it IS a real product name).

**Fix:**
```go
// Before:
return nil, fmt.Errorf("Discord IPC socket not found. Make sure Discord is running")
// After:
return nil, fmt.Errorf("discord IPC socket not found: make sure Discord is running")
```

#### D-3: logger/logger.go:20,45 (unused — var currentFile)
**Decision:** Delete both the declaration (line 20) and the assignment (line 45). Genuine dead code — grep confirmed zero readers.

**Verification before deletion:** The planner must grep one more time for `currentFile` across the entire repo (not just logger/) to catch any cross-package reference that might have been missed. If zero hits outside logger/logger.go → delete both lines.

#### D-4: main.go:558 (unused — func getGitBranch)
**Decision:** Delete the entire function from main.go. Duplicate of the active implementation at `server/server.go:1557` (confirmed by grep: only caller is `server.go:1087` which calls the server.go version).

**Historical context:** Phase 6.04 SUMMARY explicitly documented this as code duplication that survived the main.go → server package refactor. This task finishes that cleanup.

**Verification before deletion:** The planner must confirm via grep that `main.go:558`'s `getGitBranch` is NOT called from main.go itself. Grep already showed only the server.go call site, but re-verify in planner step.

### Claude's Discretion (not discussed)

- **Commit strategy:** One atomic commit for the fix — all 11 findings fixed together with a single message `fix(lint): address all 11 golangci-lint v2 findings`. Rationale: they're all part of the same post-migration cleanup, share the same root cause story (v1 config parse error masked them), and should ship as one unit.
- **Test verification:** After the fix, the executor runs `go test -race ./...` locally to ensure no regressions. The TRUE lint verification is the CI run on push (can't run golangci-lint v2 locally without installing it).
- **Build verification:** `go build ./...` must succeed as a sanity check after each file edit.

</decisions>

<specifics>
## Specific Ideas

- **slog attribution style:** Use existing project pattern — `slog.Warn("message", "key", value, ...)` with positional key-value pairs (not `slog.Attr` value form). Verified by reading server.go existing slog calls.
- **Error wrapping:** Not applicable here — all fixes are either error-ignore, error-rephrase, or dead-code removal, not error-wrapping.
- **Go version:** Project is on Go 1.25 per CLAUDE.md. All proposed patterns are stdlib-compatible.
</specifics>

<canonical_refs>
## Canonical References (MCP-verified)

- **Go Code Review Comments — Error Strings:** https://go.dev/wiki/CodeReviewComments#error-strings (authoritative source for ST1005 rationale and proper-noun exemption)
- **Structured Logging with slog:** https://go.dev/blog/slog (log level semantics)
- **OneUptime production HTTP server pattern:** https://oneuptime.com/blog/post/2026-02-20-go-http-server-production/view (reference for `writeJSON` helper pattern)
- **golangci-lint migration guide (errcheck.exclude-functions):** https://golangci-lint.run/docs/product/migration-guide (alternative approach considered and rejected in favor of inline `_ =`)
- **Phase 6.04 SUMMARY (repo-internal):** `.planning/phases/06-hook-system-overhaul-.../06-04-SUMMARY.md` line 316 — confirms main.go `getGitBranch` is a known duplicate of server.go version
</canonical_refs>
