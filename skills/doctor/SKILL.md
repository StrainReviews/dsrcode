---
name: dsrcode-doctor
description: Run diagnostics across 7 categories
---

# /dsrcode:doctor -- Diagnostics

You run a comprehensive diagnostic check of the DSR Code Presence installation, inspired by `flutter doctor`. Display results with status icons: [OK], [!] (warning), [X] (error).

## Category 1: Binary & Daemon
1. Check binary exists: `ls ~/.claude/bin/cc-discord-presence-* 2>/dev/null`
   - Found: [OK] Binary found at {path}
   - Not found: [X] Binary not found. Run `/clear` to trigger auto-download.
2. Check binary version: `~/.claude/bin/cc-discord-presence-* --version 2>/dev/null`
   - Matches expected: [OK] Version {version}
   - Mismatch: [!] Version {version}, expected latest. Run `/dsrcode:update`.
3. Check daemon running: `curl -sf http://127.0.0.1:19460/health 2>/dev/null`
   - Running: [OK] Daemon running (uptime: {uptime})
   - Not running: [X] Daemon not running. Run `/clear` to restart.

## Category 2: Discord Desktop
1. Check Discord process:
   - Windows: `tasklist | grep -i discord`
   - macOS: `pgrep -x Discord`
   - Linux: `pgrep -x discord`
2. If running: [OK] Discord desktop is running
3. If not: [X] Discord desktop not detected. Start Discord and restart the daemon.
4. Check Discord IPC connection: from GET /status discordConnected field
   - Connected: [OK] Discord IPC connected
   - Not connected: [!] Discord IPC not connected. Try restarting both Discord and the daemon.

## Category 3: Hooks & Activity Tracking
1. Read hooks/hooks.json from plugin root: verify 4 HTTP hooks exist
   - All HTTP: [OK] 4 HTTP hooks configured (PreToolUse, UserPromptSubmit, Stop, Notification)
   - Missing hooks: [!] Only {n} hooks found, expected 4
   - Command type detected: [X] Legacy command hooks detected. Hooks should be HTTP type. Run `/dsrcode:setup` to reconfigure.
2. Check hook stats from GET /status: hookStats.total
   - total > 0: [OK] {total} hooks received (last: {lastReceivedAt})
   - total == 0: [!] No hooks received yet. Activity tracking may not be working.

## Category 4: Statusline & Token/Cost
1. From GET /status sessions: check if any session has model, totalTokens, or totalCostUsd set
   - Has data: [OK] Statusline data flowing (model: {model}, tokens: {tokens})
   - No data: [!] No statusline data. This populates over time as you use Claude Code.

## Category 5: Config
1. Check config file: `cat ~/.claude/discord-presence-config.json 2>/dev/null`
   - Exists: [OK] Config found. Preset: {preset}, Display: {displayDetail}
   - Missing: [!] No config file. Using defaults. Run `/dsrcode:setup` to configure.
2. Validate JSON: try parsing the config file
   - Valid: [OK] Config is valid JSON
   - Invalid: [X] Config has JSON syntax errors. Run `/dsrcode:setup` to regenerate.

## Category 6: Sessions
1. From GET /status sessions array:
   - Has sessions: [OK] {n} active session(s)
   - Empty: [!] No sessions registered. Hooks may not be firing.
2. For each session, show: project, status, lastActivityAt

## Category 7: Platform
1. Detect OS: `uname -s`
2. Detect architecture: `uname -m`
3. Check required tools: curl, bash
4. Check optional tools: jq, gh (GitHub CLI)
5. Report: [OK] {OS} {arch}, curl available, bash available

## Summary
Count: {ok} OK, {warnings} warnings, {errors} errors
If errors > 0: "Run the suggested fixes above, then re-run `/dsrcode:doctor`"
If warnings only: "Everything is functional. Warnings are informational."
If all OK: "Everything looks good! DSR Code Presence is fully operational."
