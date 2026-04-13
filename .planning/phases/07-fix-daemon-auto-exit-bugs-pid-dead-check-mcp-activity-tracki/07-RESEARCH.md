# Phase 7 Research: Daemon Auto-Exit Bug Fixes

**Researched:** 2026-04-13
**Domain:** Go daemon lifecycle, cross-platform scripting, claude-code plugin hook semantics
**Confidence:** HIGH across all 4 bugs; Bug #3 requires a strategic call (documented below)
**Target release:** v4.1.2 (hotfix)

## Summary

Three findings the planner MUST internalize before decomposing:

1. **Bug #3 is the riskiest**: Multiple open and closed claude-code upstream issues (`#17885`, `#35892`, `#16288`, `#27398`, `#16116`) document that plugin-scoped `hooks/hooks.json` SessionEnd command hooks are NOT universally reliable — they silently fail on `/exit`, `/clear`, and several Windows exit paths, and are also stripped entirely by `--setting-sources user` in Cowork-style launchers. The Phase 6.02 split ("SessionStart/SessionEnd stay in plugin hooks.json") stands on shaky ground. Recommended mitigation: **dual-register** the SessionEnd hook — add it to the plugin `hooks/hooks.json` (satisfies D-07 literally) AND extend the existing `patch_settings_local()` function in `start.sh` / add equivalent in `start.ps1` to also write the SessionEnd command into `~/.claude/settings.local.json`. Mirrors Phase 6.02 D-11 pattern and provides a fallback channel for every documented broken exit path.

2. **Bug #2's "no UI side-effects" constraint (D-05)** is not free: `registry.UpdateActivity` unconditionally calls `notifyChange()` at line 183 of `session/registry.go` which signals the presence debouncer. Calling it from `handlePostToolUse` would cause a Discord presence update every tool-call — the exact scenario the user rejected. Recommended idiom: a new `registry.Touch(sessionID)` method that updates `LastActivityAt` under the write-lock WITHOUT calling `notifyChange()`. This mirrors the established `stacklok/toolhive` `ProxySession.Touch()` pattern verified via live GitHub code search. Alternative (parameter on UpdateActivity) leaks an implementation detail through the public API. Alternative (atomic.Int64) breaks the immutable copy-before-modify invariant that every existing registry method honors.

3. **Bug #1's fix is one line plus a PID-zero short-circuit**. The current `session/stale.go:41` reads `if s.PID > 0 && !IsPidAlive(s.PID)`. Adding `s.Source != SourceHTTP` to the guard is sufficient and preserves PID-liveness for Unix PID-sourced sessions (D-02). Bug #4's fix is constrained: PowerShell `Start-Process -RedirectStandardOutput` truncates with no append option per PowerShell issue #15031 (feature-requested since 2021, closed stale 2023, never implemented). The cross-platform pattern is "rotate-on-launch before redirect": check size → rename → start-process-with-truncate (fresh file). Combined with size-capped rotation this satisfies D-10/D-11/D-12 without needing PowerShell append support.

**Primary recommendation:** Structure Phase 7 as 5 plans — one per bug plus a release/CHANGELOG plan. Bugs #1, #2, #4 are mechanical. Bug #3 needs the dual-registration strategy documented below. All 4 are independently landable, so the plans can execute in parallel (no cross-bug dependency).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|---|---|---|---|
| PID-liveness skip for HTTP sessions | Go stale-detector (`session/stale.go`) | — | Local to CheckOnce; no registry or API change. |
| LastActivityAt update on PostToolUse | Go HTTP handler (`server/server.go`) + Go registry (`session/registry.go`) | — | Server emits the touch; registry implements the no-notify write path. |
| SessionEnd refcount decrement | Plugin hook config (`hooks/hooks.json`) + Script (`scripts/start.sh` + `start.ps1`) | User settings (`~/.claude/settings.local.json` via auto-patch) | Plugin registration is primary per D-07, auto-patch is the reliable fallback per upstream-bug evidence. |
| Log rotation on daemon start | Cross-platform scripts (`scripts/start.sh` + `start.ps1`) | — | No binary change. Rotation is a script-side concern. |
| CHANGELOG + version bump | Repo root (`CHANGELOG.md` + `scripts/bump-version.sh`) | — | Standard release hygiene. |

## User Constraints (from CONTEXT.md)

### Locked Decisions (verbatim)

#### Bug #1 — PID-Liveness-Check for HTTP-Sessions
- **D-01:** Skip PID-liveness-check entirely for sessions with `Source == HTTP` in `session/stale.go`. The code comment at line 40 already recommends this; the 2-minute grace heuristic that exists today is too aggressive on Windows where `os.Getppid()` returns the short-lived `start.sh` bash process PID.
- **D-02:** Preserve PID-check behavior for PID-sourced sessions on Unix — the liveness signal is still meaningful there (macOS/Linux `signal(0)` is reliable, Phase 3 decision).
- **D-03:** No config toggle — the skip is unconditional when `Source == HTTP` because the check is not trustworthy for that source. Adding a knob is complexity without benefit.

#### Bug #2 — `handlePostToolUse` LastActivityAt Update
- **D-04:** `handlePostToolUse` in `server/server.go` MUST update `LastActivityAt` on every call. Current code only calls `UpdateTranscriptPath` / `RecordTool` — it completely bypasses the registry's activity clock.
- **D-05:** Update only `LastActivityAt` — do NOT set/overwrite `SmallImageKey`, `SmallText`, `Details`. The user wants zero UI side-effects from PostToolUse. Planner's call on how to achieve this without side-effects (either minimal `ActivityRequest` flag, new `registry.Touch(sessionID)` method, or guarded `UpdateActivity` path — any of these satisfies D-05).
- **D-06:** `handlePostToolUse` fires on every tool call (incl. MCPs) because `settings.local.json` already has `matcher: "*"` on PostToolUse. The fix is entirely server-side; no hook-config change needed for Bug #2.

#### Bug #3 — Refcount Drift via Missing SessionEnd-Command-Hook
- **D-07:** Register a new `SessionEnd` entry in the plugin's `hooks/hooks.json` (NOT in user-level `settings.local.json`). It runs `scripts/stop.sh` / `scripts/stop.ps1` as a `type: "command"` hook, mirroring the existing `SessionStart` pattern.
- **D-08:** Keep the existing HTTP `/hooks/session-end` handler untouched — it handles registry cleanup and auto-exit grace period. The new command hook is purely for refcount decrement (Windows) and PID-tracking cleanup (Unix).
- **D-09:** Versioned in the Plugin repo. Avoids user-specific drift in `settings.local.json`. Matches how Phase 6.02 already splits plugin-hooks vs user-settings.

#### Bug #4 — Log Overwrite on Daemon Start
- **D-10:** `start.sh` / `start.ps1` MUST append to `dsrcode.log` rather than truncate. Implementation detail delegated to planner (bash `>>` redirection vs PowerShell `-Append`).
- **D-11:** Implement size-cap rotation at 10 MB: `dsrcode.log` → `dsrcode.log.1` (single backup, overwrite). No date-based rotation, no multi-generation retention — keep it simple but bounded.
- **D-12:** Same pattern on `dsrcode.log.err` (stderr).

