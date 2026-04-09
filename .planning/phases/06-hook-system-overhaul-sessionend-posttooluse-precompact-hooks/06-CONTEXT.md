# Phase 6: Hook System Overhaul - Context

**Gathered:** 2026-04-09 (updated from 2026-04-08)
**Status:** Ready for planning

<domain>
## Phase Boundary

Vollstaendige Ueberarbeitung des Hook-Systems in dsrcode: (1) Sechs neue Hooks — SessionEnd fuer opportunistisches Session-Teardown, PostToolUse fuer Token-Daten via transcript_path nach Tool-Ausfuehrung, PreCompact fuer Compaction-Baseline, PostCompact fuer Compaction-Verifizierung, StopFailure fuer API-Error-Display, SubagentStart fuer frueheres Subagent-Tracking. (2) JSONL-Polling/Watching/Discovery entfernen — ingestJSONLFallback(), findMostRecentJSONL(), jsonlPollLoop(), fsnotify JSONL-Watcher (~250 Zeilen in main.go). JSONL-Parsing-Logik BEHALTEN in analytics/ fuer hook-getriggerte Reads. (3) Binary Auto-Exit — Dual-Trigger (SessionEnd + Stale-Detection), konfigurierbare Grace-Period, Discord Activity clearen vor Shutdown. (4) Stale-Handling bleibt primaer Timer-basiert (30s) — SessionEnd ist zu unzuverlaessig. (5) Alle Matcher auf Wildcard `*`. (6) 15 Hook-Events total (14 in settings.local.json + SessionStart in Plugin hooks.json). (7) Hook-Fehlerbehandlung verbessern. (8) StopFailure Display mit neuem "error" Icon.

Repo: `C:\Users\ktown\Projects\cc-discord-presence` (Go, MIT License)
GitHub: `StrainReviews/dsrcode`
Binary: `dsrcode`
Module: `github.com/StrainReviews/dsrcode`
Version: v4.0.0

</domain>

<decisions>
## Implementation Decisions

### Token-Pipeline nach JSONL-Entfernung (D-01, D-02, D-03)
- **D-01:** Hook-Triggered JSONL Reads — JSONL-Polling/Watching/Discovery entfernen (~250 Zeilen main.go: ingestJSONLFallback(), findMostRecentJSONL(), parseJSONLSession(), jsonlPollLoop(), fsnotify JSONL-Watcher). JSONL-Parsing-Logik BEHALTEN und nach analytics/ verschieben fuer gezielte Reads via transcript_path aus Hook-Events.
- **D-02:** PostToolUse als Token-Read-Trigger — PostToolUse feuert NACH Tool-Ausfuehrung, JSONL ist dann aktuell. Daemon liest transcript_path aus PostToolUse-Payload und parst JSONL fuer Token-Breakdown (input/output/cache). Interne Drosselung: max 1 JSONL-Read pro 10s um File-IO zu begrenzen.
- **D-03:** Claude Code Hooks liefern KEINE token_usage in Payloads — Research bestaetigt (April 2026, v2.1.98): PostToolUse sendet nur session_id, transcript_path, cwd, tool_name, tool_input, tool_result. Token-Breakdown ist NUR via JSONL-Parsing verfuegbar. Issue #11008 (17 Upvotes, offen) fordert token_usage in Payloads — kein Fix geplant.

### Binary Auto-Exit (D-04, D-05, D-06, D-07)
- **D-04:** Dual-Trigger Auto-Exit — SessionEnd (opportunistisch, ~30% Zuverlaessigkeit) + Stale-Detection (zuverlaessiger Fallback, 100%). Wenn BEIDE einen Session-Remove ausloesen und Refcount=0 wird, startet Grace-Timer. Robust gegen SessionEnd-Bugs.
- **D-05:** Konfigurierbare Grace-Period — Neues Config-Feld `shutdownGracePeriod` (Duration, Default 30s). Spezialwert 0 = deaktiviert (Binary laeuft unbegrenzt, aktuelles Verhalten). Config hot-reload via bestehendem fsnotify-Watcher.
- **D-06:** Grace-Abort bei neuem SessionStart — SessionStart waehrend Grace-Period setzt den Shutdown-Timer zurueck via context.WithCancel. Binary bleibt am Leben, Refcount wird inkrementiert. Nahtloser Uebergang.
- **D-07:** Discord Activity clearen vor Shutdown — Shutdown-Sequenz: (1) ClearActivity (leere SetActivity), (2) Close Discord IPC, (3) Shutdown HTTP Server (drainiert in-flight Requests), (4) Exit(0). Research bestaetigt: explizites Clear + IPC Close ist sauberer als abrupter Disconnect.

