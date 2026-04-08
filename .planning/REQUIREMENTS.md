# cc-discord-presence Requirements

## Core Presence (Phase 1)

| ID | Requirement | Status |
|----|-------------|--------|
| D-01 | Discord IPC handshake with Application ID | Complete |
| D-02 | Set Activity with project name, branch, model | Complete |
| D-03 | Session duration via startTimestamp | Complete |
| D-04 | Token count and cost display | Complete |
| D-05 | Activity type icons (coding, terminal, searching, reading, thinking, idle) | Complete |
| D-06 | Graceful shutdown on SIGINT/SIGTERM | Complete |
| D-07 | PID file management for daemon lifecycle | Complete |
| D-08 | HTTP hook endpoint for PreToolUse events | Complete |
| D-09 | HTTP hook endpoint for UserPromptSubmit events | Complete |
| D-10 | HTTP hook endpoint for Stop events | Complete |
| D-11 | HTTP hook endpoint for Notification events | Complete |
| D-12 | Async hook processing with 1s timeout | Complete |
| D-13 | JSONL session file parsing as fallback | Complete |
| D-14 | Model pricing table for cost calculation | Complete |
| D-15 | Config file loading from ~/.claude/discord-presence-config.json | Complete |
| D-16 | Environment variable overrides (CC_DISCORD_*) | Complete |
| D-17 | CLI flag parsing (--port, --preset, -v, -q) | Complete |
| D-18 | Priority chain: CLI > env > config > defaults | Complete |
| D-19 | Config hot-reload via fsnotify | Complete |
| D-20 | Directory-level watching for atomic-save editors | Complete |
| D-21 | Minimal preset with terse messages | Complete |
| D-22 | Professional preset with corporate-friendly messages | Complete |
| D-23 | Dev-humor preset with lighthearted messages | Complete |
| D-24 | go:embed for preset JSON files in binary | Complete |
| D-25 | 200+ messages per preset across categories | Complete |
| D-26 | HashString-based StablePick for deterministic message selection | Complete |
| D-27 | Multi-session registry tracking N concurrent instances | Complete |
| D-28 | Aggregated stats line for multi-session display | Complete |
| D-29 | getMostRecentSession for SmallImage icon selection | Complete |
| D-30 | Channel-based debounce pattern for presence updates | Complete |
| D-31 | Discord 15s rate limit enforcement in debouncer | Complete |
| D-32 | Synthetic session IDs (jsonl- prefix) for JSONL sessions | Complete |
| D-33 | Cross-platform connection (Unix sockets + Windows named pipes) | Complete |
| D-34 | Exponential backoff for Discord reconnection | Complete |
| D-35 | Chaotic preset with unhinged energy messages | Complete |
| D-36 | German preset with Deutsche Nachrichten | Complete |
| D-37 | Hacker preset with terminal aesthetic | Complete |
| D-38 | Streamer preset with content creator vibe | Complete |
| D-39 | Weeb preset with anime references | Complete |
| D-40 | Buttons field support (up to 2 buttons in presence) | Complete |
| D-41 | X-Claude-PID header for PID sourcing in hooks | Complete |
| D-42 | os.Getppid() fallback for PID | Complete |
| D-43 | 20 messages per icon per preset | Complete |
| D-44 | External test packages for black-box testing | Complete |
| D-45 | Backoff.Next() returns current then doubles | Complete |
| D-46 | durationOrInt custom JSON unmarshaler | Complete |
| D-47 | slog.LevelVar for runtime log level changes | Complete |
| D-48 | Auto-create session on first hook event | Complete |
| D-49 | Handler() method for httptest compatibility | Complete |
| D-50 | 5-platform release pipeline (macOS/Linux arm64+amd64, Windows amd64) | Complete |
| D-51 | Stale session detection and cleanup | Complete |
| D-52 | Idle timeout with configurable duration | Complete |
| D-53 | Remove timeout for abandoned sessions | Complete |
| D-54 | Large image text with project and branch | Complete |
| D-55 | Small image text with activity description | Complete |
| D-56 | Conn interface for testable Discord IPC | Complete |

