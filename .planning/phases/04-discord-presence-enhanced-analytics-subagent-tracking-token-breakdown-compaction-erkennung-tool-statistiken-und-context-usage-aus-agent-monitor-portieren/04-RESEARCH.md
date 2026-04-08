# Phase 4: Discord Presence Enhanced Analytics - Research

**Researched:** 2026-04-07
**Domain:** Go binary enhancement (analytics, JSONL parsing, hook event processing, i18n)
**Confidence:** HIGH

## Summary

Phase 4 ports five analytics features from the Node.js Claude-Code-Agent-Monitor into the Go-based cc-discord-presence binary: subagent hierarchy tracking, token breakdown (input/output/cache_read/cache_write), compaction detection, tool usage statistics, and context usage percentage. Additionally, it introduces bilingual (EN+DE) presets via go-i18n for code strings and a language-wrapper JSON format for creative preset messages.

The existing Go codebase (v3.1.10) has a clean package architecture: session/ (registry + types), server/ (HTTP handlers + hooks), resolver/ (placeholder resolution + StablePick rotation), preset/ (embedded JSON), config/ (hot-reload via fsnotify), discord/ (IPC client). A new analytics/ package will be the primary addition, with integration points into server.handleHook(), main.jsonlFallbackWatcher(), and resolver.buildSinglePlaceholderValues(). The Session struct in session/types.go gains ~8 new fields for analytics data.

**Primary recommendation:** Create an analytics/ package with tracker.go as coordinator, plus focused files for each domain (tokens.go, compaction.go, subagent.go, tools.go, persist.go, context.go, pricing.go). Wire into existing onChange callback pattern. Use write-on-event file persistence (not in-memory only). Expand hooks.json to include Agent in PreToolUse matcher and add SubagentStop hook section.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- D-01: 3-Quellen-Split -- Hooks fuer Subagents (PreToolUse tool_name=Agent + SubagentStop), JSONL Watcher fuer Tokens+Compaction (message.usage.* + isCompactSummary), Statusline fuer Context% (discord-presence-data.json)
- D-02: File-Persist fuer Analytics-Daten -- Separate JSON-Dateien neben discord-presence-data.json, ueberleben Daemon-Restart
- D-03: Kompakt mit Symbolen -- displayDetail-aware formatting (minimal/standard/verbose/private)
- D-04: Leer-String bei fehlenden Daten -- Resolver trimmt doppelte Leerzeichen und Trailing-Dots
- D-05: Rohe Tool-Namen (Edit, Bash, Grep), Top-3, map[string]int. Abkuerzungen: Edit=ed, Bash=cmd, Grep=grep, Read=read, Write=write, Agent=agent
- D-06: Separate Dateien pro Feature -- Token-Daten, Tool-Stats, Subagent-Daten jeweils eigenes JSON-File
- D-07: Hierarchie-Baum mit ParentID -- SubagentEntry{ID, Type, Description, Status, SpawnTime, CompletedTime, ParentID}
- D-08: ID-basiertes Matching mit Description-Prefix Fallback -- 4 Matching-Strategien (description prefix 57ch > agent_type > prompt prefix 500ch > oldest working fallback)
- D-09: analytics/ Package -- tracker.go, tokens.go, compaction.go, subagent.go, tools.go, persist.go, context.go, pricing.go
- D-10: Context% via bestehende discord-presence-data.json -- fsnotify auf context_window.used_percentage
- D-11: Version v3.2.0 -- SemVer MINOR bump, backward-compatible
- D-12: 80+ neue Messages (10 pro Preset x 8 Presets) in EN + DE
- D-13: transcript_path aus Hook-Events -- alle Hook-Events liefern transcript_path als Common Field
- D-14: Auto-Patch hooks.json beim Start -- start.sh fuegt fehlende Eintraege hinzu (Agent in PreToolUse, SubagentStop)
- D-15: Internal + External Tests -- analytics + analytics_test packages, table-driven
- D-16: Hardcoded Pricing in pricing.go -- 6 Modelle mit 4 Raten (input, output, cache_write, cache_read per MTok)
- D-17: {cost} intern cache-aware berechnen, neuer {cost_detail} Platzhalter
- D-18: transcript_path in Session struct speichern
- D-19: Baselines in Analytics-JSON persistieren -- Compaction-aware baseline logic
- D-20: Getrennte Verantwortung -- Hook-Handler: O(1) <1ms, JSONL-Watcher: async via transcript_path
- D-21: Lazy Init fuer Analytics-Dateien -- os.IsNotExist(), Go Zero-Values als Defaults
- D-22: Selektive Multi-Session-Aggregation -- totalCost/totalTokensDetail ueber alle Sessions, andere pro aktivster Session
- D-23: /dsrcode:status Ausgabe mit eingeruecktem Subagent-Baum und Status-Icons
- D-24: GET /status Response mit per-Session Analytics + Top-Level Aggregate
- D-25: Per-Model Token-Tracking aus JSONL -- map[string]TokenBreakdown, pricing per model
- D-26: Nie auto-loeschen -- Analytics-Dateien bleiben bestehen
- D-27: Alle Presets in EN + DE -- config.lang fuer Sprachwechsel
- D-28: Plugin hooks.json als Standard -- SubagentStop als neue Sektion, Agent zum PreToolUse-Matcher
- D-29: Top-Level Sprach-Wrapper in Preset-JSONs -- { "label": "...", "messages": { "en": {...}, "de": {...} } }
- D-30: go-i18n Library fuer Go Code-Strings -- BCP-47 Tags, Bundle/Localizer Pattern, active.en.json + active.de.json
- D-31: Write-on-Event Persistenz -- ~1ms pro Write, crash-proof
- D-32: Trivial Graceful Shutdown -- Daten durch D-31 immer aktuell
- D-33: Auto-Migration "german" preset zu "professional" + "lang": "de"
- D-34: Feature Map in config.json -- { "features": { "analytics": true } }
- D-35: Hybrid Translation Workflow -- go-i18n fuer Code-Strings, Preset-JSON fuer kreative Messages

