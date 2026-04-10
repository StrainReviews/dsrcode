---
phase: 06-hook-system-overhaul
plan: 02
status: completed
completed: 2026-04-10
commits:
  - 25bd59b feat(06-02): add settings.local.json auto-patch with 13 dsrcode HTTP hooks
  - c3ca907 feat(06-02): add settings.local.json cleanup to stop.sh
---

# Plan 06-02: Summary

## Scope

Extend `scripts/start.sh` to auto-patch `~/.claude/settings.local.json` with 13 dsrcode HTTP hooks on daemon start (idempotent, preserves user hooks), and extend `scripts/stop.sh` to clean up those hooks when the last session ends. Implements decisions D-13 (hook deployment split: 13 HTTP hooks in settings.local.json) and D-14 (start.sh auto-patch + stop.sh cleanup).

## Artifacts

| File | Role | Change |
|------|------|--------|
| `scripts/start.sh` | Plugin hook: SessionStart | +80 lines: `patch_settings_local()` function + call |
| `scripts/stop.sh`  | Plugin hook: SessionEnd   | +62 lines: `cleanup_settings_local()` function + call |

## Requirements Traceability

| Req | Description | Status |
|-----|-------------|--------|
| D-13 | 13 HTTP hooks in `settings.local.json` | implemented |
| D-14 | Idempotent auto-patch in start.sh, cleanup in stop.sh on plugin uninstall | implemented |

## The 13 Hooks Registered

All hooks are HTTP type with `url: http://127.0.0.1:19460/hooks/{slug}` and `timeout: 1` (seconds).

| # | Event | Matcher | URL slug |
|---|-------|---------|----------|
| 1 | PreToolUse         | `*`            | pre-tool-use |
| 2 | PostToolUse        | `*`            | post-tool-use |
| 3 | PostToolUseFailure | `*`            | post-tool-use-failure |
| 4 | UserPromptSubmit   | (none)         | user-prompt-submit |
| 5 | Stop               | (none)         | stop |
| 6 | StopFailure        | `*`            | stop-failure |
| 7 | Notification       | `idle_prompt`  | notification |
| 8 | SubagentStart      | `*`            | subagent-start |
| 9 | SubagentStop       | `*`            | subagent-stop |
| 10 | PreCompact        | `*`            | pre-compact |
| 11 | PostCompact       | `*`            | post-compact |
| 12 | CwdChanged        | (none)         | cwd-changed |
| 13 | SessionEnd        | (none)         | session-end |

SessionStart remains in plugin `hooks.json` as the bootstrap and is not registered here. SessionEnd uses dual registration (plugin + local) per D-13 to mitigate `anthropics/claude-code#33458`.

## Design Choices

### node -e over jq
Consistent with the existing `patch_hooks_json` function in start.sh. Uses the same double-quoted bash heredoc + inline JS pattern with `2>/dev/null || true` for silent fallback.

### URL-based idempotency
Both patch and cleanup identify dsrcode hooks by URL containing `127.0.0.1:19460`. The port is hardcoded in the daemon, so any hook with this URL is owned by dsrcode.

### Atomic write pattern
Each write uses `fs.writeFileSync(tmp, ...)` followed by `fs.renameSync(tmp, settingsPath)`. `fs.renameSync` overwrites destination atomically on POSIX and best-effort on Windows (known EPERM edge case under heavy AV-scanner load is acceptable since the existing `patch_hooks_json` has the same characteristic).

### Cleanup invocation point
`cleanup_settings_local` is called in `stop.sh` at line 166, in the common path AFTER both Windows refcount and Unix PID-file session counting. Early-exit branches (ACTIVE_SESSIONS > 0) skip the cleanup entirely. This means cleanup runs ONLY when the last session ends.

### Delete-during-iteration safety
The cleanup function uses `Object.keys(settings.hooks)` to create a snapshot array before iterating, so `delete settings.hooks[event]` during the loop is safe (ES5 12.6.4 and modern engines).

## Idempotency Guarantees

### start.sh patch
- First run on empty `{}`: adds 13 hooks, final size 3041 bytes
- Second run: adds 0 hooks, files byte-identical
- With pre-existing user hooks: user hook stays at `[0]`, dsrcode appended at `[1]`
- Double-run detection: URL `127.0.0.1:19460` existence check per event array

### stop.sh cleanup
- Round-trip from `{}`: `{}` -> 13 hooks -> `{}` (empty `hooks` key removed)
- Round-trip with user hooks: byte-identical preservation via `JSON.stringify` deep-equality check
- Missing file: caught, exit 0, no write
- Malformed JSON: SyntaxError caught, exit 0, no write
- User-only hooks (no dsrcode): removed 0, file untouched

