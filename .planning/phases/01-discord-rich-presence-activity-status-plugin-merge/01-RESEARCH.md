# Phase 1: Discord Rich Presence + Activity Status Plugin Merge - Research

**Researched:** 2026-04-02
**Domain:** Go binary development, Discord IPC, HTTP server, cross-platform build
**Confidence:** HIGH

## Summary

This phase merges two existing projects -- cc-discord-presence (Go, Rich Presence) and claude-code-discord-status (Node.js, Activity Status) -- into a single Go binary under DSR-Labs/cc-discord-presence. The Go binary already has working Discord IPC (Named Pipes on Windows, Unix Sockets on Linux/macOS), fsnotify file watching, and JSONL fallback parsing. The Node.js project contributes session registry, multi-session management, preset-based message rotation (stablePick with Knuth hash), activity counting, idle/stale detection, and an HTTP server for hook events.

The existing Go codebase (main.go ~510 lines, discord/client.go ~167 lines) is well-structured but simple -- single session, no HTTP server, no presets, no activity icons. The Node.js codebase has the complete feature set for activity tracking, presets, multi-session, and stats formatting that needs to be ported to Go. Both repos use MIT license.

**Primary recommendation:** Use the existing Go binary as the skeleton. Add net/http server (no framework needed -- only ~6 routes), port SessionRegistry/resolver/presets as Go packages, embed preset data with `go:embed`, use lumberjack for log rotation. Keep the existing discord/ IPC package and fsnotify watcher intact.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Hybrid-Ansatz -- Go-Binary als Basis (cc-discord-presence), Activity-Status-Features aus claude-code-discord-status integrieren
- **D-02:** Go-Binary behaelt bestehende Architektur: fsnotify File-Watching, Statusline + JSONL Fallback, Discord IPC via Named Pipes (Windows) / Unix Sockets
- **D-03:** Kein Node.js Daemon -- alles in einer Go-Binary
- **D-04:** HTTP-Hooks direkt an Go HTTP-Server (Claude Code `"type": "http"` Hooks). Kein Bash-Script, kein File-Intermediaer fuer Activity-Daten
- **D-05:** Go-Binary startet HTTP-Server (Default Port 19460, konfigurierbar) der Hook-Events empfaengt
- **D-06:** Hook-Events: PreToolUse (Edit/Write/Bash/Read/Grep/Glob/WebSearch/Task), UserPromptSubmit, Stop, Notification -- alle als HTTP-Hooks
- **D-07:** Activity-Mapping: Write/Edit -> coding, Bash -> terminal, Read -> reading, Grep/Glob -> searching, WebSearch/WebFetch -> searching, Task -> thinking, UserPromptSubmit -> thinking, Stop -> idle/finished, Notification -> idle/waiting
- **D-08 to D-15:** Layout D Maximum Output -- Details: "{Activity} . {ProjectName} ({Branch})", State: "{Model} | {Tokens} tokens | ${Cost}", Large Image fixed "dsr-code", Small Image dynamic activity icon, Timestamps for elapsed timer, up to 2 preset-dependent buttons
- **D-16:** App-Name "DSR Code" -- eigene Discord Application
- **D-17 to D-20:** Fork under DSR-Labs, GitHub Actions 5-platform release, Claude Code Plugin System
- **D-21 to D-25:** 8 Presets from scratch (~50 messages each): minimal, professional, dev-humor, chaotic, german, hacker, streamer, weeb. Message rotation with Knuth hash 5-min buckets. Multi-session support with activity counters and stats line.
- **D-26 to D-28:** Complete multi-session logic: session registry, activity counter, stats line, dominant mode, stale check. 1 session direct, 2-4 tier messages, 5+ overflow with {n}.
- **D-29 to D-31:** Idle/stale: 10min idle, 30min remove, 30s PID check. Configurable.
- **D-32:** Preset-dependent buttons (max 2, Discord limit).
- **D-33 to D-35:** Config priority: CLI flags > env vars > JSON config > defaults. Config at ~/.claude/discord-presence-config.json.
- **D-36 to D-38:** Statusline wrapper POSTs to HTTP server, no file intermediary.
- **D-39 to D-41:** Hybrid hooks: SessionStart/SessionEnd as command hooks (start.sh/stop.sh), activity as HTTP hooks. Refcount (Windows) / PID files (Unix).
- **D-42 to D-44:** Exponential backoff (1s->30s max), silent failure on activity updates, crash recovery via start.sh.
- **D-45 to D-47:** AI-generated icons via fal.ai MCP (1024x1024 PNG), 10 icons.
- **D-48:** Statusline delivers total_cost_usd -- no own pricing table needed except JSONL fallback.
- **D-49 to D-50:** Config hot-reload via fsnotify. HTTP endpoint POST /config for programmatic preset switch.
- **D-51:** JSONL fallback retained -- three tiers of accuracy.
- **D-52 to D-54:** Built-in log file ~/.claude/discord-presence.log with rotation (5MB), verbosity flags (-v/-q), config logLevel.
- **D-55:** HTTP server default 127.0.0.1, configurable to 0.0.0.0. No auth token.
- **D-56:** Discord Application "DSR Code" setup as task in plan.

### Claude's Discretion
- HTTP-Server implementation in Go (net/http or chi/echo)
- Go package structure and module organization
- Preset data format (embedded Go structs vs external JSON)
- Exact exponential backoff implementation
- Windows-specific adjustments (Named Pipes, PowerShell scripts)
- JSONL fallback pricing map maintenance
- Log rotation implementation
- fsnotify config watch debouncing

