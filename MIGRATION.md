# Migration Guide: v3.x to v4.0.0

## What Changed in v4.0.0

v4.0.0 renames the project from `cc-discord-presence` to `dsrcode`. This is a breaking change affecting binary names, config file paths, and the Go module path.

### Binary Name
- **Old:** `cc-discord-presence-{os}-{arch}[.exe]`
- **New:** `dsrcode[.exe]`
- **Location:** `${CLAUDE_PLUGIN_DATA}/bin/` (was `~/.claude/bin/`)

### Runtime Files
| Old Name | New Name |
|----------|----------|
| `~/.claude/discord-presence-config.json` | `~/.claude/dsrcode-config.json` |
| `~/.claude/discord-presence-data.json` | `~/.claude/dsrcode-data.json` |
| `~/.claude/discord-presence.log` | `~/.claude/dsrcode.log` |
| `~/.claude/discord-presence.pid` | `~/.claude/dsrcode.pid` |
| `~/.claude/discord-presence.refcount` | `~/.claude/dsrcode.refcount` |
| `~/.claude/discord-presence-sessions/` | `~/.claude/dsrcode-sessions/` |
| `~/.claude/discord-presence-analytics/` | `~/.claude/dsrcode-analytics/` |

### Go Module Path
- **Old:** `github.com/tsanva/cc-discord-presence`
- **New:** `github.com/StrainReviews/dsrcode`

### Plugin Install Command
- **Old:** `claude plugin install DSR-Labs/cc-discord-presence`
- **New:** `claude plugin install StrainReviews/dsrcode`

## What Auto-Migrates

The following migrations happen automatically on first run of v4.0.0:

1. **Config file:** The daemon checks for `dsrcode-config.json` first. If not found, it reads `discord-presence-config.json`, copies it to `dsrcode-config.json`, and logs a warning. The old file is kept as a backup.
2. **Old binary:** `start.sh` detects old binaries at `~/.claude/bin/cc-discord-presence-*`, stops the old daemon, and moves the binary to the new location.
3. **PID/session files:** Stop scripts check both old and new PID file paths.

## What Needs Manual Action

1. **Rename your config file** (optional, old name works for 6 months):
   ```bash
   mv ~/.claude/discord-presence-config.json ~/.claude/dsrcode-config.json
   ```

2. **Update custom scripts** referencing the old binary name or paths.

3. **Update Go imports** if you depend on this module:
   ```bash
   go get github.com/StrainReviews/dsrcode@v4.0.0
   ```

## Backward Compatibility

Old file names are supported throughout v4.x (approximately 6 months). v5.0 will remove fallback support for old names.
