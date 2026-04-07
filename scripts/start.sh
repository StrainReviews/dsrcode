#!/bin/bash
# Start Discord Rich Presence daemon
# WARNING: Linux support is untested. Please report issues on GitHub.

set -e

# Configuration
CLAUDE_DIR="$HOME/.claude"
BIN_DIR="$CLAUDE_DIR/bin"
PID_FILE="$CLAUDE_DIR/discord-presence.pid"
LOG_FILE="$CLAUDE_DIR/discord-presence.log"
SESSIONS_DIR="$CLAUDE_DIR/discord-presence-sessions"
REFCOUNT_FILE="$CLAUDE_DIR/discord-presence.refcount"
REPO="StrainReviews/dsrcode"
VERSION="v3.2.0"

# Detect platform
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
IS_WINDOWS=false
case "$OS" in
    mingw*|msys*|cygwin*) IS_WINDOWS=true; OS="windows" ;;
esac

# Cross-platform process check
process_exists() {
    local pid=$1
    if $IS_WINDOWS; then
        tasklist //FI "PID eq $pid" 2>/dev/null | grep -q "$pid"
    else
        kill -0 "$pid" 2>/dev/null
    fi
}

# Ensure directories exist
mkdir -p "$CLAUDE_DIR" "$BIN_DIR" "$SESSIONS_DIR"

# Session tracking: Windows uses refcount (PPID unreliable), Unix uses PID files
if $IS_WINDOWS; then
    CURRENT_COUNT=$(cat "$REFCOUNT_FILE" 2>/dev/null || echo "0")
    ACTIVE_SESSIONS=$((CURRENT_COUNT + 1))
    echo "$ACTIVE_SESSIONS" > "$REFCOUNT_FILE"