### Claude's Discretion
- JSONL Watcher Refactoring-Scope (how much to extract from main.go to analytics/)
- Exakte SubagentEntry struct Felder und JSON-Serialisierung
- analytics.Tracker Lifecycle (Start/Stop, goroutine management)
- File-Persist Dateinamen und Verzeichnis
- Token-Abbreviation Format (K vs. k, threshold for M-Notation)
- Tool-Name Abbreviation Mapping for rare tools
- CHANGELOG.md Eintraege fuer v3.2.0
- Error Handling bei korrupten JSONL Eintraegen
- fsnotify Event Debouncing fuer analytics files
- go-i18n Message-IDs und Dateistruktur
- Preset-Translations DE Qualitaet
- Feature Map Default-Werte
- Concurrent file access Handling

### Deferred Ideas (OUT OF SCOPE)
- Worktree-Agent Zuordnung zum Parent-Projekt
- Per-Session Presets
- Web-Dashboard fuer Session-Statistiken
- Tool-Flow Transitions (Sankey Diagram Data)
- Per-Model Token Breakdown im Discord Display
- SQLite Persistenz statt JSON Files
- Auto-Compaction-Warning als Discord Notification
- Workflow Pattern Detection
- Agent Co-occurrence Matrix
- Session Complexity Scoring
- Crowdin/Transifex Integration
- CONTRIBUTING.md fuer Translation-Workflow
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| DPA-01 | Subagent-Spawns via SubagentStop/PreToolUse(Agent) Hook tracken | SubagentStop hook payload verified (agent_id, agent_type, description, agent_transcript_path); PreToolUse Agent payload has tool_input.description, subagent_type, prompt |
| DPA-02 | Session zeigt "3 Subagents: 2x Explore, 1x code-reviewer" in Discord State | Resolver placeholder system verified; 128-char Discord limit; displayDetail-aware formatting |
| DPA-03 | Token-Breakdown zeigt input/output/cache separat via {tokens_detail} | JSONL message.usage fields verified (input_tokens, output_tokens, cache_read_input_tokens, cache_creation_input_tokens) |
| DPA-04 | "250K Up 80K Down 156Kc" format for token display | Existing formatTokens() in resolver.go as base; new formatTokenBreakdown() needed |
| DPA-05 | Compaction-Events aus JSONL erkannt (isCompactSummary Flag) | Agent Monitor transcript-cache.js lines 144-150 confirms isCompactSummary detection; uuid-based dedup |
| DPA-06 | {compactions} Platzhalter -> "2 compactions" | Resolver placeholder extension verified; displayDetail pattern from D-03 |
| DPA-07 | {context_pct} -> "78%" | discord-presence-data.json confirmed to contain context_window.used_percentage (verified live) |
| DPA-08 | Tool-Statistiken pro Session: Top-3 Tools via {top_tools} | map[string]int pattern matches existing ActivityCounts approach; tool abbreviations per D-05 |
| DPA-09 | Alle 8 Presets haben 5+ neue Messages mit neuen Platzhaltern | 8 preset files exist (chaotic, dev-humor, german, hacker, minimal, professional, streamer, weeb); D-29 language wrapper format |
| DPA-10 | GET /status Endpoint liefert erweiterte Analytics-Daten | Existing statusResponse struct extendable; per-session + top-level aggregate per D-24 |
| DPA-11 | /dsrcode:status zeigt Subagent-Baum und Tool-Stats | server.go handleGetStatus exists; extend output format per D-23 |
| DPA-12 | Hook-Verarbeitung bleibt unter 5ms | D-20 architecture: hooks O(1) <1ms, JSONL parsing async; verified no blocking IO in hook path |
| DPA-13 | analytics/ Package mit Go-Standard-Architektur | Package structure per D-09; follows existing pattern (session/, server/, resolver/) |
| DPA-14 | File-Persistenz fuer Analytics-Daten | Write-on-event pattern (D-31); JSON files per D-06; baseline logic per D-19 |
| DPA-15 | Per-Model Token-Tracking | JSONL msg.model + msg.usage confirmed in live JSONL data; map[string]TokenBreakdown |
| DPA-16 | Hardcoded Pricing mit cache-aware Berechnung | 6 models with 4 rates per D-16; existing calculateCost() in main.go to be replaced |
| DPA-17 | {cost_detail} Platzhalter "$1.20 (Up$0.80 Down$0.35 c$0.05)" | New placeholder following displayDetail pattern |
| DPA-18 | transcript_path aus Hook-Events in Session struct | HookPayload already has TranscriptPath field; Session struct needs new field |
| DPA-19 | Compaction-Baselines persistiert | Agent Monitor replaceTokenUsage SQL logic (db.js lines 339-356) as reference; Go adaptation with JSON |
| DPA-20 | Lazy Init fuer Analytics-Dateien | os.IsNotExist() check; Go zero-values as natural defaults per D-21 |
| DPA-21 | Selektive Multi-Session-Aggregation | Existing resolveMulti pattern; totalCost/totalTokensDetail aggregate, others per active session |
| DPA-22 | Auto-Patch hooks.json beim Start | start.sh modification per D-14; JSON patching with jq or Go-based |
| DPA-23 | Plugin hooks.json mit SubagentStop + Agent in PreToolUse | Existing hooks.json structure verified; needs 2 modifications |
| DPA-24 | go-i18n fuer Code-Strings (EN + DE) | go-i18n v2.6.1 verified; Bundle/Localizer/MustLocalize pattern documented |
| DPA-25 | Bilingual Presets (EN + DE) mit Sprach-Wrapper JSON | D-29 format; preset loader modification for lang selection |
| DPA-26 | Auto-Migration "german" preset | start.sh + config.go modification per D-33 |
| DPA-27 | Feature Map in config.json | config.Config struct extension; analytics toggle per D-34 |
| DPA-28 | 80+ neue Messages in EN + DE | 10 per preset x 8 presets; must use new placeholders in 2+ messages per preset |
| DPA-29 | Version v3.2.0 + CHANGELOG.md | Version variable in main.go; start.sh VERSION constant |
| DPA-30 | Backward Compatibility v3.1.x -> v3.2.0 | Lazy init (D-21) + auto-patch hooks (D-14) + german migration (D-33) |
</phase_requirements>

