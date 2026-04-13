# Phase 7: Fix Daemon Auto-Exit Bugs — Pattern Map

**Mapped:** 2026-04-13
**Files analyzed:** 15 (5 Go source, 3 Go test, 4 scripts, 1 hook config, 2 version-metadata, 1 CHANGELOG)
**Analogs found:** 15 / 15 (100% — all analog excerpts live in this repository)

All line numbers verified against the working tree on 2026-04-13.

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `session/stale.go` | stale-detector utility | polling / read-registry-snapshot → side-effect (EndSession/TransitionToIdle) | itself (lines 41-48) | self (one-line guard tightening) |
| `session/registry.go` `Touch()` | registry mutation (new method) | CRUD (write: LastActivityAt) without change-notification | `SessionRegistry.SetLastActivityForTest` (registry.go:431-445) + structural pattern of `UpdateActivity` (registry.go:140-185) | exact (SetLastActivityForTest is the production-minus-notify sibling already in file) |
| `server/server.go` `handlePostToolUse` edit | HTTP hook handler (in-place edit) | request-response + background goroutine | itself (server.go:563-647) + the `handleHook` pattern (server.go:418-424) for session-exists-then-mutate | self (minimal insertion — 1 line + comment) |
| `hooks/hooks.json` SessionEnd entry | Plugin hook manifest | declarative config consumed by Claude Code at SessionEnd lifecycle | existing SessionStart entry in same file (hooks.json:4-19) | exact (same `type: command`, same `bash -c` wrapper, same timeout shape) |
| `scripts/start.sh` SessionEnd auto-patch | Script: node.js-embedded JSON patcher | file-I/O on `settings.local.json` | existing `patch_settings_local` loop (start.sh:447-524) + `DSRCODE_HOOKS` loop (start.sh:487-506) | role-match (extend loop for `type: command` variant alongside existing `type: http` loop) |
| `scripts/start.sh` `rotate_log` helper + `>>` append | Script: log-rotation helper | file-I/O (size-check → rename → redirect) | size-check idiom in `download_binary` (start.sh:160) `file_size=$(wc -c < "$tmp_dir/archive" ...)` | role-match (same `wc -c < FILE` + empty-guard idiom already used in this file) |
| `scripts/start.ps1` `Rotate-Log` helper + log-path split fix | Script: log-rotation helper (Windows) | file-I/O (Get-Item.Length → Move-Item → Start-Process) | size-check in `Download-Binary` (start.ps1:106) `$fileSize = (Get-Item $archivePath).Length`; `Move-Item -Force` pattern elsewhere | role-match (same `Get-Item.Length` idiom already present) |
| `scripts/start.ps1` `Rotate-Log` helper + `patch_settings_local` PowerShell mirror | Script: node.js-embedded JSON patcher (Windows) | file-I/O on `settings.local.json` | **no analog in start.ps1** — cite RESEARCH.md §Bug #3 (start.ps1 currently lacks any settings.local.json auto-patch; expansion beyond D-07 scope per RESEARCH "Open Question #2") | no-analog (use node.js reuse from start.sh) |
| `scripts/stop.sh` extended `cleanup_settings_local` filter | Script: node.js-embedded JSON filter | file-I/O on `settings.local.json` | existing `cleanup_settings_local` filter (stop.sh:53-106) — specifically the `.filter(...)` predicate at stop.sh:79-84 | exact (extend predicate to also match `h.command` containing `stop.sh` + `dsrcode`) |
| `scripts/stop.ps1` mirror cleanup (optional) | Script: Windows cleanup mirror | file-I/O | **no analog in stop.ps1** (stop.ps1 has no settings.local.json cleanup today) | no-analog (defer to research — see Open Questions) |
| `session/registry_test.go` `TestTouch*` | Unit test (table-driven) | in-memory registry state assertions | `TestRegistryUpdateActivity` (registry_test.go:65-120), `TestStaleCheck` (registry_test.go:244-270) | exact (same `session.NewRegistry(func(){...})` + `StartSession` + `SetLastActivityForTest` idiom) |
| `session/stale_test.go` (or inside `registry_test.go`) `TestStaleCheckSkipsPidCheckForHttpSource` / `TestStaleCheckPreservesPidCheckForPidSource` | Unit test | static registry → `CheckOnce` → existence assertion | `TestStaleCheck` (registry_test.go:244-270) + `TestStaleRemove` (registry_test.go:274-296) | exact (same `CheckOnce(reg, 10*time.Minute, 30*time.Minute)` call pattern) |
| `server/server_test.go` `TestHandlePostToolUseUpdatesLastActivity` | Integration test (HTTP boundary) | POST body → handler → registry state assertion | `TestHandlePostToolUse` (server_test.go:1035-1058) + `TestHandleSessionEnd` (server_test.go:987-1005) | exact (reuse `newTestServer`, `startTestSession`, `postHook`, `SetLastActivityForTest`) |
| `CHANGELOG.md` v4.1.2 section | Release notes | N/A | v4.1.1 entry (CHANGELOG.md:10-13) and v4.1.0 entry (CHANGELOG.md:15-39) | exact (Keep-a-Changelog `## [X.Y.Z] - YYYY-MM-DD` + `### Fixed` / `### Changed`) |
| `main.go` + `.claude-plugin/plugin.json` + `.claude-plugin/marketplace.json` + `scripts/start.sh` + `scripts/start.ps1` version constants | Release plumbing | sed in-place | `scripts/bump-version.sh` (already present, performs all 5 edits) | exact (tool already exists — just run `./scripts/bump-version.sh 4.1.2`) |

