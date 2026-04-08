# Phase 2: DSRCodePresence Setup Wizard - Research

**Researched:** 2026-04-03
**Domain:** Claude Code Plugin System, Go HTTP Endpoints, Discord Rich Presence, Hook Migration
**Confidence:** HIGH

## Summary

Phase 2 extends the existing cc-discord-presence Go binary and Claude Code plugin with 7 slash commands (/dsrcode:*), migrates activity hooks from command type (bash shelling out to curl) to native HTTP type, extends the hook payload with tool_input parsing for file/command/query extraction, adds a displayDetail system (4 levels orthogonal to presets), expands all 8 presets from ~15-20 to 40 messages per activity category with new placeholders, and adds 3 new HTTP endpoints (GET /presets, GET /status, POST /preview).

The codebase is well-structured Go with 8 packages (config, discord, logger, preset, resolver, server, session, main), all tests passing, go:embed for presets, and immutable copy-before-modify patterns throughout. The plugin system uses .claude-plugin/plugin.json manifest with hooks defined in hooks/hooks.json. Claude Code's plugin system is mature (23+ hook events, 4 hook types including native HTTP), and the official docs confirm session_id is now present in all hook events -- though the open issue #37339 suggests this may be version-dependent.

**Primary recommendation:** Implement in 3 workstreams -- (1) Go binary changes (payload parsing, displayDetail, new endpoints, preset expansion), (2) Plugin structure changes (hook migration to HTTP, slash command .md files), (3) Script updates (start.sh version check, hook-forward.sh deletion). The Go workstream is the largest and should be tackled first since the plugin commands depend on the new endpoints.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- D-01: Claude-as-Wizard Pattern -- Slash-Commands (.md Dateien) instruieren Claude, den User durch Setup/Konfiguration zu fuehren. Kein Go TUI (charmbracelet/huh), kein MCP Server, kein Stack-Wechsel zu TypeScript.
- D-02: Go Daemon bleibt unveraendert in seiner Kernarchitektur -- Discord IPC, Multi-Session Registry, Config Hot-Reload, JSONL-Fallback. Nur neue HTTP Endpoints und erweitertes Payload-Parsing.
- D-03: Kein MCP Server -- Discord IPC ist Singleton, MCP Server stirbt mit Session, Node.js discord-rpc abandoned.
- D-04: Plugin-Struktur nutzt Claude Code Plugin-System: commands/ fuer Slash-Commands, hooks/ fuer Activity-Tracking, scripts/ fuer Daemon Lifecycle, .claude-plugin/plugin.json fuer Manifest.
- D-05: Verzeichnisstruktur festgelegt (commands/, hooks/, scripts/, .claude-plugin/).
- D-06 to D-12: 7 Slash-Commands mit spezifischen Verantwortlichkeiten (setup, preset, status, log, doctor, update, demo).
- D-13: Activity hooks migrieren von command zu native HTTP type in hooks.json. hook-forward.sh wird geloescht.
- D-14: 4 HTTP hooks: PreToolUse, UserPromptSubmit, Stop, Notification. Alle async mit 1s Timeout.
- D-15: SessionStart/SessionEnd bleiben command hooks in plugin.json (Daemon Lifecycle).
- D-16: Notification hook mapped auf idle icon + "Waiting for input".
- D-17: PostToolUse bewusst NICHT -- wird sofort vom naechsten PreToolUse ueberschrieben.
- D-18: HookPayload struct erweitern: + tool_input, hook_event_name, transcript_path, permission_mode, user_prompt, reason, message.
- D-19: tool_input per Tool-Typ parsen: Edit/Read/Write -> file_path, Bash -> command, Grep/Glob -> pattern, Task/Agent -> prompt.
- D-20: Session struct erweitern: + LastFile, LastFilePath, LastCommand, LastQuery.
- D-21: 4 Display-Levels: minimal (Default), standard, verbose, private.
- D-22: Config-Option displayDetail als Enum in discord-presence-config.json.
- D-23: Alle 8 Presets von ~15-20 auf 40 Messages pro Activity-Typ erweitern.
- D-24: Neue Platzhalter: {file}, {filepath}, {command}, {query}, {activity}, {sessions}.
- D-25: Doppelte Variation: 40 Messages rotieren + wechselnde Dateinamen/Commands.
- D-26: Multi-Session Platzhalter: {projects}, {models}, {totalCost}, {totalTokens}.
- D-27: GET /presets Endpoint.
- D-28: GET /status Endpoint mit detailliertem Status.
- D-29: POST /preview Endpoint fuer Screenshot/Demo.
- D-30: Self-triggering Hook fuer Session-Identifikation.
- D-31: session_id Fallback: Wenn leer -> slog.Warn + synthetische ID "http-{projectName}".
- D-32: JSONL-Dedup bleibt wie implementiert.
- D-33: Preset-Wechsel ist global (Hot-Reload).
- D-34: Worktree Agents: Separate Sessions.
- D-35: Nach plugin install sind Hooks automatisch aktiv. start.sh downloaded Binary.
- D-36: First-Run Hint: keine Config -> einzeilige Ausgabe.
- D-37: Kein userConfig in plugin.json.
- D-38: Gekoppelte Versionierung: plugin.json Version = GitHub Release Tag = Binary Version.
- D-39: Binary Auto-Update: start.sh vergleicht Version, downloaded bei Mismatch.
- D-40: README in Englisch.
- D-41: Echte Discord Screenshots via /dsrcode:demo.
- D-42: README Inhalt: Features, Installation, Quick Start, Screenshots, Troubleshooting.

