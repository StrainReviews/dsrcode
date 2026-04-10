# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
