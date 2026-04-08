# Phase 5: Binary Distribution - start.sh Rewrite Research

**Researched:** 2026-04-06
**Domain:** Bash install scripts, GitHub Releases binary distribution, cross-platform OS/arch detection
**Confidence:** HIGH

## Summary

The current `start.sh` only builds from source via `go build`, requiring Go to be installed on every user's machine. The PowerShell script (`start.ps1`) already downloads from GitHub Releases but is outdated (still references `tsanva/cc-discord-presence` and version `v1.0.3`). The bash script needs to be rewritten to: (1) download pre-built binaries from GitHub Releases as the primary path, (2) fall back to `go build` if download fails and Go is available, (3) show clear install instructions if neither works, and (4) handle version mismatch by re-downloading or rebuilding.

**Primary recommendation:** Use `curl -fsSL` with the GitHub Releases direct URL pattern (`https://github.com/{owner}/{repo}/releases/download/{tag}/{asset}`) -- no API calls or `jq` needed since the binary naming convention is deterministic and the repo is public.

**Key patterns studied:** fzf install script (download + go build fallback), starship installer (platform detection + curl download), goreleaser godownloader (uname-based URL construction), jpillora/installer (OS/arch normalization). [VERIFIED: GitHub search results and official docs]

## Current State Analysis

### Current start.sh Problems

| Problem | Impact |
|---------|--------|
| Only builds from source | Requires Go installed on every user's machine |
| Searches `$ROOT` before it's defined (line 84 vs 90) | Bug: `$ROOT` is empty when first used |
| No download capability | Unlike start.ps1, never downloads binaries |
| No checksum verification | Security gap if download is added |
| Repo reference: `StrainReviews/dsrcode` | Correct org name [VERIFIED: plugin.json] |

### Current start.ps1 Problems

| Problem | Impact |
|---------|--------|
| References `tsanva/cc-discord-presence` (line 12) | Wrong repo -- should be `StrainReviews/dsrcode` |
| Version stuck at `v1.0.3` (line 13) | Current version is `v3.1.10` |
| No build-from-source fallback | Just errors if download fails |
| No version mismatch check | Never updates after first download |

### Release Workflow (Correct Reference)

From `.github/workflows/release.yml` [VERIFIED: file read]:
- Triggers on `v*` tags
- Builds 5 binaries: `darwin-arm64`, `darwin-amd64`, `linux-amd64`, `linux-arm64`, `windows-amd64`
- Publishes to GitHub Releases via `softprops/action-gh-release@v2`
- Binary naming: `cc-discord-presence-{os}-{arch}[.exe]`
- Download URL pattern: `https://github.com/StrainReviews/dsrcode/releases/download/{tag}/cc-discord-presence-{os}-{arch}[.exe]`

## Architecture: Download-First + Build Fallback Flow

```
start.sh invoked
    |
    v
[Detect OS + Arch]
    |
    v
[Construct binary name: cc-discord-presence-{os}-{arch}[.exe]]
    |
    v
[Binary exists at ~/.claude/bin/{name}?] --NO--> [Download from GitHub Releases]
    |                                                     |
   YES                                              SUCCESS? --NO--> [Go installed?]
    |                                                     |                  |
    v                                                    YES               YES --> [go build]
[Version matches?] --YES--> [Start daemon]                |                  |
    |                           ^                         v                 NO --> [Error + instructions]
   NO                           |                   [Start daemon]
    |                           |
    v                           |
[Kill old daemon]               |
    |                           |
    v                           |
[Download new version] -------->|
    |                           |
  FAIL                          |
    |                           |
    v                           |
[Go installed?] --YES-> [go build] --> [Start daemon]
    |
   NO
    |
    v
[Error: install instructions]
```

## Platform Detection Pattern

This is the standard cross-platform detection approach used by fzf, starship, and goreleaser-generated scripts [CITED: multiple GitHub install scripts]:

