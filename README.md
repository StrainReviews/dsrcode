# DSR Code Presence

[![Version](https://img.shields.io/badge/version-v4.0.0-blue.svg)](https://github.com/StrainReviews/dsrcode/releases)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8.svg)](https://go.dev)

**Discord Rich Presence for Claude Code**

Show your Claude Code sessions on Discord with real-time project info, activity tracking, and personality-driven status messages.

## Features

- **Rich Presence** showing project name, git branch, model, tokens, cost, and session duration
- **8 display presets** with distinct personalities: minimal, professional, dev-humor, chaotic, german, hacker, streamer, weeb
- **4 display detail levels** controlling data visibility: minimal, standard, verbose, private
- **Multi-session support** tracking 1-N concurrent Claude Code instances
- **Activity tracking** showing current action: editing, terminal, searching, reading, thinking, idle
- **7 slash commands** for setup, configuration, diagnostics, and demo screenshots
- **Config hot-reload** -- change preset or display detail without restarting the daemon
- **Cross-platform** support for macOS (arm64/amd64), Linux (arm64/amd64), and Windows (amd64)

## Installation

```
claude plugin install StrainReviews/dsrcode
```

That's it. The binary downloads automatically on first session start.

## Quick Start

1. Install the plugin (see above)
2. Start a Claude Code session -- presence appears on Discord automatically
3. Run `/dsrcode:setup` for guided configuration
4. Run `/dsrcode:preset` to switch display styles

## Screenshots

> Generate screenshots using `/dsrcode:demo` -- it sets Discord presence to demo data for a configurable duration so you can capture clean screenshots.

| Preset | Preview |
|--------|---------|
| minimal | ![minimal](assets/screenshots/minimal.png) |
| professional | ![professional](assets/screenshots/professional.png) |
| dev-humor | ![dev-humor](assets/screenshots/dev-humor.png) |
| chaotic | ![chaotic](assets/screenshots/chaotic.png) |
| german | ![german](assets/screenshots/german.png) |
| hacker | ![hacker](assets/screenshots/hacker.png) |
| streamer | ![streamer](assets/screenshots/streamer.png) |
| weeb | ![weeb](assets/screenshots/weeb.png) |

## Presets

Each preset defines a personality for your Discord status messages. The preset controls the *tone* of messages while display detail controls the *data level*.

| Preset | Description | Sample Message |
|--------|-------------|----------------|
| **minimal** | Terse, just the facts | `Working on {project}` |
| **professional** | Corporate-friendly | `Developing {project} on {branch}` |
| **dev-humor** | Lighthearted dev culture | `Yelling at {file} until it works` |
| **chaotic** | Unhinged energy | `UNLEASHING CHAOS in {project}` |
| **german** | Deutsche Nachrichten | `Programmiere an {project}` |
| **hacker** | Terminal aesthetic | `root@{project}:~$ editing {file}` |
| **streamer** | Content creator vibe | `LIVE: coding {project}` |
| **weeb** | Anime references | `{project}-chan needs debugging~` |

## Display Detail Levels

Display detail is orthogonal to presets. Presets set the tone; detail levels set how much data is shown.

| Level | `{file}` | `{command}` | `{query}` | Use Case |
|-------|----------|-------------|-----------|----------|
| **minimal** (default) | project name | `...` | `*` | Privacy-conscious, minimal data exposure |
| **standard** | filename | truncated (20 chars) | search pattern | Good balance of info and privacy |
| **verbose** | full relative path | full command | full pattern | Maximum visibility for streaming or demos |
| **private** | `file` | `...` | *(empty)* | Complete privacy, no project data shown |

## Slash Commands

| Command | Description |
|---------|-------------|
| `/dsrcode:setup` | Guided first-time setup wizard (7 phases: prerequisites, Discord test, config, advanced settings, hooks, apply, verify) |
| `/dsrcode:preset` | Quick-switch between presets with instant hot-reload |
| `/dsrcode:status` | View daemon status, Discord connection, active sessions, and hook statistics |
| `/dsrcode:log` | View recent daemon log entries from `~/.claude/dsrcode.log` |
| `/dsrcode:doctor` | Run diagnostics across 7 categories: binary, Discord, hooks, statusline, config, sessions, platform |
| `/dsrcode:update` | Update binary to the latest GitHub release |
| `/dsrcode:demo` | Preview mode for generating screenshots -- sets presence to demo data for a configurable duration |

## Demo Mode

The `/dsrcode:demo` command launches a 4-mode demo tool for generating screenshots and previewing how your Discord presence looks.

### Demo Modes

| Mode | Description |
|------|-------------|
| **Quick Preview** | Set a single preset with realistic demo data. Take one clean screenshot. |
| **Preset Tour** | Walk through all 8 presets sequentially. Claude loops through each and waits for your confirmation before advancing. |
| **Multi-Session Preview** | Simulate 2-5 concurrent projects. Discord shows aggregated multi-project stats. |
| **Message Rotation** | See how status messages rotate over time for a given preset and activity type. |

### Preview API

The daemon exposes preview endpoints for programmatic use:

**Single preset preview:**
```bash
curl -sf -X POST -H "Content-Type: application/json" \
  -d '{
    "preset": "dev-humor",
    "displayDetail": "standard",
    "duration": 120,
    "details": "Working on my-saas-app",
    "state": "Opus 4.6 | 250K tokens | $2.50 | 2h 15m",
    "smallImage": "coding",
    "smallText": "Editing auth.ts",
    "largeText": "my-saas-app (feature/auth)",
    "startTimestamp": 1775466000
  }' \
  http://127.0.0.1:19460/preview
```

**Multi-session preview:**
```bash
curl -sf -X POST -H "Content-Type: application/json" \
  -d '{
    "preset": "minimal",
    "displayDetail": "standard",
    "duration": 120,
    "sessionCount": 3,
    "fakeSessions": [
      {"projectName": "my-saas-app", "model": "Opus 4.6", "totalTokens": 250000, "totalCost": 2.50, "activity": "coding"},
      {"projectName": "api-gateway", "model": "Sonnet 4.6", "totalTokens": 180000, "totalCost": 1.20, "activity": "terminal"},
      {"projectName": "docs-site", "model": "Haiku 4.5", "totalTokens": 50000, "totalCost": 0.30, "activity": "reading"}
    ]
  }' \
  http://127.0.0.1:19460/preview
```

**Message rotation preview:**
```bash
curl -sf "http://127.0.0.1:19460/preview/messages?preset=dev-humor&activity=coding&count=5&detail=standard"
```

Preview duration is 5-300 seconds (default 60). The preview auto-expires and normal presence resumes.

## Configuration

### Config File

Location: `~/.claude/dsrcode-config.json`

```json
{
  "discordClientId": "",
  "preset": "minimal",
  "port": 19460,
  "bindAddr": "127.0.0.1",
  "idleTimeout": "10m",
  "removeTimeout": "30m",
  "staleCheckInterval": "30s",
  "reconnectInterval": "15s",
  "logLevel": "info",
  "logFile": "~/.claude/dsrcode.log",
  "displayDetail": "minimal",
  "buttons": [
    {
      "label": "My GitHub",
      "url": "https://github.com/yourusername"
    }
  ]
}
```

All fields are optional. Unset fields use compiled defaults.

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `CC_DISCORD_PORT` | HTTP server port | `19460` |
| `CC_DISCORD_PRESET` | Active preset name | `minimal` |
| `CC_DISCORD_BIND_ADDR` | Bind address | `127.0.0.1` |
| `CC_DISCORD_CLIENT_ID` | Discord Application ID | *(empty)* |
| `CC_DISCORD_LOG_LEVEL` | Log level (debug, info, warn, error) | `info` |
| `CC_DISCORD_DISPLAY_DETAIL` | Display detail level | `minimal` |

### Priority Chain

Configuration is resolved with the following priority (highest to lowest):

1. **CLI flags** (`--port`, `--preset`, `-v`, `-q`)
2. **Environment variables** (`CC_DISCORD_*`)
3. **Config file** (`~/.claude/dsrcode-config.json`)
4. **Compiled defaults**

## How It Works

### Activity Tracking

The plugin uses HTTP hooks to track Claude Code activity in real-time:

- **PreToolUse** -- Detects editing, terminal commands, file reading, searching
- **UserPromptSubmit** -- Tracks when you send a prompt
- **Stop** -- Marks session end or pause
- **Notification** -- Detects idle state when Claude waits for input

Hooks fire asynchronously with a 1-second timeout so they never slow down your session.

### Session Data Sources

1. **HTTP Hooks** (primary) -- Real-time activity data from Claude Code hook events
2. **JSONL Fallback** -- Parses Claude Code session files from `~/.claude/projects/` when hooks are unavailable

### Multi-Session Support

The daemon tracks all concurrent Claude Code sessions. When multiple sessions are active, Discord presence shows aggregated stats (total cost, total tokens, project list). Each session is identified by its working directory and process ID.

## Troubleshooting

### Discord not showing presence

1. Make sure the **Discord desktop app** is running (browser Discord does not support Rich Presence)
2. Check that the daemon is running: `/dsrcode:status`
3. Verify Discord connection: `/dsrcode:doctor`
4. In Discord settings, ensure **Activity Status** is enabled under Activity Privacy

### Hooks not firing

1. Verify `hooks/hooks.json` exists in the plugin directory
2. Check hook statistics with `/dsrcode:status` -- look at the hook stats section
3. Run `/dsrcode:doctor` to diagnose hook configuration
4. Ensure the daemon is listening on port 19460 (or your configured port)

### Binary not found

The binary auto-downloads on first session start. If it fails:

1. Run `/dsrcode:update` to manually download the latest release
2. Or build from source: `go build -o dsrcode .`

### Wrong version

Run `/dsrcode:update` to download the latest release from GitHub.

### Presence disappears after a while

The daemon has an idle timeout (default 10 minutes). If no hook activity is received, the session transitions to idle and eventually gets removed (default 30 minutes). Adjust timeouts in config:

```json
{
  "idleTimeout": "30m",
  "removeTimeout": "60m"
}
```

## Development

### Build from source

```bash
go build -o dsrcode .
```

### Run tests

```bash
go test -v -race ./...
```

### Development mode

Load the plugin from a local directory instead of the marketplace:

```bash
claude --plugin-dir /path/to/cc-discord-presence
```

### Project Structure

```
cc-discord-presence/
  .claude-plugin/
    plugin.json         # Plugin manifest (v4.0.0)
  commands/
    dsrcode:setup.md    # Setup wizard
    dsrcode:preset.md   # Preset switcher
    dsrcode:status.md   # Status viewer
    dsrcode:log.md      # Log viewer
    dsrcode:doctor.md   # Diagnostics
    dsrcode:update.md   # Binary updater
    dsrcode:demo.md     # Demo/preview mode
  hooks/
    hooks.json          # HTTP activity hooks
  scripts/
    start.sh            # Daemon lifecycle (start)
    stop.sh             # Daemon lifecycle (stop)
  config/               # Configuration loading
  discord/              # Discord IPC client
  preset/               # Preset loader + types
  resolver/             # Placeholder resolution
  server/               # HTTP API server
  session/              # Session registry
  logger/               # Structured logging
  main.go               # Entry point
```

## License

MIT License -- see [LICENSE](LICENSE) for details.
