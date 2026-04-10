#!/bin/bash
# Start dsrcode daemon (Discord Rich Presence for Claude Code)
# Downloads pre-built binary from GitHub Releases, falls back to go build.
# WARNING: Linux support is untested. Please report issues on GitHub.

# ---- Configuration ----
CLAUDE_DIR="$HOME/.claude"
REPO="StrainReviews/dsrcode"
VERSION="v4.0.0"

# Binary storage: CLAUDE_PLUGIN_DATA (official, persistent) with fallback per DIST-19
PLUGIN_DATA="${CLAUDE_PLUGIN_DATA:-$HOME/.claude/plugins/data/dsrcode}"
BIN_DIR="$PLUGIN_DATA/bin"

# Runtime files use new dsrcode-* naming per DIST-29
PID_FILE="$CLAUDE_DIR/dsrcode.pid"
LOG_FILE="$CLAUDE_DIR/dsrcode.log"
SESSIONS_DIR="$CLAUDE_DIR/dsrcode-sessions"
REFCOUNT_FILE="$CLAUDE_DIR/dsrcode.refcount"

# ---- Platform Detection ----
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
IS_WINDOWS=false
case "$OS" in
    mingw*|msys*|cygwin*) IS_WINDOWS=true; OS="windows" ;;
    darwin) ;;
    linux) ;;
    *)
        echo "Error: Unsupported OS: $OS" >&2
        echo "Supported: macOS, Linux, Windows (Git Bash/MSYS2)" >&2
        exit 1
        ;;
esac

ARCH="$(uname -m)"
case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *)
        echo "Error: Unsupported architecture: $ARCH" >&2
        exit 1
        ;;
esac

# ---- Binary Name Construction ----
# Archive: dsrcode_X.Y.Z_os_arch.tar.gz (or .zip for Windows)
# Binary inside archive: dsrcode (or dsrcode.exe)
BINARY_NAME="dsrcode"
if $IS_WINDOWS; then
    BINARY_NAME="dsrcode.exe"
fi
BINARY="$BIN_DIR/$BINARY_NAME"

# ---- Cross-platform Helpers ----
process_exists() {
    local pid=$1
    if $IS_WINDOWS; then
        tasklist //FI "PID eq $pid" 2>/dev/null | grep -q "$pid"
    else
        kill -0 "$pid" 2>/dev/null
    fi
}

# ---- Ensure Directories ----
mkdir -p "$CLAUDE_DIR" "$BIN_DIR" "$SESSIONS_DIR"

# ---- Session Tracking ----
# Windows uses refcount (PPID unreliable), Unix uses PID files
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

# ---- Check Running Daemon ----
if [[ -f "$PID_FILE" ]]; then
    OLD_PID=$(cat "$PID_FILE")
    if process_exists "$OLD_PID"; then
        echo "dsrcode already running (PID: $OLD_PID, sessions: $ACTIVE_SESSIONS)"
        exit 0
    fi
fi

# ---- Lock-File Protection (DIST-26) ----
LOCK_FILE="$BIN_DIR/.dsrcode-update.lock"

acquire_lock() {
    mkdir -p "$(dirname "$LOCK_FILE")"
    if command -v flock &>/dev/null; then
        exec 9>"$LOCK_FILE"
        flock -n 9 || { echo "Another update in progress, waiting..."; flock 9; }
    else
        local attempts=0
        while [[ -f "$LOCK_FILE.pid" ]] && kill -0 "$(cat "$LOCK_FILE.pid" 2>/dev/null)" 2>/dev/null; do
            sleep 1
            attempts=$((attempts + 1))
            [[ $attempts -gt 30 ]] && { echo "Lock timeout" >&2; rm -f "$LOCK_FILE.pid"; break; }
        done
        echo $$ > "$LOCK_FILE.pid"
    fi
}

release_lock() {
    if command -v flock &>/dev/null; then
        exec 9>&-
    fi
    rm -f "$LOCK_FILE.pid"
}