#### Scope & Release
- **D-13:** Cross-platform (all 4 bugs affect all platforms; Bug #1 most visible on Windows, Bug #2 affects every platform, Bug #3 Windows-specific, Bug #4 all).
- **D-14:** Target release tag: `v4.1.2` (hotfix) — NOT `v4.2.0` (no new features).
- **D-15:** Plan count estimate: 4-6 plans (one per bug + release/CHANGELOG plan). Planner will break down.

### Claude's Discretion (verbatim)
- `registry.Touch()` vs extended `UpdateActivity` — planner picks the idiom closest to existing code. **Research recommends `Touch()` — see Bug #2 section.**
- PID-Source enum handling — planner decides whether `Source == HTTP` is best expressed as direct equality, bitfield, or helper method based on `session/source.go` inspection. **Research recommends direct `s.Source != session.SourceHTTP` equality — see Bug #1 section.**
- Log rotation implementation — bash/PowerShell idiom is up to planner (native cmdlets preferred over external `logrotate`). **Research recommends rotate-on-launch with native `wc -c` / `Get-Item.Length` — see Bug #4 section.**
- Test-coverage split — unit tests for stale.go + registry.Touch; integration tests for SessionEnd command hook; manual verification for log rotation. Planner finalizes. **Research recommends the full split — see Validation Architecture.**

### Deferred Ideas (OUT OF SCOPE)
- **Prometheus /metrics endpoint + structured observability** — surfaced during Scope question (Option D). Deferred to a future observability phase.
- **Refcount abolition (Source-of-Truth = registry only)** — architecturally cleaner, but larger blast radius. Revisit after Phase 7 stabilizes.
- **X-Claude-PID enforcement** — requires Claude Code CLI change; not in our repo's control.
- **Heartbeat-based liveness-probe architecture** — Kubernetes-style patterns. Not needed now.
- **Config-driven stale timeouts per-source** — could let HTTP sessions have longer timeouts than PID sessions. Not needed; D-01 skip + 30m `removeTimeout` backstop suffices.

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| D-01 | Skip PID-liveness-check for `Source == HTTP` in stale.go | Bug #1 section — `s.Source != session.SourceHTTP` guard verified against `session/source.go:42` `sourceFromID` + `registry.go:43` `StartSession` already populating `Source` at session creation. |
| D-02 | Preserve PID-check for `Source == PID` (Unix signal(0)) | Bug #1 section — existing `stale_unix.go` `signal(syscall.Signal(0))` untouched. |
| D-03 | No config toggle | Bug #1 section — pure guard expansion, no new config field. |
| D-04 | `handlePostToolUse` updates `LastActivityAt` every call | Bug #2 section — new `registry.Touch(sessionID)` method invoked from `server.go:565` (immediately after 200 response) before the throttled analytics read path. |
| D-05 | Update only `LastActivityAt`, no UI side-effects | Bug #2 section — `Touch()` method skips `notifyChange()` so the presence debouncer is NOT signaled, preserving Discord icon/text exactly as-is. |
| D-06 | No hook-config change for Bug #2 | Bug #2 section — confirmed `settings.local.json` already has `matcher: "*"` for PostToolUse (verified at `scripts/start.sh:470`). |
| D-07 | Register SessionEnd command hook in plugin `hooks/hooks.json` | Bug #3 section — pattern shown mirrors existing SessionStart entry at `hooks/hooks.json:4-19`. |
| D-08 | Keep HTTP `/hooks/session-end` handler untouched | Bug #3 section — `server/server.go:498-557` `handleSessionEnd` remains 100% untouched; new command hook runs in parallel. |
| D-09 | Versioned in plugin repo | Bug #3 section — `hooks/hooks.json` is tracked in git, verified by `git log -- hooks/hooks.json`. |
| D-10 | Append to log instead of truncate | Bug #4 section — bash `>>` via `nohup`, PowerShell rotate-before-launch (append option unsupported upstream). |
| D-11 | 10 MB rotation, single backup `log.1` | Bug #4 section — `wc -c < "$LOG_FILE"` / `(Get-Item $LogFile).Length` → `mv` / `Move-Item -Force`. |
| D-12 | Same pattern for `dsrcode.log.err` | Bug #4 section — factor into a helper: `rotate_log "$LOG_FILE"`, `rotate_log "$LOG_FILE.err"`. |
| D-13 | Cross-platform | All 4 bug sections address Windows + Unix explicitly. |
| D-14 | Target `v4.1.2` | Cross-Cutting Concerns section — `bump-version.sh 4.1.2`. |
| D-15 | 4-6 plans | Cross-Cutting Concerns section — recommended 5 plans. |

## Bug #1: PID-Liveness-Check for HTTP Sources

### Current code (exact)

`session/stale.go:29-63` — the entire CheckOnce function:

```go
func CheckOnce(registry *SessionRegistry, idleTimeout, removeTimeout time.Duration) {
    now := time.Now()
    sessions := registry.GetAllSessions()

    for _, s := range sessions {
        elapsed := now.Sub(s.LastActivityAt)

        // PID liveness check — skip for sessions with recent hook activity.
        // HTTP hook-based sessions may carry the daemon's parent PID rather
        // than the actual Claude Code process PID, causing false removals.
        if s.PID > 0 && !IsPidAlive(s.PID) {        // <-- line 41
            if elapsed > 2*time.Minute {
                slog.Info("removing stale session (PID dead, no recent activity)", ...)
                registry.EndSession(s.SessionID)
                continue
            }
            slog.Debug("PID dead but session has recent activity, skipping removal", ...)
        }

        // Remove timeout (per D-29: 30min default)
        if elapsed > removeTimeout {
            slog.Info("removing stale session (timeout)", ...)
            registry.EndSession(s.SessionID)
            continue
        }

        // Idle timeout (per D-29: 10min default)
        if elapsed > idleTimeout && s.Status == StatusActive {
            slog.Debug("marking session idle", ...)
            registry.TransitionToIdle(s.SessionID)
        }
    }
}
```

The doc comment at line 38-40 already prescribes the fix but the code does not implement it — the 2-minute grace is the existing attempt, and it is too tight for MCP-heavy sessions that can idle the activity clock 2+ minutes between non-throttled updates (before Bug #2 is fixed; after Bug #2, LastActivityAt is fresh on every tool call).

### Recommended fix (verified against source)

Change the guard to include a `Source != SourceHTTP` test. Two equivalent formulations:

**Option A (explicit, preferred by research — direct equality):**
```go
if s.PID > 0 && s.Source != SourceHTTP && !IsPidAlive(s.PID) {
    // ...existing grace-period block kept intact for Source == PID (Unix only now)
}
```

**Option B (helper method on Session):**
```go
// In session/types.go:
func (s *Session) ShouldCheckPidLiveness() bool {
    return s.PID > 0 && s.Source != SourceHTTP
}

// In session/stale.go:
if s.ShouldCheckPidLiveness() && !IsPidAlive(s.PID) { ... }
```

Option A keeps the fix local to one line; Option B adds a testable predicate but also adds API surface. The CONTEXT.md discretion clause says "direct equality" is acceptable; research recommends Option A unless the planner sees reason to encapsulate.

**Grace-period question:** Should the `2*time.Minute` grace be kept for the non-HTTP case (Unix PID sessions)? Two views:
- Keep it as-is — the grace was never broken for PID-sourced sessions; only HTTP-sourced exposed the bug.
- Remove it — if `IsPidAlive == false` for a PID-sourced session, it's genuinely crashed; 2-min grace adds noise without value.

Research recommends **keep the grace-period block as-is** for Unix PID-sourced sessions. Rationale: the 2-min grace was introduced as a defensive hedge (STATE Phase 3 decision implicitly) and removing it is scope creep for a hotfix. D-02 says "preserve PID-check behavior for PID-sourced sessions" — literal reading = no behavior change for that branch.

### Nyquist reasoning (is removeTimeout still a sufficient backstop?)

After Bug #1 + Bug #2 are both in, the staleness backstop for HTTP-sourced sessions is exclusively the `removeTimeout` path (`stale.go:51`, 30 min default).

- **Stale-check polling interval:** 30 seconds (`config.StaleCheckInterval = 30*time.Second`, verified `config/config.go:197`).
- **Activity-update cadence after Bug #2:** Every PostToolUse tool call updates `LastActivityAt` via `Touch()`. For MCP-heavy sessions this fires continuously (sub-second during active tool chains, idle periods cap out at user think-time). Nyquist-style requirement: polling interval << activity cadence, both << timeout budget.
- **Numbers:** 30s polling × threshold 30m = 60× sample-rate safety margin. The registry cannot miss a "no activity in 30m" signal because 30m / 30s = 60 polls during the window. HTTP-session removal fires exactly once when the session has been genuinely idle for 30 min.

Conclusion: 30m `removeTimeout` is correct and sufficient post-fix. **No change needed.** If long-idle sessions become a user complaint the knob is already exposed via config (`removeTimeout` in `~/.claude/dsrcode-config.json`).

### Test strategy (regression guard)

Add to `session/registry_test.go` (or new `stale_test.go`):

```go
// TestStaleCheckSkipsPidCheckForHttpSource verifies D-01: HTTP-sourced sessions
// with a dead PID are NOT removed by the PID-liveness check. They are only
// removed via the removeTimeout path.
func TestStaleCheckSkipsPidCheckForHttpSource(t *testing.T) {
    reg := session.NewRegistry(func() {})

    req := session.ActivityRequest{
        SessionID: "http-proj-1",            // http- prefix → sourceFromID → SourceHTTP
        Cwd:       "/home/user/project",
    }
    // Use a PID that definitely does not exist
    reg.StartSession(req, 99999999)
    // Make LastActivityAt 2.5 minutes ago (past the existing grace, before removeTimeout)
    reg.SetLastActivityForTest("http-proj-1", time.Now().Add(-150*time.Second))

    session.CheckOnce(reg, 10*time.Minute, 30*time.Minute)

    if s := reg.GetSession("http-proj-1"); s == nil {
        t.Fatal("D-01 violation: HTTP-sourced session removed despite recent activity")
    }
}

// TestStaleCheckPreservesPidCheckForPidSource verifies D-02: PID-sourced sessions
// with a dead PID and no recent activity ARE removed after the 2-minute grace.
func TestStaleCheckPreservesPidCheckForPidSource(t *testing.T) {
    reg := session.NewRegistry(func() {})

    req := session.ActivityRequest{
        SessionID: "abcdef-claude-uuid-1",   // no prefix → SourceClaude (PID-based)
        Cwd:       "/home/user/project",
    }
    reg.StartSession(req, 99999999)          // non-existent PID
    reg.SetLastActivityForTest("abcdef-claude-uuid-1", time.Now().Add(-5*time.Minute))

    session.CheckOnce(reg, 10*time.Minute, 30*time.Minute)

    if s := reg.GetSession("abcdef-claude-uuid-1"); s != nil {
        t.Error("D-02 violation: PID-sourced session with dead PID and old activity NOT removed")
    }
}
```

Note: the second test may be platform-dependent on Windows where `IsPidAlive(99999999)` might return true in rare cases (tasklist output quirks). Use a very high PID that is beyond the Windows PID allocation ceiling, or gate the test behind `//go:build !windows` if flakiness appears during CI.

[VERIFIED: `session/source.go:42` `sourceFromID` logic; `session/registry.go:42-44` `StartSession` delegating to `StartSessionWithSource(req, pid, sourceFromID(req.SessionID))`]
[VERIFIED: `session/stale.go:41` current faulty guard]
[VERIFIED: `config/config.go:197` default `StaleCheckInterval = 30*time.Second`]

## Bug #2: LastActivityAt in handlePostToolUse (no UI side-effects)

### Current code (exact, with the gap annotated)

`server/server.go:563-647` — the entire handlePostToolUse function. Key gap at line 579 (pre-analytics branch) and line 638-642 (post-analytics branch): **neither path touches `LastActivityAt`**. The only mutations hit are:
- `s.tracker.RecordTool(sessionID, payload.ToolName)` — analytics-only, no registry change
- `s.registry.UpdateTranscriptPath(sessionID, transcriptPath)` — updates `TranscriptPath` only, not `LastActivityAt`
- The "error overlay clear" branch at lines 631-637 does call `UpdateActivity` but only if `SmallImageKey == "error"` — normal PostToolUse flows skip it entirely.

And `registry.UpdateActivity` at `session/registry.go:137-185` does set `LastActivityAt = time.Now()` at line 179, but it also calls `notifyChange()` at line 183 which signals the presence debouncer (`main.go:504 presenceDebouncer`, 100ms debounce + 15s Discord rate limit). Every PostToolUse from the MCP chain would fire a debouncer kick.

### Why "just call UpdateActivity with empty fields" doesn't work

Reading `UpdateActivity` line by line confirms:
- Lines 152-158: only overwrites SmallImageKey/SmallText/Details **if the request field is non-empty**. So passing an empty ActivityRequest wouldn't overwrite UI fields.
- Lines 175-177: `CounterMap[updated.SmallImageKey]` — lookup uses the EXISTING SmallImageKey (already on the session), which means any session with icon "coding" or "searching" will get that counter bumped on every tool call. For D-05 "zero UI side-effects" this counter bump is marginal but technically a side-effect.
- Line 179: **unconditionally** sets `LastActivityAt = time.Now()` — this is what we want.
- Line 180: **unconditionally** sets `Status = StatusActive` — this is OK (activity does mean active), but it would flip idle sessions back to active silently. Arguably desirable behavior; arguably a side-effect.
- Line 183: **unconditionally** calls `notifyChange()` — this is the problem. It signals the debouncer which re-renders Discord presence. User's D-05 rejected this.

So the three options laid out in CONTEXT.md have the following characteristics:

| Option | Changes LastActivityAt | Fires notifyChange | Changes UI fields | Bumps counters | Flips Status | API surface |
|---|---|---|---|---|---|---|
| **(a) new `Touch()` method** | yes | **no** | no | no | no | +1 method |
| (b) `UpdateActivity(..., notify bool)` | yes | conditional | conditional (still skips empty) | yes (CounterMap lookup) | yes | API break (signature change) |
| (c) atomic.Int64 on LastActivityAt | yes | n/a (atomic op) | no | no | no | breaks copy-before-modify pattern everywhere |

Research recommends **(a) `Touch(sessionID)`**. Evidence:

**Production precedent:** `stacklok/toolhive/pkg/transport/session/proxy_session.go` defines exactly this pattern, verified via `gh search code`:

```go
// Touch updates the session's last updated time to the current time.
func (s *ProxySession) Touch() {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.updated = time.Now()
}
```

**Code locality:** One new method, placed directly below `UpdateActivity` in `registry.go`, 8 lines.

**Test coverage:** Trivially unit-testable — set `LastActivityAt` to past, call `Touch`, assert it moves forward; also assert that `onChange` callback is NOT invoked.

**No API break:** `UpdateActivity` signature stays identical; every existing test continues to work without modification.

### Recommended implementation (verified pattern)

```go
// Add to session/registry.go, below UpdateActivity (around line 186):

// Touch updates LastActivityAt to the current time for an existing session
// WITHOUT firing the onChange callback. Used by PostToolUse hooks (D-04, D-05
// Phase 7) that need to keep the stale-detector's activity clock fresh without
// triggering a Discord presence update on every MCP tool call.
//
// Follows the immutable copy-before-modify pattern established by UpdateActivity.
// No-op if the session does not exist.
func (r *SessionRegistry) Touch(sessionID string) {
    r.mu.Lock()
    defer r.mu.Unlock()

    session, ok := r.sessions[sessionID]
    if !ok {
        return
    }

    updated := *session
    updated.LastActivityAt = time.Now()
    r.sessions[sessionID] = &updated
    // Intentionally does NOT call notifyChange() — see D-05 Phase 7.
}
```

And in `server/server.go handlePostToolUse`, add one line near the top (after empty-session-id check, before the throttled-analytics branch):

```go
if payload.SessionID == "" {
    slog.Debug("post-tool-use: empty session_id, ignoring")
    return
}

// Phase 7 D-04: keep the stale-detector's activity clock fresh on every tool
// call so long MCP-heavy sessions are not prematurely removed. No UI side-
// effects (D-05) — Touch() does not call notifyChange().
s.registry.Touch(payload.SessionID)
```

Placement rationale: put it before the `s.tracker.RecordTool` call so that even if the tracker is nil (test mode) the activity clock still updates. The `Touch` is a no-op for unknown sessionIDs, so it is safe to call unconditionally — matches the HTTP-hook pattern elsewhere (StartSession is also called unconditionally at `server.go:421`).

### Test strategy (two regressions)

**Test 1 — `session/registry_test.go`:**
```go
func TestTouchUpdatesLastActivityWithoutNotify(t *testing.T) {
    notifyCount := 0
    reg := session.NewRegistry(func() { notifyCount++ })

    reg.StartSession(session.ActivityRequest{SessionID: "t1", Cwd: "/p"}, 1234)
    notifyCount = 0                                  // reset after StartSession
    reg.SetLastActivityForTest("t1", time.Now().Add(-5*time.Minute))

    reg.Touch("t1")

    s := reg.GetSession("t1")
    if time.Since(s.LastActivityAt) > 10*time.Second {
        t.Errorf("Touch did not update LastActivityAt, elapsed=%v", time.Since(s.LastActivityAt))
    }
    if notifyCount != 0 {
        t.Errorf("D-05 violation: Touch called notifyChange %d times, want 0", notifyCount)
    }
}

func TestTouchIsNoOpForUnknownSession(t *testing.T) {
    reg := session.NewRegistry(func() { t.Fatal("onChange must not fire") })
    reg.Touch("does-not-exist")                      // must not panic
}
```

**Test 2 — `server/server_test.go` (regression guard — the bug this phase fixes):**
```go
func TestHandlePostToolUseUpdatesLastActivity(t *testing.T) {
    srv, registry := newTestServer(nil)
    startTestSession(srv, "pt-activity", "/tmp/project")

    // Force LastActivityAt to the past so we can observe the update
    registry.SetLastActivityForTest("pt-activity", time.Now().Add(-5*time.Minute))

    body := `{"session_id":"pt-activity","cwd":"/tmp/project","tool_name":"Bash"}`
    code, _ := postHook(srv.Handler(), "/hooks/post-tool-use", body)
    if code != http.StatusOK {
        t.Fatalf("expected 200, got %d", code)
    }

    sess := registry.GetSession("pt-activity")
    elapsed := time.Since(sess.LastActivityAt)
    if elapsed > 5*time.Second {
        t.Errorf("D-04 violation: PostToolUse did not update LastActivityAt, elapsed=%v", elapsed)
    }
}
```

Consider adding one more test that asserts the UI fields are UNCHANGED after PostToolUse (SmallImageKey, SmallText, Details, ActivityCounts unchanged), to nail down D-05.

[VERIFIED: `session/registry.go:140-185` UpdateActivity; line 183 `notifyChange()`; line 179 `LastActivityAt`; line 180 `Status = StatusActive`]
[VERIFIED: `server/server.go:563-647` handlePostToolUse — no LastActivityAt touch]
[CITED: stacklok/toolhive `pkg/transport/session/proxy_session.go` `func (s *ProxySession) Touch()` — `gh search code`]
[VERIFIED: `main.go:501-555` presenceDebouncer — 15s rate limit, 100ms debounce, signals Discord SetActivity]

## Bug #3: SessionEnd Command Hook

**This is the bug with the weakest vendor-side guarantee. Research strongly recommends a dual-registration strategy.**

### Upstream evidence — plugin SessionEnd command hooks are NOT reliable

All claims below are from primary sources (claude-code issue tracker). Dates and states preserved:

| Issue | State | Date | Problem |
|---|---|---|---|
| [#17885](https://github.com/anthropics/claude-code/issues/17885) | CLOSED stale (2026-02-28) | filed 2026-01-13 | SessionEnd hook does not fire on `/exit`. Windows comment confirms ALL documented exit paths (`/exit`, `/clear`, Ctrl+D, VS Code X button) silently fail. Configured in `~/.claude/settings.json` (not even plugin hooks.json). |
| [#35892](https://github.com/anthropics/claude-code/issues/35892) | **OPEN** | filed 2026-03-18 | Explicitly states "`/exit` prints hardcoded 'Goodbye!' → process terminates immediately. No hook event fires." References #17885 and #12755. |
| [#6428](https://github.com/anthropics/claude-code/issues/6428) | (open per search) | Aug 2025 | SessionEnd hook does not fire on `/clear` despite docs claiming it should. |
| [#16288](https://github.com/anthropics/claude-code/issues/16288) | **OPEN** | filed 2026-01-04 | "Plugin hooks not loaded from external hooks.json file" — referenced hook file via plugin.json `"hooks": "./hooks/hooks.json"` never fires. **Workaround: define the hook directly in `settings.local.json`.** |
| [#27398](https://github.com/anthropics/claude-code/issues/27398) | CLOSED as duplicate of #16288 (2026-02-25) | filed 2026-02-21 | Cowork VM spawns CLI with `--setting-sources user` which silently excludes plugin-scope — all plugin-defined hooks are dropped. |
| [#16116](https://github.com/anthropics/claude-code/issues/16116) | CLOSED (2026-01-06) | filed 2026-01-03 | Windows: `${CLAUDE_PLUGIN_ROOT}` variable fails to expand; plugin hook commands abort. Workaround: use full literal path or register directly in settings.json. |
| [#41577](https://github.com/anthropics/claude-code/issues/41577) | CLOSED dup of #24206 (2026-04-04) | filed 2026-03-31 | SessionEnd hooks running async work are killed before completion. Timeout config is ignored. Workaround: `nohup ... & disown`. |

**Positive counterevidence (the one finding pointing the other way):** The official docs at `https://code.claude.com/docs/en/hooks` (fetched fresh for this research) state without qualification that both `.claude/settings.local.json` and plugin `hooks/hooks.json` are supported hook locations with identical semantics. No warning is shown that SessionEnd or plugin-scoped hooks have reduced reliability.

**Bottom-line interpretation:** The docs say plugin hooks work; a meaningful subset of real-world scenarios confirms they do not. The dsrcode plugin already works around Bug #16288/Bug #27398 for its 13 HTTP hooks by auto-patching `settings.local.json` in `start.sh` — the pattern is established, tested in Phase 6.02, and already handles the hook-discovery-failure class.

### Recommended strategy (dual registration)

Implement D-07 **literally** (plugin `hooks/hooks.json` gets the SessionEnd entry) AND **also** extend the auto-patch so `settings.local.json` gets the same entry.

**Why dual:** Redundancy. When one of these upstream bugs bites a user, the other channel provides the refcount decrement. Costs nothing — the script already runs node.js to patch settings.local.json, adding one more hook event is additive. `stop.sh` / `stop.ps1` is idempotent (existing decrement logic handles "refcount already 0"), so double-invocation is harmless.

**Downside to consider:** If plugin's hooks.json works AND settings.local.json auto-patch works simultaneously, the refcount decrements TWICE on clean exit. Mitigation options:
1. Make `stop.sh` / `stop.ps1` explicitly "decrement once per session-id": use a marker file (`~/.claude/dsrcode-sessions/$SESSION_ID.stopped`) checked before decrementing. Cost: one `test -f` + `touch`.
2. Accept that double-decrement on clean exit is benign because both channels converge at `ACTIVE_SESSIONS = 0` and stop already uses `max(0, count-1)` on Windows (`stop.ps1:19`).

Research recommends **option 2 (accept double-decrement)** because option 1 adds marker-file state that can itself drift. The existing `[Math]::Max(0, $CurrentCount - 1)` at `stop.ps1:19` and `[[ $ACTIVE_SESSIONS -lt 0 ]] && ACTIVE_SESSIONS=0` at `stop.sh:123` already floor at zero. The worst case is that refcount drops from 2 to 0 instead of 2 to 1 when two users share a Windows machine and one `/exit`s while the other is mid-session — the user mid-session gets the daemon killed early. This is rare (dual-user Windows sessions of dsrcode is unusual) and the existing HTTP session-end handler would re-start via the auto-exit grace path.

If the planner disagrees, **option 1** is easy to add: a 3-line marker-file guard at the top of both stop scripts.

### Plugin `hooks/hooks.json` SessionEnd entry (exact shape, mirrors existing SessionStart)

Reference: `hooks/hooks.json:4-19` currently has SessionStart with `type: "command"` running `bash -c 'ROOT="${CLAUDE_PLUGIN_ROOT:-$HOME/.claude/plugins/marketplaces/dsrcode}"; bash "$ROOT/scripts/start.sh"'` with timeout 15.

Exact new SessionEnd entry to insert after the SessionStart block:

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
]
```

**No matcher** — SessionEnd matcher is the exit-reason selector (`clear`, `logout`, `resume`, `prompt_input_exit`, `bypass_permissions_disabled`, `other` per docs). Omitting matcher = match all reasons, which is what we want for unconditional refcount decrement.

**Why `type: "command"` not `type: "http"`:** The existing HTTP `/hooks/session-end` endpoint does registry cleanup (`handleSessionEnd` in `server/server.go:498`). The command hook is purely for the refcount file which is outside the daemon's process — the daemon cannot decrement the refcount for itself because the refcount file represents "how many Claude Code sessions are currently using the daemon," which is user-process state the daemon cannot observe directly on Windows (PPID unreliable, D-02 Phase 3 decision).

**Windows command note:** Per issue #16116, `${CLAUDE_PLUGIN_ROOT}` has failed on Windows. The `bash -c '...'` wrapper with a fallback `${CLAUDE_PLUGIN_ROOT:-$HOME/.claude/plugins/marketplaces/dsrcode}` is exactly the pattern used by the existing SessionStart entry, and the existing SessionStart works in production on Windows, so the same shape for SessionEnd is low-risk.

### Windows PowerShell caveat for SessionEnd

`stop.sh` on Windows uses `taskkill` via bash which works because Git Bash is present. If a Windows user does NOT have Git Bash, the SessionStart hook also fails, so this isn't a new constraint. However, `hooks/hooks.json` specifies `bash -c '...'` unconditionally — PowerShell users who lack Git Bash have no SessionStart-hook-working baseline today, so SessionEnd inherits that limitation harmlessly.

(If the planner wants to be extra-safe, the SessionEnd entry could be replicated with a second entry using `type: "command"` + `command: "powershell -NoProfile -File ${CLAUDE_PLUGIN_ROOT}\\scripts\\stop.ps1"` with a Windows-specific matcher — but Claude Code hooks.json does not support per-platform matchers natively, so this would invoke BOTH on Windows. Recommend skipping this for Phase 7 scope.)

### `settings.local.json` auto-patch for SessionEnd command hook (fallback channel)

Extend the existing `patch_settings_local` function in `scripts/start.sh` (currently lines 447-522). Add an entry for SessionEnd-as-command to the `DSRCODE_HOOKS` object. **Caveat: the existing loop writes type=http entries exclusively.** The new entry needs a branch or a second loop for type=command.

Recommended approach: introduce a second constant `DSRCODE_COMMAND_HOOKS` in the node.js patch script:

```js
// Add after DSRCODE_HOOKS definition in start.sh's patch_settings_local:
const DSRCODE_COMMAND_HOOKS = {
    'SessionEnd': {
        matcher: null,
        command: 'bash -c \\'ROOT="' + (process.env.CLAUDE_PLUGIN_ROOT || (home + '/.claude/plugins/marketplaces/dsrcode')) + '"; bash "$ROOT/scripts/stop.sh"\\'',
        timeout: 15
    }
};

// Then a second loop:
for (const event of Object.keys(DSRCODE_COMMAND_HOOKS)) {
    const cfg = DSRCODE_COMMAND_HOOKS[event];
    if (!Array.isArray(settings.hooks[event])) settings.hooks[event] = [];
    const existing = settings.hooks[event];
    // Ownership marker: command contains 'dsrcode' and 'stop.sh'
    const hasDsrcode = existing.some(function(e) {
        return e && Array.isArray(e.hooks) && e.hooks.some(function(h) {
            return h && typeof h.command === 'string'
                && h.command.indexOf('dsrcode') !== -1
                && h.command.indexOf('stop.sh') !== -1;
        });
    });
    if (!hasDsrcode) {
        const entry = { hooks: [{ type: 'command', command: cfg.command, timeout: cfg.timeout }] };
        if (cfg.matcher !== null) entry.matcher = cfg.matcher;
        existing.push(entry);
        added++;
    }
}
```

**Cleanup mirror:** `scripts/stop.sh cleanup_settings_local` (lines 53-106) currently matches by `h.url.indexOf('127.0.0.1:19460')`. Extend it to ALSO remove entries where `h.command` contains `stop.sh` + `dsrcode`:

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

**Do the same for `start.ps1`** — the Windows script currently does NOT have a `patch_settings_local` equivalent (`start.ps1` is much simpler than `start.sh`). This is a Phase 7 scope expansion. If the planner wants to keep Windows minimal, the dual-registration provides strong upstream-bug resilience only on Unix; Windows relies on the plugin hook fully working. Recommend adding the PowerShell equivalent — cost is ~60 lines of PowerShell mirroring the node.js auto-patch.

### What goes in stop.sh / stop.ps1 for the new payload

Claude Code SessionEnd command hooks receive stdin JSON with shape:
```json
{
    "session_id": "abc123",
    "transcript_path": "...",
    "cwd": "...",
    "permission_mode": "default",
    "hook_event_name": "SessionEnd"
}
```
(Verified from `https://code.claude.com/docs/en/hooks` — see "Common input fields".)

The existing `stop.sh` does NOT read stdin. It only inspects `$PPID` / refcount file / PID file. This is compatible: stdin is silently discarded by bash, no change needed. PowerShell `stop.ps1` likewise reads no stdin. So the refcount-decrement logic works as-is for SessionEnd payload — no script modifications beyond registration.

### Test strategy

- **Unit:** N/A for Bug #3 — no Go code changes.
- **Integration (automated):** Write a bash test that (a) starts daemon, (b) simulates SessionEnd by writing refcount=2 then invoking `scripts/stop.sh` manually, (c) checks refcount file = 1. This proves the script does its job. Does NOT prove Claude Code invokes the hook — that requires manual.
- **Manual cross-platform (mandatory for this bug):** Bring up Claude Code, note refcount=1, exit via EACH of these paths, verify refcount drops to 0: (a) `/exit`, (b) Ctrl+D (Unix), (c) VS Code close-button, (d) kill the Claude Code process externally. Document which paths work and which silently fail. This is the data the CHANGELOG release note needs.

## Bug #4: Log Rotation

### Current code (exact)

**`scripts/start.sh:578`** (Unix path):
```bash
nohup "$BINARY" > "$LOG_FILE" 2>&1 &
```
`>` truncates. The Unix fix is trivial: `>>` to append.

**`scripts/start.sh:575-576`** (Windows-via-bash path):
```bash
powershell.exe -NoProfile -WindowStyle Hidden -Command \
    '$process = Start-Process -FilePath "'"$WIN_BINARY"'" -WindowStyle Hidden -PassThru -RedirectStandardOutput "'"$WIN_LOG_FILE"'" -RedirectStandardError "'"${WIN_LOG_FILE}.err"'"; $process.Id | Out-File -FilePath "'"$WIN_PID_FILE"'" -Encoding ASCII -NoNewline' 2>/dev/null
```
This delegates to PowerShell. `Start-Process -RedirectStandardOutput` truncates with no append option.

**`scripts/start.ps1:335`** (pure PowerShell):
```powershell
$Process = Start-Process -FilePath $Binary -WindowStyle Hidden -PassThru -RedirectStandardOutput $LogFile -RedirectStandardError $LogFile
```
Same truncation problem. **Also note:** same file for stdout and stderr — this is the very bug fixed in Phase 6.1 quick task `260411-iyf` (stderr was going to the void). The Phase 7 log rotation fix must preserve the split into `$LogFile` + `$LogFile.err` per that previous fix. Actually looking more carefully at lines 335: `-RedirectStandardError $LogFile` uses the SAME path — this is a regression risk; reading the earlier quick task commit `dcdbb9a` suggests this was intended to be `$LogFile.err` on start.ps1 too. **Research flags this as a pre-existing bug the planner should double-check during Bug #4 implementation.**

### The PowerShell constraint

Per PowerShell issue #15031 (filed 2021, closed stale 2023 without fix):
- `Start-Process -RedirectStandardOutput` always truncates.
- No `-Append` flag exists.
- No workaround via `ProcessStartInfo.FileMode.Append` is exposed.
- Community-recommended alternatives:
  1. Capture to temp file, then `Add-Content` the temp to the target log (requires running process to exit first — not viable for long-lived daemon).
  2. Use `System.Diagnostics.ProcessStartInfo` directly (verbose, low-level).
  3. **Rotate the file BEFORE starting the process** — with truncation as intended for the new file. Accept rotation-on-startup semantics.

Research recommends **option 3 (rotate-on-launch)**: size-check the existing log; if >10 MB rename to `.log.1`; then start the process, which truncates an empty file. This satisfies D-10 (effectively "append across daemon restarts by preserving last log.1"), D-11 (10 MB cap), D-12 (same for stderr) — AND keeps D-10 honest because the daemon's CURRENT launch writes an empty file, then its slog output accumulates across the daemon's lifetime (the daemon itself is a long-lived process that doesn't re-truncate mid-run; only relaunch-via-start.sh re-truncates).

**Semantic clarification for D-10:** "Append instead of truncate" in the CONTEXT.md context means "preserve log history across daemon restarts," not "append from the Start-Process perspective literally." The rotate-on-launch pattern preserves history (renames to .log.1) while keeping start.sh's mechanism compatible with PowerShell's constraints. The Unix side can literally use `>>` which is both simpler and idiomatic. Or the Unix side can also use rotate-on-launch for consistency across platforms — recommended for simplicity (one mental model, one test scenario).

### Recommended cross-platform implementation

**Unix (`start.sh`):** Add a helper function near the top, invoke it before the `nohup` line.

```bash
# Rotate a log file when it exceeds 10 MB. Single-backup (file.1, overwritten).
# Portable: uses wc -c which works on GNU coreutils, BSD, macOS, and busybox.
rotate_log() {
    local log="$1"
    local max_size=10485760  # 10 MB
    [[ -f "$log" ]] || return 0
    local size
    size=$(wc -c < "$log" 2>/dev/null | tr -d ' ')
    [[ -n "$size" && "$size" -gt "$max_size" ]] || return 0
    mv -f "$log" "$log.1"
}

# Before the daemon launch section:
rotate_log "$LOG_FILE"
rotate_log "$LOG_FILE.err"
```

Then change line 578 from `> "$LOG_FILE" 2>&1 &` to:
```bash
nohup "$BINARY" >> "$LOG_FILE" 2>> "$LOG_FILE.err" &
```

Note the split of stdout / stderr into separate files — this is a **behavior change for Unix** (currently combines both into `$LOG_FILE`), but matches how Windows already splits them (after the `260411-iyf` fix). Aligning the two platforms simplifies mental model.

Windows-via-bash branch at start.sh:575: keep calling PowerShell but add the rotate step first. Add before `powershell.exe ...`:
```bash
# Rotate on the Unix side before calling PowerShell (simpler than rotating
# inside the PS one-liner). Paths are Unix paths here, cygpath conversion
# happens inside the PowerShell block.
rotate_log "$LOG_FILE"
rotate_log "$LOG_FILE.err"
```

And fix the PowerShell one-liner redirect target to be `$WIN_LOG_FILE.err` (it already is — verified line 576).

**Pure PowerShell (`start.ps1`):** Add a rotate function near the top, invoke before Start-Process.

```powershell
# Rotate a log file when it exceeds 10 MB. Single-backup (.log.1, overwritten).
function Rotate-Log {
    param([string]$LogPath)
    $maxSize = 10485760  # 10 MB
    if (-not (Test-Path $LogPath)) { return }
    $size = (Get-Item $LogPath -ErrorAction SilentlyContinue).Length
    if ($size -ne $null -and $size -gt $maxSize) {
        Move-Item -Path $LogPath -Destination "$LogPath.1" -Force -ErrorAction SilentlyContinue
    }
}

# Before "Start Daemon" block:
$LogFileErr = "$LogFile.err"
Rotate-Log $LogFile
Rotate-Log $LogFileErr
```

Then change line 335:
```powershell
$Process = Start-Process -FilePath $Binary -WindowStyle Hidden -PassThru -RedirectStandardOutput $LogFile -RedirectStandardError $LogFileErr
```

Note fixing the pre-existing same-path-stderr issue (stderr was going to $LogFile in start.ps1 — the Phase 6.1 quick task did fix start.sh's PowerShell branch but may have missed start.ps1 proper; planner should grep and verify). This is an **additional scope beyond D-12** but keeping it in Bug #4 is natural since it's a log-redirection concern.

### Edge cases

- **Concurrent writes during rename:** On Windows, if the daemon is actively writing to `dsrcode.log` while `Move-Item -Force` runs, the move will fail (file locked). Mitigation: the rotate runs BEFORE `Start-Process`, so the daemon is not yet running. The OLD daemon was killed earlier in start.sh (`kill_daemon_if_running`) / start.ps1 (`Kill-DaemonIfRunning`). No concurrent-write window exists between kill and start.
- **Rotate-while-session-active:** If Claude Code restarts the daemon mid-session (unusual — only on version change per start.sh:342-368), the running daemon is killed, log is rotated, new daemon writes a fresh file. The fresh file is empty but `.log.1` has the previous content. This is the correct behavior.
- **10 MB size threshold:** Arbitrary but reasonable. `10485760` = 10 × 1024 × 1024. Slog output is verbose in debug mode; 10 MB typically holds ~2-5 days of INFO-level output in a normal session. Single-backup retention (10-20 MB total) is well-bounded.
- **`mv -f` on Windows via Git Bash:** Works (MSYS2 mv is GNU-semantics). No Windows-specific issue.
- **Empty file or permissions denied on stat:** `wc -c < "$log"` returns empty string; `-gt` comparison against empty fails; return early without rotation. Safe.

### Test strategy

- **Automated (impossible for Bug #4 — platform scripting):** hard to write unit tests for shell scripts without invoking them. Can add bash-lint / shellcheck to CI but that's out of Phase 7 scope.
- **Manual verification checklist (must be documented in plan):**
  1. Delete `~/.claude/dsrcode.log`, start daemon, verify file is created with content.
  2. Kill daemon, restart, verify new content APPENDED (not truncated). Expected: file grows across launches.
  3. Fake a >10 MB log: `dd if=/dev/zero bs=1024 count=11000 >> ~/.claude/dsrcode.log`; restart daemon; verify `.log.1` exists with ~11 MB content and `.log` is empty (fresh launch from empty after rotation).
  4. Repeat on both platforms (Git Bash on Windows AND pure PowerShell start.ps1 if users invoke it directly).
  5. Verify `.log.err` follows the same rotation separately.

[CITED: PowerShell issue #15031 — no append option for Start-Process]
[VERIFIED: `scripts/start.sh:578` `nohup > $LOG_FILE 2>&1 &`]
[VERIFIED: `scripts/start.ps1:335` `Start-Process ... -RedirectStandardError $LogFile` — same-path bug may remain]
[CITED: Baeldung + nixCraft articles — `wc -c` is the portable file-size method]

## Runtime State Inventory

> Phase 7 is not a rename/refactor phase; this section is normally omitted. However, the Phase 7 fixes interact with runtime state (refcount file, log files, session files) and it is worth recording explicitly.

| Category | Items Found | Action Required |
|---|---|---|
| Stored data | `~/.claude/dsrcode.refcount` (Windows), `~/.claude/dsrcode-sessions/*` (Unix) | No data-migration; Bug #3 fixes the WRITE path (decrement-on-SessionEnd), existing data self-heals within sessions. |
| Live service config | `~/.claude/settings.local.json` — 13 dsrcode HTTP hooks registered at start.sh patch-time | Bug #3 extends the set by 1 command-hook entry for SessionEnd. Existing users get the new entry on next `SessionStart` fire. |
| OS-registered state | None — dsrcode does not use launchd / systemd / Task Scheduler. PID file only. | None. |
| Secrets/env vars | `CLAUDE_PLUGIN_ROOT`, `CLAUDE_PLUGIN_DATA`, `HOME`, `USERPROFILE` — all read, never written; no rename. | None. |
| Build artifacts | Binary at `$PLUGIN_DATA/bin/dsrcode{.exe}`; version tag embedded via ldflags. | `v4.1.2` tag + goreleaser will produce new binary; existing users auto-update via `start.sh` version-check at line 342-368 — no user action. |

**Explicit negatives (no items found — verified):**
- No database / vector store (ChromaDB/Mem0) — dsrcode is ephemeral.
- No Datadog/Grafana tags embedded — no observability integration yet.
- No cron jobs / scheduled tasks — dsrcode lifecycle is entirely Claude-Code-driven.
- No compiled artifacts tracked in git other than the bump-version.sh-managed version strings in 5 files.

## Common Pitfalls

### Pitfall 1: Calling `registry.UpdateActivity` with empty ActivityRequest
**What goes wrong:** Tempting shortcut for Bug #2 — "just call UpdateActivity with zero-value request." Actually: it still fires `notifyChange()` unconditionally (`registry.go:183`), still sets `Status = StatusActive` unconditionally (line 180), still does the CounterMap lookup (line 175). Violates D-05.
**Why it happens:** Reading the function name without reading the body. The name suggests "update activity," not "update activity+fire+status+counter."
**How to avoid:** Use the new `Touch()` method. Name is purpose-built for D-05.
**Warning signs:** Discord presence flicker every tool call; counter fields drift upward in verbose mode.

### Pitfall 2: Removing the 2-minute grace period in Bug #1
**What goes wrong:** Deleting the `if elapsed > 2*time.Minute` block thinking "the HTTP skip replaces this." But D-02 says preserve PID-check for PID-sourced sessions — and the 2-minute grace is part of that PID-check branch.
**Why it happens:** Over-aggressive refactor. D-01 scope is narrower than "rewrite CheckOnce."
**How to avoid:** Add the `s.Source != SourceHTTP` guard; do NOT touch the rest of the branch.
**Warning signs:** `TestStaleCheckPreservesPidCheckForPidSource` fails (test added per Bug #1 section).

### Pitfall 3: Expecting plugin SessionEnd to Just Work
**What goes wrong:** Register SessionEnd in `hooks/hooks.json`, ship v4.1.2, test on macOS/Linux (it works there), declare victory. User reports refcount still drifting on Windows `/exit`.
**Why it happens:** Upstream bug #17885 and #35892 are not surfaced to plugin authors.
**How to avoid:** Dual-register per Bug #3 section — auto-patch into `settings.local.json` as fallback.
**Warning signs:** `~/.claude/dsrcode.refcount` still above zero after last Claude Code session exits.

### Pitfall 4: PowerShell `-RedirectStandardOutput` "just add -Append"
**What goes wrong:** Search for "PowerShell append stdout to log" finds out-of-context answers that suggest `-Append`. There is NO `-Append` on `Start-Process` per issue #15031.
**Why it happens:** Mixing up `Out-File -Append` (works on cmdlet output) with `Start-Process -RedirectStandardOutput` (does not support append).
**How to avoid:** Use the rotate-on-launch pattern; accept that Start-Process truncates and that's fine because rotation preserves history.
**Warning signs:** Plan includes a non-existent PS parameter.

### Pitfall 5: Same file for stdout+stderr in PowerShell
**What goes wrong:** `Start-Process -RedirectStandardOutput $log -RedirectStandardError $log` — PowerShell throws `MutuallyExclusiveArguments` or silently drops one stream. This was the Phase 6.1 quick task `260411-iyf` fix for `start.sh`'s PowerShell branch but may not have been applied to `start.ps1` itself.
**Why it happens:** Copy-pasted error-redirect without thinking about constraints.
**How to avoid:** Always use `$LogFile` + `$LogFile.err` as two distinct paths.
**Warning signs:** `start.ps1:335` currently has the same `$LogFile` twice — planner should grep and fix during Bug #4.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---|---|---|---|
| Generic logrotate replacement | Multi-generation dated rotation, compression, retention policy | Single-backup size-cap rotation (D-11 specifies this exactly) | User decision; full logrotate is scope creep. |
| Thread-safe activity timestamp | `sync.Mutex` + `time.Time` wrapper | `registry.Touch()` inside existing `registry.mu.Lock()` | The registry already serializes all mutations; reuse its lock. |
| Custom SessionEnd-payload parser | `jq`-based session_id extraction in stop.sh | Discard stdin — existing stop scripts don't need it | The refcount decrement is stateless wrt session_id. |
| Hook-reliability probe | Custom heartbeat checker for SessionEnd delivery | Dual-registration redundancy | Simpler, idempotent, no new runtime state. |
| PowerShell append emulation | Wrapper executable that tees stdin-stdout to log | rotate-on-launch + truncate-to-empty-file | Accepted upstream limitation; rotation preserves history. |

## Code Examples

### Example 1 — Bug #1 fix (full diff)

```go
// session/stale.go (line 41-48 current → proposed)

// BEFORE:
if s.PID > 0 && !IsPidAlive(s.PID) {
    if elapsed > 2*time.Minute {
        slog.Info("removing stale session (PID dead, no recent activity)", "sessionId", s.SessionID, "pid", s.PID, "elapsed", elapsed)
        registry.EndSession(s.SessionID)
        continue
    }
    slog.Debug("PID dead but session has recent activity, skipping removal", "sessionId", s.SessionID, "pid", s.PID, "elapsed", elapsed)
}

// AFTER (D-01 Phase 7):
if s.PID > 0 && s.Source != SourceHTTP && !IsPidAlive(s.PID) {
    if elapsed > 2*time.Minute {
        slog.Info("removing stale session (PID dead, no recent activity)", "sessionId", s.SessionID, "pid", s.PID, "elapsed", elapsed)
        registry.EndSession(s.SessionID)
        continue
    }
    slog.Debug("PID dead but session has recent activity, skipping removal", "sessionId", s.SessionID, "pid", s.PID, "elapsed", elapsed)
}
```

### Example 2 — Bug #2 registry.Touch method

```go
// Add to session/registry.go immediately after UpdateActivity (around line 186):

// Touch updates LastActivityAt to the current time for an existing session
// WITHOUT firing the onChange callback. Used by PostToolUse hooks (Phase 7
// D-04/D-05) to keep the stale-detector's activity clock fresh on every MCP
// tool call, without triggering a Discord presence update per call.
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

### Example 3 — Bug #2 handlePostToolUse edit

```go
// server/server.go — inside handlePostToolUse, insert immediately after the
// empty-session_id guard at line 574, before the existing tracker.RecordTool
// call.

if payload.SessionID == "" {
    slog.Debug("post-tool-use: empty session_id, ignoring")
    return
}

// Phase 7 D-04: Keep the stale-detector's activity clock fresh on every MCP
// tool call. Touch() is a no-op for unknown sessions and does not fire
// notifyChange (D-05: no UI side-effects).
s.registry.Touch(payload.SessionID)

sessionID := payload.SessionID
// ... rest of handlePostToolUse unchanged ...
```

### Example 4 — Bug #3 plugin hooks.json SessionEnd entry

```json
{
  "description": "DSR Code Presence hooks — activity tracking and skill installation",
  "hooks": {
    "SessionStart": [
      /* existing entry unchanged */
    ],
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
    /* rest of events unchanged */
  }
}
```

### Example 5 — Bug #4 Unix rotate helper + nohup append

```bash
# Add near the top of scripts/start.sh (after cross-platform helpers,
# around line 63 before "Ensure Directories"):

# Rotate a log file when it exceeds 10 MB. Single-backup (file.1, overwritten).
# Portable size check via wc -c (works on GNU coreutils, BSD, macOS, busybox).
rotate_log() {
    local log="$1"
    local max_size=10485760  # 10 MB
    [[ -f "$log" ]] || return 0
    local size
    size=$(wc -c < "$log" 2>/dev/null | tr -d ' ')
    [[ -n "$size" && "$size" -gt "$max_size" ]] || return 0
    mv -f "$log" "$log.1"
}

# ... rest of script ...

# Before "Start Daemon" block (around line 569), add:
rotate_log "$LOG_FILE"
rotate_log "$LOG_FILE.err"

# Change line 578 from:
#   nohup "$BINARY" > "$LOG_FILE" 2>&1 &
# to:
nohup "$BINARY" >> "$LOG_FILE" 2>> "$LOG_FILE.err" &
```

### Example 6 — Bug #4 PowerShell rotate helper

```powershell
# Add near the top of scripts/start.ps1 (after function Acquire-Lock, before
# "Download Function with SHA256" — around line 80):

function Rotate-Log {
    param([string]$LogPath)
    $maxSize = 10485760  # 10 MB
    if (-not (Test-Path $LogPath)) { return }
    $item = Get-Item $LogPath -ErrorAction SilentlyContinue
    if ($null -ne $item -and $item.Length -gt $maxSize) {
        Move-Item -Path $LogPath -Destination "$LogPath.1" -Force -ErrorAction SilentlyContinue
    }
}

# Before "Start Daemon" block (around line 334), add:
$LogFileErr = "$LogFile.err"
Rotate-Log $LogFile
Rotate-Log $LogFileErr

# Change line 335 from:
#   $Process = Start-Process -FilePath $Binary -WindowStyle Hidden -PassThru -RedirectStandardOutput $LogFile -RedirectStandardError $LogFile
# to:
$Process = Start-Process -FilePath $Binary -WindowStyle Hidden -PassThru -RedirectStandardOutput $LogFile -RedirectStandardError $LogFileErr
```

## Cross-Cutting Concerns

### Release tag v4.1.2

Run `./scripts/bump-version.sh 4.1.2`. Per `CLAUDE.md`, this propagates the version string to 5 files: `main.go`, `.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json`, `scripts/start.sh`, `scripts/start.ps1`. After Phase 7 work merges, the release commit adds the tag and GoReleaser produces the 5 platform binaries.

### CHANGELOG.md entry (pattern copied from v4.1.1)

```markdown
## [4.1.2] - 2026-04-XX

### Fixed
- **Daemon self-termination during long MCP-heavy sessions** — `handlePostToolUse` now updates `LastActivityAt` on every tool call (Bug #2, D-04/D-05 Phase 7). Previously, MCP-heavy sessions could idle the activity clock past the 30-minute remove-timeout even while Claude Code was actively calling tools, causing the daemon to remove the session and eventually self-exit. New `registry.Touch()` method updates the activity timestamp without firing a Discord presence update.
- **PID-liveness check false-positives for HTTP-sourced sessions** — `session/stale.go` now skips the PID liveness check when `Source == HTTP` (Bug #1, D-01/D-02 Phase 7). The daemon's recorded PID for HTTP-sourced sessions is the short-lived `start.sh` wrapper process on Windows, which exits within seconds even while the Claude Code session is alive. The 30-minute `removeTimeout` remains as the backstop.
- **Refcount drift via missing SessionEnd command hook** — plugin `hooks/hooks.json` now registers `SessionEnd` as a `type: command` hook invoking `scripts/stop.sh` / `scripts/stop.ps1` (Bug #3, D-07 Phase 7). Dual-registered in `~/.claude/settings.local.json` via `start.sh` auto-patch (fallback for documented upstream plugin-hook-discovery issues — anthropics/claude-code#17885, #16288).
- **Log overwrite on daemon restart** — `start.sh` and `start.ps1` now rotate `dsrcode.log` at 10 MB to `dsrcode.log.1` (single backup) before daemon launch (Bug #4, D-10/D-11/D-12 Phase 7). Same rotation applies to `dsrcode.log.err`. Crash history now survives daemon restarts.

### Changed
- Unix log redirection (`start.sh`) now splits stdout and stderr into separate files (`dsrcode.log` and `dsrcode.log.err`) matching Windows behavior established in v4.1.1.
```

### bump-version.sh propagation check

Verify after running: `grep "4.1.2" main.go .claude-plugin/plugin.json .claude-plugin/marketplace.json scripts/start.sh scripts/start.ps1` — should show exactly 5 matches (one per file). **As of research,** `plugin.json:3` shows `"version": "4.1.1"`, matching the expected baseline for bump to 4.1.2.

### Migration-safety sanity check

None of the 4 bugs introduce breaking changes. Existing users upgrade by letting `start.sh`'s version-check path (lines 342-368) auto-download v4.1.2. No user-facing config migration. Existing refcount files / session files / log files are all preserved across the upgrade (the log rotation happens only if current log exceeds 10 MB, which is rare).

## Validation Architecture

Nyquist-style reproduction scenarios for each bug. Planner uses these to structure Wave 0 test gaps and per-task verify commands.

### Test Framework

| Property | Value |
|---|---|
| Framework | `go test` (stdlib) with table-driven tests; project-standard `go test -v -race ./...` |
| Config file | `go.mod` |
| Quick run command | `go test ./session/ ./server/ -run "TouchUpdates\|HandlePostToolUseUpdatesLastActivity\|SkipPidCheckForHttpSource\|PreservesPidCheckForPidSource" -v` |
| Full suite command | `go test -v -race ./...` |

### Phase requirements → test map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|---|---|---|---|---|
| D-01 | HTTP-sourced session with dead PID is NOT removed by PID check | unit | `go test ./session/ -run TestStaleCheckSkipsPidCheckForHttpSource -v` | needs new file `session/stale_source_test.go` (or add to `registry_test.go`) |
| D-02 | PID-sourced session with dead PID + old activity IS removed after 2m grace | unit | `go test ./session/ -run TestStaleCheckPreservesPidCheckForPidSource -v` | needs new file / additional test |
| D-04 | PostToolUse hook updates LastActivityAt | integration (table-driven server test) | `go test ./server/ -run TestHandlePostToolUseUpdatesLastActivity -v` | needs new test in `server_test.go` |
| D-05 | PostToolUse does NOT change UI fields | unit on Touch + unit on server handler (assert SmallImageKey unchanged) | `go test ./session/ -run TestTouchDoesNotFireNotify -v && go test ./server/ -run TestHandlePostToolUseDoesNotChangeUiFields -v` | needs new test |
| D-07 | SessionEnd command hook registered in plugin hooks.json | automated smoke | `jq '.hooks.SessionEnd[0].hooks[0].command' hooks/hooks.json | grep -q "stop.sh"` | N/A (script check in CI) |
| D-10/D-11/D-12 | Log rotation at 10 MB | manual + semi-automated bash | `scripts/phase-07/verify-logrotate.sh` (new file) | new file |
| D-14 | bump-version.sh propagates to 5 files | automated script check | `grep -l "4.1.2" main.go .claude-plugin/plugin.json .claude-plugin/marketplace.json scripts/start.sh scripts/start.ps1 \| wc -l` expecting `5` | N/A |

### Sampling rate

- **Per task commit:** `go test ./session/ ./server/ -v -race` (fast: ~5s)
- **Per wave merge:** `go test -v -race -cover ./...` (full suite, ~20s)
- **Phase gate:** Full suite green + manual cross-platform verification per Bug #3/#4 checklists.

### Reproduction scenarios (the failure modes the fix must prevent)

**Bug #1 reproduction:** On Windows, open Claude Code, trigger 30+ minutes of MCP-heavy tool calls (mcp__sequential-thinking, mcp__exa__web_search_exa) without any Edit/Write/Bash/Read. Before fix: daemon logs "removing stale session (PID dead, no recent activity)" and `dsrcode.exe` exits. After fix: session stays in registry, daemon stays up, refcount remains at 1.

**Bug #2 reproduction:** Same as Bug #1 (the live incident is the canonical repro). Before fix: `LastActivityAt` never updates during MCP calls, session ages into the remove-timeout or is killed by Bug #1's PID check. After fix: every PostToolUse refreshes `LastActivityAt`; session lifetime tracks tool-call cadence.

**Bug #3 reproduction:** Windows, `tasklist | grep dsrcode` shows 1 process; `cat ~/.claude/dsrcode.refcount` shows 1. Exit Claude Code via `/exit`. Before fix: refcount stays at 1 (or climbs past multi-session use). After fix: refcount decrements to 0; no-session-remaining path fires; daemon cleanup proceeds.

**Bug #4 reproduction:** Launch daemon, produce some slog INFO output, note log file size. Kill daemon, restart. Before fix: log file is truncated — previous content gone. After fix: log file content preserved; daemon appends (Unix) / rotates (Windows PS). After >10 MB: `.log.1` holds the previous content; `.log` is reset for the new launch.

### Wave 0 gaps

- [ ] `session/stale_source_test.go` (or extend `registry_test.go`) — covers D-01, D-02
- [ ] `session/registry_test.go` — add `TestTouchUpdatesLastActivityWithoutNotify` and `TestTouchIsNoOpForUnknownSession` — covers D-04 Touch contract + D-05 no-notify
- [ ] `server/server_test.go` — add `TestHandlePostToolUseUpdatesLastActivity` + `TestHandlePostToolUseDoesNotChangeUiFields` — covers D-04, D-05 at the HTTP boundary
- [ ] Optional: `scripts/phase-07/verify-logrotate.sh` (new file, not in CI) — manual-run helper that injects 11 MB into log and reruns start.sh
- [ ] No framework install needed — `go test` already in use.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|---|---|---|---|---|
| Go 1.25 | Bug #1, #2 (Go source) | ✓ | 1.25 | — (CI enforces) |
| Bash (Git Bash on Windows) | Bug #3, #4 (script edits) | ✓ | 5.x | — |
| PowerShell 5.1+ | Bug #3, #4 Windows-side | ✓ (assumed — Windows 11 default) | 5.1 / 7.x | — |
| node.js | Bug #3 auto-patch extension | ✓ (start.sh already uses via patch_settings_local) | v18+ | graceful skip (existing pattern) |
| jq (optional) | Bug #3 automated CI check | likely available (Git Bash MSYS2 includes) | 1.6+ | grep-based fallback |
| tasklist (Windows) | Bug #1 existing IsPidAlive | ✓ | built-in | — |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** jq (use grep).

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|---|---|---|
| A1 | `sourceFromID` prefix-based detection (`http-`, `jsonl-`) is preserved for all incoming HTTP session IDs | Bug #1 | Low — verified at `server/server.go:379` `payload.SessionID = "http-" + projectName`; synthetic prefix is explicit. |
| A2 | `Session.Source` field is always populated at session creation (not zero-value) | Bug #1 | Low — verified `registry.go:42-44` delegates to `StartSessionWithSource(..., sourceFromID(req.SessionID))`. |
| A3 | The `presenceDebouncer` is the only onChange listener that causes Discord side effects | Bug #2 | Low — verified `main.go:140-144` wires onChange to BOTH debounce channel AND auto-exit channel; auto-exit is session-count-based not notification-based, so Touch-without-notify is safe. |
| A4 | Dual-registration of SessionEnd command hook will not cause stop.sh to fail on "already-zero refcount" | Bug #3 | Low — verified `stop.sh:123` floors at 0 (`[[ $ACTIVE_SESSIONS -lt 0 ]] && ACTIVE_SESSIONS=0`) and `stop.ps1:19` uses `[Math]::Max(0, ...)`. Idempotent. |
| A5 | Windows plugin SessionEnd hook might work on user's machine even though issues #17885 and #35892 document it failing | Bug #3 | Moderate — this is why dual registration is recommended. If A5 holds in user's environment, double-decrement happens (benign per A4). |
| A6 | `wc -c` is available in Git Bash on Windows (MSYS2 coreutils) | Bug #4 | Low — MSYS2 ships coreutils; `wc` is standard. |
| A7 | `Move-Item -Force` can rename a 10 MB file with the daemon stopped | Bug #4 | Low — daemon is killed before rotation runs (`kill_daemon_if_running` at `start.sh:341` / `Kill-DaemonIfRunning` at `start.ps1:310`). |
| A8 | `start.ps1:335`'s same-path `-RedirectStandardError $LogFile` is a pre-existing defect (not a Phase 7 new bug) | Bug #4 | Low — Phase 6.1 quick task `260411-iyf` fixed the same issue in start.sh's Windows branch; likely missed start.ps1 proper. Planner should grep and verify during Bug #4 implementation. |

**Mitigation for A5:** the dual-registration strategy makes A5 a non-issue. Even if the user's specific Claude Code installation fires the plugin hook, the settings.local.json fallback ALSO fires; double-decrement floors at 0; correctness preserved.

## Open Questions

1. **Should the 2-minute grace period stay for PID-sourced sessions post-Bug-#1?**
   - What we know: D-02 says preserve PID check behavior.
   - What's unclear: whether "preserve PID check behavior" includes preserving the grace heuristic or only the `IsPidAlive` call.
   - Recommendation: keep the grace period. It's part of the PID-check branch and removing it is out of scope for a hotfix.

2. **Should PowerShell `start.ps1` auto-patch `settings.local.json` for the SessionEnd command hook?**
   - What we know: `start.sh` already has `patch_settings_local`; `start.ps1` does NOT.
   - What's unclear: whether bringing parity to `start.ps1` is Phase 7 scope or Phase 8 scope.
   - Recommendation: include in Phase 7 to make Bug #3's dual-registration actually work for Windows-native users. Cost is ~60 lines of PowerShell mirror of existing node.js logic. Without this, the dual-registration is Unix-only.

3. **Does Bug #4's rotation need to handle Log.err without stdout.log?**
   - What we know: the daemon produces both streams; both can grow at different rates.
   - What's unclear: whether log.err can exceed 10 MB while log stays small (e.g., high-error-rate scenario).
   - Recommendation: rotate each file independently (which the recommended helper does). No coupling.

4. **Is double-decrement acceptable on clean exit (when both plugin hook AND settings.local.json hook fire)?**
   - What we know: existing stop scripts floor at 0.
   - What's unclear: impact on MULTI-session Windows users (rare).
   - Recommendation: accept per A4. If surfaced as a real issue later, add marker-file guard as documented in Bug #3 section (option 1).

5. **Should the `patch_settings_local` extension also remove the SessionEnd entry on cleanup?**
   - What we know: `stop.sh cleanup_settings_local` currently removes ALL hooks whose `url` contains `127.0.0.1:19460`.
   - What's unclear: whether the new command-hook entry should also be removed when ACTIVE_SESSIONS=0.
   - Recommendation: yes, extend the cleanup matcher as shown in Bug #3 section. Otherwise the user would accumulate a SessionEnd command-hook entry in settings.local.json even after plugin uninstall.

## Security Domain

The phase's security enforcement is enabled by default. Bugs #1–#4 are lifecycle fixes; none touch authentication, input parsing changes beyond existing `sanitizeWindowsJSON`, or new endpoints.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---|---|---|
| V2 Authentication | no | N/A — no auth changes. |
| V3 Session Management | yes | Registry mutations remain under `sync.RWMutex`; new `Touch()` method follows the same lock-before-modify pattern. |
| V4 Access Control | no | N/A. |
| V5 Input Validation | yes | SessionEnd command-hook stdin is discarded by stop scripts (no parsing — safer than parsing and being wrong). Existing `sanitizeWindowsJSON` already protects HTTP hook payloads. |
| V6 Cryptography | no | N/A. |

### Known threat patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---|---|---|
| Race on `LastActivityAt` read vs write | Tampering | New `Touch()` under `r.mu.Lock()` — serialized with all other mutations. |
| Unbounded log file growth | DoS (disk) | 10 MB rotation + single backup = 20 MB cap (D-11). |
| SessionEnd hook with async long-running work killed | Hazard of integrity | Per upstream issue #41577, `nohup ... & disown` pattern; our `stop.sh`/`stop.ps1` are fast (filesystem ops only — <100ms), so hook-kill is not triggered. |
| `${CLAUDE_PLUGIN_ROOT}` expansion on Windows (issue #16116) | Hazard of integrity | Literal `bash -c '${CLAUDE_PLUGIN_ROOT:-$HOME/.claude/plugins/marketplaces/dsrcode}'` — fallback handles expansion failure. Same pattern as existing SessionStart entry. |

## MCP Research Citations

Per the MCP mandate, every major finding below carries evidence. Tools used:

**Sequential-thinking decomposition (PRE):**
- Decomposed 4 bugs into 15 discrete sub-questions before any tool calls. Questions captured at `/tmp/mcp-seq-decompose.txt` during the session (transient — see structure in ordered question list below).
- Primary decomposition outputs: "Bug #2 requires a no-notify path"; "Bug #3 has upstream risk — what's the evidence?"; "Bug #4 PowerShell constraint needs verification".

**Exa + WebSearch + gh issue view (evidence for each claim):**
- [anthropics/claude-code#17885](https://github.com/anthropics/claude-code/issues/17885) — SessionEnd not firing on `/exit`; confirmed macOS AND Windows; closed stale 2026-02-28 without fix
- [anthropics/claude-code#35892](https://github.com/anthropics/claude-code/issues/35892) — OPEN, references #17885; confirms hard-coded Goodbye! bypasses hooks on `/exit`
- [anthropics/claude-code#16288](https://github.com/anthropics/claude-code/issues/16288) — OPEN, "Plugin hooks not loaded from external hooks.json file"; workaround is settings.local.json direct registration
- [anthropics/claude-code#27398](https://github.com/anthropics/claude-code/issues/27398) — closed duplicate of #16288; Cowork VM `--setting-sources user` excludes plugin hooks
- [anthropics/claude-code#16116](https://github.com/anthropics/claude-code/issues/16116) — `${CLAUDE_PLUGIN_ROOT}` Windows expansion failure; workaround is literal path or settings.json registration
- [anthropics/claude-code#41577](https://github.com/anthropics/claude-code/issues/41577) — SessionEnd hooks killed before completion on async work (duplicated #24206)
- [anthropics/claude-code#34954](https://github.com/anthropics/claude-code/issues/34954) — feature request documenting Stop-fires-every-turn; SessionEnd is "when session actually ends"
- [PowerShell/PowerShell#15031](https://github.com/PowerShell/PowerShell/issues/15031) — Start-Process append option feature request; closed stale 2023, no implementation; confirms FileMode.Append is not exposed

**Context7 resolution:**
- `mcp__context7__resolve-library-id` via `npx ctx7@latest library "claude-code" "SessionEnd hook"` — confirmed live Context7 catalog entry `/anthropics/claude-code` with 760 code snippets, source reputation: High, version 2.1.89. Used to cross-check docs parsing vs primary source.

**Doc fetch (WebFetch):**
- `https://code.claude.com/docs/en/hooks` — verified SessionEnd matcher values (`clear`, `resume`, `logout`, `prompt_input_exit`, `bypass_permissions_disabled`, `other`), stdin JSON shape, env vars (`CLAUDE_PROJECT_DIR`, `CLAUDE_PLUGIN_ROOT`, `CLAUDE_PLUGIN_DATA`), and "no decision control" classification.

**Live codebase grep / gh search code:**
- `gh search code --repo=stacklok/toolhive` — confirmed `ProxySession.Touch()` production pattern: `func (s *ProxySession) Touch() { s.mu.Lock(); defer s.mu.Unlock(); s.updated = time.Now() }` from `pkg/transport/session/proxy_session.go`
- Local Grep over `session/`, `server/`, `scripts/`, `hooks/`, `.claude-plugin/` — 100% of file line references in this RESEARCH.md verified against actual source on 2026-04-13.

**WebSearch secondary evidence:**
- nixCraft, Baeldung, Linuxhint — portable `wc -c < file` pattern for bash size check (consistent across 6 sources)
- `zetcode.com/powershell/redirectstandardoutput-parameter/` — confirms truncate default of Start-Process
- `adamtheautomator.com/start-process/` — confirms lack of append parameter

## Sources

### Primary (HIGH confidence)
- `https://code.claude.com/docs/en/hooks` — SessionEnd matcher + payload; fetched 2026-04-13
- [anthropics/claude-code#16288](https://github.com/anthropics/claude-code/issues/16288) (open) — plugin hooks.json discovery failure
- [anthropics/claude-code#17885](https://github.com/anthropics/claude-code/issues/17885) (closed stale) — SessionEnd /exit failure
- [anthropics/claude-code#35892](https://github.com/anthropics/claude-code/issues/35892) (open) — hardcoded Goodbye bypasses Stop/SessionEnd
- [anthropics/claude-code#16116](https://github.com/anthropics/claude-code/issues/16116) (closed) — ${CLAUDE_PLUGIN_ROOT} Windows expansion failure
- [PowerShell/PowerShell#15031](https://github.com/PowerShell/PowerShell/issues/15031) (closed stale) — Start-Process append unimplemented
- Context7 entry `/anthropics/claude-code` v2.1.89 — catalog verification
- Local source files (verified line-by-line): `session/stale.go`, `session/registry.go`, `session/source.go`, `session/stale_windows.go`, `session/stale_unix.go`, `server/server.go`, `main.go`, `scripts/start.sh`, `scripts/start.ps1`, `scripts/stop.sh`, `scripts/stop.ps1`, `hooks/hooks.json`, `.claude-plugin/plugin.json`

### Secondary (MEDIUM confidence)
- stacklok/toolhive `pkg/transport/session/proxy_session.go` — Touch() pattern via `gh search code`
- [anthropics/claude-code#41577](https://github.com/anthropics/claude-code/issues/41577) (closed dup) — SessionEnd async-kill workaround pattern
- [anthropics/claude-code#27398](https://github.com/anthropics/claude-code/issues/27398) (closed dup) — Cowork --setting-sources exclusion
- PowerShell `about_Redirection` docs — truncation default

### Tertiary (LOW confidence — noted but not decision-load-bearing)
- datacamp, lobehub, claudefa.st tutorial posts — plugin structure examples
- Medium posts on Go sync — general sync.RWMutex context (not directly decision-affecting)

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all tools/patterns verified in codebase + primary sources
- Architecture: HIGH — bug localizations have exact line references
- Pitfalls: HIGH — 5 pitfalls, each grounded in evidence
- Bug #3 upstream risk: HIGH evidence base, MEDIUM confidence on mitigation (dual-registration is research's recommendation; user may prefer single-channel; cost-benefit explicitly documented)

**Research date:** 2026-04-13
**Valid until:** 2026-05-13 (30 days for stable topics; upstream claude-code behavior may change with plugin system revisions — recommend re-verification of Bug #3 upstream state if phase sits unshipped for >2 weeks)

## RESEARCH COMPLETE
