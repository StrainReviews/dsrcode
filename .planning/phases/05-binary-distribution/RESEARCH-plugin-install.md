# Phase 5: Binary Distribution for cc-discord-presence Plugin - Research

**Researched:** 2026-04-06
**Domain:** Claude Code plugin system, native binary distribution, Go cross-compilation
**Confidence:** HIGH

## Summary

The cc-discord-presence (dsrcode) plugin requires a Go binary daemon to function. Currently, the plugin's `start.sh` script only supports building from source -- if the user does not have Go installed, the plugin silently fails with no binary available. This research documents the full Claude Code plugin install lifecycle and identifies three viable strategies for distributing the pre-compiled binary to first-time users who lack a Go toolchain.

The most important discovery is the **`bin/` directory feature** (introduced v2.1.91, Week 14 2026) which adds plugin executables directly to the Bash tool's PATH. Combined with `${CLAUDE_PLUGIN_DATA}` for persistent storage and `SessionStart` hooks for lazy initialization, a robust download-on-first-run strategy is achievable without any postInstall hook (which Claude Code does not support and has explicitly declined to add per [issue #9394](https://github.com/anthropics/claude-code/issues/9394)).

**Primary recommendation:** Ship pre-compiled binaries via GitHub Releases (already have CI workflow). On SessionStart, download the correct platform binary to `${CLAUDE_PLUGIN_DATA}` if missing or outdated, and symlink or copy into the plugin `bin/` directory for PATH access.

## Plugin Install Lifecycle

### What Happens When a User Installs a Plugin

