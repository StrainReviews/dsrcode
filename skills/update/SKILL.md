---
name: update
description: Update binary to latest GitHub release
user-invocable: true
---

# /dsrcode:update -- Binary Updater

You help the user update the DSR Code Presence binary to the latest version.

## Steps

1. Check current version: `~/.claude/bin/cc-discord-presence-* --version 2>/dev/null`
   - If binary not found: "No binary found. Run `/clear` to trigger auto-download."
2. Check latest release:
   ```bash
   curl -sf https://api.github.com/repos/DSR-Labs/cc-discord-presence/releases/latest | grep -o '"tag_name": *"[^"]*"' | head -1
   ```
   - Alternative with gh CLI: `gh release view --repo DSR-Labs/cc-discord-presence --json tagName -q '.tagName' 2>/dev/null`
3. Compare versions:
   - Same: "You're already on the latest version ({version})."
   - Different: "Update available: {current} -> {latest}"
4. If update available, ask: "Update now? (y/n)"
5. If yes:
   a. Stop daemon:
      ```bash
      curl -sf -X POST http://127.0.0.1:19460/shutdown 2>/dev/null
      sleep 1
      ```
      Or kill by PID: `kill $(cat ~/.claude/discord-presence.pid) 2>/dev/null`
   b. Detect platform:
      ```bash
      OS=$(uname -s | tr '[:upper:]' '[:lower:]')
      ARCH=$(uname -m)
      case "$ARCH" in x86_64) ARCH="amd64" ;; aarch64|arm64) ARCH="arm64" ;; esac
      ```
   c. Download new binary:
      ```bash
      BINARY_NAME="cc-discord-presence-${OS}-${ARCH}"
      curl -fsSL "https://github.com/DSR-Labs/cc-discord-presence/releases/download/{version}/${BINARY_NAME}" \
        -o ~/.claude/bin/${BINARY_NAME}
      chmod +x ~/.claude/bin/${BINARY_NAME}
      ```
   d. Verify: `~/.claude/bin/cc-discord-presence-* --version`
   e. Restart daemon: run the binary in background
   f. Report: "Updated to {version} and restarted."

## Error Handling
- No internet: "Cannot reach GitHub. Check your connection."
- No releases: "No releases found for DSR-Labs/cc-discord-presence. The first release may not have been published yet."
- Download fails: "Download failed. Try manually: `gh release download --repo DSR-Labs/cc-discord-presence`"
- Permission denied: "Cannot write to ~/.claude/bin/. Check directory permissions."
