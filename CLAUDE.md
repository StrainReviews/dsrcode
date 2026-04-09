# CLAUDE.md - Project Context for Claude Code

## Project Overview

**dsrcode** is a Discord Rich Presence plugin for Claude Code. It displays real-time session information on Discord, including project name, git branch, model, session duration, token usage, and cost.

## Tech Stack

- **Language**: Go 1.25
- **Key Dependencies**:
  - `github.com/fsnotify/fsnotify` - Cross-platform file watching
  - `github.com/Microsoft/go-winio` - Windows named pipe support (Discord IPC)

## Project Structure

```
cc-discord-presence/
├── main.go               # Main entry - session tracking, data parsing, presence updates
├── discord/
│   ├── client.go         # Discord IPC client, Conn interface, presence logic
│   ├── conn_unix.go      # Unix socket connection (macOS/Linux)
│   └── conn_windows.go   # Named pipe connection (Windows, uses go-winio)
├── scripts/
│   ├── start.sh          # Plugin hook: starts daemon on SessionStart
│   ├── stop.sh           # Plugin hook: stops daemon on SessionEnd
│   ├── bump-version.sh   # Coordinated version bump across 5 files
│   ├── statusline-wrapper.sh  # Wrapper script (copied to ~/.claude/)
│   └── setup-statusline.sh    # One-time setup for statusline integration
├── .claude-plugin/
│   └── plugin.json       # Plugin manifest with SessionStart/SessionEnd hooks
├── .planning/            # GSD planning framework (phases, roadmap, state)
│   ├── PROJECT.md
│   ├── ROADMAP.md
│   ├── REQUIREMENTS.md
│   ├── STATE.md
│   ├── config.json
│   └── phases/           # Phase directories (01-06 migrated, 07+ new)
├── go.mod
└── go.sum
```

## Key Concepts

### Discord IPC Protocol
- Unix socket at `/tmp/discord-ipc-{0-9}` (macOS/Linux)
- Named pipe on Windows
- Frame format: `[opcode:4 LE][length:4 LE][JSON payload]`
- Opcodes: 0 = Handshake, 1 = Frame

### Session Data Sources (Priority Order)

1. **Statusline Data** (`~/.claude/dsrcode-data.json`)
   - Most accurate - uses Claude Code's own calculations
   - Requires user to configure statusline wrapper
   - Provides: model display name, cost, tokens directly from Claude

2. **JSONL Fallback** (`~/.claude/projects/<encoded-path>/*.jsonl`)
   - Zero configuration needed
   - Parses session transcript files
   - Calculates cost based on model pricing table
   - Finds most recently modified file (handles multiple instances)

### Plugin Hooks
- **SessionStart**: Launches daemon via `scripts/start.sh`
- **SessionEnd**: Stops daemon via `scripts/stop.sh`
- Uses PID file at `~/.claude/dsrcode.pid`

### Session Tracking (Platform-specific)
- **macOS/Linux**: PID-based tracking via files in `~/.claude/dsrcode-sessions/`
- **Windows**: Refcount-based tracking via `~/.claude/dsrcode.refcount` (PPID unreliable on Windows)

### Model Pricing (Update when new models release)
Located at top of `main.go` in `modelPricing` and `modelDisplayNames` maps.

| Model | Input $/1M | Output $/1M |
|-------|-----------|-------------|
| Opus 4.5 | $15 | $75 |
| Sonnet 4.5 | $3 | $15 |
| Sonnet 4 | $3 | $15 |
| Haiku 4.5 | $1 | $5 |

## Development Commands

```bash
go build -o dsrcode .     # Build binary
go run .                   # Run directly
./dsrcode                  # Run built binary
go test -v ./...           # Run all tests
goreleaser build --snapshot --clean  # Local release build (all platforms)
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines. Key points:
- Run `go test -v ./...` before submitting PRs
- Update CHANGELOG.md with your changes
- Contributors are credited in release notes

## Configuration

- Client ID `1455326944060248250` is hardcoded (shared "Clawd Code" Discord app)
- App icon set in Discord Developer Portal (used automatically for Rich Presence)

## Important Notes

- Polls every 3 seconds + uses file watcher
- Discord must be running for RPC to connect
- Graceful shutdown on SIGINT/SIGTERM
- Shows nudge message when using JSONL fallback encouraging statusline setup

## GSD Workflow Enforcement

Before using Edit, Write, or other file-changing tools, start work through a GSD command so planning artifacts and execution context stay in sync.

Use these entry points:
- `/gsd:quick` for small fixes, doc updates, and ad-hoc tasks
- `/gsd:debug` for investigation and bug fixing
- `/gsd:execute-phase` for planned phase work

Do not make direct repo edits outside a GSD workflow unless the user explicitly asks to bypass it.

## Releasing

Releases are automated via GoReleaser and GitHub Actions.

1. **Bump version:**
   ```bash
   ./scripts/bump-version.sh X.Y.Z
   ```
   This updates main.go, plugin.json, marketplace.json, start.sh, and start.ps1.

2. **Commit and tag:**
   ```bash
   git add main.go .claude-plugin/plugin.json .claude-plugin/marketplace.json scripts/start.sh scripts/start.ps1
   git commit -m "chore: bump version to vX.Y.Z"
   git tag vX.Y.Z
   git push origin main --tags
   ```

3. **GitHub Actions runs GoReleaser** automatically on tag push, building 5 platform binaries with SHA256 checksums.

For manual release: Use workflow_dispatch in GitHub Actions UI.
For local testing: `goreleaser build --snapshot --clean`