```bash
detect_platform() {
    local os arch

    os="$(uname -s)"
    case "$os" in
        Linux*)   os="linux" ;;
        Darwin*)  os="darwin" ;;
        MINGW*|MSYS*|CYGWIN*) os="windows" ;;
        *)
            echo "Error: Unsupported OS: $os" >&2
            return 1
            ;;
    esac

    arch="$(uname -m)"
    case "$arch" in
        x86_64|amd64) arch="amd64" ;;
        aarch64|arm64) arch="arm64" ;;
        *)
            echo "Error: Unsupported architecture: $arch" >&2
            return 1
            ;;
    esac

    IS_WINDOWS=false
    [[ "$os" == "windows" ]] && IS_WINDOWS=true

    PLATFORM_OS="$os"
    PLATFORM_ARCH="$arch"
}
```

**Key insight:** `uname -s` on Git Bash/MSYS2 returns `MINGW64_NT-*` or `MSYS_NT-*`, not `windows`. The current script already handles this correctly. [VERIFIED: current start.sh line 20-22]

## Download Strategy

### Direct URL (preferred -- no API, no jq)

Since the repo is public and binary names are deterministic:

```bash
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_NAME}"
```

This avoids the GitHub API entirely. No authentication needed, no `jq` dependency, no rate limiting concerns. [CITED: https://gist.github.com/steinwaywhw/a4cd19cda655b8249d908261a62687f8]

### curl Flags

```bash
curl -fsSL -o "$BINARY" "$DOWNLOAD_URL"
```

| Flag | Purpose |
|------|---------|
| `-f` | Fail silently on HTTP errors (returns exit code 22 instead of saving error HTML) |
| `-s` | Silent mode (no progress meter) |
| `-S` | Show errors even in silent mode |
| `-L` | Follow redirects (GitHub redirects to CDN) |
| `-o` | Write to file |

**Critical:** The `-L` flag is essential because GitHub Releases URLs redirect to `objects.githubusercontent.com`. Without it, you get a 302 HTML page saved as the "binary". [VERIFIED: standard curl behavior with GitHub]

### Temp File + Atomic Move

Download to a temp file, then move into place. This prevents a partial download from being treated as a valid binary:

```bash
TMP_BINARY="${BINARY}.tmp"
curl -fsSL -o "$TMP_BINARY" "$DOWNLOAD_URL" && mv "$TMP_BINARY" "$BINARY"
```

## Version Check Strategy

The current script already has version checking (lines 117-135) but uses `build_from_source` as the only recovery path. The new script extends this to try download first:

```bash
check_version() {
    if [[ ! -f "$BINARY" ]]; then
        return 1  # No binary, needs install
    fi

    local current_version
    current_version=$("$BINARY" --version 2>/dev/null | awk '{print $NF}' || echo "unknown")
    current_version="${current_version#v}"
    local expected="${VERSION#v}"

    if [[ "$current_version" == "$expected" ]]; then
        return 0  # Version matches
    fi

    echo "Updating cc-discord-presence from ${current_version} to ${VERSION}..."
    return 1  # Needs update
}
```

## Build-from-Source Fallback

The current script has a bug: it references `$ROOT` on line 84 before defining it on line 90. The rewrite fixes this:

```bash
find_source_dir() {
    local plugin_root="${CLAUDE_PLUGIN_ROOT:-$HOME/.claude/plugins/marketplaces/dsrcode}"
    for candidate in "$plugin_root" "$HOME/Projects/cc-discord-presence"; do
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
        echo "Error: No Go source found" >&2
        return 1
    }
    if ! command -v go &>/dev/null; then
        echo "Error: Go compiler not found" >&2
        return 1
    fi
    echo "Building cc-discord-presence from source ($source_dir)..."
    local ldflags="-X main.Version=${VERSION#v}"
    (cd "$source_dir" && go build -ldflags "$ldflags" -o "$BINARY" .) 2>&1
    if ! $IS_WINDOWS; then
        chmod +x "$BINARY"
    fi
    echo "Built successfully!"
}
```

## Error Messages with Install Instructions

When neither download nor build works, the user needs clear guidance:

```bash
show_install_help() {
    echo ""
    echo "==== cc-discord-presence: Installation Failed ===="
    echo ""
    echo "Could not download the binary and Go is not installed for building from source."
    echo ""
    echo "Option 1: Install manually"
    echo "  Download from: https://github.com/${REPO}/releases/tag/${VERSION}"
    echo "  Place the binary in: ${BIN_DIR}/"
    echo ""
    echo "Option 2: Install Go and rebuild"
    echo "  https://go.dev/dl/"
    echo "  Then restart your Claude Code session."
    echo ""
    echo "================================================="
}
```

## Common Pitfalls

### Pitfall 1: Partial Downloads Treated as Valid Binary
**What goes wrong:** Network interruption saves partial file. Next run tries to execute it and gets cryptic exec format error.
**How to avoid:** Download to `.tmp` file, only `mv` into place on success. Check file size is non-zero. [VERIFIED: pattern used by fzf, starship]

### Pitfall 2: Missing `-L` Flag on curl
**What goes wrong:** GitHub redirects to CDN. Without `-L`, curl saves the 302 HTML response as the binary.
**How to avoid:** Always use `curl -fsSL`. [VERIFIED: standard GitHub download behavior]

### Pitfall 3: Windows Git Bash `chmod` and `cygpath`
**What goes wrong:** `chmod +x` is a no-op on NTFS via Git Bash. `cygpath` may not exist in all MSYS2 environments.
**How to avoid:** Skip `chmod` on Windows. Use `cygpath` with fallback. [VERIFIED: current start.sh already handles this]

### Pitfall 4: `set -e` Kills Script on Expected Failures
**What goes wrong:** `curl` failing (no internet) or `go` not found triggers `set -e` exit before the fallback logic runs.
**How to avoid:** Use `|| true` or explicit `if` blocks for operations that are expected to fail. Only use `set -e` after the ensure-binary section. [ASSUMED]

### Pitfall 5: Race Condition on Binary Replacement
**What goes wrong:** Downloading a new version while the daemon is running. On Windows, the running `.exe` cannot be overwritten.
**How to avoid:** Kill the daemon BEFORE replacing the binary. The current script already does this (lines 124-131). [VERIFIED: current start.sh]

---

## Complete Rewritten start.sh

```bash
#!/bin/bash
# Start Discord Rich Presence daemon
# Downloads pre-built binary from GitHub Releases, falls back to go build
# WARNING: Linux support is untested. Please report issues on GitHub.

# ---- Configuration ----
CLAUDE_DIR="$HOME/.claude"
BIN_DIR="$CLAUDE_DIR/bin"
PID_FILE="$CLAUDE_DIR/discord-presence.pid"
LOG_FILE="$CLAUDE_DIR/discord-presence.log"
SESSIONS_DIR="$CLAUDE_DIR/discord-presence-sessions"
REFCOUNT_FILE="$CLAUDE_DIR/discord-presence.refcount"
REPO="StrainReviews/dsrcode"
VERSION="v3.1.10"

# ---- Platform Detection ----
detect_platform() {
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
}

detect_platform

BINARY_NAME="cc-discord-presence-${OS}-${ARCH}"
if $IS_WINDOWS; then
    BINARY_NAME="${BINARY_NAME}.exe"
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
        echo "Discord Rich Presence already running (PID: $OLD_PID, sessions: $ACTIVE_SESSIONS)"
        exit 0
    fi
fi

# ---- Binary Acquisition Functions ----
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_NAME}"

download_binary() {
    if ! command -v curl &>/dev/null; then
        echo "Warning: curl not found, skipping download" >&2
        return 1
    fi

    echo "Downloading cc-discord-presence ${VERSION} for ${OS}-${ARCH}..."
    local tmp_binary="${BINARY}.tmp"
    rm -f "$tmp_binary"

    if curl -fsSL -o "$tmp_binary" "$DOWNLOAD_URL" 2>/dev/null; then
        # Verify download is not empty / not an HTML error page
        local file_size
        file_size=$(wc -c < "$tmp_binary" 2>/dev/null | tr -d ' ')
        if [[ -z "$file_size" ]] || [[ "$file_size" -lt 1000 ]]; then
            echo "Warning: Downloaded file too small (${file_size:-0} bytes), likely an error page" >&2
            rm -f "$tmp_binary"
            return 1
        fi

        mv "$tmp_binary" "$BINARY"
        if ! $IS_WINDOWS; then
            chmod +x "$BINARY"
        fi
        echo "Downloaded successfully!"
        return 0
    else
        rm -f "$tmp_binary"
        echo "Warning: Download failed (no internet or release not found)" >&2
        return 1
    fi
}

find_source_dir() {
    local plugin_root="${CLAUDE_PLUGIN_ROOT:-$HOME/.claude/plugins/marketplaces/dsrcode}"
    for candidate in "$plugin_root" "$HOME/Projects/cc-discord-presence"; do
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
    echo "Building cc-discord-presence from source ($source_dir)..."
    local ldflags="-X main.Version=${VERSION#v}"
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

show_install_help() {
    echo "" >&2
    echo "==== cc-discord-presence: Installation Failed ====" >&2
    echo "" >&2
    echo "Could not download the binary and could not build from source." >&2
    echo "" >&2
    echo "Option 1: Download manually" >&2
    echo "  ${DOWNLOAD_URL}" >&2
    echo "  Place in: ${BIN_DIR}/" >&2
    echo "" >&2
    echo "Option 2: Install Go (https://go.dev/dl/) and restart Claude Code" >&2
    echo "" >&2
    echo "=================================================" >&2
}

# Acquire binary: download first, build fallback, error last
ensure_binary() {
    if download_binary; then
        return 0
    fi

    echo "Trying build from source as fallback..."
    if build_from_source; then
        return 0
    fi

    show_install_help
    return 1
}

# ---- Version Check & Binary Acquisition ----
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
}

if [[ ! -f "$BINARY" ]]; then
    # No binary at all -- acquire it
    if ! ensure_binary; then
        exit 1
    fi
else
    # Binary exists -- check version
    CURRENT_VERSION=$("$BINARY" --version 2>/dev/null | awk '{print $NF}' || echo "unknown")
    CURRENT_NORMALIZED="${CURRENT_VERSION#v}"
    EXPECTED_NORMALIZED="${VERSION#v}"

    if [[ "$CURRENT_NORMALIZED" != "" \
       && "$CURRENT_NORMALIZED" != "$EXPECTED_NORMALIZED" \
       && "$CURRENT_NORMALIZED" != "unknown" ]]; then
        echo "Updating cc-discord-presence from ${CURRENT_VERSION} to ${VERSION}..."
        # Must kill daemon before replacing binary (Windows locks running .exe)
        kill_daemon_if_running
        rm -f "$BINARY"
        if ! ensure_binary; then
            exit 1
        fi
    fi
fi

# Final guard
if [[ ! -f "$BINARY" ]]; then
    echo "Error: Binary not found at $BINARY" >&2
    exit 1
fi

# From this point on, fail fast
set -e

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

# ---- Wait for HTTP Server Ready (max 5s) ----
for i in $(seq 1 50); do
    if curl -sf http://127.0.0.1:19460/health > /dev/null 2>&1; then
        break
    fi
    sleep 0.1
done

echo "Discord Rich Presence started (PID: $(cat "$PID_FILE" 2>/dev/null || echo "unknown"), sessions: $ACTIVE_SESSIONS)"

# ---- First-Run Hint ----
CONFIG_FILE="$CLAUDE_DIR/discord-presence-config.json"
if [[ ! -f "$CONFIG_FILE" ]]; then
    echo "DSR Code gestartet (Preset: minimal) -- /dsrcode:setup fuer Anpassungen"
fi
```

## Design Decisions & Rationale

| Decision | Why |
|----------|-----|
| Direct URL, not GitHub API | No `jq` dependency. Public repo. Deterministic URL. No rate limits. |
| `curl -fsSL` | `-f` fails on HTTP error (no saving error HTML). `-L` follows redirects (GitHub CDN). `-sS` silent but shows errors. |
| Temp file + mv | Atomic: partial downloads never become the "installed" binary. |
| File size check (>1000 bytes) | Catches HTML error pages saved as binary (404 pages are ~1KB of HTML, real binary is ~10MB). |
| `set -e` only after binary acquisition | Download failure and build failure are expected flow control, not fatal errors. `set -e` would kill the script before reaching fallbacks. |
| Kill daemon before binary replacement | Windows locks running `.exe` files. Cannot overwrite. Must kill first. |
| `CLAUDE_PLUGIN_ROOT` first in source search | When running as a Claude plugin, the source is at the plugin root. Development checkout is secondary. |
| No checksum verification | GitHub Releases over HTTPS is sufficient for this use case. The repo is public, TLS verifies integrity. Adding checksums requires publishing a checksums file in the release workflow -- worth doing later but not blocking. |

## Differences from Current Script

| Area | Current | Rewritten |
|------|---------|-----------|
| Binary acquisition | `go build` only | Download first, `go build` fallback |
| `$ROOT` bug | Used before defined (line 84 vs 90) | Fixed: `find_source_dir()` uses `CLAUDE_PLUGIN_ROOT` directly |
| Error on failure | Generic "Go compiler required" | Detailed instructions with download URL |
| `set -e` scope | Entire script | Only after binary is secured |
| Download | None | `curl -fsSL` with temp file + size check |
| Platform detection | Inline | Extracted to `detect_platform()` function |
| Version update | Rebuild only | Download first, rebuild fallback |

## Future Improvements (Out of Scope)

| Improvement | Why Deferred |
|-------------|--------------|
| SHA256 checksum verification | Requires adding `checksums.txt` to release workflow. Good practice but HTTPS already provides integrity. |
| `wget` fallback | curl is available on macOS, all Linux distros, and Git Bash for Windows. wget fallback adds complexity for near-zero benefit. |
| Progress bar | `curl -fsSL` is silent. Could use `curl -fL#` for a progress bar, but plugin hooks should be quiet. |
| Auto-detect latest version | Would require GitHub API call + `jq`. Pinned version is simpler and more predictable for plugins. |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `set -e` should be scoped after binary acquisition to allow fallback flow | Pitfall 4 | Script would exit on first download failure without trying build fallback |
| A2 | File size >1000 bytes is a reliable indicator the download is not an error page | Download Strategy | Could false-positive on extremely small binaries, but Go binaries are always multi-MB |
| A3 | curl is available in Git Bash / MSYS2 on Windows | Download Strategy | If not available, download silently skips to build fallback -- acceptable degradation |

## Sources

### Primary (HIGH confidence)
- Current `start.sh` -- [VERIFIED: file read]
- Current `start.ps1` -- [VERIFIED: file read]
- `.github/workflows/release.yml` -- [VERIFIED: file read]
- `.claude-plugin/plugin.json` -- [VERIFIED: file read, repo is `StrainReviews/dsrcode`]
- `scripts/build.sh` -- [VERIFIED: file read, confirms binary naming convention]

### Secondary (MEDIUM confidence)
- [fzf install script pattern](https://github.com/junegunn/fzf/blob/master/install) -- download + go build fallback
- [jpillora/installer](https://github.com/jpillora/installer) -- OS/arch detection + GitHub Releases
- [steinwaywhw gist](https://gist.github.com/steinwaywhw/a4cd19cda655b8249d908261a62687f8) -- curl one-liners for GitHub Releases
- [goreleaser install pattern](https://goreleaser.com/install/) -- uname-based URL construction
- [starship installer](https://github.com/starship/starship) -- platform detection + binary download
- [zyedidia/eget](https://github.com/zyedidia/eget) -- GitHub binary downloader with version checking