else
    SESSION_PID="${PPID:-$$}"
    echo "$SESSION_PID" > "$SESSIONS_DIR/$SESSION_PID"

    # Count active sessions and clean up orphans
    ACTIVE_SESSIONS=0
    for session_file in "$SESSIONS_DIR"/*; do
        [[ -f "$session_file" ]] || continue
        pid=$(basename "$session_file")
        if process_exists "$pid"; then
            ACTIVE_SESSIONS=$((ACTIVE_SESSIONS + 1))
        else
            rm -f "$session_file"
        fi
    done
fi

# If daemon is already running, just exit
if [[ -f "$PID_FILE" ]]; then
    OLD_PID=$(cat "$PID_FILE")
    if process_exists "$OLD_PID"; then
        echo "Discord Rich Presence already running (PID: $OLD_PID, sessions: $ACTIVE_SESSIONS)"
        exit 0
    fi
fi

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
esac

BINARY_NAME="cc-discord-presence-${OS}-${ARCH}"
if [[ "$OS" == "windows" ]]; then
    BINARY_NAME="${BINARY_NAME}.exe"
fi
BINARY="$BIN_DIR/$BINARY_NAME"

# Resolve source directory for local builds
# Priority: plugin root (has go.mod), then ~/Projects/cc-discord-presence
SOURCE_DIR=""
for candidate in "$ROOT" "$HOME/Projects/cc-discord-presence"; do
    if [[ -f "$candidate/go.mod" ]]; then
        SOURCE_DIR="$candidate"
        break
    fi
done
ROOT="${CLAUDE_PLUGIN_ROOT:-$HOME/.claude/plugins/marketplaces/dsrcode}"

# Build from source helper
build_from_source() {
    if [[ -z "$SOURCE_DIR" ]]; then
        echo "Error: No Go source found (checked plugin root and ~/Projects/cc-discord-presence)" >&2
        exit 1
    fi
    if ! command -v go &> /dev/null; then
        echo "Error: Go compiler required to build cc-discord-presence" >&2
        exit 1
    fi
    echo "Building cc-discord-presence from source ($SOURCE_DIR)..."
    LDFLAGS="-X main.Version=${VERSION#v}"
    (cd "$SOURCE_DIR" && go build -ldflags "$LDFLAGS" -o "$BINARY" .) 2>&1
    if ! $IS_WINDOWS; then
        chmod +x "$BINARY"
    fi
    echo "Built successfully!"
}

# Build binary if not present
if [[ ! -f "$BINARY" ]]; then
    build_from_source
fi

# Version check: rebuild if binary version doesn't match
if [[ -f "$BINARY" ]]; then
    CURRENT_VERSION=$("$BINARY" --version 2>/dev/null | awk '{print $2}' || echo "unknown")
    CURRENT_NORMALIZED="${CURRENT_VERSION#v}"
    EXPECTED_NORMALIZED="${VERSION#v}"
    if [[ "$CURRENT_NORMALIZED" != "" && "$CURRENT_NORMALIZED" != "$EXPECTED_NORMALIZED" && "$CURRENT_NORMALIZED" != "unknown" ]]; then
        echo "Updating cc-discord-presence from $CURRENT_VERSION to $VERSION..."
        # Kill existing daemon before replacing binary
        if [[ -f "$PID_FILE" ]]; then
            OLD_PID=$(cat "$PID_FILE")
            if process_exists "$OLD_PID"; then
                kill "$OLD_PID" 2>/dev/null || true
                sleep 1
            fi
            rm -f "$PID_FILE"
        fi
        rm -f "$BINARY"
        build_from_source
    fi
fi

if [[ ! -f "$BINARY" ]]; then
    echo "Error: Binary not found at $BINARY" >&2
    exit 1
fi

# Auto-patch hooks.json per D-14: add Agent to PreToolUse and SubagentStop section
patch_hooks_json() {
    local HOOKS_FILE
    # Try plugin root first, then fallback locations
    for candidate in "$ROOT/hooks/hooks.json" "$SOURCE_DIR/hooks/hooks.json"; do
        if [[ -f "$candidate" ]]; then
            HOOKS_FILE="$candidate"
            break
        fi
    done

    if [[ -z "$HOOKS_FILE" || ! -f "$HOOKS_FILE" ]]; then
        return 0
    fi

    local NEEDS_UPDATE=false

    # Check if Agent is in PreToolUse matcher
    if ! grep -q '"Agent"' "$HOOKS_FILE" && ! grep -q '|Agent' "$HOOKS_FILE"; then
        NEEDS_UPDATE=true
    fi

    # Check if SubagentStop section exists
    if ! grep -q '"SubagentStop"' "$HOOKS_FILE"; then
        NEEDS_UPDATE=true
    fi

    if $NEEDS_UPDATE; then
        echo "Patching hooks.json: adding Agent matcher and SubagentStop..."

        if command -v node &>/dev/null; then
            node -e "
                const fs = require('fs');
                const hooks = JSON.parse(fs.readFileSync('$HOOKS_FILE', 'utf8'));

                // Add Agent to PreToolUse matcher
                if (hooks.hooks && hooks.hooks.PreToolUse && hooks.hooks.PreToolUse[0]) {
                    const matcher = hooks.hooks.PreToolUse[0].matcher;
                    if (!matcher.includes('Agent')) {
                        hooks.hooks.PreToolUse[0].matcher = matcher + '|Agent';
                    }
                }

                // Add SubagentStop section if missing
                if (hooks.hooks && !hooks.hooks.SubagentStop) {
                    hooks.hooks.SubagentStop = [{
                        matcher: '*',
                        hooks: [{
                            type: 'http',
                            url: 'http://127.0.0.1:19460/hooks/subagent-stop',
                            timeout: 1
                        }]
                    }];
                }

                fs.writeFileSync('$HOOKS_FILE', JSON.stringify(hooks, null, 2) + '\n');
                console.log('hooks.json patched successfully');
            " 2>/dev/null || true
        fi
    fi
}

patch_hooks_json

# Auto-migrate german preset to professional + lang=de per D-33
migrate_german_preset() {
    local CONFIG_FILE="$CLAUDE_DIR/discord-presence-config.json"

    if [[ ! -f "$CONFIG_FILE" ]]; then
        return 0
    fi

    if grep -q '"preset"[[:space:]]*:[[:space:]]*"german"' "$CONFIG_FILE"; then
        echo "Migrating german preset to professional + lang=de..."
        if command -v node &>/dev/null; then
            node -e "
                const fs = require('fs');
                const config = JSON.parse(fs.readFileSync('$CONFIG_FILE', 'utf8'));
                config.preset = 'professional';
                config.lang = 'de';
                fs.writeFileSync('$CONFIG_FILE', JSON.stringify(config, null, 2) + '\n');
                console.log('Config migrated: german -> professional + lang=de');
            " 2>/dev/null || true
        fi
    fi
}

migrate_german_preset

# Start the daemon in background
if $IS_WINDOWS; then
    # On Windows, convert path to Windows format and use PowerShell
    WIN_BINARY=$(cygpath -w "$BINARY" 2>/dev/null || echo "$BINARY")
    WIN_PID_FILE=$(cygpath -w "$PID_FILE" 2>/dev/null || echo "$PID_FILE")

    # Use PowerShell to start the process and capture PID (hidden window)
    powershell.exe -NoProfile -WindowStyle Hidden -Command '$process = Start-Process -FilePath "'"$WIN_BINARY"'" -WindowStyle Hidden -PassThru; $process.Id | Out-File -FilePath "'"$WIN_PID_FILE"'" -Encoding ASCII -NoNewline' 2>/dev/null
else
    nohup "$BINARY" > "$LOG_FILE" 2>&1 &
    echo $! > "$PID_FILE"
fi

# Wait for HTTP server to be ready (max 5 seconds)
for i in $(seq 1 50); do
    if curl -sf http://127.0.0.1:19460/health > /dev/null 2>&1; then
        break
    fi
    sleep 0.1
done

echo "Discord Rich Presence started (PID: $(cat "$PID_FILE" 2>/dev/null || echo "unknown"), sessions: $ACTIVE_SESSIONS)"

# First-run hint: suggest /dsrcode:setup if no config exists per D-36
CONFIG_FILE="$CLAUDE_DIR/discord-presence-config.json"
if [[ ! -f "$CONFIG_FILE" ]]; then
    echo "DSR Code gestartet (Preset: minimal) -- /dsrcode:setup fuer Anpassungen"
fi