Based on the [official Claude Code plugins reference](https://code.claude.com/docs/en/plugins-reference) and [discover plugins docs](https://code.claude.com/docs/en/discover-plugins), the install process is: [VERIFIED: code.claude.com/docs/en/plugins-reference]

1. **Marketplace sync**: Claude Code fetches the marketplace repository (git clone/pull)
2. **Cache copy**: Plugin files are copied from `~/.claude/plugins/marketplaces/{marketplace}/` to `~/.claude/plugins/cache/{marketplace}/{plugin}/{version}/`
3. **Manifest read**: `.claude-plugin/plugin.json` is parsed
4. **Component discovery**: commands/, agents/, skills/, hooks/, bin/, .mcp.json, .lsp.json are scanned
5. **Registration**: Components are registered with Claude Code
6. **Settings update**: `installed_plugins.json` is updated with scope, version, commit SHA

**Critical observations:**
- There is **NO postInstall hook**. Feature request [#9394](https://github.com/anthropics/claude-code/issues/9394) was closed as "not planned". [VERIFIED: GitHub issue search]
- There is **NO postUpdate hook**. Feature request [#11240](https://github.com/anthropics/claude-code/issues/11240) also exists for lifecycle hooks. [VERIFIED: GitHub issue search]
- The cache copy is a **shallow copy** -- files outside the plugin directory are not included (path traversal blocked). [VERIFIED: code.claude.com/docs/en/plugins-reference]
- Symlinks within the plugin directory ARE honored during cache copy. [VERIFIED: code.claude.com/docs/en/plugins-reference]
- `.gitignore`d files (like compiled binaries) are NOT in the git repo and therefore NOT in the cache. [VERIFIED: observed in dsrcode cache]

### What the Cache Contains (dsrcode)

Verified by inspecting `~/.claude/plugins/cache/dsrcode/dsrcode/3.1.10/`:
- All Go source files (main.go, go.mod, go.sum, etc.)
- All scripts (start.sh, stop.sh, build.sh)
- All plugin components (hooks/, commands/, _skills/)
- **NO compiled binary** (cc-discord-presence.exe is in .gitignore)

This confirms the core problem: the plugin cache has source code but no binary.

## Key Plugin System Features for Binary Distribution

### 1. `bin/` Directory -- Executables on PATH

**Introduced:** v2.1.91 (Week 14, March 30 - April 3, 2026) [VERIFIED: code.claude.com/docs/en/whats-new/2026-w14]

Place an executable in a `bin/` directory at the plugin root and Claude Code adds that directory to the Bash tool's PATH while the plugin is enabled. Claude can invoke the binary as a bare command from any Bash tool call.

```text
my-plugin/
  .claude-plugin/
    plugin.json
  bin/
    my-tool          <-- Available as bare command in Bash
```

**Limitation:** The `bin/` directory is inside `${CLAUDE_PLUGIN_ROOT}`, which is **overwritten on every plugin update**. Binaries placed here do not survive updates. This makes `bin/` unsuitable as the primary storage location for downloaded binaries -- use `${CLAUDE_PLUGIN_DATA}` instead, with `bin/` as a symlink or wrapper target. [VERIFIED: code.claude.com/docs/en/plugins-reference]

### 2. `${CLAUDE_PLUGIN_DATA}` -- Persistent Storage

**Purpose:** A persistent directory for plugin state that survives updates. [VERIFIED: code.claude.com/docs/en/plugins-reference]

**Location:** `~/.claude/plugins/data/{id}/` where `{id}` is the sanitized plugin identifier.

**Key properties:**
- Created automatically on first reference
- Survives plugin updates (unlike `${CLAUDE_PLUGIN_ROOT}`)
- Deleted when plugin is uninstalled from last scope (unless `--keep-data`)
- Ideal for: downloaded binaries, caches, generated files

**Official recommended pattern** for managing dependencies that change between versions:

```json
{
  "hooks": {
    "SessionStart": [{
      "hooks": [{
        "type": "command",
        "command": "diff -q \"${CLAUDE_PLUGIN_ROOT}/package.json\" \"${CLAUDE_PLUGIN_DATA}/package.json\" >/dev/null 2>&1 || (cd \"${CLAUDE_PLUGIN_DATA}\" && cp \"${CLAUDE_PLUGIN_ROOT}/package.json\" . && npm install) || rm -f \"${CLAUDE_PLUGIN_DATA}/package.json\""
      }]
    }]
  }
}
```

This pattern: compares a manifest in plugin root vs data dir, reinstalls when they differ, and removes the copied manifest on failure so next session retries. [VERIFIED: code.claude.com/docs/en/plugins-reference]

### 3. `SessionStart` Hook -- Lazy Initialization

The only reliable hook point for first-run setup. Fires when a Claude Code session begins or resumes. [VERIFIED: code.claude.com/docs/en/plugins-reference]

**Current dsrcode usage:** Already uses SessionStart to run `start.sh` which builds/starts the daemon. This is the correct pattern -- just needs to add download capability.

### 4. `${CLAUDE_PLUGIN_ROOT}` -- Plugin Source

The absolute path to the plugin's installation directory. Changes on every update (points to new cache version). Use for referencing bundled scripts and configs. [VERIFIED: code.claude.com/docs/en/plugins-reference]

## Current State Analysis

### What Works Today

1. **Developer with Go installed:** `start.sh` detects Go source in plugin root or `~/Projects/cc-discord-presence`, builds binary to `~/.claude/bin/`, starts daemon. Works perfectly.
2. **GitHub Actions CI:** `release.yml` builds binaries for 5 platforms (darwin/arm64, darwin/amd64, linux/amd64, linux/arm64, windows/amd64) and publishes to GitHub Releases via `softprops/action-gh-release`. [VERIFIED: .github/workflows/release.yml]
3. **Version management:** `start.sh` checks binary version against expected version and rebuilds if mismatched.

### What Fails Today

1. **First-time user without Go:** `start.sh` calls `build_from_source()` which fails with "Error: Go compiler required to build cc-discord-presence". The daemon never starts. Hooks silently fail (HTTP requests to 127.0.0.1:19460 timeout).
2. **No download fallback:** `start.sh` has no code path to download a pre-compiled binary from GitHub Releases.
3. **Binary location:** Currently writes to `~/.claude/bin/` which is a custom convention, not the official plugin `bin/` directory or `${CLAUDE_PLUGIN_DATA}`.

### Existing Infrastructure

| Component | Status | Notes |
|-----------|--------|-------|
| GitHub Releases CI | Working | Builds 5 platform binaries on tag push |
| Version tagging | Working | `v3.1.10` format, embedded in binary via ldflags |
| Platform detection | Working | `start.sh` detects OS/arch correctly |
| Process management | Working | PID file, session tracking, Windows/Unix split |
| SessionStart hook | Working | Fires on every session, runs `start.sh` |

## Distribution Strategy Options

### Option A: Download from GitHub Releases (RECOMMENDED)

**How it works:**
1. On SessionStart, `start.sh` checks for binary in `${CLAUDE_PLUGIN_DATA}/bin/`
2. If missing or version mismatch, downloads correct platform binary from GitHub Releases
3. Stores binary persistently in `${CLAUDE_PLUGIN_DATA}/bin/`
4. Binary survives plugin updates (only re-downloads on version bump)

**Download mechanism:**
```bash
# Construct URL
URL="https://github.com/${REPO}/releases/download/${VERSION}/${BINARY_NAME}"

# Download with curl (available on all platforms including Git Bash on Windows)
curl -fsSL -o "$BINARY" "$URL"
chmod +x "$BINARY"
```

**Pros:**
- Zero dependencies for users (curl is universal)
- Binaries already built by CI
- Version-locked downloads (each plugin version specifies its binary version)
- Persistent storage via `${CLAUDE_PLUGIN_DATA}` survives updates
- Fallback to build-from-source for developers with Go

**Cons:**
- Requires internet on first run
- GitHub rate limits (unauthenticated: 60 req/hour, but single download is fine)
- ~10MB download per platform

**Estimated implementation effort:** Small -- modify `start.sh` to add download path before `build_from_source`.

### Option B: Ship Binaries in Git Repository

**How it works:**
1. Remove binaries from `.gitignore`
2. Commit platform binaries into `bin/` directory in the repo
3. Plugin cache copy includes them automatically

**Pros:**
- Works offline
- Zero first-run delay
- Leverages new `bin/` PATH feature

**Cons:**
- 50MB+ added to git repo (5 platforms x 10MB each)
- Every version bump bloats history
- Marketplace sync becomes slow
- Git is not designed for large binaries
- Goes against git best practices

**Verdict:** Not recommended due to repo bloat.

### Option C: Git LFS for Binaries

**How it works:**
1. Track binaries in `bin/` with Git LFS
2. Binaries stored on GitHub LFS servers
3. Downloaded on git clone/checkout

**Pros:**
- Clean git history
- Standard workflow for large files
- Would work with plugin cache copy

**Cons:**
- Requires Git LFS on the machine (not always installed)
- Claude Code's plugin cache mechanism may not support LFS checkout
- GitHub LFS has bandwidth limits on free tier (1GB/month)
- Adds complexity for contributors

**Verdict:** Risky -- uncertain LFS support in plugin cache system. Not recommended.

### Option D: Hybrid bin/ + PLUGIN_DATA with Wrapper Script

**How it works:**
1. Place a thin wrapper script in `bin/cc-discord-presence` (or .sh)
2. Wrapper checks `${CLAUDE_PLUGIN_DATA}/bin/` for actual binary
3. If not found, downloads it
4. Forwards all arguments to actual binary

**Pros:**
- Wrapper script is tiny (in git, survives cache copy)
- Actual binary lives in persistent `${CLAUDE_PLUGIN_DATA}`
- Leverages `bin/` PATH feature for discovery
- Clean separation of concerns

**Cons:**
- Extra indirection layer
- Shell wrapper may have Windows compatibility issues
- `bin/` PATH feature is very new (v2.1.91)

**Verdict:** Elegant but over-engineered. The existing SessionStart hook approach is simpler.

## Recommended Architecture

### Strategy: GitHub Releases Download + PLUGIN_DATA Persistence

```
SessionStart hook fires
  |
  v
start.sh runs
  |
  v
Check ${CLAUDE_PLUGIN_DATA}/bin/ for binary
  |
  +-- Binary exists + correct version --> Start daemon
  |
  +-- Binary missing or wrong version
        |
        v
      Try download from GitHub Releases
        |
        +-- Success --> Store in ${CLAUDE_PLUGIN_DATA}/bin/, start daemon
        |
        +-- Failure (offline, rate limit, etc.)
              |
              v
            Try build from source (Go required)
              |
              +-- Success --> Store in ${CLAUDE_PLUGIN_DATA}/bin/, start daemon
              |
              +-- Failure --> Print clear error with install instructions
```

### Key Changes to start.sh

1. **Add `${CLAUDE_PLUGIN_DATA}`** as primary binary storage location (replaces `~/.claude/bin/`)
2. **Add download function:** `curl -fsSL` from GitHub Releases URL
3. **Add version manifest:** Store current version in `${CLAUDE_PLUGIN_DATA}/VERSION` for quick checks
4. **Keep build-from-source** as fallback for developers
5. **Print actionable error** if all methods fail:
   ```
   cc-discord-presence binary not available.
   Options:
   1. Ensure internet connection and restart Claude Code (auto-download)
   2. Install Go (https://go.dev) and restart Claude Code (build from source)
   3. Download manually from https://github.com/StrainReviews/dsrcode/releases
   ```

### Version Synchronization

The `VERSION` variable in `start.sh` already matches the plugin version in `plugin.json`. When the plugin updates:

1. `${CLAUDE_PLUGIN_ROOT}` changes (new cache version with new `start.sh`)
2. `${CLAUDE_PLUGIN_DATA}` persists (old binary still there)
3. On next SessionStart, `start.sh` compares `VERSION` in script vs `${CLAUDE_PLUGIN_DATA}/VERSION`
4. If different, downloads new binary

This mirrors the official pattern from the Claude Code docs for managing npm dependencies.

## Common Pitfalls

### Pitfall 1: Binary in CLAUDE_PLUGIN_ROOT Gets Deleted on Update
**What goes wrong:** Storing binary in plugin root means every plugin update deletes it.
**Why it happens:** `${CLAUDE_PLUGIN_ROOT}` points to a versioned cache directory that is replaced on update.
**How to avoid:** Store binary in `${CLAUDE_PLUGIN_DATA}` which persists across updates.
**Warning signs:** Binary disappears after `/plugin marketplace update`.

### Pitfall 2: Git Bash curl on Windows May Not Follow Redirects
**What goes wrong:** GitHub Releases URLs redirect; some curl builds don't follow by default.
**Why it happens:** Windows Git Bash bundles an older curl that may need explicit `-L` flag.
**How to avoid:** Always use `curl -fsSL` (the `-L` follows redirects).
**Warning signs:** Download returns HTML instead of binary.

### Pitfall 3: Windows Binary Naming
**What goes wrong:** Downloaded binary without `.exe` extension fails to execute on Windows.
**Why it happens:** Windows requires `.exe` extension for executables.
**How to avoid:** Platform detection already in `start.sh` appends `.exe` for Windows builds.
**Warning signs:** "Permission denied" or "not recognized" errors on Windows.

### Pitfall 4: SessionStart Hook Timeout
**What goes wrong:** Download takes too long, SessionStart hook times out (current: 15s).
**Why it happens:** Binary is ~10MB, slow connections may not finish in 15 seconds.
**How to avoid:** Increase timeout for the download hook, or make download async and start daemon after download completes.
**Warning signs:** Hook reports timeout, daemon never starts.

### Pitfall 5: No postInstall Means First Session Has Setup Delay
**What goes wrong:** User installs plugin, starts session, waits 5-15 seconds for binary download.
**Why it happens:** Download only happens on first SessionStart -- there is no install-time hook.
**How to avoid:** Cannot be avoided. Mitigate with progress messages and fast CDN.
**Warning signs:** First session after install is slow to start.

### Pitfall 6: CLAUDE_PLUGIN_DATA Not Available in All Contexts
**What goes wrong:** Environment variable not expanded in some script contexts.
**Why it happens:** `${CLAUDE_PLUGIN_DATA}` is substituted by Claude Code in hook commands and MCP configs, but may not be available in all nested script calls.
**How to avoid:** Reference `${CLAUDE_PLUGIN_DATA}` directly in the hook command string, or compute the path manually as `~/.claude/plugins/data/dsrcode-dsrcode/`.
**Warning signs:** Variable is empty or literal `${CLAUDE_PLUGIN_DATA}` string in script.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Binary downloads | Custom HTTP client | `curl -fsSL` | Universal, handles redirects, follows HTTPS |
| Cross-platform builds | Manual build matrix | GitHub Actions `softprops/action-gh-release` | Already configured, builds all 5 platforms |
| Version comparison | String parsing | Semantic version in `VERSION` file | Simple `diff -q` or string equality check |
| Persistent storage | Custom `~/.claude/bin/` | `${CLAUDE_PLUGIN_DATA}` | Official, managed by Claude Code, cleaned on uninstall |
| Process management | Custom daemon manager | Existing PID/refcount system | Already battle-tested in start.sh |

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| curl | Binary download | Yes (all platforms) | 8.x | wget (rare fallback) |
| Go | Build from source | Developer-only | 1.26 | Download pre-built binary |
| bash | start.sh execution | Yes (via Git Bash on Windows) | 5.x | PowerShell (start.ps1 exists) |
| GitHub Releases | Binary hosting | Yes (public repo) | N/A | Build from source |
| `${CLAUDE_PLUGIN_DATA}` | Persistent binary storage | Yes (v2.1.91+) | N/A | Fall back to `~/.claude/bin/` |

**Missing dependencies with no fallback:** None -- all critical paths have fallbacks.

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Build from source only | Download + build fallback | Proposed (this phase) | First-time users without Go can use plugin |
| Custom `~/.claude/bin/` | `${CLAUDE_PLUGIN_DATA}/bin/` | v2.1.91 (Week 14 2026) | Official persistent storage, managed cleanup |
| No PATH integration | Plugin `bin/` directory on PATH | v2.1.91 (Week 14 2026) | Binaries invokable as bare commands |
| postInstall hook desired | SessionStart lazy init | Issue #9394 closed as "not planned" | SessionStart is the official pattern |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | GitHub Releases URLs follow the pattern `https://github.com/{owner}/{repo}/releases/download/{tag}/{filename}` | Recommended Architecture | Download function would need different URL scheme |
| A2 | `${CLAUDE_PLUGIN_DATA}` is available inside scripts called from SessionStart hooks | Recommended Architecture | Would need to compute path manually as `~/.claude/plugins/data/dsrcode-dsrcode/` |
| A3 | curl is available in Git Bash on Windows | Environment Availability | Would need PowerShell fallback for Windows downloads |
| A4 | 15 second SessionStart timeout is sufficient for ~10MB download on typical connections | Pitfalls | May need to increase timeout or make download async |

## Open Questions

1. **SessionStart timeout budget**
   - What we know: Current timeout is 15 seconds. Binary is ~10MB.
   - What's unclear: Is 15 seconds enough for download on slower connections? Can timeout be increased per-hook?
   - Recommendation: Test on slow connection. Consider splitting into two SessionStart hooks: quick check + async download.

2. **CLAUDE_PLUGIN_DATA variable expansion in nested scripts**
   - What we know: `${CLAUDE_PLUGIN_DATA}` is substituted in hook command strings.
   - What's unclear: Whether it's available as a shell environment variable inside scripts called from those hooks, or only in the top-level command string.
   - Recommendation: Test by echoing `${CLAUDE_PLUGIN_DATA}` inside `start.sh`. If unavailable, pass it as an argument or compute the canonical path.

3. **bin/ directory for daemon binary vs. just PATH helpers**
   - What we know: `bin/` adds executables to Bash PATH. The daemon is started by `start.sh`, not invoked directly by Claude.
   - What's unclear: Whether `bin/` is beneficial for the daemon use case vs. CLI helpers.
   - Recommendation: Not needed for daemon. The daemon is started via hook script, not invoked by Claude. Reserve `bin/` for future CLI subcommands if needed.

## Sources

### Primary (HIGH confidence)
- [Claude Code Plugins Reference](https://code.claude.com/docs/en/plugins-reference) -- Complete plugin manifest schema, `${CLAUDE_PLUGIN_DATA}`, `${CLAUDE_PLUGIN_ROOT}`, `bin/` directory, file locations reference
- [Discover and Install Plugins](https://code.claude.com/docs/en/discover-plugins) -- Install lifecycle, marketplace sync, cache behavior, scopes
- [Week 14 What's New](https://code.claude.com/docs/en/whats-new/2026-w14) -- `bin/` directory feature announcement (v2.1.91)
- [plugin-dev skill (local)](C:/Users/ktown/.claude/plugins/marketplaces/claude-plugins-official/plugins/plugin-dev/) -- Plugin structure, hook development, manifest reference, marketplace considerations

### Secondary (MEDIUM confidence)
- [GitHub Issue #9394](https://github.com/anthropics/claude-code/issues/9394) -- postInstall hook feature request, closed as "not planned"
- [GitHub Issue #11240](https://github.com/anthropics/claude-code/issues/11240) -- Plugin lifecycle install/uninstall hooks feature request
- [dsrcode source](C:/Users/ktown/Projects/cc-discord-presence/) -- Current plugin implementation, start.sh, release.yml, hooks.json

### Tertiary (LOW confidence)
- Web search results for Claude Code binary distribution patterns -- confirmed by primary sources

## Metadata

**Confidence breakdown:**
- Plugin install lifecycle: HIGH -- verified against official docs
- Binary distribution strategy: HIGH -- based on official `${CLAUDE_PLUGIN_DATA}` pattern and existing CI infrastructure
- `bin/` directory feature: HIGH -- verified in v2.1.91 release notes
- Pitfalls: MEDIUM -- some based on general experience, not all tested with this specific plugin
- Open questions: Items flagged need empirical testing

**Research date:** 2026-04-06
**Valid until:** 2026-05-06 (30 days -- plugin system is evolving but core patterns stable)