## Edge Cases Covered

| Case | Behavior |
|------|----------|
| Empty `settings.local.json` | start adds 13; stop removes 13 and deletes `hooks` key |
| Missing `settings.local.json` | start creates it; stop exits cleanly (no-op) |
| Malformed JSON | start overwrites with clean state; stop exits cleanly (no-op) |
| Missing `.claude/` directory | start creates it via `fs.mkdirSync(recursive: true)` |
| Node.js not installed | both functions `return 0` (graceful degradation) |
| Pre-existing user hooks | preserved at their original indices |
| Pre-existing dsrcode hooks | start detects and skips; stop removes them |
| Mixed user + dsrcode per event | start appends at end; stop filters only dsrcode |

## Verification Performed

### Automated
- `bash -n scripts/start.sh` : SYNTAX_OK
- `bash -n scripts/stop.sh` : SYNTAX_OK
- `grep -c "settings.local.json" scripts/start.sh` : 3
- `grep -c "127.0.0.1:19460" scripts/start.sh` : 4
- `grep -c "settings.local.json" scripts/stop.sh` : 5
- `grep -c "127.0.0.1:19460" scripts/stop.sh` : 2
- All 13 event slugs present in start.sh

### Manual / Integration
1. Idempotency test: 2 sequential runs of start patch yield byte-identical output
2. Merge test: user Bash command hook and UserPromptSubmit command hook preserved alongside added dsrcode hooks; permissions.allow untouched
3. Round-trip test (empty): `{}` -> start -> stop -> `{}` (structural equality)
4. Round-trip test (user hooks): full JSON deep equality preserved after start/stop cycle
5. Missing-file cleanup: exits 0 silently
6. Malformed-JSON cleanup: exits 0 silently
7. User-only hooks cleanup: file untouched, removed 0

## MCP Usage Log

Both tasks followed the Phase-6 MCP mandate (PRE + POST rounds of sequential-thinking, context7, exa, crawl4ai):

### Task 1: start.sh patch_settings_local
- **sequential-thinking** (PRE): 3 thoughts decomposing node -e vs jq choice, edge cases, shell escaping, insertion point, idempotency identifier; (POST): reflection on D-13/D-14 consistency and sub-step coverage
- **context7** (PRE): `/websites/nodejs_latest-v22_x_api` (fs.renameSync overwrites destination); `/websites/code_claude` (HTTP hook schema: type, url, timeout, matcher); (POST): JSON.stringify indentation + fs.writeFileSync encoding
- **exa** (PRE): "Node.js fs.renameSync atomic Windows EPERM" (found npm/write-file-atomic#227 — acceptable edge case); "Claude Code hooks timeout seconds vs milliseconds" (4 sources confirmed seconds); (POST): "JSON.parse error handling fallback pattern" (confirmed try/catch + default idiom)
- **crawl4ai** (PRE): `code.claude.com/docs/en/hooks` (too large, fell back to Exa result); (POST): `code.claude.com/docs/en/settings` (confirmed settings.local.json is valid hook location with local precedence)

### Task 2: stop.sh cleanup_settings_local
- **sequential-thinking** (PRE): 1 thought decomposing into 10 sub-steps, choosing single-call-at-common-point pattern over double-call-in-branches, verifying delete-during-iteration safety; (POST): reflection on all 10 sub-steps + 8 edge cases
- **context7** (PRE): `/websites/nodejs_latest-v22_x_api` (Object.keys, Array.filter immutable pattern, process.exit semantics); (POST): Array.filter returns new array confirmation
- **exa** (PRE): "JavaScript delete property during iteration pitfall" (confirmed Object.keys snapshot is safe per ES5 12.6.4 and TheLinuxCode 2026 article); (POST): "bash idempotent cleanup json 2026" (confirmed atomic tmp+rename as recommended pattern)
- **crawl4ai** (PRE): already had hooks schema from Task 1; (POST): re-verified code.claude.com/docs/en/settings URL format

## Deviations

None. Implementation follows plan exactly: both tasks completed, both committed atomically with MCP-Verification sections in commit bodies, all acceptance criteria met, no architectural changes.

## Next Plan Readiness

Plan 06-02 is complete. Next plan: **06-03** (per ROADMAP wave sequencing). Both start.sh and stop.sh now handle the settings.local.json lifecycle; daemon can be built to receive events at the 13 HTTP endpoints.
