# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [4.2.2] - 2026-04-18

### Fixed
- **Duplicate slash commands in Claude Code autocomplete** — each `dsrcode` command appeared twice in the `/` menu (once as `/dsrcode-<x>` and once as `/dsrcode:<x>`). The seven `_skills/*/SKILL.md` files had `name: dsrcode-<x>` in the frontmatter, which registered each skill both as a flat user-scope skill (`/dsrcode-<x>`) AND under the plugin namespace (`/dsrcode:<x>`). Frontmatter `name` is now the bare basename (`name: <x>`) matching the directory basename — per the official Claude Code plugins-reference docs and community convention (anthropics/claude-code#38398 warns that `name: plugin-foo` patterns are prefix-stripped in some runtime versions; #17271, #29675, #29520, #44871 track the resulting duplicate-registration behaviour). After this fix, each command appears exactly once as `/dsrcode:<x>` via the plugin namespace from `.claude-plugin/plugin.json`.

### Changed
- **Normalized SKILL.md frontmatter names across all seven skills** (`demo`, `doctor`, `log`, `preset`, `setup`, `status`, `update`) to use the bare basename instead of the `dsrcode-` prefix. User-facing invocation is now consistently `/dsrcode:status`, `/dsrcode:update`, etc. If you previously had the legacy `/dsrcode-<x>` commands cached locally, a `/plugin update dsrcode` + restart of Claude Code flushes them.

### Notes
- Legacy user-scope skill copies under `~/.claude/skills/dsrcode-*` may need a manual one-time cleanup on systems where an older installer deployed them flat. Delete `~/.claude/skills/dsrcode-demo`, `dsrcode-doctor`, `dsrcode-log`, `dsrcode-preset`, `dsrcode-setup`, `dsrcode-status`, `dsrcode-update` if present; the namespaced plugin skills under `~/.claude/plugins/cache/dsrcode/` remain the single source of truth.

## [4.2.1] - 2026-04-18

### Fixed
- **Daemon no longer auto-exits during long-running UUID-sourced Claude Code sessions** — `session/stale.go:41` guard now also skips the PID-liveness check when `Source == SourceClaude` (Phase 9, D-01). In production, UUID session IDs arrive via the HTTP hook path carrying the wrapper-launcher PID (`start.sh`/`start.ps1`) which exits within seconds, making `IsPidAlive(wrapperPID)` a false-negative for Claude-process liveness. Live incident `~/.claude/dsrcode.log` 2026-04-18 11:37-11:44: session `2fe1b32a-ea1d-464f-8cac-375a4fe709c9`, wrapper PID 5692, 8x `"PID dead but session has recent activity, skipping removal"` debug lines before removal at 2m25s — daemon auto-exit. The Phase-7 guard (Bug #1) covered only `SourceHTTP`; Phase 9 extends it to the second hook-delivered source. The 30-minute `removeTimeout` backstop still reaches zombie sessions that have stopped sending hooks entirely.

### Changed
- **Extended godoc at `session/stale.go:38-52`** explains why the PID-liveness skip covers both `SourceHTTP` and `SourceClaude`, notes Windows orphan-reparenting absence and Unix `start.sh` parent-chain weakness, and documents the `removeTimeout` backstop contract (Phase 9, D-08).

### Added
- **Three regression tests in `session/stale_test.go`** (Phase 9, D-05): `TestStaleCheckSkipsPidCheckForClaudeSource` (D-03 inverted rename of the Phase-7 PID-source test — SourceClaude + dead PID + 5min-old activity survives), `TestStaleCheckRemovesClaudeSourceAfterRemoveTimeout` (30-minute backstop preservation — SourceClaude + 31min-old activity still removed), `TestStaleCheckSurvivesUuidSourcedLongRunningAgent` (live-incident mirror — session `2fe1b32a-ea1d-464f-8cac-375a4fe709c9`, PID 5692, elapsed 150s), plus `TestStaleCheckEmitsDebugOnGuardSkip` (negative slog assertion — the inner `"PID dead but session has recent activity"` debug line MUST NOT fire for `SourceClaude`, proving the guard skips the entire block, not just the `EndSession` call).
- **Cross-platform verify harness** at `scripts/phase-09/verify.sh` (bash) and `scripts/phase-09/verify.ps1` (PowerShell) — mirrors the Phase-8 T1..Tn pattern with log-offset + cleanup-on-exit idiom, but swaps the T3+ assertions to stale-check absence semantics (SourceClaude UUID session survives a >=150s stale-check sweep with zero `"removing stale session"` log lines emitted) (Phase 9, D-06).

## [4.2.0] - 2026-04-16

### Fixed
- **Presence updates no longer dropped during Discord rate-limit cooldown** — the previous `presenceDebouncer` silently discarded any update that arrived inside the 15-second cooldown window (root cause: no pending buffer). Updates are now coalesced in a lock-free `atomic.Pointer[Activity]` slot and flushed exactly once when the token-bucket limiter permits. Live logs went from ~70% skip-rate during MCP-heavy sessions to <5% (Phase 8, RLC-01/RLC-02/RLC-03).
- **Race condition on presence-update shared state eliminated** — `sync.Mutex` on the hot path replaced with `sync/atomic.Pointer` + `atomic.Uint64` + `atomic.Int64`. The flush path is lock-free; `go test -race ./...` is green (Phase 8, RLC-02/RLC-06).
- **Duplicate hook events silently deduped** — Claude Code's `pre-tool-use` hook was observed firing twice within 30-130 ms in production logs. A new `HookDedupMiddleware` wraps `/hooks/*` routes with a 500 ms TTL cache keyed by FNV-64a of `(route, session_id, tool_name, body)`. Non-hook routes (`/health`, `/status`, `/preview`, `/config`, `/sessions`, `/presets`, `/statusline`) pass through untouched (Phase 8, RLC-07/RLC-08/RLC-09/RLC-10/RLC-14).

### Changed
- **Presence debouncer replaced with a token-bucket coalescer** (`golang.org/x/time/rate` v0.15.0 added) — 4-second cadence, burst 2. Matches empirical Discord RPC limits and delivers the latest coalesced state within ~4 s of the final signal, never discarding intermediate updates. Cold-start UX preserved: initial burst of 2 tokens means the first two presence updates after daemon launch flush immediately (Phase 8, RLC-01).
- **FNV-64a content-hash gates every `SetActivity` call** — identical payloads (same Details/State/LargeImage/LargeText/SmallImage/SmallText/Buttons; `StartTime` intentionally excluded) never consume a rate-limit token. Deterministic across process restarts via `hash/fnv.New64a()` with an ASCII Unit Separator (0x1F) between fields (Phase 8, RLC-04/RLC-05/RLC-06).

### Added
- **Hook-dedup middleware** wrapping `/hooks/*` routes with a `sync.Map`-backed 500 ms TTL cache and a 60-second background cleanup goroutine. Request body is capped at 64 KiB via `http.MaxBytesReader` (defence-in-depth; real payloads are <10 KB) and body-preserved via `io.NopCloser(bytes.NewReader(...))` for downstream handlers (Phase 8, RLC-07/RLC-08/RLC-09/RLC-10).
- **Periodic 60-second INFO summary log** with aggregate coalescer counters: `sent`, `skipped_rate`, `skipped_hash`, `deduped`, emitted via `slog.Info("coalescer status", ...)`. Skipped entirely when all counters are zero so idle daemons produce no log noise (Phase 8, RLC-11).
- **`testing/synctest`-based unit + integration tests** for the new `coalescer/` package and `server/hook_dedup.go` middleware. Uses Go 1.25's GA virtual-time bubbles to assert token-bucket scheduling, pending-slot semantics, hash skip, shutdown idempotency, and TTL eviction without wall-clock dependencies (Phase 8, RLC-15).
- **Cross-platform release verification harness** at `scripts/phase-08/verify.sh` (bash) and `scripts/phase-08/verify.ps1` (PowerShell) exercising T1-T6: binary version check, `/health` liveness, burst-coalesce rate, identical-content hash skip, hook-dedup log evidence, and 60-second summary log appearance (Phase 8, RLC-16).

## [4.1.2] - 2026-04-13

### Fixed
- **Daemon self-termination during long MCP-heavy sessions** — `handlePostToolUse` now updates `LastActivityAt` on every tool call (Bug #2, D-04/D-05 Phase 7). Previously, MCP-heavy sessions could idle the activity clock past the 30-minute remove-timeout even while Claude Code was actively calling tools, causing the daemon to remove the session and eventually self-exit. New `registry.Touch()` method updates the activity timestamp without firing a Discord presence update.
- **PID-liveness check false-positives for HTTP-sourced sessions** — `session/stale.go` now skips the PID liveness check when `Source == SourceHTTP` (Bug #1, D-01/D-02 Phase 7). The daemon's recorded PID for HTTP-sourced sessions is the short-lived `start.sh` wrapper process on Windows, which exits within seconds even while the Claude Code session is alive. The 30-minute `removeTimeout` remains as the backstop.
- **Refcount drift via missing SessionEnd command hook** — plugin `hooks/hooks.json` now registers `SessionEnd` as a `type: command` hook invoking `scripts/stop.sh` / `scripts/stop.ps1` (Bug #3, D-07/D-09 Phase 7). Dual-registered in `~/.claude/settings.local.json` via `start.sh` / `start.ps1` auto-patch as a fallback for documented upstream plugin-hook-discovery issues ([anthropics/claude-code#17885](https://github.com/anthropics/claude-code/issues/17885) — SessionEnd hook doesn't fire on `/exit`; [anthropics/claude-code#16288](https://github.com/anthropics/claude-code/issues/16288) — plugin hooks not loaded from external `hooks.json`).
- **Log overwrite on daemon restart** — `start.sh` and `start.ps1` now rotate `dsrcode.log` at 10 MB to `dsrcode.log.1` (single backup) before daemon launch (Bug #4, D-10/D-11/D-12 Phase 7). Same rotation applies to `dsrcode.log.err`. Crash history now survives daemon restarts. The PowerShell branch additionally fixes a same-path stderr defect in `scripts/start.ps1` (`-RedirectStandardError` now points at `$LogFileErr`, mirroring the Unix split shipped in v4.1.1 and working around [PowerShell/PowerShell#15031](https://github.com/PowerShell/PowerShell/issues/15031) — `Start-Process` has no native append mode for redirected streams).

### Changed
- Unix log redirection (`scripts/start.sh`) now splits stdout and stderr into separate files (`~/.claude/dsrcode.log` and `~/.claude/dsrcode.log.err`) and uses append mode (`>>`) so crash traces survive daemon restarts — matching Windows behavior established in v4.1.1.

## [4.1.1] - 2026-04-11

### Fixed
- Windows daemon launch in `scripts/start.sh` now redirects stdout to `~/.claude/dsrcode.log` and stderr to `~/.claude/dsrcode.log.err` via PowerShell `Start-Process -RedirectStandardOutput`/`-RedirectStandardError`. Previously both streams went to the void, so dsrcode's slog output and any crash traces were silently lost on Windows, making failures impossible to diagnose. Stderr uses a separate path because `Start-Process` rejects same-path redirects with `MutuallyExclusiveArguments`.

## [4.1.0] - 2026-04-10

### Added
- SessionEnd hook handler for instant session cleanup
- PostToolUse hook handler with throttled JSONL reads (10s)
- PreCompact/PostCompact hook handlers for compaction tracking
- StopFailure hook handler with "error" icon display
- SubagentStart hook handler for immediate subagent presence
- PostToolUseFailure hook handler for error counting
- CwdChanged hook handler for project context updates
- Binary auto-exit with configurable grace period (shutdownGracePeriod, default 30s)
- settings.local.json auto-patch for 13 HTTP hooks (start.sh)
- settings.local.json cleanup on plugin removal (stop.sh)
- "error" activity icon for API error display
- Analytics sync bridge: hook-triggered token updates flow from tracker to session registry for live presence rendering

### Changed
- Hook system expanded from 5 to 15 events
- Shutdown sequence: ClearActivity -> Close IPC -> Exit (clean Discord status)
- Wildcard (*) matchers for all tool-related hooks

### Removed
- JSONL fallback polling/watching (~250 lines) — replaced by hook-triggered reads
- JSONL background watcher goroutine
- StatusLineData, SessionData, JSONLMessage types from main.go

## [3.2.0] - 2026-04-07

### Added
- Analytics package: subagent hierarchy tracking, token breakdown, compaction detection, tool statistics, context usage
- 7 new placeholders: {tokens_detail}, {top_tools}, {subagents}, {context_pct}, {compactions}, {cost_detail}, {totalTokensDetail}
- SubagentStop hook support for tracking subagent spawn/complete lifecycle
- Per-model token tracking with cache-aware pricing (input/output/cache_read/cache_write)
- Compaction baseline logic preserving accurate token totals across context compactions
- Bilingual presets (EN + DE) with language wrapper JSON format (D-29)
- go-i18n integration for bilingual code strings (status output, labels, errors)
- 80+ new preset messages using analytics placeholders across all 8 presets
- config.json: "lang" field ("en"/"de") and "features" map for analytics toggle
- GET /status returns per-session analytics + top-level aggregates
- File-based analytics persistence (survives daemon restart)
- Auto-patch hooks.json on start: adds Agent to PreToolUse, adds SubagentStop section

### Changed
- Cost calculation is now cache-aware (4 rates per model: input, output, cache_write, cache_read)
- Pricing updated: Opus 4.6 $5/$25/$6.25/$0.50, Sonnet 4.6 $3/$15/$3.75/$0.30 per MTok
- Resolver trims double spaces and trailing dots from resolved strings
- {cost} placeholder now uses cache-aware calculation internally

### Fixed
- Token counts no longer drop after context compaction (baseline accumulation)
- Empty analytics data produces clean empty strings instead of "null" or "0"

### Deprecated
- "german" preset: auto-migrated to "professional" with lang="de"

## [3.1.0] - 2026-04-06

### Fixed
- Session count bug: JSONL fallback and HTTP/UUID sessions for the same project no longer create duplicate entries. A single Claude Code window now correctly shows "1 Projekt" instead of "Aktiv in 2 Repos".

### Added
- SessionSource enum (`claude`, `http`, `jsonl`) with source-aware upgrade chain (jsonl -> http -> uuid). Higher-quality sources replace lower-quality ones in-place, preserving session data.
- Extended POST /preview with full Discord Activity control: `smallImage`, `smallText`, `largeText`, `buttons`, `startTimestamp`, `sessionCount`, `fakeSessions`.
- GET /preview/messages endpoint for previewing StablePick message rotation across time windows.
- 4-mode demo skill (`/dsrcode:demo`): Quick Preview, Preset Tour, Multi-Session Preview, Message Rotation.
- JSONL fallback deprecation warning (once per session) recommending HTTP hook migration.

### Changed
- Resolver uses unique project count (not raw session count) for single/multi-session tier selection. Two windows on the same project show single-session display.

## [3.0.0] - 2026-04-03

### Added
- 7 slash commands: `/dsrcode:setup`, `/dsrcode:preset`, `/dsrcode:status`, `/dsrcode:log`, `/dsrcode:doctor`, `/dsrcode:update`, `/dsrcode:demo`.
- HTTP hooks replacing command hooks for 10x faster activity tracking (1s timeout, async fire).
- HookPayload parsing with PreToolUse, UserPromptSubmit, Stop, and Notification event handling.
- 4 display detail levels: minimal, standard, verbose, private.
- 40 messages per icon per preset (expanded from 15+).
- GET /presets, GET /status, POST /preview endpoints.
- session_id fallback via X-Claude-PID header with os.Getppid().
- start.sh auto-update: kills existing daemon before binary replacement, version comparison.

### Changed
- hooks.json from command-based to HTTP-based hook delivery.

## [2.0.0] - 2026-04-02

### Added
- Complete Go rewrite from Node.js: single static binary, cross-platform.
- HTTP hook server on localhost:19460 for real-time activity tracking.
- 8 display presets with 200+ messages each: minimal, professional, dev-humor, chaotic, german, hacker, streamer, weeb.
- Multi-session support tracking 1-N concurrent Claude Code instances.
- Config hot-reload via fsnotify file watcher.
- Stale session detection with idle/remove timeouts.
- StablePick algorithm for anti-flicker message rotation (Knuth hash, 5-min windows).
- Exponential backoff for Discord IPC reconnection.
- 5-platform release pipeline: macOS arm64/amd64, Linux arm64/amd64, Windows amd64.

### Changed
- Architecture from Node.js to Go with session registry, resolver, and preset packages.
- Session tracking from PID file to in-memory registry with ProjectPath+PID dedup.

## [1.0.3] - 2026-01-20

### Added
- Windows support with native named pipe IPC ([@8bury](https://github.com/8bury)) [#2](https://github.com/tsanva/cc-discord-presence/pull/2)
  - PowerShell start script (`start.ps1`)
  - Windows binary cross-compilation
  - go-winio for named pipe communication
- Comprehensive test suite for both main package and discord client
  - `main_test.go`: Tests for calculateCost, formatModelName, formatNumber, readStatusLineData, parseJSONLSession, path decoding, findMostRecentJSONL
  - `discord/client_test.go`: Tests for IPC protocol, SetActivity, send/receive, frame format verification

### Contributors
- pedro ([@8bury](https://github.com/8bury)) - Windows support
- Claude ([@claude](https://github.com/claude)) - Test suite

## [1.0.2] - 2025-12-31

### Fixed
- Replaced reference counting with PID-based session tracking
  - Previous refcount could drift if sessions crashed or were killed
  - Now each session registers with its PID in `~/.claude/discord-presence-sessions/`
  - Orphaned sessions (dead PIDs) are automatically cleaned up
  - Self-healing: no manual intervention needed after crashes

## [1.0.1] - 2025-12-31

### Fixed
- Multi-instance support: daemon now stays running when multiple Claude Code instances are open
  - Added reference counting to track active instances
  - Daemon only stops when all instances are closed

## [1.0.0] - 2025-12-30

### Added
- Initial release
- Discord Rich Presence showing project name, git branch, model, tokens, and cost
- Two data sources: statusline integration (accurate) and JSONL fallback (zero-config)
- Cross-platform support: macOS (arm64/amd64), Linux (amd64/arm64), Windows (amd64)
- Automatic binary download on first run
- GitHub Actions workflow for automated releases

[Unreleased]: https://github.com/DSR-Labs/cc-discord-presence/compare/v4.1.0...HEAD
[4.1.0]: https://github.com/DSR-Labs/cc-discord-presence/compare/v4.0.0...v4.1.0
[3.2.0]: https://github.com/DSR-Labs/cc-discord-presence/compare/v3.1.0...v3.2.0
[3.1.0]: https://github.com/DSR-Labs/cc-discord-presence/compare/v3.0.0...v3.1.0
[3.0.0]: https://github.com/DSR-Labs/cc-discord-presence/compare/v2.0.0...v3.0.0
[2.0.0]: https://github.com/DSR-Labs/cc-discord-presence/compare/v1.0.3...v2.0.0
[1.0.3]: https://github.com/DSR-Labs/cc-discord-presence/compare/v1.0.2...v1.0.3
[1.0.2]: https://github.com/DSR-Labs/cc-discord-presence/compare/v1.0.1...v1.0.2
[1.0.1]: https://github.com/DSR-Labs/cc-discord-presence/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/DSR-Labs/cc-discord-presence/releases/tag/v1.0.0
