# Phase 4: Discord Presence Enhanced Analytics - Context

**Gathered:** 2026-04-07
**Updated:** 2026-04-07
**Status:** Ready for planning

<domain>
## Phase Boundary

Go-Binary (cc-discord-presence) um Analytics-Features aus dem Claude-Code-Agent-Monitor anreichern: Subagent-Hierarchie-Tracking (Spawn/Complete-Events, Agent-Tree, Tiefe), Token-Breakdown (input/output/cache_read/cache_write separat), Compaction-Erkennung (isCompactSummary aus JSONL), Tool-Nutzungsstatistiken (Top-3-Tools, Frequenz), Context-Usage-Prozent als neuer Platzhalter, Kosten-Tracking mit cache-aware Pricing, bilinguale Presets (EN+DE) mit go-i18n fuer Code-Strings, und 80+ neue Preset-Messages die diese Daten nutzen. Alles als Go-Packages im bestehenden cc-discord-presence Binary.

Repo: `C:\Users\ktown\Projects\cc-discord-presence` (Go, MIT License)
GitHub: `DSR-Labs/cc-discord-presence`
Feature-Mining Quelle: `C:\Users\ktown\Projects\Claude-Code-Agent-Monitor`

</domain>

<decisions>
## Implementation Decisions

### Datenquellen-Architektur (D-01, D-02)
- **D-01:** 3-Quellen-Split — Hooks fuer Subagents (PreToolUse tool_name=Agent + SubagentStop), JSONL Watcher fuer Tokens+Compaction (message.usage.* + isCompactSummary), Statusline fuer Context% (discord-presence-data.json). Saubere Trennung: real-time Events vs. kumulative Daten vs. UI-Status.
- **D-02:** File-Persist fuer Analytics-Daten — Separate JSON-Dateien (D-06) neben discord-presence-data.json, ueberleben Daemon-Restart. Nicht rein in-memory wie bisher.

### Platzhalter-Format (D-03, D-04)
- **D-03:** Kompakt mit Symbolen — Pfeile (Aufwaerts/Abwaerts), Multiplikatoren (x), Mittelpunkte (Dot) fuer maximale Info in 128 Zeichen Discord-Limit. Format nach displayDetail:
  - minimal: {tokens_detail}="250K", {top_tools}="42 tools", {subagents}="3 agents", {context_pct}="78%", {compactions}="2"
  - standard: {tokens_detail}="250K Up 80K Down", {top_tools}="42ed Dot 12cmd", {subagents}="3 agents (2 done)", {context_pct}="78% context", {compactions}="2 compactions"
  - verbose: {tokens_detail}="250K Up 80K Down 156Kc", {top_tools}="42ed Dot 12cmd Dot 5grep Dot 3read", {subagents}="3: 2xExplore 1xreviewer", {context_pct}="78% (156K/200K)", {compactions}="2xcompact (~500K)"
  - private: ALL = "" (leer)
- **D-04:** Leer-String bei fehlenden Daten — Kein Subagent = {subagents} wird zu "". Resolver trimmt doppelte Leerzeichen und Trailing-Dots. Kein Fallback-Text, kein N/A Marker.

### Tool-Tracking (D-05, D-06)
- **D-05:** Rohe Tool-Namen (Edit, Bash, Grep), Top-3 fuer {top_tools} Platzhalter. map[string]int im Session struct. Abgekuerzte Anzeige: Edit="ed", Bash="cmd", Grep="grep", Read="read", Write="write", Agent="agent".
- **D-06:** Separate Dateien pro Feature — Token-Daten, Tool-Stats, Subagent-Daten jeweils eigenes JSON-File. Nicht eine monolithische analytics-data.json. Unabhaengig lesbar/schreibbar.