# ---- Download Function with SHA256 Verification ----
download_binary() {
    local version_no_v="${VERSION#v}"
    local archive_name="dsrcode_${version_no_v}_${OS}_${ARCH}"
    local checksums_name="dsrcode_${version_no_v}_checksums.txt"

    if [[ "$OS" == "windows" ]]; then
        archive_name="${archive_name}.zip"
    else
        archive_name="${archive_name}.tar.gz"
    fi

    local download_url="https://github.com/${REPO}/releases/download/${VERSION}/${archive_name}"
    local checksums_url="https://github.com/${REPO}/releases/download/${VERSION}/${checksums_name}"

    if ! command -v curl &>/dev/null; then
        echo "Warning: curl not found, skipping download" >&2
        return 1
    fi

    echo "Downloading dsrcode ${VERSION} for ${OS}-${ARCH}..."

    local tmp_dir
    tmp_dir=$(mktemp -d)
    trap "rm -rf '$tmp_dir'" RETURN

    # Download archive
    if ! curl -fsSL -o "$tmp_dir/archive" "$download_url" 2>/dev/null; then
        echo "Warning: Download failed." >&2
        echo "If behind a proxy, set HTTP_PROXY/HTTPS_PROXY." >&2
        return 1
    fi

    # Verify size (HTML error pages are <10KB, real archives are >1MB)
    local file_size
    file_size=$(wc -c < "$tmp_dir/archive" 2>/dev/null | tr -d ' ')
    if [[ -z "$file_size" ]] || [[ "$file_size" -lt 100000 ]]; then
        echo "Warning: Downloaded file too small (${file_size:-0} bytes)" >&2
        return 1
    fi

    # Download and verify SHA256 checksum
    if curl -fsSL -o "$tmp_dir/checksums.txt" "$checksums_url" 2>/dev/null; then
        local expected_hash
        expected_hash=$(grep "${archive_name}" "$tmp_dir/checksums.txt" | awk '{print $1}')
        if [[ -n "$expected_hash" ]]; then
            local actual_hash
            if command -v sha256sum &>/dev/null; then
                actual_hash=$(sha256sum "$tmp_dir/archive" | awk '{print $1}')
            elif command -v shasum &>/dev/null; then
                actual_hash=$(shasum -a 256 "$tmp_dir/archive" | awk '{print $1}')
            fi
            if [[ -n "$actual_hash" && "$actual_hash" != "$expected_hash" ]]; then
                echo "ERROR: SHA256 checksum mismatch!" >&2
                echo "  Expected: $expected_hash" >&2
                echo "  Got:      $actual_hash" >&2
                return 1
            fi
        fi
    fi

    # Extract binary from archive
    mkdir -p "$BIN_DIR"
    if [[ "$OS" == "windows" ]]; then
        unzip -o "$tmp_dir/archive" "dsrcode.exe" -d "$tmp_dir/extract" 2>/dev/null || return 1
        mv "$tmp_dir/extract/dsrcode.exe" "$BINARY"
    else
        tar -xzf "$tmp_dir/archive" -C "$tmp_dir" "dsrcode" 2>/dev/null || return 1
        mv "$tmp_dir/dsrcode" "$BINARY"
        chmod +x "$BINARY"
    fi

    echo "Downloaded and verified successfully!"
    return 0
}

# ---- Build-from-Source Fallback ----
find_source_dir() {
    local plugin_root="${CLAUDE_PLUGIN_ROOT:-$HOME/.claude/plugins/marketplaces/dsrcode}"
    for candidate in "$plugin_root" "$HOME/Projects/cc-discord-presence" "$HOME/Projects/dsrcode"; do
        if [[ -f "$candidate/go.mod" ]]; then
            echo "$candidate"
            return 0
        fi
    done
    return 1
}

