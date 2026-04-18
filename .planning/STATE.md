---
gsd_state_version: 1.0
milestone: v4.0.0
milestone_name: milestone
status: Milestone complete
last_updated: "2026-04-18T10:34:35.053Z"
progress:
  total_phases: 10
  completed_phases: 8
  total_plans: 62
  completed_plans: 56
  percent: 90
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-04-08)
**Core value:** Real-time session visualization on Discord with personality-driven status messages
**Current focus:** Phase 08 — presence-rate-limit-coalescer-stop-drop-on-skip-token-bucket

## Current Position

Phase: 08
Plan: Not started
Next: Phase 6.1 (project folder rename + Claude memory migration) — planning deferred, run `/gsd-plan-phase 6.1` in a separate handoff session
Also pending: User manual release follow-up per CLAUDE.md §Releasing (git tag + push; NOT Claude's step per 3-tag-push-limit memory)

## Last Session

- Date: 2026-04-16
- Stopped at: Completed 08-04-PLAN.md — Release v4.2.0 prep. Ran `./scripts/bump-version.sh 4.2.0` propagating 4.1.2→4.2.0 across `main.go`, `.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json`, `scripts/start.sh`, `scripts/start.ps1`. Inserted `CHANGELOG.md ## [4.2.0] - 2026-04-16` section above `[4.1.2]` with 3 Fixed + 2 Changed + 3 Added bullets citing 9 RLC-NN IDs (RLC-01..RLC-16 excl. RLC-17 which IS the CHANGELOG entry). Created `scripts/phase-08/verify.sh` (mode 100755, 4161 bytes) T1-T6 bash smoke harness using `set -euo pipefail` + log-offset idiom via `wc -c`/`dd skip=$LOG_OFFSET` + PASS/FAIL console output. Created `scripts/phase-08/verify.ps1` (5668 bytes) PowerShell 5.1+ analog with `Invoke-Test` helper + `StreamReader`-seeked log offset + `verify-results.json` export. CI `-race` confirmed at `.github/workflows/test.yml:28` (RESEARCH D1 verification-only, no workflow edit). 3 task commits: `154f406` (chore: version bump + CHANGELOG), `7119a2e` (feat: verify.sh), `8383246` (feat: verify.ps1). Task 4 human-verify checkpoint: user typed "approved" after reviewing CHANGELOG entry + version propagation + verify scripts on disk; explicitly signed off on marking Phase 8 complete. Final docs commit wraps 08-04-SUMMARY.md + STATE.md + ROADMAP.md updates. Phase 8 CLOSED — 14+ task commits across 4 plans, 17 RLC requirements mapped, v4.2.0 release-ready.
- Resume: Next session is Phase 6.1 (project folder rename + Claude Code memory migration). Run `/gsd-plan-phase 6.1` to break it down. User's independent follow-up: `git tag v4.2.0 && git push origin main --tags` per CLAUDE.md §Releasing (GoReleaser CI builds the 5-platform binaries + GitHub Release on tag push).
- Next: Phase 6.1 planning session via `/gsd-plan-phase 6.1`. No other blockers; Phase 8 release artifacts ready for user's manual tag + push.

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
