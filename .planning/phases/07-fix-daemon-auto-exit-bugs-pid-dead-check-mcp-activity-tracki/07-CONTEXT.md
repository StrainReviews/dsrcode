# Phase 7: Fix daemon auto-exit bugs - Context

**Gathered:** 2026-04-13
**Status:** Ready for planning
**Trigger:** Live incident during MCP-heavy session — daemon self-exited despite active Claude Code session; `~/.claude/dsrcode.refcount` drifted to 20

<domain>
## Phase Boundary

Fix four concrete daemon lifecycle bugs that cause dsrcode.exe to self-terminate while a Claude Code session is still active, and to leak state (refcount, logs) across restarts. No new features, no new UI surfaces, no new hook types. Pure root-cause code and hook-config fixes.

**In scope (the 4 bugs):**
1. PID-liveness-check fires falsely on HTTP-sourced sessions (`session/stale.go`)
2. `handlePostToolUse` never updates `LastActivityAt`, causing stale-detection to kill active sessions (`server/server.go`)
3. Refcount in `~/.claude/dsrcode.refcount` is never decremented because no SessionEnd-command-hook calls `stop.sh/ps1` (`hooks/hooks.json`)
4. `start.sh` / `start.ps1` truncate `dsrcode.log` on every launch, destroying crash history

**Out of scope (new capabilities — deferred):**
- Monitoring/Observability (Prometheus `/metrics` endpoint, structured logs)
- Refcount replacement with internal-registry-only Source-of-Truth
- X-Claude-PID header enforcement in Claude Code itself
- GUI or status dashboard improvements

</domain>

<decisions>
## Implementation Decisions

### Bug #1 — PID-Liveness-Check for HTTP-Sessions
- **D-01:** Skip PID-liveness-check entirely for sessions with `Source == HTTP` in `session/stale.go`. The code comment at line 40 already recommends this; the 2-minute grace heuristic that exists today is too aggressive on Windows where `os.Getppid()` returns the short-lived `start.sh` bash process PID.
- **D-02:** Preserve PID-check behavior for PID-sourced sessions on Unix — the liveness signal is still meaningful there (macOS/Linux `signal(0)` is reliable, Phase 3 decision).
- **D-03:** No config toggle — the skip is unconditional when `Source == HTTP` because the check is not trustworthy for that source. Adding a knob is complexity without benefit.

### Bug #2 — `handlePostToolUse` LastActivityAt Update
- **D-04:** `handlePostToolUse` in `server/server.go` MUST update `LastActivityAt` on every call. Current code only calls `UpdateTranscriptPath` / `RecordTool` — it completely bypasses the registry's activity clock.
- **D-05:** Update only `LastActivityAt` — do NOT set/overwrite `SmallImageKey`, `SmallText`, `Details`. The user wants zero UI side-effects from PostToolUse. Planner's call on how to achieve this without side-effects (either minimal `ActivityRequest` flag, new `registry.Touch(sessionID)` method, or guarded `UpdateActivity` path — any of these satisfies D-05).
- **D-06:** `handlePostToolUse` fires on every tool call (incl. MCPs) because `settings.local.json` already has `matcher: "*"` on PostToolUse. The fix is entirely server-side; no hook-config change needed for Bug #2.

### Bug #3 — Refcount Drift via Missing SessionEnd-Command-Hook
- **D-07:** Register a new `SessionEnd` entry in the plugin's `hooks/hooks.json` (NOT in user-level `settings.local.json`). It runs `scripts/stop.sh` / `scripts/stop.ps1` as a `type: "command"` hook, mirroring the existing `SessionStart` pattern.
- **D-08:** Keep the existing HTTP `/hooks/session-end` handler untouched — it handles registry cleanup and auto-exit grace period. The new command hook is purely for refcount decrement (Windows) and PID-tracking cleanup (Unix).
- **D-09:** Versioned in the Plugin repo. Avoids user-specific drift in `settings.local.json`. Matches how Phase 6.02 already splits plugin-hooks vs user-settings.

