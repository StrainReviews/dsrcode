---
phase: 7
plan: "07-03"
subsystem: hooks+scripts
tags: [hooks, session-end, refcount, dual-registration, windows-parity]
requires: []
provides: [SessionEnd-command-hook, settings.local.json-fallback-channel]
affects: [hooks/hooks.json, scripts/start.sh, scripts/start.ps1, scripts/stop.sh]
tech_added: []
patterns_used: [Pattern-7-bash-c-wrapper, atomic-tmp-rename-write, ownership-marker-idempotency]
files_created: []
files_modified:
  - hooks/hooks.json
  - scripts/start.sh
  - scripts/start.ps1
  - scripts/stop.sh
decisions_implemented: [D-07, D-08, D-09]
commits:
  - 67803fe
  - b55b511
  - d32b570
  - 2eee19b
completed: 2026-04-13
duration: "~10min"
tasks_completed: 4
tasks_total: 4
---

# Phase 7 Plan 03: SessionEnd command hook + dual-registration to settings.local.json — Summary

Registers a new `SessionEnd` command-hook in the plugin's `hooks/hooks.json` AND — via the empirically-necessary dual-registration strategy — also patches/cleans the same entry into `~/.claude/settings.local.json` from both `start.sh` and `start.ps1`, with `stop.sh cleanup_settings_local` extended to remove both channels when ACTIVE_SESSIONS reaches 0. Zero changes to `server/server.go` (D-08 preserved). Implements D-07 + D-09.

## Decisions Made

