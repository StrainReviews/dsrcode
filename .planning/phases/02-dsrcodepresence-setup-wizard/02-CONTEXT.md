# Phase 2: DSRCodePresence Setup Wizard - Context

**Gathered:** 2026-04-03
**Status:** Ready for planning

<domain>
## Phase Boundary

Claude Code Plugin mit Slash-Commands (/dsrcode:*) fuer das DSR Code Presence Tool (Phase 1 Go Binary). Interaktiver Setup-Wizard via Claude-as-Wizard Pattern, Hook-Migration von command zu HTTP, erweiterte Activity-Daten (Dateinamen, Commands, Queries), displayDetail-System (minimal/standard/verbose/private), Preset-Erweiterung auf 40 Messages mit neuen Platzhaltern, und 7 Slash-Commands fuer Setup, Konfiguration, Diagnostik und Demo.

Repo: `C:\Users\ktown\Projects\cc-discord-presence` (Go, MIT License)
GitHub: `DSR-Labs/cc-discord-presence` (Fork unter StrainReviews/DSRCodePresence)

</domain>

<decisions>
## Implementation Decisions

### Architektur (D-01 bis D-04)
- **D-01:** Claude-as-Wizard Pattern — Slash-Commands (.md Dateien) instruieren Claude, den User durch Setup/Konfiguration zu fuehren. Kein Go TUI (charmbracelet/huh), kein MCP Server, kein Stack-Wechsel zu TypeScript.
- **D-02:** Go Daemon bleibt unveraendert in seiner Kernarchitektur — Discord IPC, Multi-Session Registry, Config Hot-Reload, JSONL-Fallback. Nur neue HTTP Endpoints und erweitertes Payload-Parsing.
- **D-03:** Kein MCP Server — Discord IPC ist Singleton (eine Verbindung pro Application ID), MCP Server stirbt mit Session (kein persistenter Status), Node.js discord-rpc Oekosystem ist abandoned.
- **D-04:** Plugin-Struktur nutzt Claude Code Plugin-System: commands/ fuer Slash-Commands, hooks/ fuer Activity-Tracking, scripts/ fuer Daemon Lifecycle, .claude-plugin/plugin.json fuer Manifest.

### Plugin-Struktur (D-05)
- **D-05:** Verzeichnisstruktur:
  ```
  DSRCodePresence/
  ├── .claude-plugin/
  │   └── plugin.json          # Manifest (kein userConfig)
  ├── commands/
  │   ├── dsrcode:setup.md     # Gefuehrte Ersteinrichtung
  │   ├── dsrcode:preset.md    # Preset wechseln
  │   ├── dsrcode:status.md    # Daemon + Discord Status
  │   ├── dsrcode:log.md       # Log-Viewer
  │   ├── dsrcode:doctor.md    # Diagnostik (7 Kategorien)
  │   ├── dsrcode:update.md    # Binary aktualisieren
  │   └── dsrcode:demo.md      # Preview-Modus fuer Screenshots
  ├── hooks/
  │   └── hooks.json           # HTTP hooks (migriert von command)
  ├── scripts/
  │   ├── start.sh             # SessionStart (Daemon starten)
  │   └── stop.sh              # SessionEnd (Daemon stoppen)
  └── README.md
  ```