### Stale-Handling (D-08, D-09, D-10)
- **D-08:** Timer bleibt PRIMAER bei 30s — SessionEnd ist zu unzuverlaessig als primaerer Mechanismus. Claude Code Issues (Stand April 2026): #33458 (OPEN, stale — Plugin SessionEnd feuert NIE), #32712 (OPEN — Ctrl+C bricht SessionEnd ab, Regression seit v2.1.72), #35892 (OPEN, stale — /exit feuert SessionEnd nicht), #40010 (OPEN — agent-type Hooks bei SessionEnd ignoriert). Timer-basierte Stale-Detection mit 30s Intervall (staleCheckInterval) bleibt unveraendert.
- **D-09:** SessionEnd als opportunistischer Bonus — Wenn SessionEnd-Hook feuert: HTTP-Handler antwortet sofort mit HTTP 200 (<10ms). Dann in Background-Goroutine: finaler JSONL-Read via analytics.ParseTranscript(transcript_path), dann Registry.Remove(sessionID). Session verschwindet instant aus Discord. Alle SessionEnd-Reasons werden einheitlich behandelt (clear, resume, logout, prompt_input_exit, bypass_permissions_disabled, other). Reason wird via slog.Info geloggt. Wenn SessionEnd NICHT feuert: Timer erkennt Idle nach idleTimeout, entfernt nach removeTimeout. Beide Pfade pruefen Refcount=0 fuer Auto-Exit-Trigger.
- **D-10:** PID-Liveness-Check bleibt als Crash-Detektor — Bestehender IsPidAlive() Check in CheckOnce() bleibt unveraendert.

### Matcher-Strategie (D-11, D-12)
- **D-11:** Wildcard `*` fuer PreToolUse — Faengt ALLE Tools inkl. MCP-Tools, neue Claude Code Tools, NotebookEdit etc. MapHookToActivity() mappt bekannte Tools auf Activity-Kategorien, unbekannte auf Default.
- **D-12:** Wildcard `*` fuer PostToolUse und ALLE neuen Events — Konsistente Strategie: StopFailure(*), PostCompact(*), PostToolUseFailure(*), SubagentStart(*), CwdChanged (kein Matcher-Support). Daemon filtert und mappt intern.

