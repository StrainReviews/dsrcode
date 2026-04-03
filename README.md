# DSR Code - Discord Rich Presence for Claude Code

Show your Claude Code session on Discord! Display your current project, git branch, model, session time, token usage, and cost in real-time.

> Fork of [tsanva/cc-discord-presence](https://github.com/tsanva/cc-discord-presence) with custom Discord application and icons.

## Features

- **Session Time** - Shows how long you've been coding with Claude
- **Project Name** - Displays the current project you're working on
- **Git Branch** - Shows your current git branch
- **Model Name** - Shows which Claude model you're using (Opus 4.6, Sonnet 4.6, Haiku 4.5)
- **Total Tokens** - Token usage counter (input + output)
- **Total Cost** - Real-time cost tracking for your session

## Installation

### As a Claude Code Plugin (Recommended)

```bash
claude plugin marketplace add StrainReviews/cc-discord-presence
claude plugin install cc-discord-presence@cc-discord-presence
```

The plugin will automatically start when you begin a Claude Code session and stop when you exit.

### Manual Installation

```bash
git clone https://github.com/StrainReviews/cc-discord-presence.git
cd cc-discord-presence
go build -o cc-discord-presence .
./cc-discord-presence
```

## How It Works

The app reads session data from Claude Code in two ways:

### 1. JSONL Fallback (Zero Config)

Parses Claude Code's session files from `~/.claude/projects/`. Works out of the box.

### 2. Statusline Integration (More Accurate)

For the most accurate token/cost data, configure the statusline integration:

```bash
./scripts/setup-statusline.sh
```

Restart Claude Code after setup.

## Platform Support

| Platform | Status |
|----------|--------|
| Windows (x64) | Tested |
| macOS (Apple Silicon) | Tested |
| macOS (Intel) | Untested |
| Linux (x64) | Untested |

## Requirements

- [Discord](https://discord.com) desktop app running
- [Claude Code](https://claude.ai/code) installed
- Go 1.25+ (only for building from source)

## License

MIT License - see [LICENSE](LICENSE) for details.

Based on [cc-discord-presence](https://github.com/tsanva/cc-discord-presence) by Vasant Paradissa Nuno Sakti.