### Deferred Ideas (OUT OF SCOPE)
- PR upstream to tsanva/cc-discord-presence
- Custom Discord App Icons via own design iteration after v1.0
- Twitch/YouTube Stream Integration (Streaming Activity Type)
- Voice-Channel Rich Presence
- New preset "corporate" for enterprise environments
- Auto-update checker in Go binary
- Web dashboard for session statistics
- Preset editor CLI/TUI
</user_constraints>

## Standard Stack

### Core (Go Binary)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/fsnotify/fsnotify` | v1.9.0 | File system watching (config reload, JSONL fallback) | Already in go.mod, cross-platform, de-facto standard |
| `github.com/Microsoft/go-winio` | v0.6.2 | Windows Named Pipe for Discord IPC | Already in go.mod, Microsoft-maintained, required for Windows |
| `gopkg.in/natefinch/lumberjack.v2` | v2.2.1 | Log file rotation (5MB max) | De-facto Go log rotation, io.Writer interface, zero-config |
| `net/http` (stdlib) | Go 1.26 | HTTP server for hook events | Only ~6 routes needed, stdlib sufficient, zero dependencies |
| `encoding/json` (stdlib) | Go 1.26 | JSON parsing for hooks, config, JSONL | Stdlib, no external dependency needed |
| `sync` (stdlib) | Go 1.26 | sync.RWMutex for session registry | Concurrent map access, standard pattern |
| `flag` (stdlib) | Go 1.26 | CLI flags (-v, -q, --preset, --port) | Simple flag parsing, no framework needed |
| `os/exec` (stdlib) | Go 1.26 | Git branch detection, PID liveness (Unix kill -0) | Already used in existing codebase |
| `embed` (stdlib) | Go 1.26 | Embed preset JSON data in binary | Single binary distribution, no external files |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `log/slog` (stdlib) | Go 1.26 | Structured logging with levels | Replaces fmt.Println for proper log levels |
| `time` (stdlib) | Go 1.26 | Timers, tickers for stale checks, debounce | Session timeouts, message rotation buckets |
| `context` (stdlib) | Go 1.26 | Graceful shutdown, HTTP server lifecycle | Clean goroutine cancellation |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| net/http | chi router | Adds dependency for ~6 routes. Go 1.22+ ServeMux has method routing. Not worth it. |
| net/http | echo/gin | Full framework overkill for internal localhost-only API. Adds 10+ transitive deps. |
| lumberjack | manual rotation | Lumberjack handles edge cases (concurrent writes, atomic rotation). 50 lines of code vs well-tested library. |
| embed presets | external JSON files | External files need file path resolution, can be lost. Embed is single-binary, zero-config. |
| sync.RWMutex | sync.Map | RWMutex is better for read-heavy workloads with known key types. sync.Map is for highly concurrent independent keys. |
| flag (stdlib) | cobra/pflag | Overkill for 5 flags. No subcommands needed. |

**Installation (new dependencies only):**
```bash
go get gopkg.in/natefinch/lumberjack.v2@v2.2.1
```

## Architecture Patterns

### Recommended Project Structure
```
cc-discord-presence/
├── main.go              # Entry point, CLI flags, orchestration
├── go.mod
├── go.sum
├── discord/
│   ├── client.go        # Discord IPC client (existing)
│   ├── client_test.go   # (existing)
│   ├── conn_unix.go     # Unix socket connection (existing)
│   └── conn_windows.go  # Windows Named Pipe (existing)
├── server/
│   ├── server.go        # net/http server, route handlers
│   └── server_test.go
├── session/
│   ├── registry.go      # SessionRegistry (ported from TS)
│   ├── types.go         # Session, ActivityCounts, Status types
│   ├── stale.go         # Stale/idle detection, PID liveness
│   ├── stale_unix.go    # Unix PID check (kill -0)
│   ├── stale_windows.go # Windows PID check (tasklist)
│   └── registry_test.go
├── resolver/
│   ├── resolver.go      # Presence resolver (single/multi session)
│   ├── stable_pick.go   # Knuth hash message rotation
│   ├── stats.go         # formatStatsLine, detectDominantMode
│   └── resolver_test.go
├── preset/
│   ├── types.go         # MessagePreset struct
│   ├── loader.go        # Load preset by name
│   ├── presets/         # Embedded JSON preset files
│   │   ├── minimal.json
│   │   ├── professional.json
│   │   ├── dev-humor.json
│   │   ├── chaotic.json
│   │   ├── german.json
│   │   ├── hacker.json
│   │   ├── streamer.json
│   │   └── weeb.json
│   └── preset_test.go
├── config/
│   ├── config.go        # Config loading (CLI > env > file > defaults)
│   ├── watcher.go       # fsnotify config hot-reload with debounce
│   └── config_test.go
├── logger/
│   ├── logger.go        # slog setup + lumberjack rotation
│   └── logger_test.go
├── scripts/
│   ├── start.sh         # Plugin SessionStart hook (existing, updated)
│   └── stop.sh          # Plugin SessionEnd hook (existing, updated)
├── .claude-plugin/
│   └── plugin.json      # Plugin manifest with HTTP + command hooks
└── .github/
    └── workflows/
        └── release.yml  # 5-platform cross-compile (existing, updated)
```