### Subagent-Anzeige (D-07, D-08)
- **D-07:** Hierarchie-Baum mit ParentID — SubagentEntry{ID, Type, Description, Status, SpawnTime, CompletedTime, ParentID}. /dsrcode:status zeigt eingerueckten Baum. Discord {subagents} Platzhalter zeigt flache Zusammenfassung (3: 2xExplore 1xreviewer).
- **D-08:** ID-basiertes Matching mit Description-Prefix Fallback — agent_id aus SubagentStop nutzen wenn vorhanden (seit CC 2.0.42, Issue #11726). Ohne agent_id: 4 Matching-Strategien in Reihenfolge: (1) Description-Prefix Match (57 chars), (2) agent_type Match, (3) Prompt-Prefix Match (500 chars), (4) Fallback: aeltester Working Subagent. Robust und zukunftssicher.

### Go Package-Struktur (D-09)
- **D-09:** Neues analytics/ Package — Enthaelt: tracker.go (Koordinator), tokens.go (TokenBreakdown JSONL-Parsing), compaction.go (isCompactSummary Erkennung), subagent.go (Hierarchie-Baum + ID/Description Matching), tools.go (Tool-Counter map[string]int), persist.go (File-basierte Persistenz), context.go (Context% aus discord-presence-data.json), pricing.go (Hardcoded Model-Preise). Session struct bekommt neue Felder (TokenBreakdown, ToolCounts, SubagentTree, CompactionCount, ContextUsagePct, TranscriptPath, CostBreakdown), analytics/ fuellt sie.

### Context% Pipeline (D-10)
- **D-10:** Context% via bestehende discord-presence-data.json — Go Binary parst context_window.used_percentage per fsnotify auf die bereits existierende Statusline-Datei. Kein neuer Endpoint, keine Wrapper-Aenderung. Research-backed: GitHub Issues #34537, #21651 + Anthropic Threads bestaetigen context_window.used_percentage im Statusline-JSON.

### Version & Preset-Messages (D-11, D-12)
- **D-11:** Version v3.2.0 — SemVer MINOR Bump. Neue Features (7 Platzhalter, analytics/ Package, SubagentStop Hook, go-i18n, bilingual), vollstaendig backward-compatible. CHANGELOG.md in Keep a Changelog Format (Fixed/Added/Changed).
- **D-12:** 80+ neue Messages (10 pro Preset x 8 Presets) in EN + DE. Jeder neue Platzhalter ({tokens_detail}, {compactions}, {context_pct}, {top_tools}, {subagents}, {cost_detail}) in 2+ Messages pro Preset. Bestehende Messages ebenfalls in DE uebersetzt. Gemischt ueber Activity-Kategorien (details+state). Natuerliche Variation, nicht jede Message nutzt alle Platzhalter.

### JSONL-Discovery (D-13)
- **D-13:** transcript_path aus Hook-Events — Alle Hook-Events (PreToolUse, PostToolUse, Stop, SubagentStop, SessionStart) liefern transcript_path als Common Field. Kein eigenes Suchen/Scannen von JSONL-Dateien noetig. Zuverlaessigster Ansatz. Pfad wird in Session struct gespeichert (D-18).

### Hook-Registrierung (D-14)
- **D-14:** Auto-Patch hooks.json beim Start — start.sh prueft hooks.json und fuegt fehlende Eintraege hinzu: (1) Agent zum PreToolUse-Matcher, (2) SubagentStop Hook-Sektion mit HTTP POST an http://127.0.0.1:19460/hooks/subagent-stop. Bestehende Eintraege bleiben erhalten. Self-healing bei jedem SessionStart.

### Testing-Strategie (D-15)
- **D-15:** Internal + External Tests — Internal Tests im analytics Package fuer JSONL-Parsing Internals (Token-Extraktion, Compaction-Detection, Baseline-Logik). External Tests im analytics_test Package fuer oeffentliche API (Tracker, Persist, Pricing). Tabellen-getriebene Tests. Konsistent mit bestehendem Pattern (session_test, resolver_test, server_test).

### Kosten-Tracking (D-16, D-17)
- **D-16:** Hardcoded Pricing in pricing.go — 6 Modelle mit 4 Raten (input, output, cache_write, cache_read per MTok):
  - Opus 4.6: $5 / $25 / $6.25 / $0.50 (1M Context, GA seit 13.03.2026, kein Aufpreis)
  - Sonnet 4.6: $3 / $15 / $3.75 / $0.30 (1M Context)
  - Haiku 4.5: $1 / $5 / $1.25 / $0.10 (200K Context)
  - Opus 4.5 (Legacy): $5 / $25 / - / -
  - Opus 4.1 (Legacy): $15 / $75 / - / -
  - Haiku 3 (Legacy): $0.25 / $1.25 / - / -
  Wildcard-Matching fuer Model-Varianten (strings.Contains). Updates kommen mit Binary via Phase 5 GoReleaser.
- **D-17:** {cost} intern cache-aware berechnen (gleiche Ausgabe $X.XX). Neuer {cost_detail} Platzhalter fuer verbose displayDetail: "$1.20 (Up$0.80 Down$0.35 c$0.05)". Folgt displayDetail-Pattern (D-03).

### transcript_path Storage (D-18)
- **D-18:** transcript_path aus Hook-Events in Session struct speichern — Neues TranscriptPath Feld. JSONL-Watcher nutzt gespeicherten Pfad statt eigenem Suchen. Auch fuer spaetere Nutzung (Export, Debugging).

### Compaction-Baselines (D-19)
- **D-19:** Baselines in Analytics-JSON persistieren — Wenn Compaction die JSONL umschreibt, sinken Token-Werte. Alte Werte werden zu Baseline addiert (wie Agent Monitor's replaceTokenUsage). Effektiver Total = current + baseline. Persistiert in der Token-Analytics-Datei (D-06). Ueberlebt Daemon-Restart.

### Performance-Architektur (D-20)
- **D-20:** Getrennte Verantwortung — Hook-Handler: nur Tool/Subagent-Tracking + transcript_path speichern (O(1), <1ms). JSONL-Watcher (fsnotify, existiert bereits in main.go): Token-Parsing + Compaction-Detection asynchron via gespeicherten transcript_path. Saubere Trennung, nutzt bestehende Architektur. Garantiert <5ms Hook-Processing (SC8).

### Backward Compatibility (D-21)
- **D-21:** Lazy Init fuer Analytics-Dateien — Keine Dateien vorab erstellen. os.IsNotExist() pruefen, bei erstem Analytics-Event schreiben. Go Zero-Values (int=0, string="") als natuerliche Defaults. D-04 (leere Platzhalter) + D-14 (auto-patch hooks.json) decken den kompletten Upgrade-Pfad v3.1.x zu v3.2.0 ab.

### Multi-Session Analytics (D-22)
- **D-22:** Selektive Multi-Session-Aggregation — {totalCost} wird cache-aware (D-17). {totalTokensDetail} aggregiert ueber alle Sessions. Andere neue Platzhalter ({context_pct}, {subagents}, {top_tools}) zeigen Daten der aktivsten Session (bestehendes "most active" Verhalten aus resolver.go). Kein neuer Multi-Session-spezifischer Platzhalter. Discord 128-Char-Limit respektiert.

### /dsrcode:status Ausgabe (D-23)
- **D-23:** Einrueckung + Status-Icons fuer Subagent-Baum — Check-Mark fuer done, Spinning fuer working. Nested Subagents eingerueckt. Tool-Stats, Token-Breakdown, Context% als eigene Zeilen. Format:
  ```
  Subagents:
    Check Explore: scan project (done, 45s)
    Spin code-reviewer: review changes (working)
      Spin typescript-reviewer (working, nested)
  Tools: 42 edits Dot 12 cmds Dot 5 searches
  Tokens: 250K Up 80K Down 156Kc | $1.20
  Context: 78% | 2 compactions
  ```
  Cross-platform sicher (kein Unicode Box-Drawing).

### GET /status Response (D-24)
- **D-24:** Per-Session Analytics eingebettet + Top-Level Aggregate — Jede Session im Response enthaelt eigene Analytics (tokens, tools, subagents, context_pct, compactions, cost). Zusaetzliches Top-Level "analytics" Objekt mit Summen ueber alle Sessions. Bestehende Felder (version, config, sessions) unveraendert. Backward-compatible.

### Per-Model Token-Tracking (D-25)
- **D-25:** Per-Model Tokens aus JSONL — JSONL-Watcher parst msg.model + msg.usage pro Eintrag. map[string]TokenBreakdown im Session struct. Pricing-Berechnung (D-16) nutzt exakte Per-Model-Raten. Beispiel: Session mit Opus (Main) + Haiku (Subagents) = verschiedene Preise pro Token-Typ.

### Analytics Lifecycle (D-26)
- **D-26:** Nie auto-loeschen — Analytics-Dateien (wenige KB pro Session) bleiben bestehen. User kann manuell loeschen. Daemon laedt nur aktive Sessions. Baselines (D-19) bleiben erhalten fuer spaetere Analyse.

### Bilinguale Presets (D-27)
- **D-27:** Alle Presets in EN + DE — Jeder englische Text muss auch auf Deutsch existieren. Globale Config-Einstellung "lang" fuer Sprachwechsel. Dediziertes "german" Preset entfaellt — alle Presets unterstuetzen beide Sprachen. Bestehende Messages werden uebersetzt + 80 neue Messages in beiden Sprachen.

### Hook-Installation (D-28)
- **D-28:** Plugin hooks.json als Standard — Hooks werden in der Plugin hooks.json definiert (auto-discovered by Claude Code). SubagentStop als neue Sektion. Agent zum PreToolUse-Matcher. Kein Eingriff in User's settings.json. README dokumentiert manuellen Fallback fuer bekannten Cowork-Bug (#27398, #40495). Standard-Approach fuer Marketplace-Plugins.

### Preset-Dateiformat (D-29)
- **D-29:** Top-Level Sprach-Wrapper in Preset-JSONs — Struktur: { "label": "chaotic", "description": {...}, "messages": { "en": { "singleSessionDetails": { "coding": [...] }, ... }, "de": { "singleSessionDetails": { "coding": [...] }, ... } } }. Resolver liest config.lang, waehlt Sub-Objekt. StablePick Rotation bleibt unveraendert (waehlt Message, dann Sprache).

### go-i18n Integration (D-30)
- **D-30:** go-i18n Library (github.com/nicksnyder/go-i18n/v2) fuer Go Code-Strings — BCP-47 Tags (language.English, language.German), automatischer Fallback, Bundle/Localizer Pattern. Fuer: /dsrcode:status Ausgabe, Fehlermeldungen, Status-Texte. NICHT fuer Preset-Messages (siehe D-35). active.en.json + active.de.json im i18n/ Verzeichnis. goi18n CLI fuer Extract/Merge.

### Write-on-Event Persistenz (D-31)
- **D-31:** Sofort auf Disk bei jedem Analytics-Event — ~1ms pro Write, ~1-10 Events/Sekunde bei aktivem Coding. SSD-Impact: 0.01%. Crash-proof (Daten immer aktuell). Kein Shutdown-Flush noetig. Kein Datenverlust moeglich. Events sind selten (Tool-Use, Subagent-Spawn, nicht jeder Keystroke).

### Graceful Shutdown (D-32)
- **D-32:** Trivial — Daten sind durch D-31 (write-on-event) immer aktuell auf Disk. Daemon stoppt, alle Daten sind bereits persistiert. Kein Flush-Handler noetig. SIGINT/SIGTERM beendet Goroutines sauber.

### German Preset Migration (D-33)
- **D-33:** Auto-Migration beim Start — start.sh erkennt config.json mit "preset": "german", migriert zu "preset": "professional" + "lang": "de". Nahtloser Upgrade. Gleicher Self-Healing-Ansatz wie D-14 (auto-patch hooks.json).

### Feature Map (D-34)
- **D-34:** Feature Map in config.json — { "features": { "analytics": true } }. Skaliert auf N Features (notifications, dashboard, auto-warnings etc.). Default: alle true. Wenn analytics=false: Hook-Handler ueberspringt Analytics-Processing, Platzhalter leer (D-04). Ein Config-Abschnitt, nicht N einzelne Booleans.

### Hybrid Translation Workflow (D-35)
- **D-35:** go-i18n fuer Code-Strings + Preset-JSON fuer kreative Messages — Natuerliche Trennung: funktionale Strings (Status, Fehler) via go-i18n mit goi18n CLI, Hash-Tracking, automatischem Fallback. Kreative Preset-Messages (Humor, Ton, Persoenlichkeit) im D-29 JSON-Format mit Kontext-Buendeln pro Activity-Kategorie. go-i18n ist FALSCH fuer Presets weil: (1) Kontext-Verlust bei isolierten Message-IDs, (2) Array-Index-Kopplung, (3) StablePick-Inkompatibilitaet, (4) kreative Uebersetzung ≠ wörtliche Uebersetzung, (5) Plural-Overhead ohne Nutzen. CONTRIBUTING.md dokumentiert beide Workflows.

### Claude's Discretion
- JSONL Watcher Refactoring-Scope (wie viel aus main.go nach analytics/ extrahieren)
- Exakte SubagentEntry struct Felder und JSON-Serialisierung
- analytics.Tracker Lifecycle (Start/Stop, goroutine management)
- File-Persist Dateinamen und Verzeichnis (neben discord-presence-data.json oder eigenes Verzeichnis)
- Token-Abbreviation Format (K vs. k, Schwellwert fuer M-Notation)
- Tool-Name Abbreviation Mapping (welche Abkuerzungen fuer seltene Tools)
- CHANGELOG.md Eintraege fuer v3.2.0
- Error Handling bei korrupten JSONL Eintraegen
- fsnotify Event Debouncing fuer discord-presence-data.json und transcript_path
- go-i18n Message-IDs und Dateistruktur fuer Code-Strings
- Preset-Translations DE Qualitaet und kreative Lokalisierung
- Feature Map Default-Werte fuer zukuenftige Features
- Concurrent file access Handling bei mehreren Sessions

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Feature-Mining Quelle (Agent Monitor) — Deep-Scan Ergebnisse
- `C:\Users\ktown\Projects\Claude-Code-Agent-Monitor\server\lib\transcript-cache.js` lines 137-165 — JSONL Token-Parsing (message.usage.input_tokens, output_tokens, cache_read_input_tokens, cache_creation_input_tokens), isCompactSummary Detection (lines 144-150), Incremental Read Strategy (lines 121-135), LRU Cache (200 entries, lines 209-219)
- `C:\Users\ktown\Projects\Claude-Code-Agent-Monitor\server\routes\hooks.js` lines 88-129 — Subagent Spawn Detection (PreToolUse tool_name=Agent), Name Inference (description > subagent_type > prompt prefix > "Subagent"), Parent Agent Inference via deepest working subagent (lines 100-113)
- `C:\Users\ktown\Projects\Claude-Code-Agent-Monitor\server\routes\hooks.js` lines 202-257 — SubagentStop 4-Strategy Matching (description prefix 57ch > agent_type > prompt 500ch > oldest working fallback), Status "working" to "completed"
- `C:\Users\ktown\Projects\Claude-Code-Agent-Monitor\server\routes\hooks.js` lines 335-401 — Transcript Token Extraction + Compaction Agent Creation (compactId = sessionId-compact-uuid, dedup by uuid)
- `C:\Users\ktown\Projects\Claude-Code-Agent-Monitor\server\db.js` lines 51-66 — agents table schema (id, session_id, name, type, subagent_type, status, task, current_tool, started_at, ended_at, parent_agent_id, metadata)
- `C:\Users\ktown\Projects\Claude-Code-Agent-Monitor\server\db.js` lines 81-90 — token_usage table (session_id, model, input/output/cache_read/cache_write + baseline columns)
- `C:\Users\ktown\Projects\Claude-Code-Agent-Monitor\server\db.js` lines 339-356 — replaceTokenUsage Baseline Logic (when new < old, accumulate old to baseline)
- `C:\Users\ktown\Projects\Claude-Code-Agent-Monitor\server\db.js` lines 275-295 — findDeepestWorkingAgent Recursive CTE
- `C:\Users\ktown\Projects\Claude-Code-Agent-Monitor\server\routes\workflows.js` lines 609-633 — buildAgentTree() recursive function
- `C:\Users\ktown\Projects\Claude-Code-Agent-Monitor\server\routes\workflows.js` lines 104-118 — Agent Depth Recursive CTE
- `C:\Users\ktown\Projects\Claude-Code-Agent-Monitor\server\routes\workflows.js` lines 225-251 — Tool Flow Transitions (source > target > count)
- `C:\Users\ktown\Projects\Claude-Code-Agent-Monitor\server\routes\analytics.js` — Tool usage counts, event type aggregation, daily stats
- `C:\Users\ktown\Projects\Claude-Code-Agent-Monitor\server\routes\pricing.js` lines 7-48 — Cost Calculation with Wildcard Pattern Matching, per-model USD breakdown
- `C:\Users\ktown\Projects\Claude-Code-Agent-Monitor\statusline\statusline.py` lines 66-85 — Context% Rendering aus context_window.used_percentage

### Go Binary (cc-discord-presence) — Deep-Scan Ergebnisse
- `C:\Users\ktown\Projects\cc-discord-presence\main.go` — Entry point (773 lines). JSONL Watcher: ingestJSONLFallback() (lines 490-526), readStatusLineData() (lines 532-566), findMostRecentJSONL() (lines 593-657), parseJSONLSession() (lines 659-716), calculateCost() (lines 718-730). Discord IPC (lines 337-373), HTTP Server (lines 218-285), Config Watch (lines 290-318).
- `C:\Users\ktown\Projects\cc-discord-presence\session\types.go` — Session struct (lines 32-53): SessionID, ProjectPath, ProjectName, PID, Source, Model, Branch, Details, SmallImageKey, SmallText, TotalTokens, TotalCostUSD, ActivityCounts, Status, StartedAt, LastActivityAt, LastFile, LastFilePath, LastCommand, LastQuery. ActivityCounts (lines 18-24): Edits, Commands, Searches, Reads, Thinks.
- `C:\Users\ktown\Projects\cc-discord-presence\session\registry.go` — SessionRegistry with RWMutex, onChange callback, Copy-before-modify immutable pattern (line 83)
- `C:\Users\ktown\Projects\cc-discord-presence\server\server.go` — HTTP Handler (816 lines). Routes: /hooks/{hookType} (lines 286-357), /statusline (lines 359-388), /config (lines 390-407), /health (lines 466-477), /sessions (lines 587-594), /status (lines 562-585), /presets (lines 495-549), /preview (lines 409-464). Activity Mapping (lines 31-41): Edit/Write="coding", Bash="terminal", Grep/Glob="searching", Read="reading", Task="thinking".
- `C:\Users\ktown\Projects\cc-discord-presence\resolver\resolver.go` — Placeholder Resolution (404 lines). Single: {activity}, {sessions}, {duration}, {filepath}, {project}, {branch}, {file}, {command}, {query}, {model}, {tokens}, {cost}. Multi: {n}, {sessions}, {projects}, {models}, {totalCost}, {totalTokens}, {stats}, {mode}. StablePick 5-min Knuth hash rotation.
- `C:\Users\ktown\Projects\cc-discord-presence\hooks\hooks.json` — Current hooks: SessionStart (start.sh + install-skills.sh), PreToolUse (Edit|Write|Bash|Read|Grep|Glob|WebSearch|WebFetch|Task), UserPromptSubmit, Stop, Notification. MISSING: Agent in PreToolUse, SubagentStop.
- `C:\Users\ktown\Projects\cc-discord-presence\preset\presets\*.json` — 8 Presets: chaotic, dev-humor, german (to be migrated D-33), hacker, minimal, professional, streamer, weeb. Each with singleSessionDetails/State by activity category.
- `C:\Users\ktown\Projects\cc-discord-presence\cmd\` — EMPTY directory. main.go stays in root.

### Claude Code Hook-System
- SubagentStop Hook: Common Fields: session_id, transcript_path, cwd, permission_mode. Event-specific: description, agent_type, agent_id (since CC 2.0.42, Issue #11726), agent_transcript_path (since CC 2.0.42). Matcher: "*" for all subagent stops.
- PreToolUse(Agent): tool_input.description, tool_input.subagent_type, tool_input.prompt verfuegbar
- Statusline JSON: context_window.used_percentage bestaetigt (GitHub Issues #34537, #21651)
- Plugin hooks.json format: { "hooks": { "EventName": [...] } } with optional "description"
- Settings.json format: { "EventName": [...] } without wrapper
- Known Issues: anthropics/claude-code#27398 (Plugin hooks don't fire in Cowork), anthropics/claude-code#40495 (Cowork ignores all hooks/settings — sandbox bug)
- Discord Presence Rate Limit: max 5 updates per 20 seconds (Discord API docs)

### Pricing (Stand: Maerz 2026, GA)
- Opus 4.6: $5/$25/$6.25/$0.50 per MTok (input/output/cache_write/cache_read) — 1M Context, kein Aufpreis seit GA 13.03.2026
- Sonnet 4.6: $3/$15/$3.75/$0.30 per MTok — 1M Context
- Haiku 4.5: $1/$5/$1.25/$0.10 per MTok — 200K Context
- Quelle: claudelab.net, serenitiesai.com (verifiziert April 2026)

### i18n
- go-i18n v2: github.com/nicksnyder/go-i18n/v2 — Bundle/Localizer Pattern, BCP-47 Tags, JSON message files, goi18n CLI (extract/merge)
- Context7 docs bestaetigen: RegisterUnmarshalFunc("json", json.Unmarshal) + LoadMessageFile("active.de.json")

### Prior Phase Context
- `.planning/phases/02-dsrcodepresence-setup-wizard/02-CONTEXT.md` — D-21/D-22 (displayDetail System), D-18-20 (HookPayload Parsing), D-24 (Platzhalter)
- `.planning/phases/03-fix-discord-presence-session-count-and-enhance-demo-mode/03-CONTEXT.md` — D-01-05 (Session Dedup), D-18 (Version v3.1.0), D-25 (Externe Test Packages)
- `.planning/phases/01-discord-rich-presence-activity-status-plugin-merge/01-CONTEXT.md` — Alle Grundlagen-Decisions (Activity Mapping, Presets, Multi-Session, Config)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `session.SessionRegistry`: Thread-safe Session Management mit RWMutex und onChange Callback — analytics.Tracker koppelt sich an onChange
- `resolver.replacePlaceholders()`: Generischer Template-Renderer — erweitern fuer 7 neue Platzhalter ({tokens_detail}, {top_tools}, {subagents}, {context_pct}, {compactions}, {cost_detail})
- `resolver.StablePick()`: Anti-Flicker Rotation (Knuth-Hash, 5-Min-Fenster) — unveraendert, neue Messages profitieren automatisch
- `server.handleHook()`: HTTP Hook Handler — erweitern fuer SubagentStop Route + Agent tool_input Parsing
- `main.ingestJSONLFallback()`: JSONL Watcher — erweitern fuer Token-Breakdown-Parsing + Compaction-Detection via transcript_path
- `statusline-wrapper.sh`: Speichert vollstaendiges Statusline-JSON in discord-presence-data.json — context_window.used_percentage bereits enthalten
- `session.Source` Enum (Phase 3): SessionSource iota mit String() + MarshalJSON — Pattern fuer SubagentStatus Enum
- `main.calculateCost()`: Bestehende Kosten-Berechnung (lines 718-730) — ersetzen durch analytics/pricing.go mit Per-Model-Raten
- `config.Config`: fsnotify Hot-Reload — erweitern fuer "lang" und "features" Felder

### Established Patterns
- Immutable Session Updates: Copy-before-modify Pattern in Registry (registry.go:83)
- Activity Counter Map: SmallImageKey zu Counter-Feld Mapping (types.go:63-69) — analog fuer ToolCounts
- External test packages: session_test, resolver_test, server_test fuer black-box Testing
- Config Hot-Reload via fsnotify — gleiches Pattern fuer analytics data.json Watching + Sprach-Wechsel
- displayDetail Enum mit ParseDisplayDetail Fallback — analog fuer neue Platzhalter-Resolution

### Integration Points
- analytics.Tracker registriert sich als Listener auf SessionRegistry.onChange
- server.handleHook() ruft analytics.Tracker.RecordTool() und analytics.Tracker.RecordSubagentSpawn() auf
- server.handleSubagentStop() ruft analytics.Tracker.RecordSubagentComplete() auf
- JSONL Watcher ruft analytics.Tracker.UpdateTokens() und analytics.Tracker.RecordCompaction() auf (via transcript_path aus Session)
- Statusline data.json Watcher ruft analytics.Tracker.UpdateContextUsage() auf
- resolver.resolveSingle/Multi() liest analytics-Daten aus Session struct fuer Platzhalter-Resolution
- go-i18n Localizer in resolver fuer Code-String Uebersetzungen
- Preset-Loader liest config.lang und waehlt Sprach-Sub-Objekt

</code_context>

<specifics>
## Specific Ideas

- Agent Monitor als Referenz-Implementation: Alle 5 Core-Features sind dort produktiv im Einsatz (Node.js + SQLite), werden nach Go portiert (in-memory + File-Persist)
- Token-Abbreviation wie in Success Criteria: "250K Up 80K Down 156Kc" — K=Kilo (1000), c=cache, Up/Down=Pfeile fuer Input/Output
- Subagent-Baum in /dsrcode:status: Eingerueckte Darstellung mit Status-Icons (Check/Spin/X fuer Done/Working/Error)
- Tool-Stats Abbreviation: Standard Claude Code Tool-Namen (Edit, Bash, Grep, Read, Write, Glob, Agent) auf 2-5 Buchstaben gekuerzt
- Compaction-Recovery: Baseline-Logik portiert aus Agent Monitor (replaceTokenUsage). Effektiver Total = current + baseline
- Context% Warnung: Bei context_pct >= 80% koennte der Preset eine Warnung-Message zeigen ("Warning: context almost full")
- Performance: SubagentStop Hook + Tool-Counting sind O(1) Operationen. JSONL-Parsing ist IO-bound aber asynchron (D-20). Alles unter 5ms Hook-Processing erreichbar.
- Bilinguale Presets: Kreative Lokalisierung statt woertlicher Uebersetzung. "YOLO-editing {project}" -> "Ballert {project} ohne Ruecksicht". Ton und Humor muessen zum Preset passen.
- Feature Map: Zukunftssicher fuer Notifications, Dashboard, Auto-Warnings etc. Ein Config-Abschnitt statt N einzelne Booleans.
- German Preset Migration: Nahtloser Upgrade fuer bestehende User — kein manueller Eingriff noetig.
- go-i18n fuer Code-Strings: goi18n extract/merge fuer automatische Sync-Detection. Community kann active.fr.json, active.es.json etc. per PR beitragen.

</specifics>

<deferred>
## Deferred Ideas

- Worktree-Agent Zuordnung zum Parent-Projekt (Phase 2 D-34) — eigene Phase
- Per-Session Presets (Phase 2 deferred) — verschiedene Presets fuer verschiedene Projekte
- Web-Dashboard fuer Session-Statistiken — eigene Phase
- Tool-Flow Transitions (Sankey Diagram Data) — Agent Monitor Feature, zu komplex fuer Discord
- Per-Model Token Breakdown im Discord Display (Agent Monitor trackt pro Model) — fuer Discord reicht Total pro Session
- SQLite Persistenz statt JSON Files — bei Bedarf spaeter, aktuell Overkill
- Auto-Compaction-Warning als Discord Notification — eigene Mini-Phase
- Token-Cost Calculation aus Breakdown (per-model pricing) — JETZT in Phase 4 (D-16/D-17/D-25)
- Workflow Pattern Detection (welche Subagent-Typen folgen aufeinander) — Agent Monitor Feature, eigene Phase
- Agent Co-occurrence Matrix — Agent Monitor Feature, eigene Phase
- Session Complexity Scoring — Agent Monitor Feature, eigene Phase
- Crowdin/Transifex Integration fuer professionelle Uebersetzungen — spaeter wenn 5+ Sprachen
- CONTRIBUTING.md fuer Translation-Workflow — erstellen wenn Community waechst

</deferred>

---

*Phase: 04-discord-presence-enhanced-analytics-subagent-tracking-token-breakdown-compaction-erkennung-tool-statistiken-und-context-usage-aus-agent-monitor-portieren*
*Context gathered: 2026-04-06, updated: 2026-04-07*