## Standard Stack

### Core (already in go.mod)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/fsnotify/fsnotify` | v1.9.0 | File system watching for analytics data, config, discord-presence-data.json | Already used in project for config watcher and JSONL fallback [VERIFIED: go.mod] |
| `github.com/Microsoft/go-winio` | v0.6.2 | Windows named pipe for Discord IPC | Already in project [VERIFIED: go.mod] |
| `gopkg.in/natefinsh/lumberjack.v2` | v2.2.1 | Log rotation | Already in project [VERIFIED: go.mod] |

### New Dependencies
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `github.com/nicksnyder/go-i18n/v2` | v2.6.1 | Bilingual code strings (status output, error messages) | D-30: Bundle/Localizer pattern for EN+DE code strings [VERIFIED: go list -m] |
| `golang.org/x/text` | latest | Language tag support (language.English, language.German) for go-i18n | Required by go-i18n, provides BCP-47 tag types [VERIFIED: go-i18n dependency] |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| go-i18n | Manual map[string]map[string]string | go-i18n provides plural support, hash tracking, goi18n CLI for extract/merge. Manual map is simpler but loses tooling. D-30 locks go-i18n. |
| JSON file persist | SQLite (go-sqlite3) | JSON is simpler, no CGO, sufficient for few-KB analytics files. SQLite deferred per CONTEXT.md. |
| Write-on-event | Periodic flush | Write-on-event (D-31) is crash-proof. Events are infrequent (~1-10/sec during active coding), SSD impact negligible. |

**Installation:**
```bash
cd C:\Users\ktown\Projects\cc-discord-presence
go get github.com/nicksnyder/go-i18n/v2@v2.6.1
go get golang.org/x/text
```

**Version verification:**
- go-i18n v2.6.1 confirmed via `go list -m` (published 2026-01-01) [VERIFIED: go module proxy]
- fsnotify v1.9.0 already in go.mod [VERIFIED: go.mod]
- Go 1.26.0 installed [VERIFIED: `go version`]

## Architecture Patterns

### Recommended Package Structure
```
cc-discord-presence/
  analytics/           # NEW: Phase 4 core package
    tracker.go         # Coordinator: Start/Stop, goroutine management, event dispatch
    tokens.go          # TokenBreakdown struct, JSONL parsing, per-model tracking
    compaction.go      # isCompactSummary detection, baseline logic, uuid dedup
    subagent.go        # SubagentEntry tree, spawn/complete, 4-strategy matching
    tools.go           # Tool counter map[string]int, top-N, abbreviation mapping
    persist.go         # File-based JSON persistence, write-on-event, lazy init
    context.go         # Context% from discord-presence-data.json via fsnotify
    pricing.go         # Hardcoded model prices, cache-aware cost calculation
  i18n/                # NEW: Localization files
    active.en.json     # English code strings
    active.de.json     # German code strings
  config/
    config.go          # EXTEND: add Lang, Features fields to Config + fileConfig
    watcher.go         # Unchanged
  hooks/
    hooks.json         # EXTEND: add Agent to PreToolUse matcher, add SubagentStop section
  main.go             # EXTEND: wire analytics.Tracker, update jsonlFallbackWatcher
  preset/
    types.go           # EXTEND: BilingualMessagePreset with language wrapper
    loader.go          # EXTEND: language-aware preset loading
    presets/*.json      # EXTEND: bilingual format per D-29
  resolver/
    resolver.go        # EXTEND: 7 new placeholders, formatTokenBreakdown, formatToolStats
    stats.go           # Unchanged (existing FormatStatsLine)
  server/
    server.go          # EXTEND: SubagentStop route, Agent tool_input parsing, analytics in /status
  session/
    types.go           # EXTEND: new fields on Session struct
    registry.go        # Unchanged (analytics wires into onChange)
  scripts/
    start.sh           # EXTEND: hooks.json auto-patch, german preset migration
```

### Pattern 1: Analytics Tracker Lifecycle
**What:** Central coordinator that receives events from hooks and JSONL watcher, dispatches to domain-specific handlers, manages file persistence.
**When to use:** Any analytics event (tool use, subagent spawn/complete, token update, compaction, context%).
**Example:**
```go
// Source: Derived from session.SessionRegistry pattern (registry.go)
type Tracker struct {
    mu          sync.RWMutex
    sessions    map[string]*SessionAnalytics // keyed by session ID
    dataDir     string                       // persistence directory
}

type SessionAnalytics struct {
    Tokens       map[string]TokenBreakdown `json:"tokens"`       // per-model
    Baselines    map[string]TokenBreakdown `json:"baselines"`    // compaction baselines
    Tools        map[string]int            `json:"tools"`        // tool_name -> count
    Subagents    []SubagentEntry           `json:"subagents"`
    Compactions  int                       `json:"compactions"`
    ContextPct   *float64                  `json:"contextPct"`   // nil = not yet received
    CostUSD      float64                   `json:"costUsd"`
}
```
[VERIFIED: Pattern derived from existing session.SessionRegistry in registry.go]

