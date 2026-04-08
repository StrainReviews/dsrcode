# Phase 6: Hook-System Overhaul - Context

**Gathered:** 2026-04-08
**Status:** Ready for planning

<domain>
## Phase Boundary

Vollstaendige Ueberarbeitung des Hook-Systems in cc-discord-presence: (1) Drei neue Hooks — SessionEnd fuer opportunistisches Session-Teardown, PostToolUse fuer Token-Daten via transcript_path nach Tool-Ausfuehrung, PreCompact fuer Compaction-Erkennung ohne JSONL-Polling. (2) JSONL-Polling/Watching/Discovery entfernen — ingestJSONLFallback(), findMostRecentJSONL(), jsonlPollLoop(), fsnotify JSONL-Watcher (~250 Zeilen in main.go). JSONL-Parsing-Logik BEHALTEN in analytics/ fuer hook-getriggerte Reads. (3) Binary Auto-Exit — Dual-Trigger (SessionEnd + Stale-Detection), konfigurierbare Grace-Period, Discord Activity clearen vor Shutdown. (4) Stale-Handling bleibt primaer Timer-basiert (30s) — SessionEnd ist zu unzuverlaessig (Claude Code Bugs #33458, #32712, #17885). (5) Alle Matcher auf Wildcard `*`. (6) hooks.json auf 9 Hook-Events erweitern. (7) Hook-Fehlerbehandlung verbessern.

Repo: `C:\Users\ktown\Projects\cc-discord-presence` (Go, MIT License)
GitHub: `DSR-Labs/cc-discord-presence`

</domain>

<decisions>
## Implementation Decisions

### Token-Pipeline nach JSONL-Entfernung (D-01, D-02, D-03)
- **D-01:** Hook-Triggered JSONL Reads — JSONL-Polling/Watching/Discovery entfernen (~250 Zeilen main.go: ingestJSONLFallback(), findMostRecentJSONL(), jsonlPollLoop(), fsnotify JSONL-Watcher). JSONL-Parsing-Logik BEHALTEN und nach analytics/ verschieben fuer gezielte Reads via transcript_path aus Hook-Events.
- **D-02:** PostToolUse als Token-Read-Trigger — PostToolUse feuert NACH Tool-Ausfuehrung, JSONL ist dann aktuell. Daemon liest transcript_path aus PostToolUse-Payload und parst JSONL fuer Token-Breakdown (input/output/cache). Interne Drosselung: max 1 JSONL-Read pro 10s um File-IO zu begrenzen.
- **D-03:** Claude Code Hooks liefern KEINE token_usage in Payloads — Research bestaetigt: PostToolUse sendet nur session_id, transcript_path, cwd, tool_name, tool_input, tool_result. Token-Breakdown ist NUR via JSONL-Parsing verfuegbar. Deshalb Hook-Triggered Reads statt kompletter JSONL-Entfernung. Phase 4 D-03 Token-Formate bleiben erhalten.

### Binary Auto-Exit (D-04, D-05, D-06, D-07)
- **D-04:** Dual-Trigger Auto-Exit — SessionEnd (instant wenn es feuert, ~30% Zuverlaessigkeit) + Stale-Detection (zuverlaessiger Fallback, 100%). Wenn BEIDE einen Session-Remove ausloesen und Refcount=0 wird, startet Grace-Timer. Robust gegen SessionEnd-Bugs.
- **D-05:** Konfigurierbare Grace-Period — Neues Config-Feld `shutdownGracePeriod` (Duration, Default 30s). Spezialwert 0 = deaktiviert (Binary laeuft unbegrenzt, aktuelles Verhalten). Config hot-reload via bestehendem fsnotify-Watcher.
- **D-06:** Grace-Abort bei neuem SessionStart — SessionStart waehrend Grace-Period setzt den Shutdown-Timer zurueck via context.WithCancel. Binary bleibt am Leben, Refcount wird inkrementiert. Nahtloser Uebergang.
- **D-07:** Discord Activity clearen vor Shutdown — Shutdown-Sequenz: (1) ClearActivity (leere SetActivity), (2) Close Discord IPC, (3) Shutdown HTTP Server (drainiert in-flight Requests), (4) Exit(0). Research bestaetigt: explizites Clear + IPC Close ist sauberer als abrupter Disconnect (Discord entfernt Activity nach ~15-30s bei Disconnect, sofort bei Clear).

### Stale-Handling (D-08, D-09, D-10)
- **D-08:** Timer bleibt PRIMAER bei 30s — SessionEnd ist zu unzuverlaessig als primaerer Mechanismus. Claude Code Issues: #33458 (Plugin SessionEnd feuert NIE), #32712 (Ctrl+C bricht SessionEnd ab), #41577 (SessionEnd Hooks werden vor Abschluss gekillt), #17885 (/exit feuert SessionEnd nicht, Windows komplett kaputt). Timer-basierte Stale-Detection mit 30s Intervall (staleCheckInterval) bleibt unveraendert.
- **D-09:** SessionEnd als opportunistischer Bonus — Wenn SessionEnd-Hook feuert: sofort Registry.Remove(sessionID) nach finalem JSONL-Token-Read. Session verschwindet instant aus Discord. Wenn SessionEnd NICHT feuert (Normalfall bei Plugin-Hooks): Timer erkennt Idle nach idleTimeout (10min), entfernt nach removeTimeout (30min). Beide Pfade pruefen Refcount=0 fuer Auto-Exit-Trigger.
- **D-10:** PID-Liveness-Check bleibt als Crash-Detektor — Bestehender IsPidAlive() Check in CheckOnce() bleibt unveraendert. Erkennt abgestuerzte Claude Code Prozesse (kill -9, Crash) wo weder SessionEnd noch Timer-basierte Idle-Detection schnell genug reagieren.

### Matcher-Strategie (D-11, D-12)
- **D-11:** Wildcard `*` fuer PreToolUse — Faengt ALLE Tools inkl. MCP-Tools, neue Claude Code Tools, NotebookEdit etc. MapHookToActivity() mappt bekannte Tools auf Activity-Kategorien, unbekannte auf Default. Keine Maintenance bei neuen Tools. HTTP-Overhead ~1ms pro Call vernachlaessigbar.
- **D-12:** Wildcard `*` fuer PostToolUse — Jedes Tool-Ende triggert Token-Read via transcript_path. Maximale Token-Daten-Frische. Daemon drosselt intern (D-02: max 1 JSONL-Read pro 10s). Unbekannte Tools werden fuer Tool-Counting mitgezaehlt aber nicht in Activity-Mapping aufgenommen.

### hooks.json Erweiterung (D-13, D-14)
- **D-13:** 9 Hook-Events in hooks.json — SessionStart (bestehend), SessionEnd (NEU, HTTP), PreToolUse (Wildcard), PostToolUse (NEU, Wildcard, HTTP), UserPromptSubmit (bestehend), Stop (bestehend), SubagentStop (bestehend), PreCompact (NEU, HTTP), Notification (bestehend). Alle neuen Hooks als HTTP-Type an http://127.0.0.1:19460/hooks/{event}.
- **D-14:** Auto-Patch erweitern (Phase 4 D-14) — start.sh prueft hooks.json und fuegt fehlende Hook-Sektionen hinzu: SessionEnd, PostToolUse (Wildcard), PreCompact. Bestehende Eintraege bleiben erhalten. Prueft auch PreToolUse-Matcher und aktualisiert auf Wildcard `*` wenn alte Liste gefunden.

### PreCompact Hook (D-15)
- **D-15:** PreCompact fuer Compaction-Baseline — PreCompact-Hook feuert VOR Compaction mit transcript_path und trigger (auto/manual). Daemon liest aktuelle Token-Totals aus JSONL und speichert als Baseline (Phase 4 D-19 Pattern). Nach Compaction sind JSONL-Werte zurueckgesetzt — Baseline-Addierung stellt korrekte Totals wieder her. Ersetzt bisheriges JSONL isCompactSummary Parsing.

### Hook-Fehlerbehandlung (D-16)
- **D-16:** HTTP 200 immer zurueckgeben — Auch bei internen Fehlern (DB-Fehler, Parse-Fehler, Analytics-Fehler) gibt der Server HTTP 200 zurueck um Claude Code nicht zu blockieren. Fehler werden via slog geloggt. Kein Error-Body der Claude Code verwirrt. Research bestaetigt: Non-2xx HTTP-Responses erzeugen non-blocking Errors in Claude Code, aber trotzdem sauberer mit 200 + interner Fehlerbehandlung.

### JSONL-Entfernungs-Scope (D-17, D-18)
- **D-17:** Entfernen aus main.go (~250 Zeilen): ingestJSONLFallback() (Zeilen ~523-648), findMostRecentJSONL() (Zeilen ~649-714), parseJSONLSession() (Zeilen ~715-825), jsonlPollLoop() (Zeilen ~507-522), fsnotify JSONL-Watcher Setup (Zeilen ~466-505), readStatusLineData() JSONL-Session-Creation-Logik (Zeilen ~818-823). Dazu: Tests in main_test.go (TestParseJSONLSession, TestPathDecoding, TestFindMostRecentJSONL).
- **D-18:** Behalten in analytics/: Token-Parsing-Logik (message.usage.input_tokens, output_tokens, cache_read_input_tokens, cache_creation_input_tokens), Compaction-Baseline-Logik (Phase 4 D-19), Per-Model-Token-Tracking (Phase 4 D-25). Refactored als analytics.ParseTranscript(path) fuer hook-getriggerte Aufrufe.

### Claude's Discretion
- Exakte Drosselungs-Logik fuer JSONL-Reads (Timer-basiert vs. Event-Counter)
- parseJSONLSession() Refactoring-Details (was in analytics/ wandert vs. neu geschrieben)
- PostToolUse Route Handler Implementation in server.go
- SessionEnd Route Handler Implementation in server.go
- PreCompact Route Handler Implementation in server.go
- Auto-Exit Goroutine Koordination (sync.Once, atomic.Bool, oder Channel-basiert)
- Shutdown-Sequenz Timing (wie lange auf HTTP drain warten)
- start.sh Auto-Patch Reihenfolge und Fehlermeldungen
- Test-Strategie fuer neue Hooks (welche Szenarien, Mocking-Approach)
- CHANGELOG.md Eintraege fuer die neue Version
- Version Bump (v3.3.0 oder v4.0.0 wegen Breaking Change JSONL-Entfernung)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Claude Code Hook System (Research-verified April 2026)
- All 12 Hook Events documented at claudefa.st: SessionStart, UserPromptSubmit, PreToolUse, PermissionRequest, PostToolUse, PostToolUseFailure, SubagentStart, SubagentStop, Stop, PreCompact, Setup, SessionEnd, Notification
- PostToolUse payload: session_id, transcript_path, cwd, permission_mode, hook_event_name, tool_name, tool_input, tool_result — NO token_usage data
- SessionEnd payload: session_id, reason (clear/logout/prompt_input_exit/other), cwd — NO token data
- PreCompact payload: session_id, transcript_path, trigger (auto/manual), custom_instructions — NO token data
- HTTP hooks: POST body = same JSON as stdin for command hooks. Return 200 always. Non-2xx = non-blocking error.
- Async hooks: `async: true` supported since Jan 2026

### SessionEnd Reliability Issues (CRITICAL)
- `anthropics/claude-code#33458` (open) — Plugin hooks.json SessionEnd NEVER fires. Root cause: process teardown sends ABORT_ERR before hook execution.
- `anthropics/claude-code#32712` (open) — Ctrl+C cancels SessionEnd hook with "Request interrupted by user"
- `anthropics/claude-code#41577` (closed as dup of #24206) — SessionEnd hooks killed before completion. Workaround: nohup/disown for async work.
- `anthropics/claude-code#17885` (closed stale) — /exit doesn't fire SessionEnd. Windows: NO exit method fires SessionEnd.
- Summary: SessionEnd fires in ~30% of cases (Ctrl+D on macOS from settings.json). Plugin hooks.json SessionEnd = 0% fire rate.

### Go Binary (cc-discord-presence) — Code to Modify
- `C:\Users\ktown\Projects\cc-discord-presence\main.go` — JSONL code to remove: ingestJSONLFallback() (lines ~523-648), findMostRecentJSONL() (lines ~649-714), parseJSONLSession() (lines ~715-825), jsonlPollLoop() (lines ~507-522), fsnotify JSONL watcher (lines ~466-505)
- `C:\Users\ktown\Projects\cc-discord-presence\main_test.go` — Tests to remove: TestParseJSONLSession, TestPathDecoding, TestFindMostRecentJSONL
- `C:\Users\ktown\Projects\cc-discord-presence\server\server.go` — Handler() routes to extend: POST /hooks/session-end, POST /hooks/post-tool-use, POST /hooks/pre-compact
- `C:\Users\ktown\Projects\cc-discord-presence\session\stale.go` — CheckStaleSessions() and CheckOnce() — keep unchanged, stale detection remains primary
- `C:\Users\ktown\Projects\cc-discord-presence\session\registry.go` — SessionRegistry for refcount tracking and auto-exit coordination
- `C:\Users\ktown\Projects\cc-discord-presence\analytics\` — Token parsing logic target (refactored from main.go)
- `C:\Users\ktown\Projects\cc-discord-presence\hooks\hooks.json` — Extend from 6 to 9 hook events
- `C:\Users\ktown\Projects\cc-discord-presence\config\config.go` — Add shutdownGracePeriod field

### Prior Phase Context
- `.planning/phases/04-discord-presence-enhanced-analytics-.../04-CONTEXT.md` — D-01 (3-source split, JSONL for tokens), D-13 (transcript_path from hooks), D-14 (auto-patch hooks.json), D-19 (compaction baselines), D-20 (hook vs JSONL responsibility split)
- `.planning/phases/19-separate-dsrcode-.../19-CONTEXT.md` — Deferred Idea: "HTTP-Only Hooks" noting prerequisite that Claude Code would need token_usage in hook payloads (which it doesn't have)

### Discord IPC Behavior
- Discord Rich Presence: ClearActivity + Close IPC = clean exit (activity removed immediately)
- Abrupt disconnect: Discord removes activity after ~15-30s delay
- Source: discord/discord-rpc issues #56, #164, #215

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `session.SessionRegistry`: Thread-safe Session Management mit RWMutex und onChange Callback — Refcount-Tracking fuer Auto-Exit
- `session.CheckStaleSessions()` / `session.CheckOnce()`: Timer-basierte Stale-Detection — bleibt primaer, unveraendert
- `session.IsPidAlive()`: Cross-platform PID Liveness Check — bleibt als Crash-Detektor
- `server.handleHook()`: Generischer HTTP Hook Handler — erweitern fuer neue Routes
- `server.MapHookToActivity()`: Tool-Name zu Activity-Icon Mapping — erweitern fuer unbekannte Tools (Default-Mapping)
- `analytics.Tracker`: Analytics Coordinator aus Phase 4 — erweitern fuer hook-getriggerte Reads
- `main.parseJSONLSession()`: Token-Parsing Logik — refactorn nach analytics.ParseTranscript()
- `config.Config.StaleCheckInterval`: Bestehend (30s Default) — beibehalten
- `discord.Client.SetActivity()` / `discord.Client.Close()`: Discord IPC — fuer sauberen Shutdown

### Established Patterns
- Immutable Session Updates: Copy-before-modify in Registry — beibehalten
- HTTP Hook Handler: JSON unmarshal + session lookup + activity update — gleiches Pattern fuer neue Hooks
- External test packages: session_test, server_test, analytics_test — neue Tests im gleichen Pattern
- Config Hot-Reload via fsnotify — gleiches Pattern fuer shutdownGracePeriod
- Channel-basierte Goroutine Koordination (Debounce-Channel aus Phase 1) — fuer Auto-Exit Timer

### Integration Points
- Neue server Routes: /hooks/session-end, /hooks/post-tool-use, /hooks/pre-compact
- analytics.Tracker.ReadTranscript(path) aufgerufen von PostToolUse und PreCompact Handlers
- Auto-Exit Goroutine registriert sich auf SessionRegistry.onChange (Refcount=0 Trigger)
- Shutdown-Sequenz koordiniert: Discord → HTTP Server → Process Exit

</code_context>

<specifics>
## Specific Ideas

- JSONL-Entfernung als "Architektur-Vereinfachung": von 3-Quellen-Split (Phase 4 D-01: Hooks/JSONL/Statusline) zu 2-Quellen-Split (Hooks/Statusline) mit gezielten JSONL-Reads statt Background-Polling
- PostToolUse als Token-Trigger: Die sauberste Loesung weil Daten NACH Tool-Ausfuehrung aktuell sind
- Auto-Exit Grace-Period analog zu Kubernetes terminationGracePeriodSeconds: konfigurierbar, abbrechbar, mit Cleanup-Sequenz
- SessionEnd-Bug-Dokumentation in README: User informieren dass SessionEnd unzuverlaessig ist und warum Timer primaer bleibt
- Wildcard-Matcher + Default-Activity: Unbekannte Tools mappen auf "thinking" Icon — neutral und zukunftssicher
- PreCompact Baseline-Snapshot: Gleiche Logik wie Phase 4 D-19 replaceTokenUsage, aber getriggert von PreCompact Hook statt JSONL isCompactSummary Parsing

</specifics>

<deferred>
## Deferred Ideas

- **Token-Usage in Hook-Payloads** — Wenn Anthropic token_usage zu PostToolUse/SessionEnd Payloads hinzufuegt, kann JSONL-Parsing komplett entfernt werden. Aktuell keine Plaene seitens Anthropic (Stand April 2026).
- **SessionEnd Fix Monitoring** — anthropics/claude-code#33458 beobachten. Wenn Plugin SessionEnd gefixt wird, kann Stale-Timer-Intervall erhoeht werden (30s → 5min) und SessionEnd wird zum primaeren Mechanismus.
- **PostToolUseFailure Hook** — Fuer Error-Tracking wenn Tools fehlschlagen. Aktuell nicht in Phase 6 Scope.
- **SubagentStart Hook** — Fuer frueheres Subagent-Tracking (aktuell via PreToolUse Agent-Matcher). Niedrige Prioritaet.
- **Setup Hook** — Fuer Plugin-Installation und Maintenance. Eigene Mini-Phase.
- **StatusLine-Only Token-Tracking** — Falls JSONL-Parsing-Maintenance zu aufwaendig wird, Fallback auf StatusLine totalTokens (verliert Breakdown).

None — discussion stayed within phase scope

</deferred>

---

*Phase: 06-hook-system-overhaul-sessionend-posttooluse-precompact-hooks*
*Context gathered: 2026-04-08*