### 7 Slash-Commands (D-06 bis D-12)
- **D-06:** `/dsrcode:setup` — Gefuehrter Setup-Wizard in 7 Phasen: A) Voraussetzungen (Binary, Discord, Port, Daemon), B) Discord-Verbindungstest, C) Basis-Konfiguration (Preset-Auswahl, Discord App ID), D) Erweiterte Einstellungen optional (Port, Bind, Log-Level, Timeouts, Buttons), E) Hook-Diagnose (HTTP hooks + Statusline), F) Config schreiben + Daemon (re)starten, G) Verifikation + Zusammenfassung
- **D-07:** `/dsrcode:preset` — Quick-Switch: Zeigt aktuellen Preset + alle 8, User waehlt, sofort gewechselt via POST /config (Hot-Reload)
- **D-08:** `/dsrcode:status` — Daemon-Status, Discord-Verbindung, alle Sessions (eigene markiert via cwd+PID Match), aktiver Preset, Uptime, Hook-Stats
- **D-09:** `/dsrcode:log` — Letzte Log-Eintraege aus ~/.claude/discord-presence.log
- **D-10:** `/dsrcode:doctor` — 7-Kategorien-Diagnostik nach flutter doctor Vorbild: 1) Binary & Daemon, 2) Discord Desktop, 3) Hooks & Activity Tracking, 4) Statusline & Token/Cost, 5) Config, 6) Sessions, 7) Plattform. Status-Icons (OK/Warning/Error), konkrete Fix-Vorschlaege
- **D-11:** `/dsrcode:update` — Binary auf neueste Version aktualisieren (GitHub Release Download)
- **D-12:** `/dsrcode:demo` — Preview-Modus: Preset + displayDetail waehlen, Demo-Daten an POST /preview, User macht Screenshot. Fuer README und Marketplace Screenshots

### Hook-Migration (D-13 bis D-17)
- **D-13:** Activity hooks migrieren von command (bash hook-forward.sh → curl) zu native HTTP type in hooks.json. hook-forward.sh wird komplett geloescht.
- **D-14:** 4 HTTP hooks in hooks.json: PreToolUse, UserPromptSubmit, Stop, Notification (NEU). Alle mit async und 1s Timeout.
- **D-15:** SessionStart/SessionEnd bleiben command hooks in plugin.json (Daemon Lifecycle — Shell-Prozess starten/stoppen).
- **D-16:** Notification hook mapped auf "idle" icon + "Waiting for input" — loest das "No activity" Problem wenn Claude auf User-Eingabe wartet.
- **D-17:** PostToolUse bewusst NICHT — wird sofort vom naechsten PreToolUse ueberschrieben, bringt nichts.

### Erweiterter Hook-Payload (D-18 bis D-20)
- **D-18:** HookPayload struct erweitern: + tool_input (json.RawMessage), hook_event_name, transcript_path, permission_mode, user_prompt (nur intern), reason (Stop), message (Notification)
- **D-19:** tool_input per Tool-Typ parsen: Edit/Read/Write → file_path, Bash → command, Grep/Glob → pattern, Task/Agent → prompt (nur intern)
- **D-20:** Session struct erweitern: + LastFile (Dateiname), LastFilePath (relativer Pfad), LastCommand (Bash-Befehl gekuerzt), LastQuery (Grep/Glob-Pattern)

### displayDetail System (D-21 bis D-22)
- **D-21:** 4 Display-Levels orthogonal zu Presets — Presets bestimmen den TON, displayDetail bestimmt die DATENMENGE:
  - `minimal` (Default): {file} → Projektname, {command} → "...", {query} → "*"
  - `standard`: {file} → Dateiname, {command} → gekuerzt 20 Zeichen, {query} → Pattern
  - `verbose`: {file} → voller relativer Pfad, {command} → voll, {project} → voller Name
  - `private`: {file} → "file", {project} → "Project", {branch} → "", {tokens} → "", {cost} → ""
- **D-22:** Config-Option: `"displayDetail": "minimal"` in discord-presence-config.json. Ein Enum statt 8 Boolean-Flags.

### Preset-Erweiterung (D-23 bis D-26)
- **D-23:** Alle 8 Presets von 20 auf 40 Messages pro Activity-Typ erweitern. Alte {project}-only Messages durch neue Platzhalter ersetzen. Keine Ueberbleibsel vom alten System.
- **D-24:** Neue Platzhalter: {file}, {filepath}, {command}, {query}, {activity}, {sessions}. Der Resolver mappt die WERTE je nach displayDetail (kein Fallback-Syntax, kein Conditional).
- **D-25:** Doppelte Variation: 40 Messages rotieren (StablePick) + wechselnde Dateinamen/Commands = Hunderte einzigartige Kombinationen bei standard/verbose.
- **D-26:** Multi-Session Platzhalter: {projects} (kommaseparierte Namen), {models} (genutzte Models), {totalCost}, {totalTokens}. Resolver: gleiches Projekt → "2x SRS" statt "SRS, SRS".