## Pattern Assignments

### `session/stale.go` — Bug #1 PID-skip (one-line tightening)

**Role:** Background stale-session detector. **Data flow:** polling → read registry snapshot → per-session branch (PID liveness, remove timeout, idle transition).

**Analog:** itself — surgical edit.

**Exact current code** (stale.go:38-48, verbatim):
```go
// PID liveness check — skip for sessions with recent hook activity.
// HTTP hook-based sessions may carry the daemon's parent PID rather
// than the actual Claude Code process PID, causing false removals.
if s.PID > 0 && !IsPidAlive(s.PID) {
    if elapsed > 2*time.Minute {
        slog.Info("removing stale session (PID dead, no recent activity)", "sessionId", s.SessionID, "pid", s.PID, "elapsed", elapsed)
        registry.EndSession(s.SessionID)
        continue
    }
    slog.Debug("PID dead but session has recent activity, skipping removal", "sessionId", s.SessionID, "pid", s.PID, "elapsed", elapsed)
}
```

**Change (D-01 / D-02 / D-03):** Insert `s.Source != SourceHTTP &&` into the outer guard at stale.go:41. Do NOT touch the 2-minute grace block (D-02: preserve PID-check behavior for PID-sourced sessions).

**Key invariant:** `Source` is always populated at session creation by `StartSession` → `StartSessionWithSource(req, pid, sourceFromID(req.SessionID))` (registry.go:42-44), with `sourceFromID` at source.go:42-50 handling the `http-` prefix → `SourceHTTP` mapping. No nil-check needed.

**Do NOT import.** `SourceHTTP` is in the same package (`session/source.go:15`), direct reference is idiomatic.

---

### `session/registry.go` — new `Touch(sessionID string)` method

**Role:** Registry mutation (CRUD subset). **Data flow:** acquire write-lock → immutable copy-before-modify → update LastActivityAt only → store → **NO** `notifyChange()`.

**Primary analog — structural twin that ALREADY lives in this file:** `SetLastActivityForTest` (registry.go:431-445). This is literally the production-minus-test-gate version of what `Touch` needs to do. It already uses the exact pattern: write-lock, nil-check, immutable copy, assign `LastActivityAt`, re-store, and crucially NO `notifyChange()` call.

**Excerpt to model after** (registry.go:431-445, verbatim):
```go
// SetLastActivityForTest overwrites LastActivityAt for a session.
// Intended for testing stale detection with controlled timestamps.
func (r *SessionRegistry) SetLastActivityForTest(sessionID string, t time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, ok := r.sessions[sessionID]
	if !ok {
		return
	}

	updated := *session
	updated.LastActivityAt = t
	r.sessions[sessionID] = &updated
}
```

**Secondary analog — the established immutable-copy-before-modify invariant:** `UpdateActivity` (registry.go:140-185). Of special note:
- registry.go:149 `updated := *session` — the copy-before-modify line the planner is referencing as "registry.go:149"
- registry.go:179 `updated.LastActivityAt = time.Now()` — the single line Touch reuses (swap `t` for `time.Now()` vs SetLastActivityForTest)
- registry.go:183 `r.notifyChange()` — the line Touch MUST NOT add (D-05)

**Recommended `Touch` shape** (8 lines, placed directly below UpdateActivity around registry.go:186 per RESEARCH Example 2):
```go
// Touch updates LastActivityAt to the current time for an existing session
// WITHOUT firing the onChange callback. Used by PostToolUse hooks (Phase 7
// D-04/D-05) to keep the stale-detector's activity clock fresh on every MCP
// tool call without triggering a Discord presence update per call.
//
// Follows the immutable copy-before-modify pattern. No-op if sessionID unknown.
func (r *SessionRegistry) Touch(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.sessions[sessionID]
	if !ok {
		return
	}
	updated := *s
	updated.LastActivityAt = time.Now()
	r.sessions[sessionID] = &updated
	// Intentionally does NOT call notifyChange() — D-05 Phase 7.
}
```

**Lock semantics:** `r.mu.Lock()` (write-lock), matching every other mutation on `SessionRegistry`. `r.notifyChange()` comment (registry.go:32) states "Must be called with mu held (write lock)" — we honor the "with write lock held" invariant by NOT calling notifyChange at all; lock acquisition alone is correct.

---

### `server/server.go` — `handlePostToolUse` single-line edit

**Role:** HTTP hook handler. **Data flow:** POST body parse → 200 response → side-effects (analytics throttle, error overlay clear, transcript path sync) → background goroutine for JSONL parse.

**Analog:** itself (server.go:563-647) plus the `handleHook` pattern for ordering (server.go:418-424 — existing-check-before-mutate idiom).

**Current empty-session_id guard and tracker call** (server.go:571-582, verbatim):
```go
	if payload.SessionID == "" {
		slog.Debug("post-tool-use: empty session_id, ignoring")
		return
	}

	sessionID := payload.SessionID
	transcriptPath := payload.TranscriptPath

	// Record tool usage in analytics.
	if s.tracker != nil && payload.ToolName != "" {
		s.tracker.RecordTool(sessionID, payload.ToolName)
	}
```

**Insertion point (D-04 / D-05):** Immediately after the empty-session_id return at server.go:573-574, before `sessionID := payload.SessionID`:
```go
	// Phase 7 D-04: Keep the stale-detector's activity clock fresh on every
	// MCP tool call. Touch() is a no-op for unknown sessions and does not
	// fire notifyChange (D-05: zero UI side-effects).
	s.registry.Touch(payload.SessionID)
```