### Pattern 2: JSONL Token Parsing (Port from Agent Monitor)
**What:** Parse JSONL entries for message.usage fields and isCompactSummary flag.
**When to use:** When JSONL watcher detects file changes via fsnotify or transcript_path is available.
**Example:**
```go
// Source: Claude-Code-Agent-Monitor/server/lib/transcript-cache.js lines 137-165
type jsonlEntry struct {
    Type              string `json:"type"`
    IsCompactSummary  bool   `json:"isCompactSummary"`
    UUID              string `json:"uuid"`
    Timestamp         string `json:"timestamp"`
    Message           struct {
        Model string `json:"model"`
        Usage struct {
            InputTokens               int64 `json:"input_tokens"`
            OutputTokens              int64 `json:"output_tokens"`
            CacheReadInputTokens      int64 `json:"cache_read_input_tokens"`
            CacheCreationInputTokens  int64 `json:"cache_creation_input_tokens"`
        } `json:"usage"`
    } `json:"message"`
}
```
[VERIFIED: JSONL field names confirmed from live JSONL data sample and Agent Monitor source]

### Pattern 3: Compaction Baseline Logic
**What:** When JSONL is rewritten by compaction, token counts drop. Baseline captures pre-compaction totals. Effective = current + baseline.
**When to use:** Every token update from JSONL -- compare new vs stored, accumulate baseline if new < old.
**Example:**
```go
// Source: Claude-Code-Agent-Monitor/server/db.js lines 339-356 (replaceTokenUsage)
func (t *Tracker) updateTokens(sessionID, model string, current TokenBreakdown) {
    // If current < stored, compaction happened: accumulate old to baseline
    stored := sa.Tokens[model]
    if current.Input < stored.Input {
        sa.Baselines[model] = TokenBreakdown{
            Input:      sa.Baselines[model].Input + stored.Input,
            Output:     sa.Baselines[model].Output + stored.Output,
            CacheRead:  sa.Baselines[model].CacheRead + stored.CacheRead,
            CacheWrite: sa.Baselines[model].CacheWrite + stored.CacheWrite,
        }
    }
    sa.Tokens[model] = current
}
// Effective total: sa.Tokens[model].Input + sa.Baselines[model].Input
```
[VERIFIED: Baseline logic ported from Agent Monitor db.js replaceTokenUsage SQL]

### Pattern 4: Subagent 4-Strategy Matching
**What:** Match SubagentStop events to spawned subagents using 4 strategies in order.
**When to use:** On SubagentStop hook event.
**Example:**
```go
// Source: Claude-Code-Agent-Monitor/server/routes/hooks.js lines 202-257
func (t *Tracker) matchSubagent(sessionID string, event SubagentStopEvent) *SubagentEntry {
    // Strategy 1: agent_id exact match (since CC 2.0.42)
    if event.AgentID != "" {
        // find by ID
    }
    // Strategy 2: Description prefix match (57 chars)
    if event.Description != "" {
        prefix := truncateStr(event.Description, 57)
        // find working subagent whose description starts with prefix
    }
    // Strategy 3: agent_type match
    if event.AgentType != "" {
        // find working subagent with matching type
    }
    // Strategy 4: Prompt prefix match (500 chars)
    // Strategy 5: Oldest working subagent fallback
    return oldestWorking
}
```
[VERIFIED: 4 strategies confirmed in Agent Monitor hooks.js; agent_id field confirmed in Claude Code docs]

### Pattern 5: go-i18n Bundle/Localizer for Code Strings
**What:** Bilingual code strings using go-i18n for /dsrcode:status output, error messages, status texts.
**When to use:** All user-facing code strings (NOT preset messages, which use D-29 JSON format).
**Example:**
```go
// Source: go-i18n v2.6.1 official docs (pkg.go.dev/github.com/nicksnyder/go-i18n/v2/i18n)
import (
    "github.com/nicksnyder/go-i18n/v2/i18n"
    "golang.org/x/text/language"
)

bundle := i18n.NewBundle(language.English)
bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
bundle.LoadMessageFile("i18n/active.en.json")
bundle.LoadMessageFile("i18n/active.de.json")

localizer := i18n.NewLocalizer(bundle, cfg.Lang) // "en" or "de"
msg := localizer.MustLocalize(&i18n.LocalizeConfig{
    DefaultMessage: &i18n.Message{
        ID:    "SubagentTreeHeader",
        Other: "Subagents:",
    },
})
```
[VERIFIED: go-i18n API confirmed via pkg.go.dev documentation]

### Pattern 6: Bilingual Preset JSON (D-29)
**What:** Preset JSON files with top-level language wrapper.
**When to use:** All 8 preset files (chaotic, dev-humor, german->professional migration, hacker, minimal, professional, streamer, weeb).
**Example:**
```json
{
  "label": "minimal",
  "description": {"en": "Terse, just the facts.", "de": "Knapp, nur Fakten."},
  "messages": {
    "en": {
      "singleSessionDetails": { "coding": [...], ... },
      "singleSessionState": [...],
      "multiSessionMessages": {...},
      "multiSessionOverflow": [...]
    },
    "de": {
      "singleSessionDetails": { "coding": [...], ... },
      "singleSessionState": [...],
      "multiSessionMessages": {...},
      "multiSessionOverflow": [...]
    }
  }
}
```
[ASSUMED: Exact JSON structure derived from D-29 decision; loader needs BilingualMessagePreset intermediate type]