### Claude's Discretion
- Exakte Message-Texte fuer die 40 Messages pro Activity pro Preset
- Go-Paketstruktur fuer erweiterten Payload-Parser
- Exakte Formatierung der /dsrcode:doctor Checks
- Slash-Command .md Datei-Inhalte (Claude-Instruktionen)
- POST /preview Timeout-Mechanismus (Timer vs. naechster Hook)
- start.sh Version-Check Implementierung (--version Flag vs. eingebettete Version)

### Deferred Ideas (OUT OF SCOPE)
- Per-Session Presets (verschiedene Presets fuer verschiedene Projekte)
- Activity-abhaengige State-Messages
- Worktree-Agent Zuordnung zum Parent-Projekt
- PR upstream an tsanva/cc-discord-presence
- Web-Dashboard fuer Session-Statistiken
- Twitch/YouTube Stream-Integration
- Auto-Update-Checker im Go-Binary
- charmbracelet/huh TUI Wizard
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DSR-01 | /dsrcode:setup Slash-Command (7-phase wizard) | Plugin commands/ directory with .md files; Claude-as-Wizard pattern via markdown instructions |
| DSR-02 | /dsrcode:preset Slash-Command | GET /presets endpoint + POST /config for hot-reload |
| DSR-03 | /dsrcode:status Slash-Command | GET /status endpoint with session/hook/uptime data |
| DSR-04 | /dsrcode:log Slash-Command | Direct file read of ~/.claude/discord-presence.log |
| DSR-05 | /dsrcode:doctor Slash-Command (7 categories) | flutter doctor-style diagnostics via bash checks + GET /status |
| DSR-06 | /dsrcode:update Slash-Command | GitHub Releases API + binary download via start.sh pattern |
| DSR-07 | /dsrcode:demo Slash-Command | POST /preview endpoint for screenshot mode |
| DSR-08 | Hook migration: command -> HTTP type | Native HTTP hooks in hooks.json confirmed working for localhost |
| DSR-09 | PreToolUse HTTP hook | hooks.json format: type "http", url, timeout, matcher |
| DSR-10 | UserPromptSubmit HTTP hook | session_id now documented as always present |
| DSR-11 | Stop HTTP hook | Hook fires when Claude finishes responding |
| DSR-12 | Notification HTTP hook (NEW) | notification_type matcher supports "idle_prompt" |
| DSR-13 | SessionStart/SessionEnd remain command hooks | Already implemented in plugin.json |
| DSR-14 | hook-forward.sh deletion | File exists at scripts/hook-forward.sh, safe to delete |
| DSR-15 | HookPayload struct extension | Add tool_input, hook_event_name, transcript_path, etc. |
| DSR-16 | tool_input per-tool parsing | Confirmed tool_input schemas per tool type from official docs |
| DSR-17 | Session struct extension (LastFile, LastCommand, LastQuery) | Immutable copy pattern in registry.go |
| DSR-18 | displayDetail system (4 levels) | New DisplayDetail field in Config struct |
| DSR-19 | displayDetail: minimal level | Default -- {file} -> project name, {command} -> "..." |
| DSR-20 | displayDetail: standard level | {file} -> filename, {command} -> truncated 20 chars |
| DSR-21 | displayDetail: verbose level | {file} -> full relative path, {command} -> full |
| DSR-22 | displayDetail: private level | All placeholders redacted |
| DSR-23 | Config option for displayDetail | Enum in discord-presence-config.json |
| DSR-24 | Preset expansion to 40 messages | Current: 15-20 per category; target: 40 |
| DSR-25 | New placeholders: {file}, {filepath}, {command}, {query}, {activity}, {sessions} | Extend replacePlaceholders in resolver.go |
| DSR-26 | Double variation (messages x data) | StablePick rotation + changing file/command values |
| DSR-27 | Multi-session placeholders | {projects}, {models}, {totalCost}, {totalTokens} |
| DSR-28 | GET /presets endpoint | New handler in server.go |
| DSR-29 | GET /status endpoint | New handler with comprehensive status response |
| DSR-30 | POST /preview endpoint | Temporary presence override with duration timer |
| DSR-31 | session_id fallback fix | Replace HTTP 400 with slog.Warn + synthetic ID |
| DSR-32 | Self-triggering hook for session identification | /dsrcode:status triggers Bash -> PreToolUse -> identifies session |
| DSR-33 | JSONL dedup unchanged | Already implemented in main.go:423-428 |
| DSR-34 | Global preset switch (Hot-Reload) | Already implemented via POST /config + fsnotify |
| DSR-35 | Auto-installation via plugin install | start.sh handles binary download |
| DSR-36 | First-run hint | start.sh checks config existence |
| DSR-37 | No userConfig in plugin.json | Confirmed -- all config via /dsrcode:setup |
| DSR-38 | Coupled versioning (plugin.json = release tag = binary) | release.yml uses tag-triggered builds |
| DSR-39 | Binary auto-update via start.sh | Version comparison + re-download |
| DSR-40 | README in English | International plugin, not SRS-specific |
| DSR-41 | Real Discord screenshots via /dsrcode:demo | POST /preview sets exact presence |
| DSR-42 | README content structure | Features, Installation, Quick Start, Screenshots, Troubleshooting |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- Immutable data patterns -- create new objects, never mutate (relevant for Go Session struct updates)
- Files < 800 lines, Functions < 50 lines
- Conventional Commits: feat, fix, refactor, docs, test, chore
- 80%+ Test-Coverage
- Error handling: explicit at every level
- GSD Workflow Enforcement: use /gsd:execute-phase for planned work