## Setup Wizard (Phase 2)

| ID | Requirement | Status |
|----|-------------|--------|
| DSR-01 | Phase 1: Prerequisites check (Go, Discord app) | Complete |
| DSR-02 | Phase 2: Discord connection test | Complete |
| DSR-03 | Phase 3: Config file generation | Complete |
| DSR-04 | Phase 4: Advanced settings (timeouts, logging) | Complete |
| DSR-05 | Phase 5: Hook configuration | Complete |
| DSR-06 | Phase 6: Apply configuration | Complete |
| DSR-07 | Phase 7: Verification and status check | Complete |
| DSR-08 | Interactive preset selection with live preview | Complete |
| DSR-09 | DisplayDetail enum (minimal, standard, verbose, private) | Complete |
| DSR-10 | ParseDisplayDetail with fallback to minimal | Complete |
| DSR-11 | ExtractToolContext for hook payload analysis | Complete |
| DSR-12 | Synthetic session_id with http- prefix | Complete |
| DSR-13 | Sorted map iteration for deterministic output | Complete |
| DSR-14 | Preview duration clamped 5-300s, 60s default | Complete |
| DSR-15 | Timer cancel via Timer.Stop() before replacement | Complete |
| DSR-16 | 40 messages per category (15 original + 25 new) | Complete |
| DSR-17 | HTTP hooks with 1s timeout replacing command hooks | Complete |
| DSR-18 | Notification hook with idle_prompt matcher | Complete |
| DSR-19 | start.sh auto-update with daemon kill before replacement | Complete |
| DSR-20 | atomic.Bool for discordConnected state | Complete |
| DSR-21 | cfgMu sync.RWMutex for config reads | Complete |
| DSR-22 | onPreviewEnd signals debounce channel | Complete |
| DSR-23 | Config watcher updates preset and displayDetail under lock | Complete |
| DSR-24 | git hash-object for Windows NTFS colon filenames | Complete |
| DSR-25 | /dsrcode:setup slash command | Complete |
| DSR-26 | /dsrcode:preset slash command | Complete |
| DSR-27 | /dsrcode:status slash command | Complete |
| DSR-28 | /dsrcode:log slash command | Complete |
| DSR-29 | /dsrcode:doctor slash command | Complete |
| DSR-30 | /dsrcode:update slash command | Complete |
| DSR-31 | /dsrcode:demo slash command | Complete |
| DSR-32 | Quick Preview demo mode | Complete |
| DSR-33 | Preset Tour demo mode | Complete |
| DSR-34 | Multi-Session Preview demo mode | Complete |
| DSR-35 | Message Rotation demo mode | Complete |
| DSR-36 | Preview REST API (POST /preview) | Complete |
| DSR-37 | Multi-session preview endpoint | Complete |
| DSR-38 | Message rotation endpoint (GET /preview/messages) | Complete |
| DSR-39 | POST /config for runtime config updates | Complete |
| DSR-40 | /dsrcode:status hook statistics section | Complete |
| DSR-41 | Doctor diagnostics across 7 categories | Complete |
| DSR-42 | Binary update from GitHub Releases | Complete |

## Session Management (Phase 3)