### Pattern 1: Session Registry with RWMutex
**What:** In-memory concurrent-safe session map with change notification
**When to use:** All session lifecycle operations (start, activity, end, stale check)
**Example:**
```go
// Source: Ported from claude-code-discord-status/src/daemon/sessions.ts
type SessionRegistry struct {
    mu       sync.RWMutex
    sessions map[string]*Session
    onChange  func()
}

func (r *SessionRegistry) UpdateActivity(sessionID string, req ActivityRequest) *Session {
    r.mu.Lock()
    defer r.mu.Unlock()

    session, ok := r.sessions[sessionID]
    if !ok {
        return nil
    }

    // Update fields (immutable-style: create new session value)
    updated := *session
    if req.Details != "" {
        updated.Details = req.Details
    }
    if req.SmallImageKey != "" {
        updated.SmallImageKey = req.SmallImageKey
    }
    // Increment activity counter
    if counterKey, ok := counterMap[updated.SmallImageKey]; ok {
        updated.ActivityCounts[counterKey]++
    }
    updated.LastActivityAt = time.Now()
    updated.Status = StatusActive

    r.sessions[sessionID] = &updated
    if r.onChange != nil {
        r.onChange()
    }
    return &updated
}
```

### Pattern 2: Stable Message Pick (Anti-Flicker)
**What:** Deterministic message selection from a pool using Knuth multiplicative hash with 5-minute time buckets
**When to use:** Every presence update to avoid rapid message switching
**Example:**
```go
// Source: Ported from claude-code-discord-status/src/daemon/resolver.ts
const MessageRotationInterval = 5 * time.Minute

func StablePick(pool []string, seed int64, now time.Time) string {
    bucket := now.Unix() / int64(MessageRotationInterval.Seconds())
    // Knuth multiplicative hash
    index := uint32(bucket*2654435761+seed) % uint32(len(pool))
    return pool[index]
}
```

### Pattern 3: HTTP Hook Receiver
**What:** Lightweight HTTP server receiving Claude Code hook events as POST requests
**When to use:** All activity events (PreToolUse, UserPromptSubmit, Stop, Notification)
**Example:**
```go
// Source: Claude Code hooks docs (https://code.claude.com/docs/en/hooks)
mux := http.NewServeMux()

// Activity hook from Claude Code HTTP hooks
mux.HandleFunc("POST /hooks/{hookType}", func(w http.ResponseWriter, r *http.Request) {
    hookType := r.PathValue("hookType")
    var body HookPayload
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        http.Error(w, "invalid json", 400)
        return
    }
    // Map hook to activity, update session
    activity := mapHookToActivity(hookType, body)
    registry.UpdateActivity(body.SessionID, activity)
    w.WriteHeader(200) // Empty 2xx = success for Claude Code
})

// Statusline data receiver
mux.HandleFunc("POST /statusline", handleStatusline)

// Config update endpoint
mux.HandleFunc("POST /config", handleConfigUpdate)

// Health/sessions endpoints
mux.HandleFunc("GET /health", handleHealth)
mux.HandleFunc("GET /sessions", handleSessions)

server := &http.Server{
    Addr:    net.JoinHostPort(cfg.BindAddr, strconv.Itoa(cfg.Port)),
    Handler: mux,
}
```

### Pattern 4: Config Hot-Reload with Debounce
**What:** fsnotify watches config file, debounces rapid events (editors emit 3-12 events per save), reloads after 100ms silence
**When to use:** Config file changes at ~/.claude/discord-presence-config.json
**Example:**
```go
func WatchConfig(path string, onReload func(Config)) {
    watcher, _ := fsnotify.NewWatcher()
    watcher.Add(filepath.Dir(path)) // Watch directory, not file (handles atomic renames)

    var debounceTimer *time.Timer

    for event := range watcher.Events {
        if filepath.Base(event.Name) != filepath.Base(path) {
            continue
        }
        if debounceTimer != nil {
            debounceTimer.Stop()
        }
        debounceTimer = time.AfterFunc(100*time.Millisecond, func() {
            if cfg, err := LoadConfigFile(path); err == nil {
                onReload(cfg)
            }
        })
    }
}
```

### Pattern 5: Cross-Platform PID Liveness Check
**What:** Build-tagged files for Windows (tasklist) vs Unix (kill -0)
**When to use:** Stale session detection every 30 seconds
**Example:**
```go
// stale_unix.go
//go:build !windows

func IsPidAlive(pid int) bool {
    proc, err := os.FindProcess(pid)
    if err != nil {
        return false
    }
    // Signal 0 doesn't kill, just checks existence
    return proc.Signal(syscall.Signal(0)) == nil
}

// stale_windows.go
//go:build windows

func IsPidAlive(pid int) bool {
    cmd := exec.Command("tasklist", "/FI", fmt.Sprintf("PID eq %d", pid), "/NH")
    output, err := cmd.Output()
    if err != nil {
        return false
    }
    return strings.Contains(string(output), strconv.Itoa(pid))
}
```

