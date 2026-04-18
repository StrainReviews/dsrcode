---
description: View daemon status, Discord connection, and sessions
---

# /dsrcode:status -- Status Display

You display the current DSR Code Presence status.

## Self-Triggering Hook (D-30)
Before fetching status, run a quick command to trigger the PreToolUse hook, which identifies this session:
```bash
echo "dsrcode-status-trigger" > /dev/null
```
This Bash call triggers the PreToolUse hook, which sends session_id to the server. This allows the server to identify which session belongs to the current Claude Code instance.

## Steps

1. Trigger self-identifying hook (see above)
2. Fetch status: `curl -sf http://127.0.0.1:19460/status`
3. Parse the JSON response and display in a readable format:

### Daemon
- **Version:** {version}
- **Uptime:** {uptime}
- **Discord:** {discordConnected ? "Connected" : "Disconnected"}

### Config
- **Preset:** {config.preset}
- **Display Detail:** {config.displayDetail}
- **Port:** {config.port}
- **Bind Address:** {config.bindAddr}

### Sessions ({sessions.length} active)
For each session:
- **Project:** {projectName from cwd} ({branch})
- **Status:** {status} (active/idle)
- **Model:** {model}
- **Tokens:** {totalTokens} | **Cost:** ${totalCostUsd}
- **Last Activity:** {lastActivityAt}
- **Last File:** {lastFile}
- **Last Command:** {lastCommand}
- Mark the current session with "[this session]" (match by cwd + most recent lastActivityAt)

### Hook Statistics
- **Total hooks received:** {hookStats.total}
- **By type:** list each type with count
- **Last received:** {hookStats.lastReceivedAt}

## Error Handling
- If daemon not running: "Daemon is not running. It starts automatically with each Claude Code session. Run `/clear` to trigger a SessionStart."
- If no sessions: "No active sessions. This is normal if you just started -- hooks will register the session as you work."
- If discordConnected is false: "Discord is not connected. Make sure the Discord desktop app is running."