**Rationale for this placement:**
1. Before `s.tracker` branch → so tracker-nil test mode still exercises the touch.
2. Before background goroutine → so the synchronous activity-clock update cannot be lost to panic in the goroutine.
3. After empty-session_id guard → so we skip no-op Touches for garbage payloads.

**Do NOT add** a `registry.GetSession(...) == nil` pre-check. `Touch` is already internally nil-safe (returns without write if session unknown), matching the existing `UpdateTranscriptPath` call at server.go:464 / 641 which also handles the unknown-session case internally.

---

### `hooks/hooks.json` — SessionEnd command-hook entry (D-07)

**Role:** Plugin hook manifest. **Data flow:** declarative JSON consumed by Claude Code CLI at SessionEnd lifecycle → spawns `bash -c` subprocess.

**Analog:** existing SessionStart entry in the same file (hooks.json:4-19).

**Excerpt to mirror verbatim** (hooks.json:4-19):
```json
"SessionStart": [
  {
    "matcher": "startup|clear|compact|resume",
    "hooks": [
      {
        "type": "command",
        "command": "bash -c 'ROOT=\"${CLAUDE_PLUGIN_ROOT:-$HOME/.claude/plugins/marketplaces/dsrcode}\"; bash \"$ROOT/scripts/start.sh\"'",
        "timeout": 15
      },
      {
        "type": "command",
        "command": "bash -c 'ROOT=\"${CLAUDE_PLUGIN_ROOT:-$HOME/.claude/plugins/marketplaces/dsrcode}\"; bash \"$ROOT/scripts/install-skills.sh\"'",
        "timeout": 10
      }
    ]
  }
],
```

**New SessionEnd entry (per RESEARCH Example 4 — insert after SessionStart block, before PreToolUse):**
```json
"SessionEnd": [
  {
    "hooks": [
      {
        "type": "command",
        "command": "bash -c 'ROOT=\"${CLAUDE_PLUGIN_ROOT:-$HOME/.claude/plugins/marketplaces/dsrcode}\"; bash \"$ROOT/scripts/stop.sh\"'",
        "timeout": 15
      }
    ]
  }
],
```

**Deliberately omitted:** `matcher` key. Per `https://code.claude.com/docs/en/hooks` (cited in RESEARCH), SessionEnd matcher is the exit-reason selector (`clear`, `logout`, `resume`, `prompt_input_exit`, `bypass_permissions_disabled`, `other`); omitting it means "match all reasons" — the desired unconditional refcount-decrement behavior per D-08.

**`bash -c '...'` wrapper:** preserved verbatim from SessionStart — the `${CLAUDE_PLUGIN_ROOT:-fallback}` idiom defends against Windows issue #16116 (`${CLAUDE_PLUGIN_ROOT}` expansion failure) exactly the way the live SessionStart entry does.

---

### `scripts/start.sh` — SessionEnd command-hook auto-patch (D-07 dual-register)

**Role:** node.js-embedded JSON patcher in `patch_settings_local()`. **Data flow:** read `~/.claude/settings.local.json` → mutate in memory → atomic tmp-write + rename.

**Analog:** the existing `DSRCODE_HOOKS` loop in `patch_settings_local` (start.sh:447-524).

**Key excerpts to model after (verbatim):**

*The `DSRCODE_HOOKS` declaration + main loop (start.sh:468-506):*
```js
const DSRCODE_HOOKS = {
    'PreToolUse':         { matcher: '*',            slug: 'pre-tool-use' },
    'PostToolUse':        { matcher: '*',            slug: 'post-tool-use' },
    'PostToolUseFailure': { matcher: '*',            slug: 'post-tool-use-failure' },
    // ... 10 more entries ending with ...
    'SessionEnd':         { matcher: null,           slug: 'session-end' }
};

const BASE_URL = 'http://127.0.0.1:19460/hooks/';
let added = 0;

for (const event of Object.keys(DSRCODE_HOOKS)) {
    const config = DSRCODE_HOOKS[event];
    if (!Array.isArray(settings.hooks[event])) {
        settings.hooks[event] = [];
    }
    const existing = settings.hooks[event];
    const hasDsrcode = existing.some(function(e) {
        return e && Array.isArray(e.hooks) && e.hooks.some(function(h) {
            return h && typeof h.url === 'string' && h.url.indexOf('127.0.0.1:19460') !== -1;
        });
    });
    if (!hasDsrcode) {
        const entry = { hooks: [{ type: 'http', url: BASE_URL + config.slug, timeout: 1 }] };
        if (config.matcher !== null) {
            entry.matcher = config.matcher;
        }
        existing.push(entry);
        added++;
    }
}
```