build_from_source() {
    local source_dir
    source_dir=$(find_source_dir) || {
        return 1
    }
    if ! command -v go &>/dev/null; then
        return 1
    fi
    echo "Building dsrcode from source ($source_dir)..."
    # Use lowercase version per DIST-06 for GoReleaser compat
    local ldflags="-X main.version=${VERSION#v}"
    if (cd "$source_dir" && go build -ldflags "$ldflags" -o "$BINARY" .) 2>&1; then
        if ! $IS_WINDOWS; then
            chmod +x "$BINARY"
        fi
        echo "Built successfully!"
        return 0
    else
        echo "Warning: Build failed" >&2
        return 1
    fi
}

# ---- Install Help Message (DIST-27) ----
show_install_help() {
    echo "" >&2
    echo "==== dsrcode: Installation Failed ====" >&2
    echo "" >&2
    echo "Could not download the binary and could not build from source." >&2
    echo "" >&2
    echo "Option 1: Download manually" >&2
    echo "  https://github.com/${REPO}/releases/tag/${VERSION}" >&2
    echo "  Place the binary in: ${BIN_DIR}/" >&2
    echo "" >&2
    echo "Option 2: Install Go (https://go.dev/dl/) and restart Claude Code" >&2
    echo "" >&2
    echo "If behind a proxy, set HTTP_PROXY/HTTPS_PROXY." >&2
    echo "=================================================" >&2
}

# ---- Acquire Binary: Download First, Build Fallback, Error Last ----
ensure_binary() {
    acquire_lock
    # Re-check after acquiring lock (another process may have installed)
    if [[ -f "$BINARY" ]]; then
        release_lock
        return 0
    fi
    if download_binary; then
        release_lock
        return 0
    fi
    echo "Trying build from source as fallback..."
    if build_from_source; then
        release_lock
        return 0
    fi
    release_lock
    show_install_help
    return 1
}

# ---- Kill Running Daemon ----
kill_daemon_if_running() {
    if [[ -f "$PID_FILE" ]]; then
        local old_pid
        old_pid=$(cat "$PID_FILE")
        if process_exists "$old_pid"; then
            if $IS_WINDOWS; then
                taskkill //F //PID "$old_pid" >/dev/null 2>&1 || true
            else
                kill "$old_pid" 2>/dev/null || true
            fi
            sleep 1
        fi
        rm -f "$PID_FILE"
    fi
    # Also check old PID file path for migration scenarios
    local old_pid_file="$CLAUDE_DIR/discord-presence.pid"
    if [[ -f "$old_pid_file" ]]; then
        local old_pid
        old_pid=$(cat "$old_pid_file")
        if process_exists "$old_pid"; then
            if $IS_WINDOWS; then
                taskkill //F //PID "$old_pid" >/dev/null 2>&1 || true
            else
                kill "$old_pid" 2>/dev/null || true
            fi
            sleep 1
        fi
        rm -f "$old_pid_file"
    fi
}

# ---- Old Binary Migration (DIST-37) ----
migrate_old_binary() {
    local old_bin_dir="$HOME/.claude/bin"
    local migrated=false

    for old_binary in "$old_bin_dir"/cc-discord-presence-*; do
        [[ -f "$old_binary" ]] || continue

        echo "Migrating old binary: $(basename "$old_binary") -> $BIN_DIR/dsrcode"
        kill_daemon_if_running

        mkdir -p "$BIN_DIR"
        cp "$old_binary" "$BINARY"
        if ! $IS_WINDOWS; then
            chmod +x "$BINARY"
        fi

        rm -f "$old_binary"
        migrated=true
        echo "Migration complete."
        break  # Only migrate the first match
    done

    $migrated
}

# ---- Version Check + Binary Acquisition (DIST-21, DIST-22) ----
if [[ ! -f "$BINARY" ]]; then
    # Try migrating old binary first
    if ! migrate_old_binary; then
        # No old binary -- acquire fresh
        if ! ensure_binary; then
            exit 1
        fi
    fi
