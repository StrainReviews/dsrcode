---
name: log
description: View recent daemon log entries
user-invocable: true
---

# /dsrcode:log -- Log Viewer

You display recent log entries from the DSR Code Presence daemon.

## Steps

1. Read the log file: `tail -50 ~/.claude/discord-presence.log 2>/dev/null`
2. If file doesn't exist or is empty: "No log file found at ~/.claude/discord-presence.log. The daemon may not have started yet."
3. Display the last 50 lines formatted for readability:
   - Highlight ERROR level entries (prefix with [ERROR])
   - Highlight WARN level entries (prefix with [WARN])
   - Show timestamps in a readable format
4. Ask: "Show more? (Enter a number for more lines, or 'all' for the full log)"
5. If user wants more: `tail -N ~/.claude/discord-presence.log` or `cat ~/.claude/discord-presence.log`
6. If user asks to filter: `grep "PATTERN" ~/.claude/discord-presence.log`

## Tips
- For real-time monitoring, suggest: set logLevel to "debug" via `/dsrcode:setup` advanced settings
- To clear the log: `> ~/.claude/discord-presence.log`
- Log file location: `~/.claude/discord-presence.log` (configurable via `logFile` in config)
- Common things to look for:
  - "discord" entries for connection issues
  - "hook" entries for activity tracking issues
  - "config" entries for configuration reload events
  - "session" entries for session lifecycle events