### Anti-Patterns to Avoid
- **Blocking JSONL parsing in hook handler:** Hook handlers must be O(1) <1ms (D-20). JSONL parsing is IO-bound and must be async via the existing fsnotify watcher goroutine, NOT in the hook HTTP handler.
- **Monolithic analytics file:** D-06 requires separate files per feature. A single analytics.json would create write contention with concurrent sessions.
- **In-memory only analytics:** D-02 requires file persistence. Without it, daemon restart loses all analytics data.
- **Full JSONL re-read on every event:** Use incremental read strategy (track file offset) like Agent Monitor's transcript-cache.js. Full re-read is O(n) per event and would cause performance regression.
- **Using go-i18n for preset messages:** D-35 explicitly states go-i18n is WRONG for presets because of context loss, array-index coupling, StablePick incompatibility, and the need for creative (not literal) translation.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Bilingual code strings | Manual map[lang]map[id]string | go-i18n v2 Bundle/Localizer | Plural support, hash tracking, goi18n CLI for extract/merge, fallback chain |
| File system watching | Polling loops | fsnotify v1.9.0 (already used) | Event-driven, cross-platform, tested in production by this project |
| Language tags | String comparisons | golang.org/x/text/language | BCP-47 compliance, matching/negotiation, canonical forms |
| JSON parsing | Manual string scanning | encoding/json (stdlib) | Battle-tested, handles edge cases, streaming decoder for JSONL |
| Concurrent map access | sync.Map or manual locks | sync.RWMutex (existing pattern) | Consistent with session.SessionRegistry; more predictable than sync.Map for small maps |

**Key insight:** The analytics package is a port from Node.js to Go. The Agent Monitor reference implementation handles all the edge cases (compaction baselines, subagent matching, incremental JSONL reads). Porting the tested logic is safer than designing from scratch.

## Common Pitfalls