**Extension pattern (per RESEARCH Bug #3 §"settings.local.json auto-patch"):** Add a second constant `DSRCODE_COMMAND_HOOKS` below `DSRCODE_HOOKS` and a second loop below the first. The existing loop writes `type: http` with a URL ownership marker; the new loop writes `type: command` with a `command`-string ownership marker (substring match on `dsrcode` + `stop.sh`).

**Atomic write pattern already in place (start.sh:513-516):**
```js
// Atomic write via tmp file + rename
const tmp = settingsPath + '.tmp.' + process.pid;
fs.writeFileSync(tmp, JSON.stringify(settings, null, 2) + '\n');
fs.renameSync(tmp, settingsPath);
```
Reuse without modification — the existing tmp+rename handles the new hook entry transparently.

**Ownership-marker pattern (match across both loops):**
- HTTP loop: `h.url.indexOf('127.0.0.1:19460') !== -1` (exists)
- Command loop: `h.command.indexOf('dsrcode') !== -1 && h.command.indexOf('stop.sh') !== -1` (new)

Use string substring checks, NOT path equality — `CLAUDE_PLUGIN_ROOT` expansion varies per machine.

---

### `scripts/start.sh` — `rotate_log` helper + `>>` append redirection (D-10/D-11/D-12)

**Role:** Bash helper: size-capped rotation. **Data flow:** stat → conditional rename.

**Analog for size-check idiom:** already in this file at start.sh:160-164 (inside `download_binary`):
```bash
local file_size
file_size=$(wc -c < "$tmp_dir/archive" 2>/dev/null | tr -d ' ')
if [[ -z "$file_size" ]] || [[ "$file_size" -lt 100000 ]]; then
    echo "Warning: Downloaded file too small (${file_size:-0} bytes)" >&2
    return 1
fi
```

**Exact `wc -c < FILE` + `tr -d ' '` + empty-guard idiom is already idiomatic in this file.** Reuse for the rotate helper.

**Recommended helper shape (per RESEARCH Example 5 — place after the `process_exists` helper at start.sh:62, before `mkdir -p` at line 65):**
```bash
# Rotate a log file when it exceeds 10 MB. Single-backup (file.1, overwritten).
# Portable size check via wc -c (works on GNU coreutils, BSD, macOS, busybox/MSYS2).
rotate_log() {
    local log="$1"
    local max_size=10485760  # 10 MB
    [[ -f "$log" ]] || return 0
    local size
    size=$(wc -c < "$log" 2>/dev/null | tr -d ' ')
    [[ -n "$size" && "$size" -gt "$max_size" ]] || return 0
    mv -f "$log" "$log.1"
}
```

**Invocation site — before the Unix daemon launch at start.sh:577** (just before `nohup`):
```bash
rotate_log "$LOG_FILE"
rotate_log "$LOG_FILE.err"
```

**Redirect change at start.sh:578 (Unix branch):** from
```bash
nohup "$BINARY" > "$LOG_FILE" 2>&1 &
```
to
```bash
nohup "$BINARY" >> "$LOG_FILE" 2>> "$LOG_FILE.err" &
```

Behavior change: split stdout / stderr into separate files (matches post-v4.1.1 Windows behavior; noted in CHANGELOG `### Changed` per RESEARCH §Cross-Cutting Concerns).

**Windows-via-bash branch** (start.sh:570-576): same rotate-before-launch invocation in the `if $IS_WINDOWS; then` block. The `powershell.exe` one-liner at line 576 already uses the split `$WIN_LOG_FILE` + `${WIN_LOG_FILE}.err` paths — no redirect change needed, only the pre-launch `rotate_log` calls.

---

### `scripts/start.ps1` — `Rotate-Log` helper + log-path split fix (D-10/D-11/D-12 + pitfall #5 from RESEARCH)

**Role:** PowerShell helper + existing Start-Process redirect fix. **Data flow:** `Get-Item.Length` → conditional `Move-Item -Force`.

**Analog for size-check idiom:** already in this file at start.ps1:105-107 (inside `Download-Binary`):
```powershell
$archivePath = Join-Path $tmpDir "archive.zip"
$fileSize = (Get-Item $archivePath).Length
if ($fileSize -lt 100000) {
```

**Exact `(Get-Item $path).Length` idiom is already idiomatic.** Reuse for `Rotate-Log`.

**Analog for `Move-Item -Force`:** every `Remove-Item` and `Move-Item` in this file uses `-Force` + `-ErrorAction SilentlyContinue` (e.g., `Remove-Item $PidFile -Force -ErrorAction SilentlyContinue` at start.ps1:246, `Move-Item` or equivalent in cleanup). Reuse the `-Force -ErrorAction SilentlyContinue` resilience pattern.

**Recommended helper shape (per RESEARCH Example 6 — place after `Release-Lock` at start.ps1:78, before `Download-Binary` at line 80):**
```powershell
# Rotate a log file when it exceeds 10 MB. Single-backup (.log.1, overwritten).
function Rotate-Log {
    param([string]$LogPath)
    $maxSize = 10485760  # 10 MB
    if (-not (Test-Path $LogPath)) { return }
    $item = Get-Item $LogPath -ErrorAction SilentlyContinue
    if ($null -ne $item -and $item.Length -gt $maxSize) {
        Move-Item -Path $LogPath -Destination "$LogPath.1" -Force -ErrorAction SilentlyContinue
    }
}
```

**Invocation site — before `Start Daemon` block at start.ps1:334, after `$ErrorActionPreference = "Stop"`:**
```powershell
$LogFileErr = "$LogFile.err"
Rotate-Log $LogFile
Rotate-Log $LogFileErr
```

**Start-Process fix at start.ps1:335 (CRITICAL — pitfall #5 / RESEARCH Assumption A8):**
The current line has the same path twice:
```powershell
$Process = Start-Process -FilePath $Binary -WindowStyle Hidden -PassThru -RedirectStandardOutput $LogFile -RedirectStandardError $LogFile
```
Change to use the split `$LogFileErr` path:
```powershell
$Process = Start-Process -FilePath $Binary -WindowStyle Hidden -PassThru -RedirectStandardOutput $LogFile -RedirectStandardError $LogFileErr
```

Same root cause as Phase 6.1 quick-task `260411-iyf` (v4.1.1 CHANGELOG.md:12-13) — fixed in start.sh's Windows-via-bash branch then, but missed start.ps1 proper. Planner must include this fix in Bug #4.

---

### `scripts/start.ps1` — `patch_settings_local` PowerShell mirror (RESEARCH Open Question #2)

**Role:** Windows-side node.js-embedded JSON patcher mirror. **Data flow:** file-I/O on `settings.local.json` from the Windows script.

**Analog:** **none in start.ps1.** This would be a Phase 7 scope expansion per RESEARCH §Bug #3 "Do the same for `start.ps1`".

**Fallback pattern (per RESEARCH):** start.sh already invokes node.js directly; start.ps1 can do the same via `node -e "..."` with a PowerShell here-string, mirroring the exact node.js script from start.sh:452-522 but with `$env:USERPROFILE` instead of `$HOME`. The node.js embedded script itself is platform-agnostic — `process.env.HOME || process.env.USERPROFILE` at start.sh:455 already handles both.

**Decision deferred to planner:** Per RESEARCH Open Question #2, include in Phase 7 for dual-registration parity on Windows (cost ~60 lines of PowerShell), OR defer (cost: Windows users rely on plugin-hook path only, which is what has upstream reliability issues). Research recommendation: include.

**If included, pattern to follow:** invoke node.js exactly as start.sh does (node.js is already listed in RESEARCH §Environment Availability as `✓ (start.sh already uses via patch_settings_local)`). The PowerShell shell merely prepares the `-e` argument via here-string; the JSON manipulation logic stays identical.

---

### `scripts/stop.sh` — extend `cleanup_settings_local` filter (complement D-07)

**Role:** node.js-embedded JSON cleanup filter. **Data flow:** read `~/.claude/settings.local.json` → filter → atomic write.

**Analog:** the existing `cleanup_settings_local` filter predicate in the same file (stop.sh:53-106) — specifically the `.filter(...)` at stop.sh:79-84.

**Exact current predicate (stop.sh:79-84, verbatim):**
```js
const filtered = entries.filter(function(e) {
    if (!e || !Array.isArray(e.hooks)) return true;
    return !e.hooks.some(function(h) {
        return h && typeof h.url === 'string' && h.url.indexOf('127.0.0.1:19460') !== -1;
    });
});
```

**Extension pattern (per RESEARCH Bug #3 §"Cleanup mirror"):** extend the inner `.some(...)` to also match `h.command` substrings:
```js
const filtered = entries.filter(function(e) {
    if (!e || !Array.isArray(e.hooks)) return true;
    return !e.hooks.some(function(h) {
        if (!h) return false;
        if (typeof h.url === 'string' && h.url.indexOf('127.0.0.1:19460') !== -1) return true;
        if (typeof h.command === 'string'
            && h.command.indexOf('dsrcode') !== -1
            && h.command.indexOf('stop.sh') !== -1) return true;
        return false;
    });
});
```

**Object.keys() snapshot-iteration pattern (already honored at stop.sh:76):**
```js
// Object.keys creates a snapshot so delete-during-iteration is safe
for (const event of Object.keys(settings.hooks)) {
```
Keep verbatim — the extension is entirely inside the predicate, not in the iteration pattern.

---

### `scripts/stop.ps1` — mirror of stop.sh cleanup (optional, RESEARCH Open Question #5)

**Role:** Windows cleanup mirror.

**Analog:** **none in stop.ps1** (current stop.ps1 has no settings.local.json cleanup).

**Decision deferred to planner:** include only if start.ps1 gained `patch_settings_local` parity (previous file). Otherwise, skip — if start.ps1 doesn't patch, stop.ps1 has nothing to clean up.

**If included:** mirror stop.sh:53-106 via `node -e "..."` from PowerShell (same approach as discussed for start.ps1).

---

### `session/registry_test.go` — Touch tests

**Role:** Unit test (table-friendly). **Data flow:** in-memory registry state → assertion.

**Analogs in same file:**

*`TestRegistryUpdateActivity` setup pattern (registry_test.go:65-76, verbatim):*
```go
func TestRegistryUpdateActivity(t *testing.T) {
	changed := 0
	reg := session.NewRegistry(func() { changed++ })

	req := session.ActivityRequest{
		SessionID: "sess-1",
		Cwd:       "/home/user/project",
	}
	reg.StartSession(req, 1234)
	changed = 0 // reset after start
```

*`TestStaleCheck` activity-time manipulation (registry_test.go:252-257, verbatim):*
```go
reg.StartSession(req, os.Getpid())

// Set LastActivityAt to 11 minutes ago
reg.SetLastActivityForTest("sess-stale", time.Now().Add(-11*time.Minute))

// Run stale check with 10min idle timeout, 30min remove timeout
session.CheckOnce(reg, 10*time.Minute, 30*time.Minute)
```

**Copy the pattern verbatim:**
1. Construct registry with counting `onChange` callback (line 67 idiom).
2. `reg.StartSession(ActivityRequest{SessionID, Cwd}, pid)`.
3. `changed = 0 // reset after start` (line 74).
4. `reg.SetLastActivityForTest(..., time.Now().Add(-N*time.Minute))` (line 255).
5. `reg.Touch(...)` — new call.
6. `reg.GetSession(...).LastActivityAt` + `time.Since(...)` assertion.
7. `if changed != 0 { t.Errorf(...) }` — the D-05 assertion (copy from the inverted form at registry_test.go:102-104 where changed MUST be 1; here it MUST stay 0).

**TestTouchIsNoOpForUnknownSession (second test):** Use the fatal-on-invocation callback pattern — `session.NewRegistry(func() { t.Fatal("onChange must not fire") })`. Mirrors the defensive-test style used elsewhere in this file.

**Framework:** `go test` with table-driven tests per `rules/golang/testing.md` (global rule); race-flag usage per `go test -race ./...` per same rule.

---

### `session/stale_test.go` (or inside `registry_test.go`) — HTTP-source skip + PID-source preservation tests

**Role:** Unit test. **Data flow:** registry with forced-dead PID + forced-past activity → `CheckOnce` → existence assertion.

**Analogs in same file:**

*`TestStaleCheck` (registry_test.go:244-270):* demonstrates the full `StartSession` → `SetLastActivityForTest` → `CheckOnce` → `GetSession` assertion chain.

*`TestStaleRemove` (registry_test.go:274-296):* demonstrates the "session should be nil" assertion pattern after `CheckOnce`:
```go
s := reg.GetSession("sess-remove")
if s != nil {
    t.Error("session should have been removed after exceeding removeTimeout")
}
```

**Key differences for the new tests:**
1. The HTTP-source test uses `SessionID: "http-proj-1"` — the `http-` prefix triggers `sourceFromID` → `SourceHTTP` at source.go:42-50 → `StartSession` → `StartSessionWithSource(..., SourceHTTP)` at registry.go:42-44.
2. The PID-source test uses any UUID-shaped ID without a prefix → `SourceClaude`.
3. The PID must be non-existent (e.g., `99999999`) for `IsPidAlive` to return false; the test gates on this.

**Platform gate (for PID-source test):** Per RESEARCH §"Test strategy", `IsPidAlive(99999999)` may return true in rare cases on Windows due to tasklist quirks. Consider `//go:build !windows` build constraint for just the PID-source-liveness test if flakiness appears during CI, or use an obviously-invalid PID like `0x7FFFFFFF`.

**Do NOT introduce a new testing framework.** Stay on stdlib `testing` + table-driven, per project rules + per existing test files.

---

### `server/server_test.go` — `TestHandlePostToolUseUpdatesLastActivity`

**Role:** Integration test (HTTP boundary). **Data flow:** POST body → handler → registry state assertion.

**Analogs in same file:**

*`newTestServer` (server_test.go:28-40) + `startTestSession` (server_test.go:975-982) + `postHook` (server_test.go:947-954):* the three primitives for server-boundary tests.

*`TestHandlePostToolUse` (server_test.go:1035-1058) — **the nearest sibling test** to what's being added:*
```go
func TestHandlePostToolUse(t *testing.T) {
	srv, registry := newTestServer(nil)
	startTestSession(srv, "pt-1", "/tmp/project")

	body := `{"session_id":"pt-1","cwd":"/tmp/project","tool_name":"Bash","transcript_path":"/tmp/transcript.jsonl"}`
	code, elapsed := postHook(srv.Handler(), "/hooks/post-tool-use", body)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if elapsed > 100*time.Millisecond {
		t.Errorf("post-tool-use should respond quickly: got %v", elapsed)
	}

	// Transcript path stored per D-18.
	sess := registry.GetSession("pt-1")
	if sess == nil {
		t.Fatal("session pt-1 disappeared")
	}
	if sess.TranscriptPath != "/tmp/transcript.jsonl" {
		t.Errorf("TranscriptPath: got %q, want /tmp/transcript.jsonl",
			sess.TranscriptPath)
	}
}
```

**Pattern for the new test:**
1. Copy the three-line setup (`newTestServer`, `startTestSession`, body-build).
2. Between `startTestSession` and `postHook`, call `registry.SetLastActivityForTest("pt-activity", time.Now().Add(-5*time.Minute))` (pattern from registry_test.go:255).
3. POST to `/hooks/post-tool-use`.
4. Assert `time.Since(sess.LastActivityAt) < 5*time.Second`.

**Optional companion test (D-05 UI-invariance):** save `SmallImageKey`/`SmallText`/`Details` before POST, assert unchanged after POST.

**Framework:** same `net/http/httptest` + stdlib `testing` used throughout this file.

---

### `CHANGELOG.md` — v4.1.2 section

**Role:** Release notes. **Data flow:** N/A (markdown).

**Analog:** v4.1.1 and v4.1.0 entries in the same file.

**v4.1.1 excerpt to model after (CHANGELOG.md:10-13, verbatim):**
```markdown
## [4.1.1] - 2026-04-11

### Fixed
- Windows daemon launch in `scripts/start.sh` now redirects stdout to `~/.claude/dsrcode.log` and stderr to `~/.claude/dsrcode.log.err` via PowerShell `Start-Process -RedirectStandardOutput`/`-RedirectStandardError`. Previously both streams went to the void, so dsrcode's slog output and any crash traces were silently lost on Windows, making failures impossible to diagnose. Stderr uses a separate path because `Start-Process` rejects same-path redirects with `MutuallyExclusiveArguments`.
```

**Style conventions extracted from existing entries:**
- Semver header `## [X.Y.Z] - YYYY-MM-DD` (Keep-a-Changelog format declared at CHANGELOG.md:5).
- Sub-sections in this order: `### Added`, `### Changed`, `### Removed`, `### Fixed` — v4.1.1 uses only `### Fixed`; v4.1.2 will use `### Fixed` (primary) and `### Changed` (for Unix log-split per RESEARCH §Cross-Cutting Concerns).
- Bullet entries start with a bold user-visible symptom, then the code-path reference, then the rationale. See v4.1.1's single entry for shape.
- Backticked file paths (`scripts/start.sh`), inline symbol names (`Start-Process -RedirectStandardOutput`), and upstream-issue cross-references (format: `anthropics/claude-code#NNNN`).

**v4.1.2 section content (per RESEARCH §Cross-Cutting Concerns → CHANGELOG) is already drafted.** Copy into CHANGELOG.md, replacing `2026-04-XX` with the real release date.

---

### Version constants — `bump-version.sh 4.1.2`

**Role:** Release plumbing. **Data flow:** sed in-place on 5 files.

**Analog:** `scripts/bump-version.sh` already exists and performs all 5 edits (main.go, `.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json`, `scripts/start.sh`, `scripts/start.ps1`).

**Current baseline verified:** `.claude-plugin/plugin.json:3` shows `"version": "4.1.1"` — matches expected pre-bump state.

**Invocation (verbatim from CLAUDE.md §Releasing):**
```bash
./scripts/bump-version.sh 4.1.2
```

**Verification pattern (per RESEARCH §Cross-Cutting Concerns):** After running, 5 exact matches of `4.1.2` must appear across the 5 files. No custom planner logic — trust `bump-version.sh`.

---

## Shared Patterns

### Pattern 1: Immutable copy-before-modify in registry mutations

**Source:** `session/registry.go` — `UpdateActivity` (line 149 `updated := *session`), `upgradeSession` (line 124 `upgraded := *existing`), `UpdateSessionData` (line 214 `updated := *session`), `SetLastActivityForTest` (line 442 `updated := *session`). Every mutation method honors this.

**Apply to:** `Touch` — new method MUST follow the same pattern even though it changes one field.

**Concrete excerpt to copy (registry.go:149-182 shape):**
```go
// Immutable update: copy the session struct
updated := *session
updated.LastActivityAt = time.Now()
r.sessions[sessionID] = &updated
```

**Do NOT** modify `session` directly in place — breaks downstream readers holding `GetSession()` results.

---

### Pattern 2: Write-lock + nil-check + no-op-on-unknown

**Source:** `registry.go:137-147` (UpdateActivity prologue), `registry.go:205-212` (UpdateSessionData prologue), `registry.go:433-440` (SetLastActivityForTest prologue).

**Apply to:** `Touch` — every non-constructor method on `SessionRegistry` grabs write-lock then nil-checks the map.

**Excerpt pattern:**
```go
r.mu.Lock()
defer r.mu.Unlock()

session, ok := r.sessions[sessionID]
if !ok {
    return  // (or "return nil" if the method returns *Session)
}
```

---

### Pattern 3: Panic-recovered background goroutines in hook handlers

**Source:** `server/server.go:522-527` (handleSessionEnd), `server/server.go:599-605` (handlePostToolUse existing goroutine), `server/server.go:751-754` (another async hook). Established Phase 6 D-09 pattern.

**Apply to:** Any new background work added to `handlePostToolUse`. (D-04's Touch is SYNCHRONOUS so this pattern does not apply to the Bug #2 fix itself, but if the planner chooses to defer the Touch into a goroutine, the recover-defer pattern is mandatory.)

**Excerpt (server.go:600-605, verbatim):**
```go
go func() {
    defer func() {
        if r := recover(); r != nil {
            slog.Error("post-tool-use background panic",
                "session", sessionID, "panic", r)
        }
    }()
```

**Recommendation:** keep Touch synchronous (the registry lock is held microseconds; no benefit from goroutine; simpler correctness).

---

### Pattern 4: node.js-embedded JSON manipulation (chosen over jq per Phase 6.02 D-11)

**Source:** `start.sh:412-440` (patch_hooks_json), `start.sh:452-522` (patch_settings_local), `stop.sh:58-106` (cleanup_settings_local).

**Conventions:**
- `command -v node &>/dev/null || return 0` — graceful-skip when node absent.
- `process.env.HOME || process.env.USERPROFILE` — cross-platform home directory.
- Atomic write: `const tmp = settingsPath + '.tmp.' + process.pid; fs.writeFileSync(tmp, ...); fs.renameSync(tmp, settingsPath);` (start.sh:514-516, stop.sh:98-100).
- `Object.keys(obj)` snapshot-iteration when deleting keys during iteration (stop.sh:76).
- Ownership markers via `indexOf` substring checks — not equality (start.sh:495, stop.sh:82).

**Apply to:** Bug #3 SessionEnd-command-hook auto-patch (extend start.sh) and complementary cleanup filter (extend stop.sh).

---

### Pattern 5: Cross-platform file-size check idioms

**Bash (MSYS2-compatible):** `wc -c < FILE 2>/dev/null | tr -d ' '` — used at start.sh:160 (download validation). Reuse for `rotate_log`.

**PowerShell:** `(Get-Item $path -ErrorAction SilentlyContinue).Length` — used at start.ps1:106 (download validation). Reuse for `Rotate-Log`.

**Rationale:** both idioms already live in the same files that need the rotate helpers — no new dependency, consistent mental model.

---

### Pattern 6: Windows file encoding for refcount + PID files

**Source:** `start.ps1:35` (`$ActiveSessions | Out-File -FilePath $RefcountFile -Encoding ASCII -NoNewline`), `start.ps1:73` (lock pid), `start.ps1:336` (PID file), `stop.ps1:23` (decremented refcount).

**Convention:** `-Encoding ASCII -NoNewline` on every PowerShell `Out-File` that writes integer state.

**Apply to:** Any PowerShell code in Phase 7 that writes integer or path state to a `.claude/*` file. Bug #3 SessionEnd doesn't add any new such writes, but Bug #4 rotate helper writes no content (uses `Move-Item`), so this pattern is more a background invariant than an active Phase 7 requirement.

---

### Pattern 7: `bash -c '${CLAUDE_PLUGIN_ROOT:-fallback}; bash "$ROOT/script.sh"'` wrapper

**Source:** `hooks/hooks.json:10` (SessionStart line 1), `hooks/hooks.json:15` (install-skills.sh).

**Convention:** every plugin-hook `command` string uses this exact shape:
```
bash -c 'ROOT="${CLAUDE_PLUGIN_ROOT:-$HOME/.claude/plugins/marketplaces/dsrcode}"; bash "$ROOT/scripts/NAME.sh"'
```

**Rationale per RESEARCH §Bug #3:** defends against Windows issue #16116 (`${CLAUDE_PLUGIN_ROOT}` expansion failure) the same way the live SessionStart entry does. Production-proven.

**Apply to:** new SessionEnd entry in `hooks/hooks.json` — verbatim wrapper, only the script name changes (`stop.sh` not `start.sh`).

---

### Pattern 8: `go test` with table-driven style and race detection

**Source:** project-wide test files (`session/registry_test.go`, `server/server_test.go`, `analytics/*_test.go`) + `.claude/rules/golang/testing.md` global rule.

**Conventions:**
- `package xxx_test` (external test package) — visible in every `_test.go` at line 1.
- Table-driven for multi-case: `for _, tt := range tests { t.Run(tt.name, func(t *testing.T) { ... }) }` — see `TestActivityMapping` at server_test.go:96.
- Race flag mandatory: `go test -race ./...`.
- Helpers: `t.Helper()` marker — see `waitForSessionCount` at server_test.go:960.
- Coverage target: 80% per `.claude/rules/common/testing.md`.

**Apply to:** all new tests in Phase 7 — `TestTouch*`, `TestStaleCheckSkipsPidCheckForHttpSource`, `TestStaleCheckPreservesPidCheckForPidSource`, `TestHandlePostToolUseUpdatesLastActivity`, `TestHandlePostToolUseDoesNotChangeUiFields`.

---

## No Analog Found

Files with no close match in the existing codebase (planner should use RESEARCH.md patterns + research-recommended approach):

| File | Role | Data Flow | Reason | RESEARCH Reference |
|------|------|-----------|--------|--------------------|
| `scripts/start.ps1` — `patch_settings_local` PowerShell mirror | Script JSON patcher on Windows | file-I/O | start.ps1 has no existing settings.local.json auto-patch; this is a scope expansion beyond D-07 literal reading | RESEARCH §Bug #3 "Do the same for `start.ps1`" and Open Question #2 — research recommends including in Phase 7 for Windows parity |
| `scripts/stop.ps1` — cleanup mirror | Script JSON cleanup on Windows | file-I/O | stop.ps1 has no existing settings.local.json cleanup; only included if start.ps1 gained the patch | RESEARCH Open Question #5 — recommended if start.ps1 mirror is included |

**Planner's decision point:** Both entries above are optional Phase 7 expansions. The cheapest path (Unix-only dual-registration) satisfies D-07 literally; the fuller path (Windows parity) provides the upstream-bug resilience on all platforms per RESEARCH's dual-registration mitigation.

---

## Cross-Cutting Conventions (Checklist for Planner)

- [ ] Immutable copy-before-modify at registry.go:149 (applies to `Touch`)
- [ ] Write-lock + nil-check prologue in every registry method (applies to `Touch`)
- [ ] Panic-recovered background goroutines (applies ONLY if Bug #2 fix chooses async; synchronous `Touch` is simpler)
- [ ] Windows refcount/PID ASCII + NoNewline encoding (background invariant; no new writes in Phase 7)
- [ ] `node -e` chosen over `jq` in start.sh/stop.sh settings.local.json manipulation (Phase 6.02 D-11) — extend, do not replace
- [ ] `Object.keys()` snapshot pattern for delete-during-iteration safety — preserve verbatim when extending cleanup filter
- [ ] `bash -c '${CLAUDE_PLUGIN_ROOT:-fallback}'` wrapper on all plugin-hook commands (applies to new SessionEnd entry)
- [ ] `go test -race ./...` must be green after Phase 7 merge per global Go testing rule
- [ ] `gofmt`/`goimports` mandatory per `.claude/rules/golang/coding-style.md`
- [ ] Keep-a-Changelog format `## [X.Y.Z] - YYYY-MM-DD` + `### Fixed` / `### Changed` sub-sections (CHANGELOG.md:1-5)
- [ ] `./scripts/bump-version.sh 4.1.2` is the one true way to propagate version to 5 files — do not hand-edit

---

## Metadata

**Analog search scope:** `session/`, `server/`, `scripts/`, `hooks/`, `.claude-plugin/`, `CHANGELOG.md`.
**Files scanned:** 21 (10 Go, 1 hook JSON, 4 scripts, 1 plugin JSON, 1 marketplace JSON, 1 CHANGELOG, plus 3 read for cross-reference).
**Line references verified:** 2026-04-13 against the working tree at branch `main`. All cited line numbers match the actual source on disk.
**Pattern extraction date:** 2026-04-13.

---

## PATTERN MAPPING COMPLETE