else
    # Binary exists -- check version
    CURRENT_VERSION=$("$BINARY" --version 2>/dev/null | awk '{print $2}' || echo "unknown")
    CURRENT_NORMALIZED="${CURRENT_VERSION#v}"
    EXPECTED_NORMALIZED="${VERSION#v}"

    # Skip update check for dev/unknown versions per DIST-22
    if [[ "$CURRENT_NORMALIZED" == "dev" || "$CURRENT_NORMALIZED" == "unknown" ]]; then
        echo "dsrcode running dev/local build, skipping version check"
    elif [[ "$CURRENT_NORMALIZED" != "" \
         && "$CURRENT_NORMALIZED" != "$EXPECTED_NORMALIZED" ]]; then
        echo "Updating dsrcode from ${CURRENT_VERSION} to ${VERSION}..."
        # Must kill daemon before replacing binary (Windows locks running .exe)
        kill_daemon_if_running
        rm -f "$BINARY"
        acquire_lock
        if ! download_binary; then
            echo "Trying build from source as fallback..."
            if ! build_from_source; then
                release_lock
                show_install_help
                exit 1
            fi
        fi
        release_lock
    fi
fi

# Final guard
if [[ ! -f "$BINARY" ]]; then
    echo "Error: Binary not found at $BINARY" >&2
    exit 1
fi

# From this point on, fail fast
set -e