### Anti-Patterns to Avoid
- **Goroutine leak on shutdown:** Always use context.Context and cancel functions for all goroutines (HTTP server, stale checker, config watcher, Discord reconnect). The existing codebase uses signal.Notify but doesn't propagate cancellation.
- **Discord update flooding:** Discord rate-limits presence updates to 1 per 15 seconds. The SDK queues internally but rapid updates can empty presence. Always debounce before calling SetActivity.
- **Shared state without mutex:** Session registry is accessed from HTTP handlers (multiple goroutines), stale checker (ticker goroutine), and presence resolver (update goroutine). Must use RWMutex.
- **Watching file instead of directory:** fsnotify on a file breaks when editors do atomic save (write temp + rename). Watch the parent directory and filter by filename.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Log rotation | Custom file size checker + rename | `lumberjack.v2` | Handles concurrent writes, atomic rotation, cross-platform. 3 lines of config. |
| Discord IPC protocol | Raw pipe/socket protocol | Existing `discord/` package | Already implemented and tested. Binary framing, handshake, JSON payloads. |
| Cross-platform builds | Manual GOOS/GOARCH scripts | GitHub Actions matrix + `softprops/action-gh-release` | Already working in release.yml. Just update Go version. |
| PID liveness (Windows) | WMI queries or syscalls | `tasklist /FI "PID eq N"` via exec | Simple, reliable, no CGO dependency. Already used in start.sh. |
| Config file watching | Manual polling | `fsnotify` + debounce timer | Already a dependency. Handles inotify/kqueue/ReadDirectoryChangesW. |

**Key insight:** The Go codebase already has the hardest parts solved (Discord IPC cross-platform, binary distribution pipeline). The Node.js features being ported are pure application logic (session management, presets, stats) that translate cleanly to Go.

## Discord Rich Presence API Constraints

