# cc-discord-presence

## What This Is

Discord Rich Presence plugin for Claude Code. A Go binary with HTTP hooks that displays real-time session information on Discord -- project name, git branch, model, session duration, token usage, cost, and activity type. Features personality-driven status messages via 8 display presets and 4 detail levels.

## Core Value

Real-time session visualization on Discord with personality-driven status messages. Users see what they're working on, how long they've been at it, and what it costs -- all reflected live in their Discord profile with customizable tone and privacy controls.

## Requirements

### Validated

- [x] Discord IPC connection via Unix sockets and Windows named pipes -- Validated in Phase 1: discord-rich-presence-activity-status-plugin-merge
- [x] HTTP hooks for real-time activity tracking (PreToolUse, UserPromptSubmit, Stop, Notification) -- Validated in Phase 1
- [x] 8 display presets with 200+ messages each (minimal, professional, dev-humor, chaotic, german, hacker, streamer, weeb) -- Validated in Phase 1
- [x] Multi-session tracking for concurrent Claude Code instances -- Validated in Phase 1
- [x] Config hot-reload via fsnotify directory watcher -- Validated in Phase 1
- [x] JSONL fallback for session data when hooks unavailable -- Validated in Phase 1
- [x] 7-phase guided setup wizard with interactive configuration -- Validated in Phase 2: dsrcodepresence-setup-wizard
- [x] 4 display detail levels (minimal, standard, verbose, private) -- Validated in Phase 2
- [x] Preview/demo mode for screenshot generation -- Validated in Phase 2
- [x] Session count accuracy with PID-based tracking (Unix) and refcount (Windows) -- Validated in Phase 3: fix-discord-presence-session-count-demo-mode
- [x] Enhanced demo mode with 4 modes (quick preview, preset tour, multi-session, message rotation) -- Validated in Phase 3
- [x] Subagent tracking via SubagentStart/SubagentStop hooks -- Validated in Phase 4: discord-presence-enhanced-analytics
- [x] Token breakdown by model with cost aggregation -- Validated in Phase 4
- [x] Compaction detection via file size monitoring -- Validated in Phase 4
- [x] Tool usage statistics and context utilization display -- Validated in Phase 4
- [x] Bilingual message presets (English/German) with runtime language switching -- Validated in Phase 4

### Active

- [ ] GitHub Releases binary distribution with goreleaser
- [ ] Cross-platform start.sh rewrite for reliable daemon lifecycle
- [ ] Automated build pipeline for 5-platform release (macOS/Linux arm64+amd64, Windows amd64)
- [ ] SessionEnd/PostToolUse/PreCompact hooks for event-driven architecture
- [ ] JSONL polling removal in favor of hook-triggered reads
- [ ] Binary auto-exit when all sessions end

### Out of Scope

- Mobile Discord -- Rich Presence is desktop-only (Discord limitation)
- Custom Discord bot -- Uses Discord IPC Rich Presence protocol only
- GUI configuration -- CLI/config file only, no Electron/web UI
- Metrics dashboard -- Session data is ephemeral, not persisted long-term
- Plugin marketplace hosting -- Relies on Claude Code plugin system

## Context

### Tech Stack

- **Language:** Go 1.25 (single binary, cross-platform compilation)
- **Discord IPC:** Direct IPC via Unix sockets (macOS/Linux) and Windows named pipes (go-winio)
- **File watching:** fsnotify for config hot-reload and session file monitoring
- **HTTP server:** net/http with chi router for hook endpoints and preview API
- **Presets:** go:embed JSON files compiled into binary for single-file distribution
- **Logging:** slog with LevelVar for runtime log level changes

### Architecture

- HTTP hooks receive events from Claude Code (PreToolUse, UserPromptSubmit, Stop, Notification)
- Session registry tracks multiple concurrent Claude Code instances
- Resolver maps session state to Discord presence fields using active preset
- Debouncer coalesces rapid updates respecting Discord's 15s rate limit
- Config watcher monitors directory for atomic-save editor compatibility

## Constraints

- **Cross-platform:** Must run on macOS (arm64/amd64), Linux (arm64/amd64), Windows (amd64)
- **Single binary:** No runtime dependencies -- presets embedded, no external config required
- **MIT license:** Open source, contributions welcome
- **Plugin protocol:** Must conform to Claude Code plugin manifest and hook specifications
- **Discord rate limit:** 15-second minimum between presence updates
- **Backward compatibility:** Config format and CLI flags must remain stable across versions

## Key Decisions

| Decision | Rationale | Phase |
|----------|-----------|-------|
| HTTP hooks over command hooks | 10x faster activity tracking, async with 1s timeout | Phase 1 |
| go:embed for presets | Single binary distribution, no external files needed | Phase 1 |
| Channel-based debounce | Non-blocking send to buffered channel of 1, respects Discord 15s rate limit | Phase 1 |
| Synthetic session IDs for JSONL | jsonl- prefix prevents duplicate registry entries | Phase 1 |
| DisplayDetail enum with fallback | ParseDisplayDetail returns minimal for unknown values | Phase 2 |
| Preview duration 5-300s, 60s default | Clamped range prevents accidental long previews | Phase 2 |
| PID-based tracking (Unix) + refcount (Windows) | PPID unreliable on Windows, refcount is safer | Phase 3 |
| json.RawMessage for analytics | Avoids circular imports between session/ and analytics/ | Phase 4 |
| BilingualMessagePreset with map | map[string]*MessagePreset for O(1) language selection | Phase 4 |
| External test packages | Black-box testing pattern for all packages | Phase 1-4 |

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition:**
1. Requirements invalidated? Move to Out of Scope with reason
2. Requirements validated? Move to Validated with phase reference
3. New requirements emerged? Add to Active
4. Decisions to log? Add to Key Decisions
5. "What This Is" still accurate? Update if drifted

---
*Last updated: 2026-04-08 -- Project initialized as standalone GSD-managed project after migration from StrainReviewsScanner. Phases 1-4 complete (34 plans), phases 5-6 pending.*
