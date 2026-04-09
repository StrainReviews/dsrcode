#!/bin/bash
# Stop dsrcode daemon (Discord Rich Presence for Claude Code)
# WARNING: Linux support is untested. Please report issues on GitHub.

# Configuration -- new paths with fallback to old names during migration
CLAUDE_DIR="$HOME/.claude"
PID_FILE="$CLAUDE_DIR/dsrcode.pid"
OLD_PID_FILE="$CLAUDE_DIR/discord-presence.pid"
SESSIONS_DIR="$CLAUDE_DIR/dsrcode-sessions"
OLD_SESSIONS_DIR="$CLAUDE_DIR/discord-presence-sessions"
REFCOUNT_FILE="$CLAUDE_DIR/dsrcode.refcount"
OLD_REFCOUNT_FILE="$CLAUDE_DIR/discord-presence.refcount"

# Detect platform
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
IS_WINDOWS=false
case "$OS" in
    mingw*|msys*|cygwin*) IS_WINDOWS=true ;;
esac

# Cross-platform process operations
process_exists() {
    local pid=$1
    if $IS_WINDOWS; then
        tasklist //FI "PID eq $pid" 2>/dev/null | grep -q "$pid"
    else
        kill -0 "$pid" 2>/dev/null
    fi
}

kill_process() {
    local pid=$1
    if $IS_WINDOWS; then
        taskkill //F //PID "$pid" >/dev/null 2>&1
    else
        # SIGTERM first, wait up to 5 seconds, then SIGKILL
        kill "$pid" 2>/dev/null
        local waited=0
        while kill -0 "$pid" 2>/dev/null && [[ $waited -lt 5 ]]; do
            sleep 1
            waited=$((waited + 1))
        done
        if kill -0 "$pid" 2>/dev/null; then
            kill -9 "$pid" 2>/dev/null
        fi
    fi
}

# Session tracking: Windows uses refcount, Unix uses PID files
if $IS_WINDOWS; then
    # Try new refcount file first, fall back to old
    ACTUAL_REFCOUNT=""
    if [[ -f "$REFCOUNT_FILE" ]]; then
        ACTUAL_REFCOUNT="$REFCOUNT_FILE"
    elif [[ -f "$OLD_REFCOUNT_FILE" ]]; then
        ACTUAL_REFCOUNT="$OLD_REFCOUNT_FILE"
    fi

    CURRENT_COUNT=1
    if [[ -n "$ACTUAL_REFCOUNT" ]]; then
        CURRENT_COUNT=$(cat "$ACTUAL_REFCOUNT" 2>/dev/null || echo "1")
    fi
    ACTIVE_SESSIONS=$((CURRENT_COUNT - 1))
    [[ $ACTIVE_SESSIONS -lt 0 ]] && ACTIVE_SESSIONS=0

    if [[ $ACTIVE_SESSIONS -gt 0 ]]; then
        # Write decremented count to new path (migrate forward)
        echo "$ACTIVE_SESSIONS" > "$REFCOUNT_FILE"
        # Clean up old file if it was the source
        [[ "$ACTUAL_REFCOUNT" = "$OLD_REFCOUNT_FILE" ]] && rm -f "$OLD_REFCOUNT_FILE"
        echo "dsrcode still in use by $ACTIVE_SESSIONS session(s)"
        exit 0
    fi
    # No sessions remain -- clean up both refcount files
    rm -f "$REFCOUNT_FILE" "$OLD_REFCOUNT_FILE"
else
    SESSION_PID="${PPID:-$$}"
    # Remove session file from both new and old directories
    rm -f "$SESSIONS_DIR/$SESSION_PID"
    rm -f "$OLD_SESSIONS_DIR/$SESSION_PID"

    # Count remaining active sessions across both directories
    ACTIVE_SESSIONS=0
    for dir in "$SESSIONS_DIR" "$OLD_SESSIONS_DIR"; do
        if [[ -d "$dir" ]]; then
            for session_file in "$dir"/*; do
                [[ -f "$session_file" ]] || continue
                pid=$(basename "$session_file")
                if process_exists "$pid"; then
                    ACTIVE_SESSIONS=$((ACTIVE_SESSIONS + 1))
                else
                    rm -f "$session_file"
                fi
            done
        fi
    done

    if [[ $ACTIVE_SESSIONS -gt 0 ]]; then
        echo "dsrcode still in use by $ACTIVE_SESSIONS session(s)"
        exit 0
    fi
    # No sessions remain -- clean up both directories
    rm -rf "$SESSIONS_DIR" "$OLD_SESSIONS_DIR"
fi

# Stop the daemon
# Try new PID file first, then old
ACTUAL_PID_FILE=""
if [[ -f "$PID_FILE" ]]; then
    ACTUAL_PID_FILE="$PID_FILE"
elif [[ -f "$OLD_PID_FILE" ]]; then
    ACTUAL_PID_FILE="$OLD_PID_FILE"
fi

if [[ -n "$ACTUAL_PID_FILE" ]]; then
    PID=$(cat "$ACTUAL_PID_FILE")
    if process_exists "$PID"; then
        kill_process "$PID"
        echo "dsrcode stopped (PID: $PID)"
    fi
    # Clean up both PID files
    rm -f "$PID_FILE" "$OLD_PID_FILE"
else
    # Fallback: kill by process name
    if $IS_WINDOWS; then
        taskkill //F //IM "dsrcode.exe" >/dev/null 2>&1 || true
        # Also try old name during migration period
        taskkill //F //IM "cc-discord-presence-windows-amd64.exe" >/dev/null 2>&1 || true
    else
        pkill -f "dsrcode" 2>/dev/null || true
        # Also try old name during migration period
        pkill -f "cc-discord-presence" 2>/dev/null || true
    fi
fi
