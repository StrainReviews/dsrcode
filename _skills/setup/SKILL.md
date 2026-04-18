---
name: setup
description: Guided first-time setup wizard (7 phases)
---

# /dsrcode:setup -- Guided Setup Wizard

You are guiding the user through DSR Code Presence setup. Follow these phases IN ORDER. Use Bash tool for checks and Read/Write for config files.

## Phase A: Prerequisites
1. Check binary exists:
   - Primary: `ls "${CLAUDE_PLUGIN_DATA:-$HOME/.claude/plugins/data/dsrcode}"/bin/dsrcode* 2>/dev/null`
   - Fallback (migration period): `ls ~/.claude/bin/cc-discord-presence-* 2>/dev/null`
2. If not found, inform user: "Binary not found. It will be downloaded automatically on next session start. Run `/clear` to trigger SessionStart hook."
3. Check if daemon is running: `curl -sf http://127.0.0.1:19460/health 2>/dev/null`
4. If not running, try to start: find binary and run it in background
5. Check Discord desktop is running (platform-specific process check)

## Phase B: Discord Connection Test
1. Call `curl -sf http://127.0.0.1:19460/health`
2. If status is "ok", report "Discord daemon is running"
3. Call `curl -sf http://127.0.0.1:19460/status` to check discordConnected
4. If discordConnected is false, advise: "Make sure Discord desktop app is running and restart the daemon"

## Phase C: Base Configuration
1. Show the 8 available presets by calling: `curl -sf http://127.0.0.1:19460/presets`
2. Display each preset with name, description, and 3 sample messages
3. Ask user to choose a preset (default: minimal)
4. Ask user for Discord Application ID (show where to find it: Discord Developer Portal > Applications)
5. If user doesn't have one, explain: "You need a Discord Application for custom icons. Use the default DSR Code app ID or create your own at https://discord.com/developers/applications"

## Phase D: Advanced Settings (Optional)
Ask: "Would you like to configure advanced settings? (port, bind address, log level, timeouts, display detail, buttons)"
If yes:
1. Port (default 19460)
2. Bind address (default 127.0.0.1)
3. Log level (debug/info/warn/error, default info)
4. Display detail (minimal/standard/verbose/private, default minimal) -- explain each level:
   - **minimal**: Shows project name only. Files, commands, and queries are hidden.
   - **standard**: Shows file names (truncated to 20 chars), shortened commands, and search patterns.
   - **verbose**: Shows full relative paths, full commands, and full project names.
   - **private**: Hides all identifying information (file, project, branch, tokens, cost).
5. Idle timeout (default 10m)
6. Buttons (up to 2 clickable links, label + URL)
If no: skip to Phase E

## Phase E: Hook Diagnosis
1. Read hooks/hooks.json from plugin root and verify all 4 hooks are HTTP type
2. Check each hook fires correctly:
   - PreToolUse: Should be HTTP type with url `http://127.0.0.1:19460/hooks/pre-tool-use`
   - UserPromptSubmit: Should be HTTP type with url `http://127.0.0.1:19460/hooks/user-prompt-submit`
   - Stop: Should be HTTP type with url `http://127.0.0.1:19460/hooks/stop`
   - Notification: Should be HTTP type with url `http://127.0.0.1:19460/hooks/notification`
3. Check statusline is working: `curl -sf http://127.0.0.1:19460/health`
4. Report hook status

## Phase F: Write Config + Restart Daemon
1. Build config JSON from user choices:
   ```json
   {
     "discordClientId": "USER_CHOSEN_OR_DEFAULT",
     "preset": "chosen-preset",
     "port": 19460,
     "bindAddr": "127.0.0.1",
     "logLevel": "info",
     "displayDetail": "minimal",
     "idleTimeout": "10m",
     "buttons": []
   }
   ```
2. Write to ~/.claude/dsrcode-config.json using Write tool
3. The daemon will hot-reload the config automatically (fsnotify watches the config dir)
4. Wait 2 seconds, then verify reload: `curl -sf http://127.0.0.1:19460/status`

## Phase G: Verification + Summary
1. Call GET /status and display:
   - Version, uptime
   - Discord connected status
   - Active preset
   - Display detail level
   - Number of active sessions
2. Show "Setup complete!" summary with chosen settings
3. Suggest: "Try `/dsrcode:status` to see your current status, or `/dsrcode:demo` to preview how it looks on Discord"

## Error Handling
- If daemon never starts: "The daemon could not be started. Check the log file at ~/.claude/dsrcode.log for errors."
- If Discord not connected: "Discord desktop must be running before the daemon starts. Start Discord, then run `/clear` to restart the daemon."
- If config write fails: "Could not write config file. Check permissions on ~/.claude/ directory."
