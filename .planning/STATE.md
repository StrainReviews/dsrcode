---
gsd_state_version: 1.0
milestone: v4.0.0
milestone_name: milestone
status: Ready to execute
last_updated: "2026-04-18T14:30:00.000Z"
progress:
  total_phases: 10
  completed_phases: 9
  total_plans: 66
  completed_plans: 58
  percent: 90
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-04-08)
**Core value:** Real-time session visualization on Discord with personality-driven status messages
**Current focus:** Phase 09 COMPLETE — v4.2.1 hotfix (stale-session false-positive for UUID-sourced Claude sessions)

## Current Position

Phase: 09 — COMPLETE (user approved 2026-04-18 14:26)
Plan: Both plans complete (09-01 core hotfix, 09-02 release harness)
Next: Phase 6.1 (project folder rename + Claude memory migration) — still deferred, run `/gsd-plan-phase 6.1` in a separate handoff session
Also pending: User manual release follow-ups per CLAUDE.md §Releasing:
  - `git tag v4.2.0 && git push origin main --tags` (from Phase 8)
  - `git tag v4.2.1 && git push origin main --tags` (from Phase 9)
  NOT Claude's step per 3-tag-push-limit memory.

## Last Session

- Date: 2026-04-18
- Stopped at: Phase 9 COMPLETE. Wave 1 (09-01) shipped the core hotfix: `TestStaleCheckPreservesPidCheckForPidSource` renamed + inverted to `TestStaleCheckSkipsPidCheckForClaudeSource` (RED commit `3682a90`), guard at `session/stale.go:41` expanded from `s.Source != SourceHTTP` to `s.Source != SourceHTTP && s.Source != SourceClaude` with 15-line D-08 godoc replacing the 3-line comment (GREEN commit `9f05fff`, net +13 lines), three regression tests added: `TestStaleCheckRemovesClaudeSourceAfterRemoveTimeout` (30min backstop), `TestStaleCheckSurvivesUuidSourcedLongRunningAgent` (live-incident mirror — session `2fe1b32a-ea1d-464f-8cac-375a4fe709c9`, PID 5692, 150s elapsed), and `TestStaleCheckEmitsDebugOnGuardSkip` (NEGATIVE slog assertion — inner Debug line MUST NOT fire for SourceClaude because it lives inside the now-skipped guard block) in commit `78b963d`. Wave 1 SUMMARY at `d2f8b09`. Wave 2 (09-02) shipped the v4.2.1 release harness: `./scripts/bump-version.sh 4.2.1` propagated the version across 5 canonical files (commit `535f035`), CHANGELOG [4.2.1] section inserted between [Unreleased] and [4.2.0] with live-incident forensic anchor (session UUID + PID 5692 + 2m25s + 8x debug lines + Phase 9 D-01/D-05/D-06/D-08 citations) in commit `847c965`, `scripts/phase-09/verify.sh` (bash T1-T3 harness, mode 0755, 106 lines) in commit `7325629`, `scripts/phase-09/verify.ps1` (PowerShell parity, 163 lines, verify-results.json export) in commit `2c2719d`. Task 5 live verify: rebuilt v4.2.1 binary, stopped old daemon (PID 43716), replaced binary in `~/.claude/plugins/data/dsrcode/bin/`, restarted via `start.sh`, ran `DSRCODE_BIN=... bash scripts/phase-09/verify.sh` — all T1/T2/T3 PASS (`dsrcode --version contains 4.2.1` + `/health HTTP 200` + 0 `removing stale session` lines for the injected `verify09-*` UUID across the 150s sweep window). Task 6 human-verify checkpoint: user typed "approved" at 14:26 after reviewing <what-built> / <how-to-verify>. Claude did NOT execute `git tag v4.2.1` or `git push origin main --tags` — confirmed via `git tag --list 'v4.2.1'` returning empty. PRE+POST 4-MCP rounds documented for all 10 tasks in `09-SUMMARY.md §MCP-Rounds` (sequential-thinking + context7 + exa + crawl4ai per task; crawl4ai documented as skipped/Python-only per mandate allowance for each task). No new deferred items surfaced; carry-over (HTTP bind-retry race, PID-recycling via starttime, Option-A classification refactor, refcount replacement) unchanged.
- Resume: User's independent follow-up is the manual tag + push for both v4.2.0 (still pending from Phase 8) AND v4.2.1 (new in Phase 9) per CLAUDE.md §Releasing. GoReleaser CI builds the 5-platform binaries + GitHub Release on each tag push. Next planning session is Phase 6.1 (project folder rename + Claude Code memory migration). Run `/gsd-plan-phase 6.1` to break it down.
- Next: Phase 6.1 planning session via `/gsd-plan-phase 6.1`. No other blockers; Phase 8 + Phase 9 release artefacts ready for user's manual tags + push.