| ID | Requirement | Status |
|----|-------------|--------|
| D-01 | PID-based session tracking on macOS/Linux | Complete |
| D-02 | Refcount-based session tracking on Windows | Complete |
| D-03 | Session file cleanup on daemon stop | Complete |
| D-04 | Accurate session count in presence display | Complete |
| D-05 | Stale session detection via PID liveness check | Complete |
| D-06 | Windows tasklist-based PID verification | Complete |
| D-07 | Session directory at ~/.claude/discord-presence-sessions/ | Complete |
| D-08 | Enhanced demo Quick Preview mode | Complete |
| D-09 | Enhanced demo Preset Tour mode | Complete |
| D-10 | Enhanced demo Multi-Session Preview mode | Complete |
| D-11 | Enhanced demo Message Rotation mode | Complete |
| D-12 | Preview API single preset endpoint | Complete |
| D-13 | Preview API multi-session endpoint | Complete |
| D-14 | Preview API message rotation endpoint | Complete |
| D-15 | Demo duration validation (5-300s) | Complete |
| D-16 | Fake session generation for demos | Complete |
| D-17 | Preview auto-expiry and normal presence resume | Complete |
| D-18 | Session registry thread safety | Complete |
| D-19 | Concurrent session stress testing | Complete |
| D-20 | PID file locking for daemon exclusivity | Complete |
| D-21 | Session cleanup on SIGTERM | Complete |
| D-22 | Session count aggregation in multi-session display | Complete |
| D-23 | Cross-platform session tracking abstraction | Complete |
| D-24 | Session start/stop hook integration | Complete |
| D-25 | CheckOnce exported for external stale detection tests | Complete |

## Analytics (Phase 4)

| ID | Requirement | Status |
|----|-------------|--------|
| DPA-01 | Subagent tracking via SubagentStart hook | Complete |
| DPA-02 | Subagent tracking via SubagentStop hook | Complete |
| DPA-03 | SubagentTree data structure | Complete |
| DPA-04 | Spawn() preserves pre-set Status | Complete |
| DPA-05 | Token breakdown by model | Complete |
| DPA-06 | Cost aggregation across models | Complete |
| DPA-07 | Compaction detection via file size monitoring | Complete |
| DPA-08 | Compaction count in analytics | Complete |
| DPA-09 | Tool usage statistics collection | Complete |
| DPA-10 | Tool category grouping | Complete |
| DPA-11 | Context utilization percentage | Complete |
| DPA-12 | Context window size detection | Complete |
| DPA-13 | Analytics tracker configuration (DataDir) | Complete |
| DPA-14 | Persistent analytics state | Complete |
| DPA-15 | Persist errors silently discarded | Complete |
| DPA-16 | SubagentStop route before wildcard | Complete |
| DPA-17 | SetTracker method for optional injection | Complete |
| DPA-18 | string(detail) conversion for analytics format | Complete |
| DPA-19 | LoadPreset backward compatibility via LoadPresetWithLang | Complete |
| DPA-20 | BilingualMessagePreset with map[string]*MessagePreset | Complete |
| DPA-21 | German preset becomes bilingual with professional content | Complete |
| DPA-22 | ParseMessageFileBytes for embedded FS | Complete |
| DPA-23 | Parse error logged as warning for resilient degradation | Complete |
| DPA-24 | json.RawMessage bridge in session.AnalyticsUpdate | Complete |
| DPA-25 | Analytics placeholder resolution in presets | Complete |
| DPA-26 | Subagent count in presence display | Complete |
| DPA-27 | Token/cost breakdown formatting | Complete |
| DPA-28 | Compaction indicator in status | Complete |
| DPA-29 | Runtime language switching (EN/DE) | Complete |
| DPA-30 | Registry test coverage for analytics methods | Complete |

## Distribution (Phase 5)

| ID | Requirement | Status |
|----|-------------|--------|
| DIST-01 | goreleaser configuration for automated builds | Pending |
| DIST-02 | GitHub Actions workflow for release pipeline | Pending |
| DIST-03 | Cross-compilation for 5 platforms | Pending |
| DIST-04 | Binary naming convention for releases | Pending |
| DIST-05 | start.sh rewrite for reliable daemon lifecycle | Pending |
| DIST-06 | start.ps1 parity with start.sh | Pending |
| DIST-07 | Version embedding via ldflags | Pending |
| DIST-08 | Release notes auto-generation | Pending |
| DIST-09 | Binary checksum verification | Pending |
| DIST-10 | Download retry with exponential backoff | Pending |

## Hook System Overhaul (Phase 6)

Requirements TBD -- phase not yet planned. See Phase 6 CONTEXT.md for 18 implementation decisions gathered.

---
*Last updated: 2026-04-08*