### Hook-Deployment (D-13, D-14) — AKTUALISIERT
- **D-13:** 15 Hook-Events total — Aufgeteilt auf zwei Orte:
  - **Plugin hooks.json (2 Events):** SessionStart (Daemon-Start via start.sh), SessionEnd (Dual Registration fuer Fix-Tag von #33458)
  - **settings.local.json (13 Events, via start.sh Auto-Patch):** PreToolUse(*), PostToolUse(*), PostToolUseFailure(*), UserPromptSubmit, Stop, StopFailure(*), Notification(idle_prompt), SubagentStart(*), SubagentStop(*), PreCompact(*), PostCompact(*), CwdChanged, SessionEnd (Dual Registration — funktioniert hier, Bug #33458 betrifft nur Plugin hooks.json)
  - Alle als HTTP-Type an http://127.0.0.1:19460/hooks/{event-slug}
  - Research-Begruendung: Plugin hooks.json hat multiple Bugs (#33458 SessionEnd, #34573 PreToolUse/PostToolUse Command-Hooks). settings.local.json hat keine bekannten Bugs, hoechste nicht-managed Prioritaet, gitignored, ueberlebt Updates.
- **D-14:** Auto-Patch in start.sh — start.sh patcht settings.local.json idempotent:
  1. Liest bestehende settings.local.json (oder {} als Default)
  2. Fuegt dsrcode HTTP-Hooks hinzu wenn nicht vorhanden (identifizierbar durch URL 127.0.0.1:19460)
  3. Bestehende User-Hooks bleiben erhalten (Merge, nicht Replace)
  4. Atomisches Schreiben (tmp-File + mv)
  5. stop.sh Cleanup: Entfernt alle Hooks mit URL matching "127.0.0.1:19460" aus settings.local.json bei Plugin-Deinstallation

### PreCompact + PostCompact (D-15) — ERWEITERT
- **D-15:** PreCompact fuer Compaction-Baseline + PostCompact fuer Verifizierung — PreCompact-Hook feuert VOR Compaction mit transcript_path und trigger. Daemon liest aktuelle Token-Totals aus JSONL und speichert als Baseline. PostCompact-Hook feuert NACH Compaction mit trigger und compact_summary. Daemon inkrementiert Compaction-Counter, loggt compact_summary, verifiziert Baseline-Addierung. PostCompact hat KEINE pre/post Token-Counts im Payload (Issue #40492 offen). PostCompact verfuegbar seit ~v2.1.89 (installiert: v2.1.98).

### Hook-Fehlerbehandlung (D-16)
- **D-16:** HTTP 200 immer zurueckgeben — Auch bei internen Fehlern gibt der Server HTTP 200 zurueck. Non-2xx Responses erzeugen non-blocking Errors in Claude Code. Fehler werden via slog geloggt.

### JSONL-Entfernungs-Scope (D-17, D-18) — ZEILENNUMMERN AKTUALISIERT
- **D-17:** Entfernen aus main.go (~250 Zeilen): jsonlFallbackWatcher() (Zeile ~472), jsonlPollLoop() (Zeile ~514), ingestJSONLFallback() (Zeile ~530), readStatusLineData() (Zeile ~588), findMostRecentJSONL() (Zeile ~655), parseJSONLSession() (Zeile ~723), readSessionData() (Zeile ~818). Dazu: Tests in main_test.go (TestParseJSONLSession Zeile ~274, TestPathDecoding Zeile ~372, TestFindMostRecentJSONL Zeile ~460).
- **D-18:** Behalten in analytics/: Token-Parsing-Logik (message.usage.input_tokens, output_tokens, cache_read_input_tokens, cache_creation_input_tokens), Compaction-Baseline-Logik, Per-Model-Token-Tracking. Refactored als analytics.ParseTranscript(path) fuer hook-getriggerte Aufrufe.

### Neue Events (D-19, D-20, D-21, D-22, D-23, D-24)
- **D-19:** StopFailure fuer API-Error-Display — StopFailure feuert bei API-Errors (7 Typen: rate_limit, authentication_failed, billing_error, invalid_request, server_error, max_output_tokens, unknown). HTTP-Handler setzt Session-Status auf Error-State. Neues "error" Icon im Discord Developer Portal (8. Activity-Icon). SmallImage wechselt auf "error", SmallText zeigt Error-Typ ("Rate Limited", "API Error", "Max Tokens"). State/Details bleiben unveraendert (Token-Info erhalten). Temporaer: zurueck nach naechstem erfolgreichen PostToolUse/Stop.
- **D-20:** PostToolUseFailure fuer Error-Tracking — Payload: tool_name, error, is_interrupt. Error-Counter in Analytics inkrementieren. Kein Presence-Update (zu granular). Wildcard * Matcher.
- **D-21:** CwdChanged fuer Projekt-Wechsel — Payload: old_cwd, new_cwd. Kein Matcher-Support (feuert immer). Daemon aktualisiert Projekt-Kontext wenn CWD sich aendert. Koennte Unterverzeichnis-Info in Presence updaten.
- **D-22:** SubagentStart fuer frueheres Subagent-Tracking — Payload: agent_id, agent_type. SmallImage wechselt auf "thinking" Icon, SmallText zeigt "Subagent: {agent_type}". SubagentTree.Spawn() wird mit agent_id + agent_type aufgerufen. Presence zeigt sofort dass Subagent laeuft, nicht erst bei erstem Tool-Use.
- **D-23:** PostCompact fuer Compaction-Verifizierung — Payload: trigger (manual/auto), compact_summary. Compaction-Counter inkrementieren, compact_summary loggen, Baseline-Addierung verifizieren. Ergaenzt PreCompact (D-15).
- **D-24:** SessionEnd Timeout-Strategie — Default SessionEnd-Timeout ist 1.5s (konfigurierbar via CLAUDE_CODE_SESSIONEND_HOOKS_TIMEOUT_MS seit v2.1.74). Fuer dsrcode irrelevant: HTTP-Handler antwortet in <10ms, Background-Goroutine hat kein Zeitlimit. Selbst bei Timeout waere es ein non-blocking error (Claude macht weiter, Daemon empfaengt trotzdem den POST). Env-Var wird NICHT automatisch gesetzt (start.sh laeuft als SessionStart-Hook, Claude Code ist bereits gestartet).

### Claude's Discretion
- Exakte Drosselungs-Logik fuer JSONL-Reads (Timer-basiert vs. Event-Counter)
- parseJSONLSession() Refactoring-Details (was in analytics/ wandert vs. neu geschrieben)
- PostToolUse Route Handler Implementation in server.go
- SessionEnd Route Handler Implementation in server.go
- PreCompact/PostCompact Route Handler Implementation in server.go
- StopFailure Route Handler Implementation in server.go
- SubagentStart Route Handler Implementation in server.go
- CwdChanged Route Handler Implementation in server.go
- PostToolUseFailure Route Handler Implementation in server.go
- Auto-Exit Goroutine Koordination (sync.Once, atomic.Bool, oder Channel-basiert)
- Shutdown-Sequenz Timing (wie lange auf HTTP drain warten)
- start.sh Auto-Patch JSON-Manipulation (node -e, python3 -c, oder jq)
- stop.sh Cleanup JSON-Manipulation und URL-Matching-Logik
- settings.local.json Backup-Strategie vor Patch
- Test-Strategie fuer neue Hooks (welche Szenarien, Mocking-Approach)
- CHANGELOG.md Eintraege fuer v4.0.0
- "error" Icon Design und Upload in Discord Developer Portal
- StopFailure temporaer-zu-normal Transition Timing

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Claude Code Hook System (Research-verified April 2026, v2.1.98)
- Offizielle Hooks-Referenz: https://code.claude.com/docs/en/hooks
- 27 Hook Events in Claude Code v2.1.98 (SessionStart, SessionEnd, PreToolUse, PostToolUse, PostToolUseFailure, PermissionRequest, PermissionDenied, UserPromptSubmit, Notification, Stop, StopFailure, SubagentStart, SubagentStop, TeammateIdle, TaskCreated, TaskCompleted, PreCompact, PostCompact, ConfigChange, CwdChanged, FileChanged, WorktreeCreate, WorktreeRemove, Setup, InstructionsLoaded, Elicitation, ElicitationResult)
- PostToolUse payload: session_id, transcript_path, cwd, permission_mode, hook_event_name, tool_name, tool_input, tool_response, tool_use_id — NO token_usage data
- SessionEnd payload: session_id, transcript_path, cwd, hook_event_name, reason — NO token data. Default timeout 1.5s, konfigurierbar via CLAUDE_CODE_SESSIONEND_HOOKS_TIMEOUT_MS (seit v2.1.74)
- StopFailure payload: session_id, transcript_path, cwd, hook_event_name, error (7 types), error_details, last_assistant_message — fire-and-forget, output ignored
- SubagentStart payload: session_id, transcript_path, cwd, hook_event_name, agent_id, agent_type
- PostCompact payload: session_id, transcript_path, cwd, hook_event_name, trigger, compact_summary — NO pre/post token counts
- CwdChanged payload: session_id, transcript_path, cwd, hook_event_name, old_cwd, new_cwd — NO matcher support
- HTTP hooks: POST body = JSON. Return 200 always. Non-2xx/timeout/connection failure = non-blocking error.
- Async hooks: async:true supported but nutzlos fuer SessionEnd (Process exits)
- Hook locations: Plugin hooks.json, settings.json, settings.local.json, ~/.claude/settings.json, Managed policy. Hooks MERGE across locations, deduplicated by URL/command.

### SessionEnd Reliability Issues (CRITICAL — Stand April 2026)
- `anthropics/claude-code#33458` (OPEN, stale) — Plugin hooks.json SessionEnd NEVER fires. Workaround: settings.local.json funktioniert.
- `anthropics/claude-code#32712` (OPEN) — Ctrl+C cancels SessionEnd hook. Regression seit v2.1.72.
- `anthropics/claude-code#35892` (OPEN, stale) — /exit does not fire SessionEnd/Stop.
- `anthropics/claude-code#40010` (OPEN) — agent-type hooks silently ignored for SessionEnd.
- `anthropics/claude-code#41577` (CLOSED, dup of #24206) — SessionEnd hooks killed before completion. Workaround: nohup/disown.
- `anthropics/claude-code#17885` (CLOSED, stale) — /exit doesn't fire SessionEnd.
- Summary: SessionEnd fires in ~30% of cases. Plugin hooks.json SessionEnd = 0% fire rate. settings.local.json SessionEnd = best option.

### Plugin hooks.json Known Bugs
- `anthropics/claude-code#33458` — SessionEnd not dispatched to plugin hooks
- `anthropics/claude-code#34573` — PreToolUse/PostToolUse command hooks silently dropped from plugin hooks.json (HTTP hooks work)
- `anthropics/claude-code#36456` — Disabled plugin hooks still fire
- `anthropics/claude-code#38714` — No proper plugin uninstall path

### Go Binary (dsrcode) — Code to Modify
- `main.go` — JSONL code to remove: jsonlFallbackWatcher() (line ~472), jsonlPollLoop() (line ~514), ingestJSONLFallback() (line ~530), readStatusLineData() (line ~588), findMostRecentJSONL() (line ~655), parseJSONLSession() (line ~723), readSessionData() (line ~818)
- `main_test.go` — Tests to remove: TestParseJSONLSession (line ~274), TestPathDecoding (line ~372), TestFindMostRecentJSONL (line ~460)
- `server/server.go` — Handler() routes to extend: POST /hooks/session-end, POST /hooks/post-tool-use, POST /hooks/pre-compact, POST /hooks/post-compact, POST /hooks/stop-failure, POST /hooks/subagent-start, POST /hooks/post-tool-use-failure, POST /hooks/cwd-changed
- `session/stale.go` — CheckStaleSessions() and CheckOnce() — keep unchanged
- `session/registry.go` — SessionRegistry for refcount tracking and auto-exit coordination
- `analytics/` — Token parsing logic target (refactored from main.go)
- `config/config.go` — Add shutdownGracePeriod field
- `scripts/start.sh` — Extend Auto-Patch for settings.local.json (13 HTTP hooks)
- `scripts/stop.sh` — Add Cleanup: remove dsrcode hooks from settings.local.json
- `preset/types.go` — Add "error" to AllActivityIcons() (8th icon)

### Prior Phase Context
- `.planning/phases/04-discord-presence-enhanced-analytics-.../04-CONTEXT.md` — D-01 (3-source split, JSONL for tokens), D-13 (transcript_path from hooks), D-14 (auto-patch hooks.json), D-19 (compaction baselines), D-20 (hook vs JSONL responsibility split)

### Discord IPC Behavior
- Discord Rich Presence: ClearActivity + Close IPC = clean exit (activity removed immediately)
- Abrupt disconnect: Discord removes activity after ~15-30s delay
- Up to 300 custom assets uploadable in Discord Developer Portal
- status_display_type field (0=name, 1=state, 2=details) available since July 2025

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `session.SessionRegistry`: Thread-safe Session Management mit RWMutex und onChange Callback — Refcount-Tracking fuer Auto-Exit
- `session.CheckStaleSessions()` / `session.CheckOnce()`: Timer-basierte Stale-Detection — bleibt primaer, unveraendert
- `session.IsPidAlive()`: Cross-platform PID Liveness Check — bleibt als Crash-Detektor
- `server.handleHook()`: Generischer HTTP Hook Handler — erweitern fuer neue Routes
- `server.MapHookToActivity()`: Tool-Name zu Activity-Icon Mapping — erweitern fuer unbekannte Tools und StopFailure
- `analytics.Tracker`: Analytics Coordinator — erweitern fuer hook-getriggerte Reads
- `main.parseJSONLSession()`: Token-Parsing Logik — refactorn nach analytics.ParseTranscript()
- `config.Config.StaleCheckInterval`: Bestehend (30s Default) — beibehalten
- `discord.Client.SetActivity()` / `discord.Client.Close()`: Discord IPC — fuer sauberen Shutdown
- `preset.AllActivityIcons()`: 7 Icons (coding, terminal, searching, thinking, reading, idle, starting) — erweitern um "error"

### Established Patterns
- Immutable Session Updates: Copy-before-modify in Registry — beibehalten
- HTTP Hook Handler: JSON unmarshal + session lookup + activity update — gleiches Pattern fuer neue Hooks
- External test packages: session_test, server_test, analytics_test — neue Tests im gleichen Pattern
- Config Hot-Reload via fsnotify — gleiches Pattern fuer shutdownGracePeriod
- Channel-basierte Goroutine Koordination (Debounce-Channel aus Phase 1) — fuer Auto-Exit Timer

### Integration Points
- Neue server Routes: /hooks/session-end, /hooks/post-tool-use, /hooks/pre-compact, /hooks/post-compact, /hooks/stop-failure, /hooks/subagent-start, /hooks/post-tool-use-failure, /hooks/cwd-changed
- analytics.Tracker.ReadTranscript(path) aufgerufen von PostToolUse, PreCompact und SessionEnd Handlers
- Auto-Exit Goroutine registriert sich auf SessionRegistry.onChange (Refcount=0 Trigger)
- Shutdown-Sequenz koordiniert: Discord → HTTP Server → Process Exit
- start.sh Auto-Patch: settings.local.json fuer 13 HTTP-Hooks
- stop.sh Cleanup: settings.local.json Hooks mit URL 127.0.0.1:19460 entfernen

</code_context>

<specifics>
## Specific Ideas

- JSONL-Entfernung als "Architektur-Vereinfachung": von 3-Quellen-Split (Phase 4 D-01: Hooks/JSONL/Statusline) zu 2-Quellen-Split (Hooks/Statusline) mit gezielten JSONL-Reads statt Background-Polling
- PostToolUse als Token-Trigger: Die sauberste Loesung weil Daten NACH Tool-Ausfuehrung aktuell sind
- Auto-Exit Grace-Period analog zu Kubernetes terminationGracePeriodSeconds: konfigurierbar, abbrechbar, mit Cleanup-Sequenz
- SessionEnd-Bug-Dokumentation in README: User informieren dass SessionEnd unzuverlaessig ist und warum Timer primaer bleibt
- Wildcard-Matcher + Default-Activity: Unbekannte Tools mappen auf "thinking" Icon — neutral und zukunftssicher
- PreCompact Baseline-Snapshot + PostCompact Verifizierung: Closed-Loop Compaction-Tracking
- StopFailure als User-sichtbarer Error-State: "Rate Limited" im Discord-Status hilft Usern zu verstehen warum Claude nicht antwortet
- SubagentStart fuer sofortige Subagent-Anzeige: Presence zeigt Subagent-Typ sofort, nicht erst bei erstem Tool-Use
- Alle Hooks in settings.local.json: Zukunftssicherste Strategie wegen multipler Plugin hooks.json Bugs (#33458, #34573, #36456)
- HTTP 200 + Background-Goroutine fuer SessionEnd: Umgeht 1.5s Timeout komplett, Go-idiomatic

</specifics>

<deferred>
## Deferred Ideas

- **Token-Usage in Hook-Payloads** — Wenn Anthropic token_usage zu PostToolUse/SessionEnd Payloads hinzufuegt (Issue #11008, 17 Upvotes), kann JSONL-Parsing komplett entfernt werden.
- **SessionEnd Fix Monitoring** — anthropics/claude-code#33458 beobachten. Wenn Plugin SessionEnd gefixt wird, koennen Hooks zurueck nach Plugin hooks.json migriert werden.
- **PostCompact Token-Counts** — Issue #40492 fordert pre_compact_token_count und post_compact_token_count im PostCompact Payload. Wuerde Baseline-Verifizierung verbessern.
- **FileChanged als fsnotify-Ersatz** — Evaluiert und verworfen: fsnotify ist zuverlaessiger da es auch ohne Claude Code funktioniert.
- **Setup-Hook** — Evaluiert und verworfen: Feuert nur bei claude --init/--maintenance, nicht bei normalem Session-Start. start.sh (SessionStart) reicht.
- **Agent Teams Events** (TeammateIdle, TaskCreated, TaskCompleted) — Experimentell, erfordert agentTeams:true. Spaetere Phase wenn Agent Teams stabil.
- **PostToolUseFailure Display** — Aktuell nur Counter in Analytics. Spaeter: Error-Rate in Presence anzeigen wenn gewuenscht.

</deferred>

---

*Phase: 06-hook-system-overhaul-sessionend-posttooluse-precompact-hooks*
*Context gathered: 2026-04-09 (updated)*