## Decisions

- [Phase 1]: HTTP hooks over command hooks (bash+curl) for 10x faster activity tracking
- [Phase 1]: go:embed presets/*.json for single-binary distribution per D-24
- [Phase 1]: JSON format for presets enables editing without recompilation
- [Phase 1]: Each preset 200+ messages across all categories for rich rotation
- [Phase 1]: HashString for StablePick seed instead of timestamp (sessionID-based determinism)
- [Phase 1]: Channel-based debounce pattern (non-blocking send to buffered channel of 1)
- [Phase 1]: Synthetic session IDs (jsonl- prefix) for JSONL fallback sessions
- [Phase 1]: Discord rate limit (15s) enforced in debouncer goroutine, not in discord client
- [Phase 1]: External test packages for black-box testing across all packages
- [Phase 1]: Buttons field uses json:"-" tag with custom map serialization
- [Phase 1]: Config watcher watches directory not file for atomic-save editor compatibility
- [Phase 1]: X-Claude-PID header with os.Getppid() fallback for PID sourcing
- [Phase 2]: DisplayDetail enum with ParseDisplayDetail fallback to minimal for unknown values
- [Phase 2]: ExtractToolContext exported for external test package access
- [Phase 2]: Synthetic session_id uses http- prefix to distinguish from real UUIDs
- [Phase 2]: Preview duration clamped 5-300s with 60s default
- [Phase 2]: HTTP hooks with 1s timeout replace command hooks for 10x faster tracking
- [Phase 2]: Notification hook with idle_prompt matcher for idle state detection
- [Phase 2]: start.sh auto-update kills existing daemon before binary replacement
- [Phase 2]: atomic.Bool for discordConnected state
- [Phase 2]: cfgMu sync.RWMutex protects cfg reads in configGetter and displayDetailGetter closures
- [Phase 3]: Windows IsPidAlive uses tasklist command (os.FindProcess always succeeds on Windows)
- [Phase 3]: PID-based tracking (macOS/Linux) + refcount (Windows) for session count accuracy
- [Phase 3]: SetLastActivityForTest helper on registry for deterministic stale tests
- [Phase 4]: External test packages for all 8 analytics files
- [Phase 4]: json.RawMessage for Session struct analytics fields to avoid circular imports
- [Phase 4]: Spawn() preserves pre-set Status to support completed agents in SubagentTree
- [Phase 4]: Persist errors silently discarded: analytics loss is cosmetic, never blocks hooks
- [Phase 4]: SubagentStop route registered before wildcard /hooks/{hookType} for routing precedence
- [Phase 4]: SetTracker method for optional analytics injection without breaking constructor
- [Phase 4]: LoadPreset delegates to LoadPresetWithLang(name, en) for backward compat
- [Phase 4]: BilingualMessagePreset uses map[string]*MessagePreset for language selection
- [Phase 4]: ParseMessageFileBytes for embedded FS; parse errors logged as warnings
- [Phase 6.01]: ParseTranscript empty path returns zero result (not error) so hook handlers can pass through unset transcript_path without branching
- [Phase 6.01]: transcriptMessage struct kept private to analytics — main.go JSONLMessage will be removed in Plan 06-04, no shared type to maintain
- [Phase 6.01]: scanner.Err() check after Scan loop is mandatory to catch ErrTooLong and partial reads (golang/go#26431, github/gh-aw#20028)
- [Phase 6.01]: ShutdownGracePeriod hot-reload requires no watcher.go change — Defaults+applyFileConfig+applyEnvVars run on every reload
- [Phase 6.01]: Zero ShutdownGracePeriod is the disabled sentinel for auto-exit goroutine in Plan 06-04
- [Phase 6.01]: AllActivityIcons returns 8 icons but "error" has no preset messages — preset_test.go skips it in pool-iteration tests (D-19 status overlay)
- [Phase 6.02]: 13 HTTP hooks registered in ~/.claude/settings.local.json via start.sh auto-patch; SessionStart remains in plugin hooks.json, SessionEnd dual-registered per D-13
- [Phase 6.02]: URL-based idempotency (127.0.0.1:19460) is the canonical ownership marker for dsrcode hooks in settings.local.json
- [Phase 6.02]: node -e chosen over jq for settings.local.json manipulation to match existing patch_hooks_json pattern in start.sh
- [Phase 6.02]: cleanup_settings_local runs only when ACTIVE_SESSIONS=0 (single call at common post-session-counting point in stop.sh, not duplicated in Windows/Unix branches)
- [Phase 6.02]: Object.keys() snapshot pattern required for delete-during-iteration safety in cleanup function
- [Phase 6.02]: Timeout unit for HTTP hooks is SECONDS (not ms), confirmed via 4 sources including anthropics/claude-code#19175
- [Phase 6.03]: All 8 new hook handlers follow D-09 pattern — HTTP 200 <10ms with expensive work deferred to panic-recovered background goroutines (defer recover() in all 3 goroutines)
- [Phase 6.03]: sync.Map-backed per-session 10-second throttle for PostToolUse-triggered JSONL reads (lastTranscriptRead) bounds file I/O without per-event ticker overhead
- [Phase 6.03]: handleCwdChanged partial D-21 — only Details field updated in Plan 06-03; full Cwd/ProjectName/ProjectPath swap needs Registry.UpdateProjectContext (deferred to Phase 6.1 or future phase)
- [Phase 6.03]: 24 server tests with 80+ sub-tests back all 8 new routes; malformed-JSON D-16 contract exhaustively verified with 40 sub-tests (8 routes x 5 bodies)
- [Phase 6.04]: abortFn closure pattern required for go vet lostcancel check on conditional WithCancel inside for-select loop (plan literal failed vet, refactored per canonical context docs)
- [Phase 6.04]: Cancel BEFORE Stop ordering in auto-exit timer abort — inverting would open race window where AfterFunc callback fires after new session arrives
- [Phase 6.04]: CancelFunc idempotency used for multi-path shutdown (sigChan, auto-exit timer, srv.Start error) — per Go stdlib guarantee, no coordination needed
- [Phase 6.04]: Reuse server.Server.Start existing 5s Shutdown drain rather than duplicating in main.go — D-07 sequence just calls cancel() and lets Start handle HTTP drain
- [Phase 6.04]: Discord Activity cleared BEFORE IPC Close (D-07) — order is load-bearing for clean Discord status disappearance
- [Phase 6.05]: SetOnAnalyticsSync setter chosen over NewServer param for consistency with SetOnAutoExit (Plan 06-04) and to avoid breaking 24 existing server tests
- [Phase 6.05]: Sync call placed BEFORE EndSession in handleSessionEnd so resolver gets one last analytics flush before session removal — order is load-bearing
- [Phase 6.05]: CHANGELOG v4.1.0 co-shipped with feature commit (not deferred to release commit) — avoids changelog-drift antipattern
- [Phase 6.05]: All 3 nil-guarded onAnalyticsSync call sites — tests pass without wiring the setter (nil-guard preserves test-mode behavior)
- [Phase 6]: Phase 6 COMPLETE — 14 commits, 5 plans, 15 hook events deployed (13 settings.local.json + 2 plugin), ~950 net LOC added, ~768 LOC JSONL removed, 100+ new tests, MCP-Mandate compliance (PRE+POST 4-MCP rounds per task = ~77 MCP calls across the phase), v4.1.0 CHANGELOG shipped and ready for git tag.
- [Phase 07]: D-04/D-05 Phase 7: registry.Touch() refreshes LastActivityAt without firing notifyChange; wired into handlePostToolUse for MCP activity tracking
- [Phase 07]: D-10/D-11/D-12: 10MB single-backup log rotation via rotate_log/Rotate-Log; Unix append+split redirect; start.ps1 stderr same-path defect fixed
- [Phase 7]: v4.1.2 hotfix release: 4 daemon-auto-exit bugs fixed (PID-source skip, MCP activity tracking, SessionEnd command hook + dual-register, log rotation). Tag/push deferred to user per CLAUDE.md \u00a7Releasing.
- [Phase 08]: Phase 8 Plan 01: synctest + rate.Limiter interop confirmed (probe PASSED); no ClockFunc fallback needed
- [Phase 08]: Phase 8 Plan 01: coalescer/ package owns rate.Limiter; single-goroutine Run enforces T-08-01-01 mitigation (no multi-goroutine Reserve race)
- [Phase 08]: Phase 8 Plan 01: -race runs in CI only (Ubuntu+CGO); local Windows dev has no gcc — not a regression, tests still green via atomic grep checks
- [Phase 08]: Plan 08-02: HashActivity uses 0x1F ASCII Unit Separator (RESEARCH D3), not \x00 — 0x1F is reserved for field delimiters and cannot break terminal/log handling
- [Phase 08]: Plan 08-02: Hash gate placed BEFORE pending.Store; hash store placed AFTER successful SetActivity only (T-08-02-05 mitigation: IPC failure does not poison lastSentHash cache)
- [Phase 08]: Plan 08-02: StartTime exclusion enforced structurally — HashActivity never references a.StartTime in code (comments only), preventing silent regression via branch flips
- [Phase 08]: Plan 08-03: Hook-dedup counter lives in server package (HookDedupMiddleware.deduped atomic.Int64); Coalescer reads via injected func() int64 — one-way server→coalescer import, no cycle (discrepancy D4)
- [Phase 08]: Plan 08-03: Dedup separator = 0x1F (mirrors coalescer/hash.go per discrepancy D3); 64 KiB body cap via http.MaxBytesReader closes unbounded io.ReadAll vector; fail-open on read errors (partial reads bypass dedup so legitimate retries are never masked)
- [Phase 08]: Plan 08-03: main.go step ordering swapped — srv := server.NewServer(...) constructed BEFORE presenceCoalescer := coalescer.New(..., srv.HookDedupedCount). Shutdown sequence unchanged (presenceCoalescer.Shutdown() BEFORE discord clear)
- [Phase 08]: Plan 08-04: Release v4.2.0 prep shipped — bump-version.sh propagated 4.1.2→4.2.0 across 5 canonical files, CHANGELOG [4.2.0] - 2026-04-16 with Fixed/Changed/Added subsections + 9 RLC-NN citations, scripts/phase-08/verify.sh (mode 100755) + verify.ps1 T1-T6 smoke harnesses cross-platform. User approved human-verify checkpoint; git tag+push remains user's exclusive follow-up per CLAUDE.md §Releasing (3-tag-push-limit memory).
- [Phase 8]: Phase 8 COMPLETE — 14+ task commits across 4 plans, v4.2.0 ready for user tag+push. MCP-compliance per user mandate (PRE+POST 4-MCP rounds per task throughout). 17 RLC requirements mapped to commits. synctest+rate.Limiter interop confirmed (RLC-15 probe PASSED, no ClockFunc fallback). 0x1F ASCII Unit Separator convention (D-03/RESEARCH-D3) applied consistently across coalescer/hash.go and server/hook_dedup.go. Dedup counter ownership in server package (D-04) with Coalescer reading via injected func() int64 getter — one-way server→coalescer import, no cycle. Zero production regressions in 11-package test suite. Tag+push remains user's exclusive follow-up per CLAUDE.md §Releasing.
- [Phase 9]: D-01 guard expansion — `s.Source != SourceClaude` inserted alongside existing `s.Source != SourceHTTP` at `session/stale.go:41` (1-symbol diff). Solves UUID-sourced SourceClaude sessions being wrongly removed on wrapper-PID death.
- [Phase 9]: D-02 no `runtime.GOOS` branch — source-based guard solves BOTH Windows (orphan non-reparenting) AND Unix (start.sh early exit). Single-codepath win.
- [Phase 9]: D-03 Phase-7 test inverted + renamed: `TestStaleCheckPreservesPidCheckForPidSource` → `TestStaleCheckSkipsPidCheckForClaudeSource`; assertion flipped from removal to survival. Sibling `TestStaleCheckSkipsPidCheckForHttpSource` kept byte-identical as Phase-7 invariance probe.
- [Phase 9]: D-04 `SetLastActivityForTest` helper used (Phase-7 style); NOT `testing/synctest` — CheckOnce is synchronous, virtual time has nothing to advance against.
- [Phase 9]: D-05 four regression tests (a/b/c/d): skip for SourceClaude, remove after removeTimeout (backstop), live-incident mirror (2fe1b32a + PID 5692 + 150s), NEGATIVE slog assertion proving the guard skips the ENTIRE block (not just EndSession). D-05(d) NEGATIVE form user-approved 2026-04-18 because positive form is semantically impossible post-D-01.
- [Phase 9]: D-07 slog.Debug line at stale.go:47 UNCHANGED — only its enclosing block is now skipped earlier for SourceClaude.
- [Phase 9]: D-08 15-line godoc at stale.go:38-52 verbatim from CONTEXT — explains both SourceHTTP + SourceClaude exclusions, Windows orphan-reparenting absence, Unix start.sh weakness, and removeTimeout backstop contract.
- [Phase 9]: D-09 CHANGELOG [4.2.1] cites live-incident forensic anchor (UUID `2fe1b32a-ea1d-464f-8cac-375a4fe709c9`, PID 5692, 8x `"PID dead but session has recent activity, skipping removal"` debug lines, 2m25s elapsed, 2026-04-18 11:37-11:44 timestamp range).
- [Phase 9]: Phase 9 COMPLETE — 8 commits across 2 plans (Wave 1: 4 commits for core hotfix, Wave 2: 4 commits for release harness). v4.2.1 artefacts ready for user tag+push. MCP-mandate full compliance (PRE+POST 4-MCP rounds across all 10 tasks, documented in 09-SUMMARY.md). Live verify harness T1/T2/T3 PASS against freshly rebuilt v4.2.1 daemon (150s wall-clock sweep, zero `removing stale session` lines for injected verify09 UUID). Full suite `go test -count=1 ./...` green across all 9 packages. Claude NEVER executed `git tag v4.2.1` or `git push origin main --tags` per CLAUDE.md §Releasing + 3-tag-push-limit memory.

## Accumulated Context

### Roadmap Evolution

- Phase 1: Discord Rich Presence + Activity Status Plugin Merge (migrated from StrainReviewsScanner Phase 13)
- Phase 2: DSRCodePresence Setup Wizard (migrated from StrainReviewsScanner Phase 15)
- Phase 3: Fix Discord Presence session count and enhance demo mode (migrated from StrainReviewsScanner Phase 16)
- Phase 4: Discord Presence Enhanced Analytics (migrated from StrainReviewsScanner Phase 17)
- Phase 5: Binary Distribution Pipeline + Full dsrcode Rename (complete, v4.0.0 shipped)
- Phase 6: Hook System Overhaul (COMPLETE 2026-04-10, 5 plans, 14 commits, v4.1.0 ready for tag)
- Phase 6.1 inserted after Phase 6: Project Folder Rename + Claude Code Memory Migration (next — deferred from Phase 5, user-requested 2026-04-10 to prevent dropping the task)
- Phase 7: REMOVED per DIST-01 (repo stays permanently at StrainReviews/dsrcode, no transfer)
- Phase 7 (new) added 2026-04-13: Fix daemon auto-exit bugs: PID-dead check, MCP activity tracking, refcount drift, log overwrite — triggered by live incident during MCP-heavy session (daemon self-exited despite active Claude Code session; refcount drifted to 20)
- Phase 8 added 2026-04-16: Presence Rate-Limit Coalescer — Stop Drop-on-Skip. Triggered by live log evidence showing ~70% "presence update skipped (rate limit)" rate during active MCP-heavy session. Root cause in main.go:504-555 presenceDebouncer: updates inside the 15s cooldown are silently DISCARDED (no pending buffer, no flush). Five fixes: (1) pending-state buffer + flusher goroutine, (2) golang.org/x/time/rate token bucket (4s cadence + burst 2, matches Discord RPC ~5/20s empirical limit), (3) FNV-64 content hash change detection, (4) hook-dedup middleware in server.go (logs show every pre-tool-use fires twice at 30–130ms spacing), (5) mutex on shared state (current lastUpdate is race-prone). Target v4.2.0.
- Phase 9 added 2026-04-18: Fix stale-session false-positive for UUID-sourced Claude sessions — daemon auto-exits during idle/long-running agent work. Triggered by live reproduction in v4.2.0 (`dsrcode.log` 2026-04-18 11:37–11:44): SessionID `2fe1b32a-…` (UUID, not `http-*`) → `sourceFromID` returns `SourceClaude` → Phase-7 Bug-#1 guard `s.Source != SourceHTTP` in `session/stale.go:41` does NOT skip, PID-liveness check removes the session 2m25s after last hook even though Claude Code is still active. Windows-structural: orphan wrapper PIDs never reparent, so `IsPidAlive(wrapperPID)` is permanently false. Target v4.2.1 hotfix.

## Blockers

(None)

### Quick Tasks Completed

| # | Description | Date | Commit | Directory |
|---|-------------|------|--------|-----------|
| 260411-iyf | Fix Windows daemon launch log redirect in scripts/start.sh | 2026-04-11 | dcdbb9a | [260411-iyf-fix-windows-daemon-launch-log-redirect-i](./quick/260411-iyf-fix-windows-daemon-launch-log-redirect-i/) |
| 260411-kcq | Fix golangci-lint v2 config migration in .golangci.yml | 2026-04-11 | 4e9c9dc | [260411-kcq-fix-golangci-lint-v2-config-migration-in](./quick/260411-kcq-fix-golangci-lint-v2-config-migration-in/) |
| 260411-kvy | Fix all 17 golangci-lint findings (11 CI + 6 research discoveries) | 2026-04-11 | 7ebf079 | [260411-kvy-fix-all-11-golangci-lint-findings-surfac](./quick/260411-kvy-fix-all-11-golangci-lint-findings-surfac/) |
| 260411-lua | Bump golangci-lint-action v7→v9 for Node.js 24 runtime | 2026-04-11 | 63cf336 | [260411-lua-bump-golangci-lint-action-v7-to-v9-for-n](./quick/260411-lua-bump-golangci-lint-action-v7-to-v9-for-n/) |