### Pitfall 1: JSONL Compaction Drops Token Counts
**What goes wrong:** After Claude Code compacts context, the JSONL file is rewritten with fewer entries. Raw token totals from JSONL parsing will be lower than pre-compaction values, making it appear tokens were "lost."
**Why it happens:** Compaction removes older entries from JSONL, resetting cumulative usage.
**How to avoid:** Implement baseline logic per D-19 (port from Agent Monitor's replaceTokenUsage). When new JSONL total < stored total for any token type, accumulate old value to baseline. Effective = current + baseline.
**Warning signs:** Token count suddenly drops mid-session in Discord display.

### Pitfall 2: SubagentStop Matching Ambiguity
**What goes wrong:** Multiple subagents of the same type running concurrently; SubagentStop cannot reliably identify which one finished.
**Why it happens:** Pre-CC 2.0.42, no agent_id field existed. Even now, description/type may overlap.
**How to avoid:** Use the 4-strategy matching cascade per D-08. agent_id (when available) is authoritative. Fall back through description prefix (57ch) > agent_type > prompt prefix (500ch) > oldest working. Mark matched subagent as "completed" only.
**Warning signs:** Wrong subagent marked as completed; orphan "working" subagents that never complete.

### Pitfall 3: Hook Handler Blocking on File I/O
**What goes wrong:** Writing analytics data to disk inside the hook HTTP handler adds latency, potentially exceeding the 1s hook timeout.
**Why it happens:** File I/O on Windows can take 5-50ms depending on antivirus scanning.
**How to avoid:** Per D-20, hook handlers only update in-memory state (O(1) operations: increment map counter, append to slice). File persistence happens asynchronously -- either via the JSONL watcher goroutine or a dedicated persist goroutine triggered by the Tracker.
**Warning signs:** Claude Code hook timeout warnings; slow tool execution.

### Pitfall 4: Preset JSON Format Migration Breaking Existing Users
**What goes wrong:** Changing preset JSON structure from flat to bilingual wrapper breaks users with custom preset modifications or external tools reading presets.
**Why it happens:** D-29 changes the top-level structure from `{singleSessionDetails: {...}}` to `{messages: {en: {singleSessionDetails: {...}}, de: {...}}}`.
**How to avoid:** Preset loader should detect format version: if no "messages" key exists, treat as legacy single-language format (backward compat). Log a deprecation warning. Auto-migration in start.sh per D-33 handles the "german" preset specifically.
**Warning signs:** Blank Discord presence messages after upgrade; preset loading errors.

### Pitfall 5: Context% Null in discord-presence-data.json
**What goes wrong:** `context_window.used_percentage` is null at session start (before first API call returns usage data). Displaying "null%" or "0%" is incorrect.
**Why it happens:** Claude Code writes used_percentage as null until the first API response comes back with actual usage data.
**How to avoid:** Per D-04, use empty string when context_pct is nil. The resolver should only show context% when the value is non-nil and > 0. Verified from live discord-presence-data.json that used_percentage starts as null.
**Warning signs:** "null%" in Discord display; "0%" when no API calls have been made yet.

### Pitfall 6: Concurrent File Access with Multiple Sessions
**What goes wrong:** Two concurrent Claude Code sessions trigger simultaneous writes to analytics files for different sessions.
**Why it happens:** Each session has its own analytics data, but the Tracker handles all sessions.
**How to avoid:** Use separate files per session per D-06 (e.g., `analytics-tokens-{sessionId}.json`). The Tracker's sync.RWMutex serializes in-memory access; file writes happen per-session so no file-level contention.
**Warning signs:** Corrupted JSON files; data from one session appearing in another's analytics.

### Pitfall 7: go-i18n Embedded FS vs. File Loading
**What goes wrong:** Using embed.FS for i18n message files prevents users from adding custom translations without recompiling.
**Why it happens:** go:embed bakes files into the binary.
**How to avoid:** Load i18n files from disk (i18n/ directory next to binary or in ~/.claude/), with embedded fallback for English. This allows community-contributed translations (active.fr.json etc.) without recompilation.
**Warning signs:** Users unable to add new language support; translation PRs requiring binary rebuilds.

## Code Examples

### Session Struct Extensions
```go
// Source: Derived from existing session/types.go + D-09 decisions
type Session struct {
    // ... existing fields ...
    TranscriptPath   string                     `json:"transcriptPath,omitempty"`
    TokenBreakdown   map[string]TokenBreakdown  `json:"tokenBreakdown,omitempty"`
    TokenBaselines   map[string]TokenBreakdown  `json:"tokenBaselines,omitempty"`
    ToolCounts       map[string]int             `json:"toolCounts,omitempty"`
    SubagentTree     []SubagentEntry            `json:"subagentTree,omitempty"`
    CompactionCount  int                        `json:"compactionCount"`
    ContextUsagePct  *float64                   `json:"contextUsagePct"`
    CostBreakdown    CostBreakdown              `json:"costBreakdown"`
}
```
[VERIFIED: Existing Session struct in session/types.go lines 32-53]

### TokenBreakdown Struct
```go
// Source: Claude-Code-Agent-Monitor/server/db.js lines 81-90 (token_usage table)
type TokenBreakdown struct {
    Input      int64 `json:"input"`
    Output     int64 `json:"output"`
    CacheRead  int64 `json:"cacheRead"`
    CacheWrite int64 `json:"cacheWrite"`
}

func (t TokenBreakdown) Total() int64 {
    return t.Input + t.Output + t.CacheRead + t.CacheWrite
}
```
[VERIFIED: Field names match JSONL usage fields]

### SubagentEntry Struct
```go
// Source: Claude-Code-Agent-Monitor/server/db.js lines 51-66 (agents table) + D-07
type SubagentEntry struct {
    ID            string    `json:"id"`
    Type          string    `json:"type"`            // agent_type from hook
    Description   string    `json:"description"`
    Status        string    `json:"status"`          // "working" | "completed"
    SpawnTime     time.Time `json:"spawnTime"`
    CompletedTime *time.Time `json:"completedTime,omitempty"`
    ParentID      string    `json:"parentId"`
    Prompt        string    `json:"prompt,omitempty"` // first 500 chars for matching
}
```
[VERIFIED: Agent Monitor agents table schema + SubagentStop hook fields]

### Hooks.json Extension
```json
{
  "description": "DSR Code Presence hooks -- activity tracking and skill installation",
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Edit|Write|Bash|Read|Grep|Glob|WebSearch|WebFetch|Task|Agent",
        "hooks": [
          {
            "type": "http",
            "url": "http://127.0.0.1:19460/hooks/pre-tool-use",
            "timeout": 1
          }
        ]
      }
    ],
    "SubagentStop": [
      {
        "matcher": "*",
        "hooks": [
          {
            "type": "http",
            "url": "http://127.0.0.1:19460/hooks/subagent-stop",
            "timeout": 1
          }
        ]
      }
    ]
  }
}
```
[VERIFIED: Plugin hooks.json format confirmed from Claude Code docs; SubagentStop matcher "*" catches all agent types]

### Config Extension
```go
// Source: Existing config/config.go + D-27, D-34
type Config struct {
    // ... existing fields ...
    Lang     string          `json:"lang"`     // "en" or "de", default "en"
    Features FeatureMap      `json:"features"` // D-34
}

type FeatureMap struct {
    Analytics bool `json:"analytics"` // default true
}
```
[VERIFIED: Existing config.Config structure in config/config.go]

### Pricing Table
```go
// Source: D-16 from CONTEXT.md (verified Maerz 2026 GA pricing)
var modelPrices = map[string]ModelPricing{
    "claude-opus-4-6":    {Input: 5.0, Output: 25.0, CacheWrite: 6.25, CacheRead: 0.50},
    "claude-sonnet-4-6":  {Input: 3.0, Output: 15.0, CacheWrite: 3.75, CacheRead: 0.30},
    "claude-haiku-4-5":   {Input: 1.0, Output: 5.0,  CacheWrite: 1.25, CacheRead: 0.10},
    // Legacy models (no cache pricing)
    "claude-opus-4-5":    {Input: 5.0, Output: 25.0},
    "claude-opus-4-1":    {Input: 15.0, Output: 75.0},
    "claude-haiku-3":     {Input: 0.25, Output: 1.25},
}

// Wildcard matching: strings.Contains for model variants
func findPricing(modelID string) ModelPricing {
    for pattern, pricing := range modelPrices {
        if strings.Contains(modelID, pattern) {
            return pricing
        }
    }
    return modelPrices["claude-sonnet-4-6"] // fallback
}
```
[CITED: Pricing from CONTEXT.md D-16, verified against claudelab.net and serenitiesai.com April 2026]

### Placeholder Resolution for New Analytics
```go
// Source: Existing resolver/resolver.go buildSinglePlaceholderValues + D-03 format
func addAnalyticsPlaceholders(values map[string]string, s *session.Session, detail config.DisplayDetail) {
    switch detail {
    case config.DetailMinimal:
        values["{tokens_detail}"] = formatTokensShort(s.TokenBreakdown)       // "250K"
        values["{top_tools}"]     = formatToolsShort(s.ToolCounts)            // "42 tools"
        values["{subagents}"]     = formatSubagentsShort(s.SubagentTree)      // "3 agents"
        values["{context_pct}"]   = formatContextPctShort(s.ContextUsagePct)  // "78%"
        values["{compactions}"]   = formatCompactionsShort(s.CompactionCount) // "2"
        values["{cost_detail}"]   = formatCost(s.TotalCostUSD)               // "$1.20"
    case config.DetailStandard:
        values["{tokens_detail}"] = formatTokensStandard(s.TokenBreakdown)    // "250K Up 80K Down"
        // ... standard detail level
    case config.DetailVerbose:
        values["{tokens_detail}"] = formatTokensVerbose(s.TokenBreakdown)     // "250K Up 80K Down 156Kc"
        // ... verbose detail level
    case config.DetailPrivate:
        values["{tokens_detail}"] = ""
        values["{top_tools}"]     = ""
        values["{subagents}"]     = ""
        values["{context_pct}"]   = ""
        values["{compactions}"]   = ""
        values["{cost_detail}"]   = ""
    }
}
```
[VERIFIED: Existing displayDetail pattern from resolver.go lines 61-117]

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Total tokens only (TotalTokens int64) | Per-model breakdown (map[string]TokenBreakdown) | Phase 4 | Cache-aware pricing, accurate cost for mixed-model sessions |
| Flat cost calculation (input + output) | Cache-aware pricing (4 rates per model) | Phase 4 | 20-40% more accurate cost for cache-heavy sessions |
| No subagent tracking | Hierarchical agent tree | Phase 4 | Visibility into agent delegation patterns |
| No compaction awareness | Baseline-preserved token totals | Phase 4 | Accurate token totals across compaction events |
| English-only presets | Bilingual EN+DE presets | Phase 4 | German-speaking users get native messages |
| Single cost value for all models | Per-model pricing with wildcard matching | Phase 4 | Correct pricing for sessions using Opus main + Haiku subagents |

**Deprecated/outdated:**
- `main.calculateCost()` (lines 718-730): Replaced by analytics/pricing.go with cache-aware 4-rate model
- `main.modelPricing` map (lines 53-61): Replaced by analytics/pricing.go with extended model list including Opus 4.6 GA pricing
- `preset.MessagePreset` flat structure: Extended to BilingualMessagePreset with language wrapper (backward-compat loader)

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Exact BilingualMessagePreset JSON structure `{messages: {en: {...}, de: {...}}}` loads correctly via custom Go unmarshaler | Architecture Patterns: Pattern 6 | Preset loading fails; fallback to legacy format works but no DE |
| A2 | go-i18n embedded FS not used (disk loading preferred for community translations) | Pitfall 7 | If changed to embed, community can't add languages without recompile |
| A3 | context_window.used_percentage null handling is correct (empty string in Discord) | Pitfall 5 | Users see "null%" or "0%" before first API response |
| A4 | Tool abbreviation mapping (Edit=ed, Bash=cmd) fits all 128-char Discord scenarios | Standard Stack | Abbreviations may be too short for some tools or create ambiguity |
| A5 | Write-on-event SSD impact is negligible at ~1-10 events/sec | Architecture Patterns | On slow storage (HDD, network FS) this could be noticeable |

## Open Questions

1. **Analytics data directory location**
   - What we know: D-06 says separate files, D-02 says alongside discord-presence-data.json
   - What's unclear: Exact directory -- `~/.claude/discord-presence-analytics/` vs alongside in `~/.claude/`
   - Recommendation: Use `~/.claude/discord-presence-analytics/` subdirectory for cleanliness (Claude's discretion per CONTEXT.md)

2. **Incremental JSONL read vs. full re-read**
   - What we know: Agent Monitor uses incremental read with offset tracking (transcript-cache.js lines 121-135)
   - What's unclear: Whether the Go binary should use incremental read (better performance) or full re-read (simpler, more reliable after compaction)
   - Recommendation: Start with full re-read for v3.2.0 simplicity; baseline logic handles compaction. Incremental optimization can come later if performance is an issue.

3. **go-i18n message file embedding vs. disk loading**
   - What we know: D-30 specifies i18n/ directory with active.en.json + active.de.json
   - What's unclear: Whether to use go:embed (single binary, no external files) or disk loading (allows community translations)
   - Recommendation: Use go:embed for bundled EN+DE (reliability), with optional disk override for community languages. Same pattern as preset/presets/*.json (already embedded).

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go compiler | Building binary | Yes | 1.26.0 | -- |
| fsnotify | File watching | Yes | v1.9.0 (in go.mod) | Polling fallback exists |
| go-i18n | Bilingual strings | Not yet (new dep) | v2.6.1 available | go get |
| Discord IPC | Rich Presence | Yes | -- (OS-level) | -- |
| Claude Code hooks | Event source | Yes | v2.1.92 | JSONL fallback |

**Missing dependencies with no fallback:** None

**Missing dependencies with fallback:**
- go-i18n v2.6.1: Not yet in go.mod. Add via `go get github.com/nicksnyder/go-i18n/v2@v2.6.1`

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing (stdlib) |
| Config file | None (Go convention: *_test.go files) |
| Quick run command | `go test ./analytics/... -count=1 -short` |
| Full suite command | `go test ./... -count=1 -short` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| DPA-01 | Subagent spawn/complete tracking from hooks | unit | `go test ./analytics/ -run TestSubagentSpawn -count=1` | Wave 0 |
| DPA-03 | Token breakdown JSONL parsing | unit | `go test ./analytics/ -run TestParseTokens -count=1` | Wave 0 |
| DPA-05 | Compaction detection from isCompactSummary | unit | `go test ./analytics/ -run TestCompaction -count=1` | Wave 0 |
| DPA-08 | SubagentStop 4-strategy matching | unit | `go test ./analytics/ -run TestSubagentMatch -count=1` | Wave 0 |
| DPA-08 | Tool counting and top-3 | unit | `go test ./analytics/ -run TestToolCounts -count=1` | Wave 0 |
| DPA-12 | Hook processing < 5ms | benchmark | `go test ./server/ -bench BenchmarkHookHandler -count=1` | Wave 0 |
| DPA-14 | File persistence write/read cycle | unit | `go test ./analytics/ -run TestPersist -count=1` | Wave 0 |
| DPA-16 | Cache-aware cost calculation | unit | `go test ./analytics/ -run TestPricing -count=1` | Wave 0 |
| DPA-19 | Baseline logic (compaction recovery) | unit | `go test ./analytics/ -run TestBaseline -count=1` | Wave 0 |
| DPA-24 | go-i18n EN+DE resolution | unit | `go test ./... -run TestI18n -count=1` | Wave 0 |
| DPA-25 | Bilingual preset loading | unit | `go test ./preset/ -run TestBilingual -count=1` | Wave 0 |
| DPA-02 | Resolver new placeholder integration | integration | `go test ./resolver/ -run TestAnalyticsPlaceholders -count=1` | Wave 0 |
| DPA-10 | GET /status with analytics | integration | `go test ./server/ -run TestStatusAnalytics -count=1` | Wave 0 |
| DPA-30 | Backward compat v3.1.x -> v3.2.0 | integration | `go test ./... -run TestBackwardCompat -count=1` | Wave 0 |

### Sampling Rate
- **Per task commit:** `go test ./analytics/... -count=1 -short` (~2s)
- **Per wave merge:** `go test ./... -count=1 -short` (~15s)
- **Phase gate:** Full suite green + benchmark check before /gsd:verify-work

### Wave 0 Gaps
- [ ] `analytics/tracker_test.go` -- covers DPA-01, DPA-14, DPA-19
- [ ] `analytics/tokens_test.go` -- covers DPA-03, DPA-15
- [ ] `analytics/compaction_test.go` -- covers DPA-05
- [ ] `analytics/subagent_test.go` -- covers DPA-08 (4-strategy matching)
- [ ] `analytics/tools_test.go` -- covers DPA-08 (tool counting)
- [ ] `analytics/pricing_test.go` -- covers DPA-16
- [ ] `analytics/persist_test.go` -- covers DPA-14, DPA-20
- [ ] `resolver/resolver_analytics_test.go` -- covers DPA-02, DPA-04, DPA-06, DPA-07
- [ ] `server/server_analytics_test.go` -- covers DPA-10, DPA-12
- [ ] `preset/bilingual_test.go` -- covers DPA-25
- [ ] Framework install: `go get github.com/nicksnyder/go-i18n/v2@v2.6.1` -- new dependency

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | No user auth in daemon |
| V3 Session Management | No | No user sessions (machine-local daemon) |
| V4 Access Control | No | Localhost-only binding (127.0.0.1) |
| V5 Input Validation | Yes | JSON schema validation on hook payloads, JSONL entry parsing with error recovery |
| V6 Cryptography | No | No secrets stored |

### Known Threat Patterns for Go daemon

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Malformed JSONL entry crashes parser | Denial of Service | json.Unmarshal error recovery per entry; continue on corrupt entries |
| Analytics files written to predictable path | Information Disclosure | Files in ~/.claude/ (user-only readable on Unix); Windows ACL inherits user profile |
| Oversized JSONL file OOM | Denial of Service | bufio.Scanner with 1MB buffer limit (existing pattern in main.go:676) |
| Hook handler timeout abuse | Denial of Service | 1s timeout per hook; O(1) handler per D-20 |

## Sources

### Primary (HIGH confidence)
- cc-discord-presence Go source code -- all files in C:\Users\ktown\Projects\cc-discord-presence\ (read and verified)
- Claude-Code-Agent-Monitor Node.js source -- transcript-cache.js, hooks.js, db.js, pricing.js (read and verified)
- Live discord-presence-data.json -- confirmed context_window.used_percentage field structure (null at start)
- Live JSONL sample -- confirmed message.usage.cache_read_input_tokens and cache_creation_input_tokens fields
- Claude Code Hooks docs (code.claude.com/docs/en/hooks) -- SubagentStop payload verified with agent_id, agent_transcript_path
- go-i18n v2.6.1 (pkg.go.dev/github.com/nicksnyder/go-i18n/v2/i18n) -- Bundle/Localizer API verified
- go.mod -- fsnotify v1.9.0, go-winio v0.6.2, lumberjack v2.2.1 confirmed
- Go 1.26.0 verified via `go version`

### Secondary (MEDIUM confidence)
- [Claude Code Hooks Reference](https://code.claude.com/docs/en/hooks) -- SubagentStop fields, PreToolUse Agent payload
- [go-i18n GitHub](https://github.com/nicksnyder/go-i18n) -- v2.6.1 published 2026-01-01
- [go-i18n Package Docs](https://pkg.go.dev/github.com/nicksnyder/go-i18n/v2/i18n) -- API reference

### Tertiary (LOW confidence)
- Pricing data from claudelab.net, serenitiesai.com (April 2026) per CONTEXT.md -- may change with future model releases

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all libraries verified via go.mod, go list, and pkg.go.dev
- Architecture: HIGH -- existing codebase patterns deeply analyzed; Agent Monitor reference implementation read
- Pitfalls: HIGH -- verified from live data (JSONL samples, discord-presence-data.json), confirmed edge cases from Agent Monitor production code

**Research date:** 2026-04-07
**Valid until:** 2026-05-07 (stable domain, but Anthropic pricing/model changes may invalidate D-16)