# ---- Auto-patch hooks.json (D-14: Agent matcher + SubagentStop) ----
patch_hooks_json() {
    local HOOKS_FILE
    local ROOT="${CLAUDE_PLUGIN_ROOT:-$HOME/.claude/plugins/marketplaces/dsrcode}"
    local SOURCE_DIR
    SOURCE_DIR=$(find_source_dir 2>/dev/null || echo "")

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

# ---- Auto-patch settings.local.json (D-13, D-14: 13 dsrcode HTTP hooks) ----
patch_settings_local() {
    if ! command -v node &>/dev/null; then
        return 0
    fi

    node -e "
        const fs = require('fs');
        const path = require('path');
        const home = process.env.HOME || process.env.USERPROFILE;
        const settingsPath = path.join(home, '.claude', 'settings.local.json');

        let settings = {};
        try {
            settings = JSON.parse(fs.readFileSync(settingsPath, 'utf8'));
        } catch (e) {
            settings = {};
        }
        if (!settings.hooks || typeof settings.hooks !== 'object') {
            settings.hooks = {};
        }

        const DSRCODE_HOOKS = {
            'PreToolUse':         { matcher: '*',            slug: 'pre-tool-use' },
            'PostToolUse':        { matcher: '*',            slug: 'post-tool-use' },
            'PostToolUseFailure': { matcher: '*',            slug: 'post-tool-use-failure' },
            'UserPromptSubmit':   { matcher: null,           slug: 'user-prompt-submit' },
            'Stop':               { matcher: null,           slug: 'stop' },
            'StopFailure':        { matcher: '*',            slug: 'stop-failure' },
            'Notification':       { matcher: 'idle_prompt',  slug: 'notification' },
            'SubagentStart':      { matcher: '*',            slug: 'subagent-start' },
            'SubagentStop':       { matcher: '*',            slug: 'subagent-stop' },
            'PreCompact':         { matcher: '*',            slug: 'pre-compact' },
            'PostCompact':        { matcher: '*',            slug: 'post-compact' },
            'CwdChanged':         { matcher: null,           slug: 'cwd-changed' },
            'SessionEnd':         { matcher: null,           slug: 'session-end' }
        };

        const BASE_URL = 'http://127.0.0.1:19460/hooks/';
        let added = 0;

        for (const event of Object.keys(DSRCODE_HOOKS)) {
            const config = DSRCODE_HOOKS[event];
            if (!Array.isArray(settings.hooks[event])) {
                settings.hooks[event] = [];
            }
            const existing = settings.hooks[event];
            const hasDsrcode = existing.some(function(e) {
                return e && Array.isArray(e.hooks) && e.hooks.some(function(h) {
                    return h && typeof h.url === 'string' && h.url.indexOf('127.0.0.1:19460') !== -1;
                });
            });
            if (!hasDsrcode) {
                const entry = { hooks: [{ type: 'http', url: BASE_URL + config.slug, timeout: 1 }] };
                if (config.matcher !== null) {
                    entry.matcher = config.matcher;
                }
                existing.push(entry);
                added++;
            }
        }

        // Ensure .claude directory exists before write
        try {
            fs.mkdirSync(path.dirname(settingsPath), { recursive: true });
        } catch (e) {}

        // Atomic write via tmp file + rename
        const tmp = settingsPath + '.tmp.' + process.pid;
        fs.writeFileSync(tmp, JSON.stringify(settings, null, 2) + '\n');
        fs.renameSync(tmp, settingsPath);

        if (added > 0) {
            console.log('settings.local.json patched: ' + added + ' dsrcode hook(s) added');
        }
    " 2>/dev/null || true
}

patch_settings_local

# ---- Auto-migrate german preset to professional + lang=de (D-33) ----
migrate_german_preset() {
    local CONFIG_FILE="$CLAUDE_DIR/dsrcode-config.json"
    # Also check old config name for backward compat
    if [[ ! -f "$CONFIG_FILE" ]]; then
        CONFIG_FILE="$CLAUDE_DIR/discord-presence-config.json"
    fi

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

# ---- Statusline-Wrapper Auto-Update (DIST-28) ----
update_statusline_wrapper() {
    local src="${CLAUDE_PLUGIN_ROOT:-}/scripts/statusline-wrapper.sh"
    local dest="$CLAUDE_DIR/statusline-wrapper.sh"
    if [[ -f "$src" && -f "$dest" ]]; then
        if ! diff -q "$src" "$dest" >/dev/null 2>&1; then
            cp "$src" "$dest"
            chmod +x "$dest"
        fi
    fi
}

update_statusline_wrapper

# ---- Start Daemon ----
if $IS_WINDOWS; then
    WIN_BINARY=$(cygpath -w "$BINARY" 2>/dev/null || echo "$BINARY")
    WIN_PID_FILE=$(cygpath -w "$PID_FILE" 2>/dev/null || echo "$PID_FILE")

    powershell.exe -NoProfile -WindowStyle Hidden -Command \
        '$process = Start-Process -FilePath "'"$WIN_BINARY"'" -WindowStyle Hidden -PassThru; $process.Id | Out-File -FilePath "'"$WIN_PID_FILE"'" -Encoding ASCII -NoNewline' 2>/dev/null
else
    nohup "$BINARY" > "$LOG_FILE" 2>&1 &
    echo $! > "$PID_FILE"
fi

# ---- Health Check on port 19460 (unchanged per DIST-40) ----
HEALTH_OK=false
for i in $(seq 1 50); do
    if curl -sf http://127.0.0.1:19460/health > /dev/null 2>&1; then
        HEALTH_OK=true
        break
    fi
    sleep 0.1
done

if $HEALTH_OK; then
    echo "dsrcode started (PID: $(cat "$PID_FILE" 2>/dev/null || echo "unknown"), sessions: $ACTIVE_SESSIONS)"
else
    DAEMON_PID=$(cat "$PID_FILE" 2>/dev/null)
    if [[ -n "$DAEMON_PID" ]] && process_exists "$DAEMON_PID"; then
        echo "WARNING: dsrcode started (PID: $DAEMON_PID) but health check timed out"
    else
        echo "ERROR: dsrcode failed to start (port 19460 may be in use). Check: ~/.claude/dsrcode.log"
        rm -f "$PID_FILE"
    fi
fi

# ---- First-Run Hint (DIST-36) ----
CONFIG_FILE="$CLAUDE_DIR/dsrcode-config.json"
if [[ ! -f "$CONFIG_FILE" ]]; then
    echo "dsrcode started (Preset: minimal) -- /dsrcode:setup for customization"
fi