## Standard Stack

### Core (Go Binary)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| Go | 1.26.0 | Language runtime | Already in use, go.mod declares 1.25 |
| fsnotify | 1.9.0 | Config hot-reload + JSONL watching | Already in use |
| go-winio | 0.6.2 | Windows Named Pipes (Discord IPC) | Already in use |
| lumberjack.v2 | 2.2.1 | Log rotation | Already in use |
| encoding/json (stdlib) | - | Payload parsing, tool_input handling | json.RawMessage for deferred parsing |

### Plugin System (Claude Code)
| Component | Version | Purpose | Why Standard |
|-----------|---------|---------|--------------|
| commands/*.md | - | Slash commands (/dsrcode:*) | Claude Code plugin convention; legacy but supported |
| hooks/hooks.json | - | HTTP hook definitions | Native plugin hook format |
| .claude-plugin/plugin.json | - | Plugin manifest | Required for Claude Code plugin system |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| commands/ (legacy) | skills/ (recommended) | skills/ is newer but requires SKILL.md subdirectory structure; commands/ is simpler for "Claude-as-Wizard" pattern where each .md IS the entire instruction set; both work identically for slash-command invocation |
| json.RawMessage for tool_input | Full struct per tool type | RawMessage is more flexible, avoids explosion of Go types, deferred parsing matches fire-and-forget hook pattern |
| Timer-based /preview timeout | "Next hook cancels preview" | Timer is more predictable; next-hook approach could leave preview stuck if no activity follows |

**No new dependencies needed.** All changes use existing Go stdlib (encoding/json, net/http, path/filepath, time) and existing third-party libraries already in go.mod.

## Architecture Patterns

### Recommended Changes to Project Structure
```
cc-discord-presence/
  .claude-plugin/
    plugin.json          # Update version to v3.0.0, keep SessionStart/SessionEnd command hooks
  commands/              # NEW directory
    dsrcode:setup.md     # 7-phase guided wizard
    dsrcode:preset.md    # Quick-switch preset
    dsrcode:status.md    # Status display
    dsrcode:log.md       # Log viewer
    dsrcode:doctor.md    # 7-category diagnostics
    dsrcode:update.md    # Binary updater
    dsrcode:demo.md      # Preview mode
  hooks/
    hooks.json           # REWRITE: command -> HTTP type
  scripts/
    start.sh             # UPDATE: version check + auto-update
    stop.sh              # UNCHANGED
    hook-forward.sh      # DELETE
  server/
    server.go            # ADD: /presets, /status, /preview handlers + payload extension
  session/
    types.go             # ADD: LastFile, LastFilePath, LastCommand, LastQuery fields
    registry.go          # ADD: UpdateLastActivity helper method
  resolver/
    resolver.go          # EXTEND: new placeholders, displayDetail-aware resolution
  config/
    config.go            # ADD: DisplayDetail field
  preset/
    types.go             # UPDATE: document new placeholders
    presets/*.json        # EXPAND: 15-20 -> 40 messages per category
  main.go               # UPDATE: Version constant to "3.0.0"
```

### Pattern 1: Claude-as-Wizard (Slash Commands)
**What:** Each /dsrcode:*.md file contains Claude instructions in markdown. When user types `/dsrcode:setup`, Claude reads the .md file and follows the instructions to guide the user through setup interactively.
**When to use:** For all 7 slash commands where Claude acts as interactive guide.
**Key insight:** The .md file IS the agent prompt. No code execution -- Claude interprets the markdown and uses its tools (Bash, Read, Write) to perform checks, write config files, and verify status.
**Example:**
```markdown
# /dsrcode:setup -- Guided Setup Wizard

You are guiding the user through DSR Code Presence setup. Follow these phases:

## Phase A: Prerequisites
1. Check binary exists: `ls ~/.claude/bin/cc-discord-presence-*`
2. Check Discord is running: attempt `curl -sf http://127.0.0.1:19460/health`
3. If daemon not running, execute: `~/.claude/bin/cc-discord-presence-* &`

## Phase B: Discord Connection Test
...
```

### Pattern 2: HTTP Hook Migration
**What:** Replace command hooks (bash + curl pipe) with native HTTP hooks in hooks.json.
**When to use:** For all activity-tracking hooks (PreToolUse, UserPromptSubmit, Stop, Notification).
**Before (current):**
```json
{
  "type": "command",
  "command": "bash ${CLAUDE_PLUGIN_ROOT}/scripts/hook-forward.sh pre-tool-use",
  "timeout": 2
}
```
**After (target):**
```json
{
  "type": "http",
  "url": "http://127.0.0.1:19460/hooks/pre-tool-use",
  "timeout": 1
}
```
**Key insight:** HTTP hooks send the full JSON payload as POST body. The Go server receives ALL hook fields natively (session_id, cwd, tool_name, tool_input, hook_event_name, transcript_path, permission_mode) -- no bash intermediary needed. Claude Code deduplicates HTTP hooks by URL.

### Pattern 3: Deferred Parsing with json.RawMessage
**What:** Accept tool_input as json.RawMessage, parse it per-tool-type only when needed.
**When to use:** In HookPayload struct for tool_input field.
**Example:**
```go
type HookPayload struct {
    SessionID      string          `json:"session_id"`
    Cwd            string          `json:"cwd"`
    ToolName       string          `json:"tool_name,omitempty"`
    ToolInput      json.RawMessage `json:"tool_input,omitempty"`
    HookEventName  string          `json:"hook_event_name,omitempty"`
    TranscriptPath string          `json:"transcript_path,omitempty"`
    PermissionMode string          `json:"permission_mode,omitempty"`
    // Event-specific fields
    UserPrompt       string `json:"prompt,omitempty"`
    Reason           string `json:"reason,omitempty"`
    Message          string `json:"message,omitempty"`
    NotificationType string `json:"notification_type,omitempty"`
}

// Per-tool input structs for deferred parsing
type FileToolInput struct {
    FilePath string `json:"file_path"`
}
type BashToolInput struct {
    Command string `json:"command"`
}
type SearchToolInput struct {
    Pattern string `json:"pattern"`
    Path    string `json:"path,omitempty"`
}
type AgentToolInput struct {
    Prompt string `json:"prompt"`
}
```

### Pattern 4: DisplayDetail Resolution
**What:** Resolver maps placeholder VALUES based on displayDetail level, not conditional syntax in messages.
**When to use:** In resolver.go when building placeholder replacement map.
**Example:**
```go
func buildPlaceholderValues(s *session.Session, detail config.DisplayDetail) map[string]string {
    values := map[string]string{
        "{project}": s.ProjectName,
        "{branch}":  s.Branch,
    }
    switch detail {
    case config.DetailMinimal:
        values["{file}"] = s.ProjectName
        values["{command}"] = "..."
        values["{query}"] = "*"
    case config.DetailStandard:
        values["{file}"] = filepath.Base(s.LastFilePath)
        values["{command}"] = truncate(s.LastCommand, 20)
        values["{query}"] = s.LastQuery
    case config.DetailVerbose:
        values["{file}"] = s.LastFilePath
        values["{command}"] = s.LastCommand
        values["{query}"] = s.LastQuery
        values["{project}"] = s.ProjectName  // full name
    case config.DetailPrivate:
        values["{file}"] = "file"
        values["{project}"] = "Project"
        values["{branch}"] = ""
        values["{command}"] = "..."
        values["{query}"] = ""
        values["{tokens}"] = ""
        values["{cost}"] = ""
    }
    return values
}
```

### Pattern 5: POST /preview with Timer-Based Reset
**What:** POST /preview sets an exact Discord presence for a specified duration (default 60s), then restores normal mode.
**When to use:** For /dsrcode:demo screenshot generation.
**Example:**
```go
type PreviewPayload struct {
    Preset        string `json:"preset"`
    DisplayDetail string `json:"displayDetail,omitempty"`
    Duration      int    `json:"duration,omitempty"` // seconds, default 60
    Details       string `json:"details,omitempty"`
    State         string `json:"state,omitempty"`
}

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
    var payload PreviewPayload
    // ... parse ...
    // Set preview activity on discord client
    // Start timer to restore normal resolution after Duration seconds
    time.AfterFunc(time.Duration(payload.Duration) * time.Second, func() {
        // Trigger normal presence update via onChange callback
    })
}
```

### Anti-Patterns to Avoid
- **Do NOT add PostToolUse hook** (D-17): Gets immediately overwritten by next PreToolUse, wastes hook calls.
- **Do NOT use userConfig in plugin.json** (D-37): All configuration flows through /dsrcode:setup, which uses Claude to write the config file directly.
- **Do NOT use skills/ directory**: While recommended for new plugins, commands/ is simpler for this use case (7 standalone .md files, no subdirectory structure needed, no SKILL.md boilerplate).
- **Do NOT parse tool_input eagerly**: Use json.RawMessage and only parse when needed for the specific tool type. Unknown tools should be ignored gracefully.
- **Do NOT break HTTP 400 for missing session_id**: D-31 requires fallback to synthetic ID, not rejection.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Message rotation anti-flicker | Custom random picker | Existing StablePick (Knuth hash, 5-min window) | Already battle-tested, deterministic, prevents Discord flickering |
| Config priority chain | Manual env/file/flag parsing | Existing config.LoadConfig | Already implements CLI > Env > JSON > Defaults |
| Config hot-reload | Manual file polling | Existing fsnotify watcher on config dir | Already handles atomic-save editors |
| Session dedup | Custom map logic | Existing registry.StartSession with ProjectPath+PID dedup | Already thread-safe with onChange callback |
| Discord IPC | Node.js discord-rpc | Existing Go discord.Client | Already handles Windows Named Pipes + Unix Sockets |
| Version comparison | Semver parsing library | Simple string comparison with VERSION constant | Version format is controlled by us (vX.Y.Z) |

**Key insight:** The existing codebase already handles all complex infrastructure (IPC, session management, config, presence resolution). This phase only extends existing patterns, never replaces them.

## Common Pitfalls

### Pitfall 1: HTTP Hook ECONNREFUSED on External URLs
**What goes wrong:** Claude Code's HTTP hook implementation has a TLS SNI bug when connecting to external HTTPS endpoints. The hostname:port gets included in the SNI field, causing TLS handshake failure.
**Why it happens:** Bun's HTTP client includes the port in the TLS SNI extension, which Cloudflare and other reverse proxies reject.
**How to avoid:** Use localhost URLs only (http://127.0.0.1:19460). This is what we're doing -- confirmed working.
**Warning signs:** ECONNREFUSED errors in Claude Code debug logs for non-localhost URLs.
**Reference:** anthropics/claude-code#30613 (OPEN, stale)

### Pitfall 2: session_id Availability Across Hook Types
**What goes wrong:** The open issue #37339 reports session_id missing in UserPromptSubmit, Stop, and PostCompact hooks.
**Why it happens:** Claude Code's hook runner historically didn't pass session_id to all event types.
**How to avoid:** D-31 mandates a fallback: if session_id is empty, log a warning and use synthetic ID "http-{projectName}". Official docs now say session_id is "always present" -- but we code defensively.
**Warning signs:** Empty session_id in hook payloads, HTTP 400 errors in server logs (current bug).
**Reference:** anthropics/claude-code#37339 (OPEN)

### Pitfall 3: Hook Timeout Too Short
**What goes wrong:** Hooks that take too long get killed by Claude Code, resulting in dropped activity updates.
**Why it happens:** HTTP hooks default to 30s timeout, but our hooks should be fire-and-forget.
**How to avoid:** Set timeout to 1 second (D-14). The Go server processes hooks in <1ms. If the daemon is down, the HTTP connect will fail fast.
**Warning signs:** Hooks silently timing out, missing activity updates in Discord.

### Pitfall 4: Concurrent Preset File Writes During Expansion
**What goes wrong:** Expanding 8 preset JSON files simultaneously could introduce inconsistencies if placeholders are used differently across presets.
**Why it happens:** New placeholders ({file}, {command}, {query}) need to be meaningful across all presets and all displayDetail levels.
**How to avoid:** Define all placeholder values centrally in the resolver. Preset messages just USE the placeholders -- the resolver decides what {file} means based on displayDetail.
**Warning signs:** Placeholders rendering as literal "{file}" in Discord (unreplaced).

### Pitfall 5: Discord Character Limits
**What goes wrong:** Details and State fields that exceed 128 characters get silently truncated by Discord, potentially cutting off meaningful content.
**Why it happens:** Discord API enforces 2-128 character limit on Details/State.
**How to avoid:** The existing truncate() function handles this. New messages with {filepath} in verbose mode could easily exceed 128 chars. Always truncate after placeholder replacement, not before.
**Warning signs:** Messages ending with "..." (ellipsis) in Discord.

### Pitfall 6: Plugin Cache Invalidation
**What goes wrong:** After `claude plugin install`, changes to the plugin during development aren't picked up because Claude Code caches plugins.
**Why it happens:** Claude Code copies marketplace plugins to ~/.claude/plugins/cache for security.
**How to avoid:** During development, use `claude --plugin-dir /path/to/plugin` instead of installing. For testing updates, bump version in plugin.json and reinstall.
**Warning signs:** Old hook behavior persisting after code changes.

### Pitfall 7: Windows Path Separators in tool_input
**What goes wrong:** file_path from tool_input uses backslashes on Windows (C:\Users\...) but forward slashes on Unix.
**Why it happens:** Claude Code passes the path as-is from the OS.
**How to avoid:** Use filepath.Base() and filepath.ToSlash() when extracting file names/paths from tool_input. Store normalized paths in Session.
**Warning signs:** Inconsistent file display on Windows vs. Unix.

## Code Examples

### HTTP Hooks Migration (hooks.json)
```json
{
  "description": "Discord Rich Presence activity tracking hooks",
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Edit|Write|Bash|Read|Grep|Glob|WebSearch|WebFetch|Task",
        "hooks": [
          {
            "type": "http",
            "url": "http://127.0.0.1:19460/hooks/pre-tool-use",
            "timeout": 1
          }
        ]
      }
    ],
    "UserPromptSubmit": [
      {
        "hooks": [
          {
            "type": "http",
            "url": "http://127.0.0.1:19460/hooks/user-prompt-submit",
            "timeout": 1
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "http",
            "url": "http://127.0.0.1:19460/hooks/stop",
            "timeout": 1
          }
        ]
      }
    ],
    "Notification": [
      {
        "matcher": "idle_prompt",
        "hooks": [
          {
            "type": "http",
            "url": "http://127.0.0.1:19460/hooks/notification",
            "timeout": 1
          }
        ]
      }
    ]
  }
}
```
Source: Claude Code official docs (code.claude.com/docs/en/hooks, code.claude.com/docs/en/plugins-reference)

### Extended HookPayload (server.go)
```go
type HookPayload struct {
    SessionID        string          `json:"session_id"`
    Cwd              string          `json:"cwd"`
    ToolName         string          `json:"tool_name,omitempty"`
    ToolInput        json.RawMessage `json:"tool_input,omitempty"`
    HookEventName    string          `json:"hook_event_name,omitempty"`
    TranscriptPath   string          `json:"transcript_path,omitempty"`
    PermissionMode   string          `json:"permission_mode,omitempty"`
    // UserPromptSubmit
    Prompt           string          `json:"prompt,omitempty"`
    // Stop
    // (no extra fields beyond base)
    // Notification
    NotificationType string          `json:"notification_type,omitempty"`
    Message          string          `json:"message,omitempty"`
    Title            string          `json:"title,omitempty"`
}
```
Source: Verified against official hook input documentation (code.claude.com/docs/en/hooks)

### tool_input Parsing per Tool Type
```go
func extractToolContext(toolName string, rawInput json.RawMessage) (file, filePath, command, query string) {
    if len(rawInput) == 0 {
        return
    }
    switch toolName {
    case "Edit", "Write", "Read":
        var input FileToolInput
        if json.Unmarshal(rawInput, &input) == nil {
            filePath = input.FilePath
            file = filepath.Base(filePath)
        }
    case "Bash":
        var input BashToolInput
        if json.Unmarshal(rawInput, &input) == nil {
            command = input.Command
        }
    case "Grep", "Glob":
        var input SearchToolInput
        if json.Unmarshal(rawInput, &input) == nil {
            query = input.Pattern
        }
    case "Task", "Agent":
        var input AgentToolInput
        if json.Unmarshal(rawInput, &input) == nil {
            // prompt is internal-only, don't expose to Discord
        }
    }
    return
}
```
Source: Confirmed tool_input schemas from official hook documentation (code.claude.com/docs/en/hooks)

### Extended Session Struct (types.go)
```go
type Session struct {
    // ... existing fields ...
    LastFile       string `json:"lastFile"`
    LastFilePath   string `json:"lastFilePath"`
    LastCommand    string `json:"lastCommand"`
    LastQuery      string `json:"lastQuery"`
}
```

### GET /status Response
```go
type StatusResponse struct {
    Version          string            `json:"version"`
    Uptime           string            `json:"uptime"`
    DiscordConnected bool              `json:"discordConnected"`
    Config           StatusConfig      `json:"config"`
    Sessions         []*session.Session `json:"sessions"`
    HookStats        HookStats         `json:"hookStats"`
}

type StatusConfig struct {
    Preset        string `json:"preset"`
    DisplayDetail string `json:"displayDetail"`
    Port          int    `json:"port"`
    BindAddr      string `json:"bindAddr"`
}

type HookStats struct {
    Total          int64            `json:"total"`
    ByType         map[string]int64 `json:"byType"`
    LastReceivedAt *time.Time       `json:"lastReceivedAt"`
}
```

### DisplayDetail Enum (config.go)
```go
type DisplayDetail string

const (
    DetailMinimal  DisplayDetail = "minimal"
    DetailStandard DisplayDetail = "standard"
    DetailVerbose  DisplayDetail = "verbose"
    DetailPrivate  DisplayDetail = "private"
)
```

### start.sh Version Check Pattern
```bash
# Version check for auto-update
CURRENT_VERSION=$("$BINARY" --version 2>/dev/null || echo "unknown")
if [[ "$CURRENT_VERSION" != "$VERSION" ]]; then
    echo "Updating cc-discord-presence from $CURRENT_VERSION to $VERSION..."
    rm -f "$BINARY"
    # Re-download logic (same as initial download)
fi
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| command hooks (bash + curl) | Native HTTP hooks in hooks.json | Claude Code ~2025 Q4 | Eliminates bash intermediary, sends full JSON payload directly, 10x faster |
| commands/ directory for slash commands | skills/ with SKILL.md recommended | Claude Code ~2026 Q1 | Both formats work; commands/ is legacy but simpler for standalone .md files |
| 14 hook events | 23+ hook events | Claude Code through 2026 | New events: Notification, SubagentStart/Stop, TaskCreated/Completed, FileChanged, CwdChanged, etc. |
| session_id missing in some hooks | session_id documented as always present | Possibly fixed in recent versions | Defensive fallback still needed (D-31) |

**Deprecated/outdated:**
- `hook-forward.sh` bash script: Replaced by native HTTP hooks. Safe to delete.
- `@supabase/auth-helpers-nextjs`: Not relevant to this phase (plugin is standalone Go).

## Open Questions

1. **session_id: Fixed or Still Broken?**
   - What we know: Official docs say session_id is "always present" in all hook events. Issue #37339 (OPEN) reports it missing in UserPromptSubmit, Stop, PostCompact.
   - What's unclear: Whether recent Claude Code versions (2.1.89 installed) actually send session_id to all hook types.
   - Recommendation: Implement D-31 fallback (synthetic ID "http-{projectName}") regardless. Test empirically during implementation. The fallback is cheap and defensive.

2. **Notification Hook Payload Structure**
   - What we know: Official docs show notification_type field with matchers (idle_prompt, permission_prompt, auth_success, elicitation_dialog). Also includes message and title fields.
   - What's unclear: Exact format when idle_prompt fires -- does it include session_id, cwd?
   - Recommendation: Treat same as other hooks -- parse session_id and cwd from payload. If missing, use fallback pattern.

3. **Binary --version Flag**
   - What we know: start.sh needs to compare local binary version with VERSION constant (D-39).
   - What's unclear: Whether to add a --version flag to the Go binary or embed version in a different way.
   - Recommendation: Add `--version` flag to the Go binary that prints the Version constant and exits. This is the standard Go pattern (cf. `go version`, `docker version`). Trivial to implement with flag package.

4. **POST /preview Duration Cleanup**
   - What we know: POST /preview should show a specific presence for N seconds, then restore.
   - What's unclear: What happens if another POST /preview comes in during active preview (override or queue?). What if Claude session ends during preview?
   - Recommendation: Use time.AfterFunc. New preview cancels any existing preview timer. Session end should trigger normal resolution via the registry onChange callback.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | Binary compilation | Yes | 1.26.0 | -- |
| curl | start.sh binary download | Yes | 8.18.0 | wget |
| bash | Shell scripts, Claude hook execution | Yes | 5.2.37 | -- |
| gh | Release management, /dsrcode:update | Yes | 2.83.0 | -- |
| jq | JSON parsing in scripts | Yes | 1.8.1 | -- |
| PowerShell | Windows daemon start | Yes | Available | -- |
| Claude Code CLI | Plugin system | Yes | 2.1.89 | -- |
| Discord Desktop | IPC connection | Required at runtime | -- | Error message in /dsrcode:doctor |

**Missing dependencies with no fallback:**
- None -- all required tools are available.

**Missing dependencies with fallback:**
- GitHub Releases: No releases yet for DSR-Labs/cc-discord-presence. First release needs to be created (v3.0.0) after implementation.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) |
| Config file | None needed -- `go test` auto-discovers |
| Quick run command | `cd C:/Users/ktown/Projects/cc-discord-presence && go test ./...` |
| Full suite command | `cd C:/Users/ktown/Projects/cc-discord-presence && go test -v -race ./...` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DSR-15 | HookPayload struct parses extended fields | unit | `go test ./server/ -run TestExtendedHookPayload -v` | No -- Wave 0 |
| DSR-16 | tool_input per-tool parsing (file, bash, grep) | unit | `go test ./server/ -run TestToolInputParsing -v` | No -- Wave 0 |
| DSR-17 | Session struct has LastFile/LastCommand/LastQuery | unit | `go test ./session/ -run TestSessionLastFields -v` | No -- Wave 0 |
| DSR-18 | displayDetail levels produce correct placeholder values | unit | `go test ./resolver/ -run TestDisplayDetail -v` | No -- Wave 0 |
| DSR-22 | private level redacts all sensitive data | unit | `go test ./resolver/ -run TestDisplayDetailPrivate -v` | No -- Wave 0 |
| DSR-23 | Config loads displayDetail from JSON | unit | `go test ./config/ -run TestDisplayDetail -v` | No -- Wave 0 |
| DSR-24 | All 8 presets have 40+ messages per activity | unit | `go test ./preset/ -run TestPresetMessageCounts -v` | No -- Wave 0 |
| DSR-25 | New placeholders replaced in messages | unit | `go test ./resolver/ -run TestNewPlaceholders -v` | No -- Wave 0 |
| DSR-28 | GET /presets returns all presets | integration | `go test ./server/ -run TestGetPresets -v` | No -- Wave 0 |
| DSR-29 | GET /status returns comprehensive status | integration | `go test ./server/ -run TestGetStatus -v` | No -- Wave 0 |
| DSR-30 | POST /preview sets temporary presence | integration | `go test ./server/ -run TestPostPreview -v` | No -- Wave 0 |
| DSR-31 | session_id fallback creates synthetic ID | unit | `go test ./server/ -run TestSessionIdFallback -v` | No -- Wave 0 |
| DSR-08 | HTTP hooks format valid in hooks.json | manual-only | Visual inspection of hooks.json structure | -- |
| DSR-01-07 | Slash command .md files contain correct instructions | manual-only | Claude loads and follows instructions | -- |

### Sampling Rate
- **Per task commit:** `cd C:/Users/ktown/Projects/cc-discord-presence && go test ./...`
- **Per wave merge:** `cd C:/Users/ktown/Projects/cc-discord-presence && go test -v -race ./...`
- **Phase gate:** Full suite green before /gsd:verify-work

### Wave 0 Gaps
- [ ] `server/server_test.go` -- add tests for extended payload, new endpoints, session_id fallback
- [ ] `session/registry_test.go` -- add tests for new Session fields (LastFile, LastCommand, LastQuery)
- [ ] `resolver/resolver_test.go` -- add tests for displayDetail levels, new placeholders
- [ ] `config/config_test.go` -- add tests for DisplayDetail field loading
- [ ] `preset/preset_test.go` -- add test for 40+ message count validation
- [ ] No new test files needed -- all existing test files cover the packages being extended

## Sources

### Primary (HIGH confidence)
- [Claude Code Plugins Reference](https://code.claude.com/docs/en/plugins-reference) -- Complete plugin manifest schema, commands/ directory, hooks.json format, environment variables
- [Claude Code Hooks Reference](https://code.claude.com/docs/en/hooks) -- All 23+ hook events, input payloads per event type, HTTP hook format, session_id availability, timeout configuration
- Go source code (cc-discord-presence repository) -- Direct reading of server.go, types.go, registry.go, resolver.go, config.go, main.go, preset/loader.go, hooks.json, plugin.json, start.sh, stop.sh, hook-forward.sh

### Secondary (MEDIUM confidence)
- [GitHub Issue #37339](https://github.com/anthropics/claude-code/issues/37339) -- session_id missing in some hook types (OPEN, may be resolved in recent versions)
- [GitHub Issue #30613](https://github.com/anthropics/claude-code/issues/30613) -- HTTP hooks ECONNREFUSED for external URLs, localhost confirmed working (TLS SNI bug in Bun)
- [GitHub Issue #37559](https://github.com/anthropics/claude-code/issues/37559) -- Stop hooks + prompt type broken, capabilities undocumented per event type (OPEN)

### Tertiary (LOW confidence)
- Community guides on Claude Code hooks (various blog posts from 2026) -- Consistent with official docs, provided supplementary examples

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- No new dependencies, all patterns verified in existing codebase
- Architecture: HIGH -- Extensions to well-understood Go code with existing patterns
- Plugin system: HIGH -- Official docs comprehensive, HTTP hooks confirmed working for localhost
- Hook payloads: HIGH -- Official docs document exact JSON schemas per event type
- session_id availability: MEDIUM -- Docs say always present, open issue says otherwise; defensive fallback covers both cases
- Pitfalls: HIGH -- Known issues documented with concrete workarounds

**Research date:** 2026-04-03
**Valid until:** 2026-05-03 (30 days -- Claude Code hook system is evolving but core patterns are stable)