### Go Binary Endpoints (D-27 bis D-29)
- **D-27:** `GET /presets` — Alle Presets mit Name, Description, Beispiel-Messages. Response inkl. aktuellem Preset und displayDetail.
- **D-28:** `GET /status` — Detaillierter Status: Version, Uptime, Discord-Verbindung, Config, alle Sessions (mit LastFile/LastCommand), Hook-Statistiken (total, byType, lastReceivedAt).
- **D-29:** `POST /preview` — Setzt Discord Presence auf exakten Text fuer Screenshot/Demo. Duration-Parameter (Default 60s), danach zurueck zum Normalmodus.

### Session-Identifikation & Multi-Session (D-30 bis D-34)
- **D-30:** Self-triggering Hook fuer Session-Identifikation: /dsrcode:status loest Bash-Aufruf aus → PreToolUse Hook feuert → Server kennt session_id fuer cwd → Claude matched via "neueste lastActivityAt fuer cwd".
- **D-31:** session_id Fallback im Server: Wenn session_id leer (anthropics/claude-code#37339) → slog.Warn + synthetische ID "http-{projectName}" statt HTTP 400 Rejection. Behebt den silent-rejection Bug.
- **D-32:** JSONL-Dedup bleibt wie implementiert: JSONL-Session wird uebersprungen wenn Hook-Session fuer gleiches Projekt existiert (main.go Zeile 423-428).
- **D-33:** Preset-Wechsel ist global (eine Config-Datei, Hot-Reload). Per-Session Presets sind deferred.
- **D-34:** Worktree Agents: Separate Sessions (eigenes cwd). Worktree-Zuordnung zum Parent-Projekt ist deferred.

### Auto-Installation & First-Run (D-35 bis D-37)
- **D-35:** Nach `claude plugin install` sind alle Hooks automatisch aktiv. start.sh downloaded Binary bei erstem SessionStart. Kein manueller Schritt noetig.
- **D-36:** First-Run Hint: start.sh prueft Config-Existenz. Keine Config → einzeilige Ausgabe: "DSR Code gestartet (Preset: minimal) — /dsrcode:setup fuer Anpassungen". Kein Sentinel-File.
- **D-37:** Kein userConfig in plugin.json — alles ueber /dsrcode:setup. Weniger Reibung bei Installation.

### Versionierung & Distribution (D-38 bis D-39)
- **D-38:** Gekoppelte Versionierung: plugin.json Version = GitHub Release Tag = Binary Version (z.B. v3.0.0).
- **D-39:** Binary Auto-Update: start.sh vergleicht lokale Binary-Version mit VERSION Konstante in Script. Bei Mismatch → neues Binary downloaden. Plugin-Update via `claude plugin update` aktualisiert Script → Script downloaded passende Binary.

### README & Marketplace (D-40 bis D-42)
- **D-40:** README in Englisch — Plugin ist international nutzbar, nicht an SRS-Projekt gebunden.
- **D-41:** Echte Discord Screenshots fuer Marketplace — generiert via /dsrcode:demo (POST /preview Endpoint).
- **D-42:** README Inhalt: Features, Installation (1 Zeile), Quick Start, Screenshots pro Preset, Preset-Uebersicht, Troubleshooting, Konfiguration.

### Claude's Discretion
- Exakte Message-Texte fuer die 40 Messages pro Activity pro Preset
- Go-Paketstruktur fuer erweiterten Payload-Parser
- Exakte SQL/Formatierung der /dsrcode:doctor Checks
- Slash-Command .md Datei-Inhalte (Claude-Instruktionen)
- POST /preview Timeout-Mechanismus (Timer vs. naechster Hook)
- start.sh Version-Check Implementierung (--version Flag vs. eingebettete Version)

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Go Binary (cc-discord-presence)
- `C:\Users\ktown\Projects\cc-discord-presence\server\server.go` — HTTP Handler, HookPayload struct (Zeile 38-43: zu erweitern), handleHook (Zeile 116-170: session_id Fallback + tool_input Parsing), ActivityMapping, alle Endpoints
- `C:\Users\ktown\Projects\cc-discord-presence\session\types.go` — Session struct (Zeile 32-48: + LastFile/LastFilePath/LastCommand/LastQuery), ActivityRequest (Zeile 52-59: + ToolInput), ActivityCounts, CounterMap
- `C:\Users\ktown\Projects\cc-discord-presence\session\registry.go` — SessionRegistry (StartSession Dedup Zeile 44, UpdateActivity Zeile 73-106, TransitionToIdle)
- `C:\Users\ktown\Projects\cc-discord-presence\resolver\resolver.go` — resolveSingle (Zeile 38-79: Platzhalter-Erweiterung), resolveMulti (Zeile 83-153: neue Multi-Session Platzhalter), replacePlaceholders, StablePick
- `C:\Users\ktown\Projects\cc-discord-presence\preset\types.go` — MessagePreset struct (Zeile 6-36: Platzhalter-Doku erweitern), Button struct
- `C:\Users\ktown\Projects\cc-discord-presence\preset\presets\hacker.json` — Referenz-Preset fuer Message-Erweiterung (20→40 Messages, {file}/{command}/{query} Platzhalter)
- `C:\Users\ktown\Projects\cc-discord-presence\config\config.go` — Config struct (Zeile 15-27: + DisplayDetail Feld)
- `C:\Users\ktown\Projects\cc-discord-presence\main.go` — JSONL-Fallback (Zeile 353-685: Dedup-Logik Zeile 423-428), Version Konstante, ClientID

### Plugin-System (Claude Code)
- `C:\Users\ktown\Projects\cc-discord-presence\.claude-plugin\plugin.json` — Aktuelles Manifest (SessionStart/SessionEnd command hooks)
- `C:\Users\ktown\Projects\cc-discord-presence\hooks\hooks.json` — Aktuelle command hooks (zu migrieren auf HTTP type)
- `C:\Users\ktown\Projects\cc-discord-presence\scripts\start.sh` — Daemon-Start, Binary-Download, Session-Tracking, First-Run Hint
- `C:\Users\ktown\Projects\cc-discord-presence\scripts\stop.sh` — Daemon-Stop, Refcount
- `C:\Users\ktown\Projects\cc-discord-presence\scripts\hook-forward.sh` — ZU LOESCHEN (command→HTTP Migration)

### Claude Code Hook-System (bekannte Issues)
- anthropics/claude-code#37339 — session_id fehlt in UserPromptSubmit, Stop, PostCompact (OFFEN)
- anthropics/claude-code#30613 — HTTP hooks ECONNREFUSED bei externen URLs, localhost funktioniert (OFFEN, stale)
- anthropics/claude-code#37559 — Stop hooks + prompt type broken, keine Capability-Matrix (OFFEN)
- anthropics/claude-code#25642 — CLAUDE_SESSION_ID Env-Var Feature Request (OFFEN, 6 Upvotes)

### Hook Input Format (bestätigt via Context7)
- Alle Hooks: session_id, transcript_path, cwd, permission_mode, hook_event_name
- PreToolUse/PostToolUse: + tool_name, tool_input (file_path/command/pattern je nach Tool)
- UserPromptSubmit: + user_prompt
- Stop: + reason
- Notification: + message (vermutlich)
- session_id fehlt in manchen Hook-Typen (Issue #37339)

### Discord API
- Rich Presence: Details/State je 2-128 Zeichen, sichtbar fuer alle Discord-User
- Discord IPC: Eine Verbindung pro Application ID (Singleton)
- Buttons: Max 2, nur fuer andere User sichtbar
- Named Pipes (Windows): `\\.\pipe\discord-ipc-0` bis `-9`
- Unix Sockets: `/tmp/discord-ipc-0` oder `$XDG_RUNTIME_DIR/discord-ipc-0`

### Phase 1 Context
- `.planning/phases/01-discord-rich-presence-activity-status-plugin-merge/01-CONTEXT.md` — Alle 56 Decisions (D-01 bis D-56), Merge-Strategie, Activity-Mapping, Presets, Multi-Session, Config-System

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `server.ActivityMapping`: Tool→Icon/Text Mapping (server.go:25-35) — erweitern fuer neue Hooks
- `resolver.replacePlaceholders()`: Generischer Template-Renderer (resolver.go:243-249) — erweitern fuer neue Platzhalter
- `resolver.StablePick()`: Anti-Flicker Rotation (Knuth-Hash, 5-Min-Fenster) — unveraendert beibehalten
- `session.SessionRegistry`: Thread-safe Session Management mit onChange Callback — erweitern fuer neue Session-Felder
- `config.Config`: JSON Config mit Hot-Reload via fsnotify — + DisplayDetail Feld
- `main.ingestJSONLFallback()`: JSONL-Dedup Logik (main.go:423-428) — unveraendert beibehalten

### Established Patterns
- Immutable Session Updates: Copy-before-modify Pattern in Registry (registry.go:83)
- Activity Counter Map: SmallImageKey → Counter-Feld Mapping (types.go:63-69)
- Hook→Activity Mapping: Switch-Case in MapHookToActivity (server.go:82-98)
- Config Priority Chain: CLI > Env > JSON > Defaults (config.go)
- PID Detection: X-Claude-PID Header mit os.Getppid() Fallback (server.go:149-157)

### Integration Points
- Plugin Manifest → Claude Code Plugin-System (auto-discovery von commands/, hooks/)
- HTTP Hooks → Go Server POST /hooks/{type} (localhost:19460)
- Config Hot-Reload → fsnotify watcher auf Config-Datei
- JSONL Fallback → fsnotify watcher auf ~/.claude/ + Polling
- Discord IPC → Named Pipes (Windows) / Unix Sockets

### Bekannte Bugs/Limitierungen
- server.go:126-129: session_id="" → HTTP 400 OHNE Log-Eintrag (silent rejection)
- session_id fehlt in manchen Claude Code Hook-Typen (anthropics/claude-code#37339)
- HTTP hooks ECONNREFUSED bei externen URLs (anthropics/claude-code#30613) — localhost OK
- CLAUDE_SESSION_ID Env-Var existiert nicht (5 offene Feature Requests)

</code_context>

<specifics>
## Specific Ideas

- "No activity" Bug als Motivator: /dsrcode:doctor haette sofort gezeigt dass Hooks nicht feuern und Session JSONL-Fallback nutzt
- flutter doctor als Design-Referenz fuer /dsrcode:doctor (Kategorien, Status-Icons, konkrete Fixes)
- Self-triggering Hook: /dsrcode:status loest selbst einen Hook aus der die eigene Session identifiziert
- POST /preview Endpoint fuer Screenshot-Generierung: Preset + displayDetail → exakte Discord-Anzeige → User screenshottet
- session_id Fallback "http-{projectName}" statt silent HTTP 400 Rejection
- Resolver mappt Platzhalter-WERTE statt Fallback-Syntax: gleiche Messages, verschiedene Ergebnisse je displayDetail
- Doppelte Variation: Messages rotieren UND Dateinamen/Commands aendern sich = fast nie gleiche Kombination

</specifics>

<deferred>
## Deferred Ideas

- Per-Session Presets (verschiedene Presets fuer verschiedene Projekte) — eigene Phase
- Activity-abhaengige State-Messages (State variiert je nach Activity-Typ) — spaetere Iteration
- Worktree-Agent Zuordnung zum Parent-Projekt — spaetere Iteration
- PR upstream an tsanva/cc-discord-presence — nach stabilem Fork
- Web-Dashboard fuer Session-Statistiken — eigene Phase
- Twitch/YouTube Stream-Integration (Streaming Activity Type) — eigene Phase
- Auto-Update-Checker im Go-Binary (statt in start.sh) — spaetere Iteration
- charmbracelet/huh TUI Wizard als Alternative fuer Non-Claude-Code-Nutzer — spaetere Phase

### Reviewed Todos (not folded)
Keine — todo match-phase 15 ergab 0 Treffer.

</deferred>

---

*Phase: 02-dsrcodepresence-setup-wizard*
*Context gathered: 2026-04-03*