### Bug #4 — Log Overwrite on Daemon Start
- **D-10:** `start.sh` / `start.ps1` MUST append to `dsrcode.log` rather than truncate. Implementation detail delegated to planner (bash `>>` redirection vs PowerShell `-Append`).
- **D-11:** Implement size-cap rotation at 10 MB: `dsrcode.log` → `dsrcode.log.1` (single backup, overwrite). No date-based rotation, no multi-generation retention — keep it simple but bounded.
- **D-12:** Same pattern on `dsrcode.log.err` (stderr).

### Scope & Release
- **D-13:** Cross-platform (all 4 bugs affect all platforms; Bug #1 most visible on Windows, Bug #2 affects every platform, Bug #3 Windows-specific, Bug #4 all).
- **D-14:** Target release tag: `v4.1.2` (hotfix) — NOT `v4.2.0` (no new features).
- **D-15:** Plan count estimate: 4-6 plans (one per bug + release/CHANGELOG plan). Planner will break down.

### Claude's Discretion
- `registry.Touch()` vs extended `UpdateActivity` — planner picks the idiom closest to existing code.
- PID-Source enum handling — planner decides whether `Source == HTTP` is best expressed as direct equality, bitfield, or helper method based on `session/source.go` inspection.
- Log rotation implementation — bash/PowerShell idiom is up to planner (native cmdlets preferred over external `logrotate`).
- Test-coverage split — unit tests for stale.go + registry.Touch; integration tests for SessionEnd command hook; manual verification for log rotation. Planner finalizes.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Core code files (the bugs live here)
- `session/stale.go` — lines 41-48 house the faulty PID-Dead check; comment at line 40 already prescribes the fix
- `server/server.go` §handlePostToolUse (around line 563-647) — missing `registry.UpdateActivity` / missing `LastActivityAt` update
- `server/server.go` §handleHook (lines 351-472) — reference pattern for how UpdateActivity is already wired
- `session/registry.go` §UpdateActivity (lines 140-185) — sets `LastActivityAt = time.Now()` at line 179
- `session/source.go` — Source enum definition (HTTP vs PID vs JSONL ranks)
- `session/stale_windows.go` + `session/stale_unix.go` — IsPidAlive implementations

### Hook config
- `hooks/hooks.json` (plugin) — add SessionEnd command entry next to existing SessionStart
- `scripts/start.sh` + `scripts/start.ps1` — log redirection at `-RedirectStandardOutput` / `> $LogFile`
- `scripts/stop.sh` + `scripts/stop.ps1` — existing refcount decrement logic that isn't being triggered

### Prior phase contracts (carry forward)
- Phase 3 D-ID (STATE.md): Windows IsPidAlive uses tasklist; PID-based tracking (macOS/Linux) + refcount (Windows)
- Phase 6 D-04: Dual-trigger auto-exit (SessionEnd + stale detection) — Bug #1 fix must preserve this
- Phase 6 D-05: 30s default grace period, 0=disabled — do not change default
- Phase 6.02 D-11: 13 HTTP hooks live in settings.local.json, SessionStart/SessionEnd stay in plugin hooks.json — D-07 follows this split
- Phase 1 D-41/D-42: X-Claude-PID header + os.Getppid() fallback — D-01 accepts that the fallback is unreliable for HTTP sessions and handles it

### Specs
- `.planning/REQUIREMENTS.md` — canonical D-01..D-56 IDs already assigned; new D-IDs for this phase must not collide
- `.planning/ROADMAP.md` — Phase 7 entry (Goal TBD, will be filled by plan-phase)
- `CHANGELOG.md` — v4.1.2 release entry pattern (copy from v4.1.0 structure)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `registry.UpdateActivity()` — already updates `LastActivityAt` correctly; D-04 fix either calls it with a minimal ActivityRequest or a new `Touch()` sibling method follows the same immutable-copy pattern.
- `sourceFromID()` in `session/source.go` — already infers source from session-ID prefix. D-01 fix reuses this for the PID-check skip.
- `IsPidAlive()` split across `stale_windows.go` and `stale_unix.go` — D-01 fix does not touch these, just skips calling them when Source=HTTP.
- `notifyChange()` callback pattern in registry.go — D-05 requires NOT firing presence-update for Touch; the existing UpdateActivity always calls notifyChange, so Touch needs a non-notifying variant OR a conditional.

### Established Patterns
- Immutable copy-before-modify in registry mutation (registry.go:149). Touch must follow this even though it only changes one field.
- Background goroutines with `defer recover()` in server.go hook handlers (D-09 Phase 6 pattern). Any new code in handlePostToolUse follows this.
- Plugin hooks.json pattern: `type: command` for lifecycle (SessionStart), `type: http` for activity events. D-07 follows the SessionStart command pattern exactly.
- Windows-specific refcount logic in start.ps1/stop.ps1 with ASCII NoNewline file encoding.

### Integration Points
- D-01 fix is local to `session/stale.go` `CheckOnce` — no API change, no registry change.
- D-04 fix is local to `server/server.go` `handlePostToolUse` + maybe `session/registry.go` Touch method — no external API change.
- D-07 new hook: plugin hooks.json + (on Windows) stop.ps1 is already the right script — no script changes needed for the command hook, just the registration.
- D-10/11 log rotation: local to scripts/, no binary changes.

### Tests affected
- `session/registry_test.go`, `session/stale_*_test.go` — add cases for HTTP-source skip and Touch
- `server/server_test.go` — add case verifying handlePostToolUse updates LastActivityAt (regression guard)
- Manual verification script (`scripts/phase-07/verify.ps1`?) to reproduce the 30-min MCP idle scenario

</code_context>

<specifics>
## Specific Ideas

- **Reproduction scenario the fix must pass:** Start Claude Code, trigger a chain of MCP tool calls lasting >2 minutes without any Edit/Write/Bash/Read (this session is the canonical reproduction). Daemon must remain up, session must remain in registry, refcount must remain 1.
- **Log rotation naming:** Match Windows and Unix to the same convention — `dsrcode.log` + `dsrcode.log.1` (period-numbered, not `.old`, not `.bak`).
- **Release note angle:** Emphasize "fixes daemon self-termination during long MCP sessions" — it's user-visible, matches the reported symptom, credits the live-incident source.

</specifics>

<deferred>
## Deferred Ideas

- **Prometheus /metrics endpoint + structured observability** — surfaced during Scope question (Option D). Deferred to a future observability phase. The fixes in Phase 7 make the daemon reliable, observability is a separate capability.
- **Refcount abolition (Source-of-Truth = registry only)** — architecturally cleaner, but larger blast radius; would require changes across start.sh, stop.sh, start.ps1, stop.ps1, setup-statusline.sh, and the dsrcode-status skill. Revisit after Phase 7 stabilizes.
- **X-Claude-PID enforcement** — requires Claude Code CLI change; not in our repo's control. File upstream when/if the fallback stops being sufficient even after D-01.
- **Heartbeat-based liveness-probe architecture** — Exa research surfaced Kubernetes-style heartbeat patterns. Not needed now; current fix restores correctness. Revisit if daemon stalls recur after Phase 7.
- **Config-driven stale timeouts per-source** — could let HTTP sessions have longer timeouts than PID sessions. Not needed; D-01 skip already removes the false-positive, and existing `removeTimeout=30m` remains the backstop.

### Reviewed Todos (not folded)
- (No pending todos matched Phase 7 scope at the time of discuss-phase.)

</deferred>

<success_hints>
## Hints for Researcher & Planner

- **Do NOT touch** auto-exit grace period, SessionEnd HTTP handler, analytics sync — those are correct and out of scope.
- **Do NOT introduce** new config fields beyond what's strictly needed. D-01 is unconditional, D-04 is unconditional, D-07 is a hook entry, D-10/11 are script constants.
- **Do verify** by running the MCP-heavy reproduction scenario end-to-end after each plan completes.
- **Plan count sanity check:** if you produce more than 6 plans, you're over-decomposing. If fewer than 3, you're bundling too much.
- **Release target:** v4.1.2 — bump_version.sh takes care of the 5-file version propagation.

</success_hints>