- **D-07 literal compliance:** SessionEnd entry inserted between SessionStart and PreToolUse in `hooks/hooks.json`, matcher-free (match-all reasons), timeout 15, `bash -c 'ROOT="${CLAUDE_PLUGIN_ROOT:-...}"; bash "$ROOT/scripts/stop.sh"'` wrapper verbatim per Pattern 7.
- **D-09 dual-channel:** Same entry also written by `patch_settings_local()` (Unix) and new `Patch-SettingsLocal` (Windows) into user-level `settings.local.json`. Justification: upstream claude-code issues #17885, #33458, #35892 document plugin SessionEnd unreliability on `/exit`/`/clear`/VS Code close across macOS and Windows.
- **Strategy 3 (temp-file) for PowerShell mirror:** Chosen over double-quoted/single-quoted here-string strategies. Writes the node.js script to `$env:TEMP\dsrcode-patch-settings-$PID.js`, invokes `node`, removes in `finally{}`. Avoids all PowerShell here-string `$` expansion ambiguity.
- **Double-decrement accepted (Open Question #4):** Existing `[[ $ACTIVE_SESSIONS -lt 0 ]] && ACTIVE_SESSIONS=0` floor in stop.sh:123 and `[Math]::Max(0, ...)` floor in stop.ps1:19 already neutralize the dual-fire edge case. No marker file added.
- **Cleanup filter extended (Open Question #5):** `stop.sh cleanup_settings_local` now matches BOTH `h.url` (`127.0.0.1:19460`) AND `h.command` (substring `dsrcode` + `stop.sh`). Object.keys snapshot + atomic tmp+rename pattern preserved.

## Files Created

None.

## Files Modified

- `hooks/hooks.json` — added SessionEnd event with single type:command hook (11 lines inserted)
- `scripts/start.sh` — extended `patch_settings_local()` with DSRCODE_COMMAND_HOOKS loop (31 lines inserted)
- `scripts/start.ps1` — added new `Patch-SettingsLocal` function + invocation before Start Daemon (98 lines inserted, Strategy 3 temp-file approach)
- `scripts/stop.sh` — extended `cleanup_settings_local` filter predicate to match command hooks too (+7/-1)

## Tests Added

No automated tests added in this plan — the artifacts are JSON/shell/PowerShell config with no Go compilation surface. Instead, three controlled smoke tests were run and passed:

1. **hooks.json JSON validity:** `node -e "JSON.parse(require('fs').readFileSync('hooks/hooks.json','utf8'))"` → exits 0.
2. **Unix patch smoke test:** temp `$HOME` → run `patch_settings_local` via sourced start.sh → verified SessionEnd contains both HTTP entry and command entry with literal `${CLAUDE_PLUGIN_ROOT:-...}` embedded correctly (not bash-expanded at write time).
3. **PowerShell patch smoke test:** temp `$env:USERPROFILE` → extract + invoke `Patch-SettingsLocal` via regex-isolated function src → verified same 2-entry SessionEnd block, command contains `stop.sh`, `dsrcode`, `${CLAUDE_PLUGIN_ROOT`.
4. **Cleanup smoke test:** seeded `settings.local.json` with HTTP + command + user-custom SessionEnd entries → ran `cleanup_settings_local` → verified HTTP+command removed, user-custom echo hook preserved, PreToolUse event key deleted entirely (was solely dsrcode).

Manual cross-platform exit-flow tests (Windows `/exit`/`/clear`/VS Code close; macOS/Linux PID tracking) are deferred to Plan 07-05 verification per the plan's own Verification §10.

## Key Findings

- **Bash heredoc escape correctness** — Task 2 required `\'` (backslash single-quote) inside the outer double-quoted `node -e "..."` heredoc so the embedded JS literal preserves the `'` around the `bash -c '...'` command. Additional `\$` escape on `${CLAUDE_PLUGIN_ROOT}` ensures the brace-expression survives as a literal into the written JSON rather than getting expanded by bash at patch time. Verified by inspecting the written `settings.local.json` — the string is exactly `bash -c 'ROOT="${CLAUDE_PLUGIN_ROOT:-<HOME>/.claude/.../dsrcode}"; bash "$ROOT/scripts/stop.sh"'` with all shell metacharacters intact for Claude Code to re-expand at hook fire time.
- **MSYS `/tmp/` vs Windows node.js** — tests against `/tmp/...` paths failed because node on Windows doesn't recognize MSYS translated paths. Switched to `C:\temp\dsrcode-test-$$` via `cygpath`. Real SessionStart hook runs don't hit this because `$HOME` is a Windows path from the start.

## Deviations from Plan

None substantive. Minor differences from the plan's verbatim JS:
- Used `home` directly (already declared at start.sh:455) instead of recomputing `(process.env.HOME || process.env.USERPROFILE)` — equivalent, removes redundant evaluation.
- In PowerShell `DSRCODE_COMMAND_HOOKS`, forward-slash normalized the home path (`home.replace(/\\/g, '/')`) so the fallback path in the `bash -c` string is shell-compatible when bash expands it on Windows via Git Bash.

## MCP Citations (per user Phase 7 mandate)

- **`mcp__sequential-thinking__sequentialthinking`** — Decomposed Bug #3 into 4 tasks: plugin hooks.json entry; Unix patch loop; PowerShell mirror with Strategy 3 selection; cleanup filter extension. Traced bash heredoc escape chain. Verified double-decrement floor logic already exists in stop.sh:123 / stop.ps1:19.
- **`mcp__exa__web_search_exa`** — Searched "anthropics/claude-code issue 17885 SessionEnd hook not firing plugin" → confirmed upstream issues #17885 (closed stale, `/exit` doesn't fire), #18500 (Windows never fires), #33458 (plugin settings.json SessionEnd dispatch skipped on macOS 2.1.74), #35892 (flagged duplicate), and cross-references #32712 (Ctrl+C aborts hook) + #34573 (command hooks silently dropped). Evidence base for dual-registration strategy.
- **`mcp__context7__query-docs`** — (not called — sequential-thinking + exa were sufficient to validate the decision space; Plan 07-03's reference to context7 was pre-identified by the planner as optional).
- **`mcp__crawl4ai__md`** — (not called — exa search surfaced full issue content with timelines, hook payloads, and reproduction steps; crawl4ai fallback not needed).

## Threat Flags

None — no new security surface. Three threats already disposed in plan's Threat Model (T-07-04 mitigate / T-07-05 accept / T-07-06 accept). No new external inputs, no new network endpoints, no new auth paths.

## Self-Check: PASSED

**Files exist:**
- FOUND: hooks/hooks.json (modified)
- FOUND: scripts/start.sh (modified)
- FOUND: scripts/start.ps1 (modified)
- FOUND: scripts/stop.sh (modified)

**Commits exist:**
- FOUND: 67803fe — feat(07-03): add SessionEnd command-hook to plugin hooks.json (D-07)
- FOUND: b55b511 — feat(07-03): dual-register SessionEnd command hook in settings.local.json (Unix)
- FOUND: d32b570 — feat(07-03): add Patch-SettingsLocal to start.ps1 (Windows dual-register parity)
- FOUND: 2eee19b — feat(07-03): extend stop.sh cleanup_settings_local to remove SessionEnd command hooks

**Must-Haves verified:**
- Truth 1 ✓ hooks.json SessionEnd entry with type:command → stop.sh
- Truth 2 ✓ start.sh writes SessionEnd command entry to settings.local.json (dsrcode + stop.sh substrings present)
- Truth 3 ✓ start.ps1 Patch-SettingsLocal produces identical SessionEnd command entry
- Truth 4 ✓ stop.sh cleanup removes both http+command dsrcode entries; preserves user hooks
- Truth 5 ✓ `git diff HEAD~4 -- server/server.go` shows 0 lines
- Truth 6 ✓ `git diff HEAD~4 -- scripts/stop.ps1` shows 0 lines; refcount logic untouched
- Artifact 1 ✓ SessionEnd block between SessionStart and PreToolUse
- Artifact 2 ✓ DSRCODE_COMMAND_HOOKS const + loop inside existing node -e heredoc
- Artifact 3 ✓ Patch-SettingsLocal function with node temp-file Strategy 3
- Artifact 4 ✓ Filter predicate matches both h.url 127.0.0.1:19460 AND h.command dsrcode+stop.sh
- Key link ✓ Both channels invoke `bash scripts/stop.sh`; stop.sh refcount-decrement logic unchanged