| Constraint | Value | Source |
|------------|-------|--------|
| Presence update rate limit | 1 per 15 seconds (SDK queues internally) | [Discord API docs issue #668](https://github.com/discord/discord-api-docs/issues/668) |
| Details field max length | 128 characters | Discord IPC protocol |
| State field max length | 128 characters | Discord IPC protocol |
| Details/State min length | 2 characters | Discord IPC protocol |
| Buttons per activity | Max 2 | Discord Activity Object spec |
| Button label length | 1-32 characters | Discord Activity Object spec |
| Custom assets per app | Max 300 | Discord Developer Portal |
| Asset keys | Auto-lowercased | Discord docs |
| Asset image size | 1024x1024 PNG recommended | Discord Rich Presence best practices |
| Buttons visibility | Only visible to OTHER users, not self | Discord spec |
| IPC pipe names | `discord-ipc-{0-9}` | Discord IPC protocol |

### Discord IPC Protocol (Already Implemented)
The existing `discord/client.go` implements the full IPC protocol:
- Binary framing: `[opcode:4bytes LE][length:4bytes LE][JSON payload]`
- Opcode 0: Handshake (`{"v": 1, "client_id": "..."}`)
- Opcode 1: Frame (SET_ACTIVITY command)
- Activity fields: details, state, assets (large_image, large_text, small_image, small_text), timestamps (start), buttons

### Buttons in IPC (Need to Add)
The existing Activity struct and SetActivity method do NOT include buttons. Need to add:
```go
type Activity struct {
    // ... existing fields ...
    Buttons []Button `json:"-"` // Custom serialization needed
}

type Button struct {
    Label string `json:"label"`
    URL   string `json:"url"`
}
```
Buttons go in the activity data alongside details/state/assets/timestamps.

## Claude Code Hooks Integration

### Hook Architecture (D-39)
Two types of hooks in the plugin manifest:

1. **Command hooks** (SessionStart, SessionEnd): start.sh/stop.sh manage binary lifecycle
2. **HTTP hooks** (PreToolUse, PostToolUse, UserPromptSubmit, Stop, Notification): POST to Go HTTP server

### Plugin Manifest Structure
```json
{
  "name": "cc-discord-presence",
  "version": "2.0.0",
  "description": "Discord Rich Presence + Activity Status for Claude Code",
  "hooks": {
    "SessionStart": [{
      "matcher": "startup|clear|compact|resume",
      "hooks": [{
        "type": "command",
        "command": "${CLAUDE_PLUGIN_ROOT}/scripts/start.sh",
        "timeout": 10
      }]
    }],
    "SessionEnd": [{
      "hooks": [{
        "type": "command",
        "command": "${CLAUDE_PLUGIN_ROOT}/scripts/stop.sh",
        "timeout": 5
      }]
    }],
    "PreToolUse": [{
      "matcher": "Edit|Write|Bash|Read|Grep|Glob|WebSearch|WebFetch|Task",
      "hooks": [{
        "type": "http",
        "url": "http://127.0.0.1:19460/hooks/pre-tool-use",
        "timeout": 2
      }]
    }],
    "UserPromptSubmit": [{
      "hooks": [{
        "type": "http",
        "url": "http://127.0.0.1:19460/hooks/user-prompt-submit",
        "timeout": 2
      }]
    }],
    "Stop": [{
      "hooks": [{
        "type": "http",
        "url": "http://127.0.0.1:19460/hooks/stop",
        "timeout": 2
      }]
    }],
    "Notification": [{
      "hooks": [{
        "type": "http",
        "url": "http://127.0.0.1:19460/hooks/notification",
        "timeout": 2
      }]
    }]
  }
}
```

### Key Hook Behaviors
- **HTTP hook timeout default:** 30 seconds (we should set 2s -- presence is non-critical)
- **HTTP hook failure:** Non-blocking. Connection failure or non-2xx = execution continues. Perfect for optional presence.
- **Empty 2xx body:** Success signal to Claude Code. Our hooks should always return 200 with empty body (fastest).
- **Matcher on UserPromptSubmit/Stop:** NOT supported (always fires). No matcher field needed.
- **PreToolUse matcher:** Regex on tool name. `"Edit|Write|Bash|Read|Grep|Glob|WebSearch|WebFetch|Task"` covers all activity-relevant tools.

### Hook Payload Structure
Claude Code sends the event's JSON input as HTTP POST body. Key fields:
- PreToolUse: `{"tool_name": "Edit", "tool_input": {...}, "session_id": "...", "cwd": "..."}`
- UserPromptSubmit: `{"session_id": "...", "cwd": "..."}`
- Stop: `{"session_id": "...", "cwd": "..."}`

### Known Issue
Hooks without a `matcher` field previously caused "hook error" (anthropics/claude-plugins-official#1007). Current docs say omitting matcher or using `""` means "match all". For safety, use explicit matcher where supported.

## Preset System Design

### Recommendation: Embedded JSON with go:embed
Presets should be JSON files embedded in the binary via `//go:embed presets/*.json`. This gives:
- Single binary distribution (no external files to lose)
- Easy to edit/review (JSON vs Go struct literals)
- Testable (load and validate at compile time)
- Extensible (future: allow external preset files to override)

### Preset Type Structure (Go)
```go
type MessagePreset struct {
    Label       string              `json:"label"`
    Description string              `json:"description"`
    // Per-activity detail messages for single session
    SingleSessionDetails    map[string][]string `json:"singleSessionDetails"`
    SingleSessionDetailsFallback []string       `json:"singleSessionDetailsFallback"`
    // State messages for single session
    SingleSessionState      []string            `json:"singleSessionState"`
    // Tier-based multi-session messages (keys: "2", "3", "4")
    MultiSessionMessages    map[string][]string `json:"multiSessionMessages"`
    MultiSessionOverflow    []string            `json:"multiSessionOverflow"`
    MultiSessionTooltips    []string            `json:"multiSessionTooltips"`
    // Preset-specific buttons
    Buttons                 []Button            `json:"buttons,omitempty"`
}
```

### Message Count Per Preset
D-21 specifies ~50 rotating messages per category. For 8 presets with 7 activity types + state messages + multi-session tiers, this is approximately:
- 7 activity types x ~15 messages = ~105 detail messages
- ~15 state messages
- 3 tiers x ~16 messages + ~20 overflow = ~68 multi-session messages
- ~25 tooltips
- **Total per preset: ~213 messages**
- **Total all presets: ~1700 messages**

This is substantial content creation but fits comfortably in an embedded JSON file (~100KB total).

## Common Pitfalls

### Pitfall 1: Discord Presence Rate Limit Flooding
**What goes wrong:** Rapid hook events (e.g., multiple Grep calls in sequence) cause presence updates faster than 1/15s, emptying the user's public presence.
**Why it happens:** The IPC SDK queues updates but can behave erratically under rapid fire.
**How to avoid:** Debounce presence updates. After any session change, wait 100ms for more changes, then update Discord. Never update more than once per 15 seconds.
**Warning signs:** Presence flickering or disappearing.

### Pitfall 2: fsnotify Atomic Save Breakage
**What goes wrong:** Config file watch stops working after an editor save.
**Why it happens:** VS Code and other editors do atomic saves (write temp file, rename). fsnotify on the original file loses the watch after rename.
**How to avoid:** Watch the parent DIRECTORY, filter events by filename. Re-add watch on RENAME events.
**Warning signs:** Config changes stop being detected after first edit.

### Pitfall 3: Windows Named Pipe Stale Connection
**What goes wrong:** Discord IPC connection appears alive but SetActivity silently fails.
**Why it happens:** Discord restarts or updates, pipe handle is stale.
**How to avoid:** Check for write errors on SetActivity. On error, close and reconnect with exponential backoff.
**Warning signs:** Presence stops updating but no error in logs.

### Pitfall 4: Goroutine Leak on Shutdown
**What goes wrong:** Binary consumes memory/CPU after all sessions end, or doesn't exit cleanly.
**Why it happens:** Stale checker ticker, config watcher, HTTP server not properly cancelled.
**How to avoid:** Use context.Context propagated from main. All goroutines select on ctx.Done().
**Warning signs:** Process stays alive after stop.sh, increasing memory usage.

### Pitfall 5: Session Deduplication Race
**What goes wrong:** Two rapid SessionStart hooks create duplicate sessions for the same project+PID.
**Why it happens:** HTTP handlers run concurrently, two requests arrive before first completes.
**How to avoid:** Dedup check inside the Lock section of SessionRegistry.StartSession. Check projectPath+pid combo atomically.
**Warning signs:** Multi-session display when only one Claude instance is running.

### Pitfall 6: PID Liveness False Positive on Windows
**What goes wrong:** `os.FindProcess` on Windows always succeeds (returns process handle even for non-existent PIDs).
**Why it happens:** Windows FindProcess opens a handle, which succeeds even for terminated processes in some cases.
**How to avoid:** Use `tasklist /FI "PID eq N"` and check output contains the PID. Already implemented in existing start.sh.
**Warning signs:** Stale sessions never get cleaned up on Windows.

## Code Examples

### Discord Activity with Buttons (new feature)
```go
// Source: Discord IPC protocol, extending existing discord/client.go
func (c *Client) SetActivity(activity Activity) error {
    activityData := map[string]interface{}{}
    if activity.Details != "" {
        activityData["details"] = activity.Details
    }
    if activity.State != "" {
        activityData["state"] = activity.State
    }
    // Assets (existing)
    assets := map[string]string{}
    if activity.LargeImage != "" { assets["large_image"] = activity.LargeImage }
    if activity.LargeText != "" { assets["large_text"] = activity.LargeText }
    if activity.SmallImage != "" { assets["small_image"] = activity.SmallImage }
    if activity.SmallText != "" { assets["small_text"] = activity.SmallText }
    if len(assets) > 0 { activityData["assets"] = assets }
    // Timestamps (existing)
    if activity.StartTime != nil {
        activityData["timestamps"] = map[string]int64{"start": activity.StartTime.Unix()}
    }
    // Buttons (NEW)
    if len(activity.Buttons) > 0 {
        buttons := make([]map[string]string, 0, 2)
        for i, b := range activity.Buttons {
            if i >= 2 { break } // Discord max 2
            if len(b.Label) > 0 && len(b.Label) <= 32 {
                buttons = append(buttons, map[string]string{
                    "label": b.Label,
                    "url":   b.URL,
                })
            }
        }
        if len(buttons) > 0 { activityData["buttons"] = buttons }
    }
    // ... rest of SetActivity (existing payload construction)
}
```

### Exponential Backoff for Discord Reconnect
```go
// Source: Pattern from claude-code-discord-status/src/daemon/discord.ts
type Backoff struct {
    min     time.Duration
    max     time.Duration
    current time.Duration
}

func NewBackoff(min, max time.Duration) *Backoff {
    return &Backoff{min: min, max: max, current: min}
}

func (b *Backoff) Next() time.Duration {
    d := b.current
    b.current *= 2
    if b.current > b.max {
        b.current = b.max
    }
    return d
}

func (b *Backoff) Reset() {
    b.current = b.min
}
```

### Embedded Preset Loading
```go
// Source: Go embed docs (https://pkg.go.dev/embed)
import "embed"

//go:embed presets/*.json
var presetFS embed.FS

func LoadPreset(name string) (*MessagePreset, error) {
    data, err := presetFS.ReadFile(fmt.Sprintf("presets/%s.json", name))
    if err != nil {
        return nil, fmt.Errorf("unknown preset %q", name)
    }
    var preset MessagePreset
    if err := json.Unmarshal(data, &preset); err != nil {
        return nil, fmt.Errorf("invalid preset %q: %w", name, err)
    }
    return &preset, nil
}

func AvailablePresets() []string {
    entries, _ := presetFS.ReadDir("presets")
    names := make([]string, 0, len(entries))
    for _, e := range entries {
        name := strings.TrimSuffix(e.Name(), ".json")
        names = append(names, name)
    }
    return names
}
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Go 1.21 in release.yml | Go 1.26 available | 2026 | Update release.yml to `go-version: '1.26'`. Go 1.22+ has method routing in ServeMux. |
| Discord IPC only | Discord IPC + Buttons | Always available | Buttons were always in the IPC protocol, just not implemented in this codebase |
| fmt.Println logging | log/slog structured logging | Go 1.21+ | slog is stdlib, supports levels, JSON output. Use with lumberjack writer. |
| Single session only | Multi-session registry | This phase | Port from Node.js project to Go |
| File-based data exchange | HTTP hooks | Claude Code 2026 | HTTP hooks are 112x faster than command hooks. Use for all activity events. |
| Fixed reconnect interval | Exponential backoff | This phase | 1s -> 2s -> 4s -> 8s -> 16s -> 30s max. Better for Discord restarts. |

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go compiler | Build | Yes | go1.26.0 | -- |
| Git | Branch detection | Yes | (system) | -- |
| Discord (running) | IPC connection | Runtime req | -- | Exponential backoff retry |
| GitHub Actions | Release pipeline | Yes | (cloud) | -- |
| fal.ai MCP | Icon generation | Runtime MCP | -- | Manual icon creation |

**Missing dependencies with no fallback:** None -- all core dependencies available.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) + go test |
| Config file | None needed (stdlib) |
| Quick run command | `cd C:/Users/ktown/Projects/cc-discord-presence && go test ./...` |
| Full suite command | `cd C:/Users/ktown/Projects/cc-discord-presence && go test -v -race ./...` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| D-05 | HTTP server starts and accepts hook POSTs | integration | `go test ./server/ -run TestHookEndpoints` | Wave 0 |
| D-07 | Activity mapping from tool name to icon | unit | `go test ./server/ -run TestActivityMapping` | Wave 0 |
| D-22 | Preset loading and validation | unit | `go test ./preset/ -run TestLoadPreset` | Wave 0 |
| D-23 | StablePick produces deterministic results | unit | `go test ./resolver/ -run TestStablePick` | Wave 0 |
| D-25 | Multi-session stats line formatting | unit | `go test ./resolver/ -run TestFormatStatsLine` | Wave 0 |
| D-26 | Session registry CRUD + concurrency | unit | `go test ./session/ -run TestRegistry -race` | Wave 0 |
| D-29 | Stale session detection and removal | unit | `go test ./session/ -run TestStaleCheck` | Wave 0 |
| D-33 | Config priority chain | unit | `go test ./config/ -run TestConfigPriority` | Wave 0 |
| D-42 | Exponential backoff timing | unit | `go test ./discord/ -run TestBackoff` | Wave 0 |
| D-49 | Config hot-reload via fsnotify | integration | `go test ./config/ -run TestConfigWatch` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./... -count=1`
- **Per wave merge:** `go test -v -race ./...`
- **Phase gate:** Full suite green + `go vet ./...` before verification

### Wave 0 Gaps
- [ ] `server/server_test.go` -- HTTP endpoint tests
- [ ] `session/registry_test.go` -- concurrent session management tests
- [ ] `resolver/resolver_test.go` -- presence resolution + stablePick tests
- [ ] `preset/preset_test.go` -- preset loading/validation tests
- [ ] `config/config_test.go` -- config priority chain tests

## Open Questions

1. **Hook Payload Schema**
   - What we know: Claude Code sends JSON POST body with hook event data. PreToolUse includes `tool_name`.
   - What's unclear: Exact JSON schema for each hook type (session_id field name, cwd location, model info in payload). The docs describe the format but a real payload capture would be definitive.
   - Recommendation: Add a debug logging endpoint that captures and logs raw payloads on first run. Use permissive JSON parsing (decode to `map[string]interface{}`).

2. **Discord Application ID for DSR Code**
   - What we know: Need to create a new Discord Application named "DSR Code" in Developer Portal.
   - What's unclear: Exact steps for creating the app, uploading assets, getting the Application ID.
   - Recommendation: Include as a manual task in the plan with step-by-step instructions.

3. **Statusline Wrapper Integration with Existing GSD Wrapper**
   - What we know: D-38 specifies the wrapper POSTs to HTTP server and passes through to next statusline.
   - What's unclear: How to chain with the existing GSD statusline wrapper if present.
   - Recommendation: Wrapper reads stdin JSON, POSTs to localhost:19460/statusline in background (`curl ... &`), then echoes JSON to stdout for the next wrapper.

## Project Constraints (from CLAUDE.md)

- Conventional Commits: feat, fix, refactor, docs, test, chore
- Immutable data patterns (new objects, never mutate) -- note: Go session registry uses pointer replacement pattern to satisfy this
- Files < 800 lines, functions < 50 lines
- 80%+ test coverage
- German error messages in UI (not applicable here -- Go binary, English logs)

## Exa Research Findings (2026-04-02)

### Go Discord IPC Libraries
Exa search found several Go Discord Rich Presence libraries. The most relevant:
- **hugolgst/rich-go** (156 stars, MIT) -- Cross-platform Discord Rich Presence for Go. Uses same IPC pattern as existing cc-discord-presence. Validates our approach.
- **axrona/go-discordrpc** (8 stars, GPL-3.0) -- Cross-platform implementation. Notes Windows Named Pipe + Unix Socket approach.
- **pqpcara/discord-rich-presence** (2026, MIT, Go 1.25) -- Most recent Go RPC lib. Confirms IPC protocol is stable.
- **Key finding:** No library offers HTTP hooks or multi-session. Our custom discord/ package is the right choice -- extend rather than replace.

### Go 1.22+ net/http ServeMux (Confirmed Best Choice)
[Source: reintech.io/blog/go-net-http-improvements-go-1-22]
- **Method routing:** `mux.HandleFunc("POST /hooks/{hookType}", handler)` -- exact syntax we need
- **Path value extraction:** `r.PathValue("hookType")` -- no gorilla/mux needed
- **Auto 405 Method Not Allowed** -- free HTTP semantics
- **O(1) lookup** for exact matches, O(n segments) for wildcards -- negligible overhead
- **Middleware pattern:** Wrap `http.HandlerFunc` -- simple closure chaining for logging
- **Testing:** `httptest.NewRequest` + `httptest.NewRecorder` + `mux.ServeHTTP` -- zero external test deps
- **Verdict:** stdlib net/http fully sufficient for our 6 routes. No chi/echo needed.

### fsnotify Config Hot-Reload (Critical Patterns)
[Source: oneuptime.com/blog/hot-reload-configuration-go]
- **Debounce pattern confirmed:** `time.AfterFunc(500ms)` after fsnotify event. Our 100ms is aggressive but fine for single-file config.
- **Watch directory, not file** -- confirmed: editors do atomic save (write temp + rename) which breaks file-level watches.
- **ConfigManager with RWMutex** -- exact pattern we planned. `Get()` returns copy under RLock.
- **OnChange callbacks** -- notify registered listeners. Use for preset reload propagation.
- **Validation before apply** -- validate new config before swapping. Reject invalid and keep old config.
- **Viper alternative considered:** Overkill. We only watch one JSON file. fsnotify + debounce + json.Unmarshal is ~40 lines.

### Claude Code HTTP Hooks (Critical — Known Issues)
[Source: claudefa.st/blog/tools/hooks/hooks-guide + anthropics/claude-code#32977]

**HTTP Hook Specifics:**
- HTTP hooks send event JSON as POST body (`Content-Type: application/json`)
- **Non-blocking errors:** Non-2xx, connection failures, timeouts → execution continues. Perfect for presence.
- **Empty 2xx body = success** -- our hooks should always return 200 with empty body (fastest path)
- **Headers with env vars:** `"Authorization": "Bearer $MY_TOKEN"` -- not needed for localhost
- **Config-only:** Must be added by editing settings JSON directly. No `/hooks` menu for HTTP.
- **4 hook types:** command, http, prompt, agent. We use command (start/stop) + http (activity).

**Known Bug (CRITICAL):**
- **anthropics/claude-code#32977** (closed as duplicate of #30170): PostToolUse HTTP hooks silently not dispatched on some versions.
- Issue was on Claude Code 2.1.72 (March 2026). May be fixed in current versions.
- **Mitigation:** Use PreToolUse instead of PostToolUse for activity tracking. PreToolUse fires BEFORE the tool runs -- perfect for "Editing a file" status. If PostToolUse HTTP is fixed, we can switch later.
- **Workaround alternative:** If HTTP hooks fail for any event type, fall back to JSONL watching (already planned as D-51 three-tier accuracy).

**Hook Matcher Syntax:**
- Matchers are regex, case-sensitive, no spaces around `|`
- `"Edit|Write|Bash|Read|Grep|Glob|WebSearch|WebFetch|Task"` is correct format
- UserPromptSubmit/Stop: omit matcher or use `""` for "match all"

### Discord Rich Presence Rate Limits (Confirmed)
[Source: discord-media.com, deepwiki.com/discord-userdoccers]

**Activity Object Structure (confirmed from DeepWiki):**
- `details`: max 128 chars, min 2 chars -- "What the user is doing"
- `state`: max 128 chars -- "User's party status"
- `status_display_type`: 0=NAME, 1=STATE, 2=DETAILS -- controls member list display
- `buttons`: max 2, label 1-32 chars. Buttons are ONLY visible to OTHER users.
- `assets.large_image/small_image`: reference asset keys uploaded to Developer Portal (auto-lowercased)
- `timestamps.start`: Unix timestamp in seconds for elapsed timer
- Activity type 0 (PLAYING) → `"Playing {name}"` format. This is what Rich Presence uses.

**Rate Limits:**
- Presence updates via IPC: **1 per 15 seconds** (SDK queues internally, confirmed)
- Custom assets per app: max 300
- Asset keys are auto-lowercased by Discord
- X-RateLimit headers in REST API don't apply to IPC (IPC has its own throttling)

### GoReleaser for Cross-Platform Builds
[Source: goreleaser.com]
- **GoReleaser** is the standard tool for Go multi-platform releases with GitHub Actions
- Existing cc-discord-presence uses manual `GOOS/GOARCH` matrix -- could migrate to GoReleaser for:
  - Automatic checksums
  - macOS Universal Binaries (arm64 + amd64 in one binary)
  - Changelog generation
  - Docker image builds
- **Recommendation:** Keep existing manual matrix for now (works, tested). Consider GoReleaser in future if release complexity grows.

### go:embed for Preset Data (Confirmed Pattern)
[Source: pkg.go.dev/embed, gobyexample.com, oneuptime.com]
- `//go:embed presets/*.json` on `var presetFS embed.FS` -- embeds all JSON files
- `presetFS.ReadFile("presets/minimal.json")` -- read at runtime
- `presetFS.ReadDir("presets")` -- list available presets
- Files are included at compile time -- zero-config single binary
- Works with `go test` -- embedded files available in tests
- **Key insight:** embed.FS is read-only. For user-supplied presets, add file system fallback: check `~/.claude/discord-presence-presets/` first, then fall back to embedded.

## Sources

### Primary (HIGH confidence)
- cc-discord-presence source code (C:\Users\ktown\Projects\cc-discord-presence) -- full codebase analysis
- claude-code-discord-status source code (C:\Users\ktown\Projects\claude-code-discord-status) -- full codebase analysis
- [Claude Code Hooks Reference](https://code.claude.com/docs/en/hooks) -- complete hook system docs
- Go stdlib documentation (embed, net/http, log/slog, sync)

### Exa Web Research (HIGH confidence)
- [Go 1.22 net/http Routing](https://reintech.io/blog/go-net-http-improvements-go-1-22-enhanced-routing-patterns) -- ServeMux method routing patterns
- [Go Config Hot-Reload](https://oneuptime.com/blog/post/2026-01-25-hot-reload-configuration-go-without-restarts/view) -- fsnotify + debounce + ConfigManager pattern
- [Claude Code Hooks Guide](https://claudefa.st/blog/tools/hooks/hooks-guide) -- HTTP hook types, matchers, response format
- [PostToolUse HTTP bug](https://github.com/anthropics/claude-code/issues/32977) -- Known issue with PostToolUse HTTP hooks
- [Discord Activity Objects](https://deepwiki.com/discord-userdoccers/discord-userdoccers/10.2-activity-objects-and-rich-presence) -- Full activity structure spec
- [GoReleaser](https://goreleaser.com/) -- Multi-platform build automation
- [Go embed](https://pkg.go.dev/embed@master) -- Embed directive documentation
- [hugolgst/rich-go](https://github.com/hugolgst/rich-go) -- Go Discord RPC reference implementation

### Secondary (MEDIUM confidence)
- [Discord Rich Presence rate limit](https://github.com/discord/discord-api-docs/issues/668) -- 15-second rate limit confirmed
- [fsnotify debounce patterns](https://medium.com/@impactarchitecture/file-watchers-lie-debounce-throttle-and-coalescing-in-build-loops-8d91cb29f712) -- editor save behavior (3-12 events per save)
- [Go net/http vs chi](https://www.calhoun.io/go-servemux-vs-chi/) -- Go 1.22+ ServeMux has method routing
- [lumberjack v2](https://github.com/natefinch/lumberjack) -- log rotation library
- [Go cross-platform PID check](https://github.com/golang/go/issues/34396) -- FindProcess always succeeds on Unix

### Tertiary (LOW confidence)
- Discord button URL max length -- not explicitly documented, "keep reasonable"
- Claude Code hook payload exact schema for each event type -- docs describe format but no formal schema

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all libraries verified, existing codebase uses most of them
- Architecture: HIGH -- direct port from well-structured Node.js codebase to Go, clear package boundaries
- Pitfalls: HIGH -- documented from real issues in both source repos and Discord API docs
- Presets: MEDIUM -- message content is creative work, structure is well-defined
- Hook integration: MEDIUM -- docs are clear but exact payload schemas need runtime verification

**Research date:** 2026-04-02
**Valid until:** 2026-05-02 (stable domain, Go stdlib, Discord IPC is long-stable)
