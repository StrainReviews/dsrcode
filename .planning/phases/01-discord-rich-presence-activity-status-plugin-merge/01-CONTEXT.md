# Phase 1: Discord Rich Presence + Activity Status Plugin Merge - Context

**Gathered:** 2026-04-02
**Status:** Ready for planning

<domain>
## Phase Boundary

Merge cc-discord-presence (Go-based Rich Presence: project, branch, model, tokens, cost, session time) und claude-code-discord-status (Node.js Activity Status: Thinking, Editing, Searching etc.) in ein einziges Go-basiertes Plugin. Fork von tsanva/cc-discord-presence als Basis, Activity-Features integrieren, Presets komplett neu designen.

Repos:
- Basis: `C:\Users\ktown\Projects\cc-discord-presence` (Go, MIT License)
- Features: `C:\Users\ktown\Projects\claude-code-discord-status` (Node.js/TypeScript, MIT License)
- Fork-Ziel: `DSR-Labs/cc-discord-presence` auf GitHub

</domain>

<decisions>
## Implementation Decisions

### Merge-Strategie (D-01 bis D-03)
- **D-01:** Hybrid-Ansatz — Go-Binary als Basis (cc-discord-presence), Activity-Status-Features aus claude-code-discord-status integrieren
- **D-02:** Go-Binary behält bestehende Architektur: fsnotify File-Watching, Statusline + JSONL Fallback, Discord IPC via Named Pipes (Windows) / Unix Sockets
- **D-03:** Kein Node.js Daemon — alles in einer Go-Binary

### Activity Status Quelle (D-04 bis D-07)
- **D-04:** HTTP-Hooks direkt an Go HTTP-Server (Claude Code `"type": "http"` Hooks). Kein Bash-Script, kein File-Intermediär für Activity-Daten
- **D-05:** Go-Binary startet HTTP-Server (Default Port 19460, konfigurierbar) der Hook-Events empfängt
- **D-06:** Hook-Events: PreToolUse (Edit/Write/Bash/Read/Grep/Glob/WebSearch/Task), UserPromptSubmit, Stop, Notification — alle als HTTP-Hooks
- **D-07:** Activity-Mapping:
  - Write/Edit → "coding" icon, "Editing a file"
  - Bash → "terminal" icon, "Running a command"
  - Read → "reading" icon, "Reading a file"
  - Grep/Glob → "searching" icon, "Searching codebase"
  - WebSearch/WebFetch → "searching" icon, "Searching the web"
  - Task → "thinking" icon, "Running a subtask"
  - UserPromptSubmit → "thinking" icon, "Thinking..."
  - Stop → "idle" icon, "Finished"
  - Notification → "idle" icon, "Waiting for input"

### Discord Display Layout — Layout D Maximum Output (D-08 bis D-15)
- **D-08:** Details-Zeile: `"{Activity} · {ProjectName} ({Branch})"` — z.B. "Editing a file · StrainReviewsScanner (main)"
- **D-09:** State-Zeile: `"{Model} | {Tokens} tokens | ${Cost}"` — z.B. "Opus 4.6 | 1.5M tokens | $0.12"
- **D-10:** Large Image: "dsr-code" App-Icon (fix, 1024x1024 im Discord Developer Portal)
- **D-11:** Small Image: Activity-Icon wechselt dynamisch (coding, terminal, searching, thinking, reading, idle, starting)
- **D-12:** Small Text: Activity-Beschreibung als Hover-Tooltip
- **D-13:** Timestamps: Session-Startzeit für elapsed-Timer
- **D-14:** Buttons: Bis zu 2 klickbare Links, preset-abhängig (z.B. hacker: "View Terminal Log", streamer: "Watch Stream"). Nur für andere User sichtbar.
- **D-15:** Status Display Type: Steuert was in der Mitgliederliste kompakt angezeigt wird

### Discord App Setup (D-16)
- **D-16:** App-Name: **"DSR Code"** — eigene Discord Application im Developer Portal mit eigener Application ID

### Plugin-Verteilung (D-17 bis D-20)
- **D-17:** Fork unter DSR-Labs/cc-discord-presence mit eigenen GitHub Releases
- **D-18:** GitHub Actions baut Binaries für 5 Plattformen: macOS arm64/amd64, Linux amd64/arm64, Windows amd64
- **D-19:** Claude Code Plugin-System: `claude plugin marketplace add DSR-Labs/cc-discord-presence`
- **D-20:** start.sh/stop.sh Download von eigenen GitHub Releases

### Output Presets — komplett neu designed (D-21 bis D-25)
- **D-21:** 8 Presets, alle from scratch mit ~50 rotierenden Messages pro Kategorie:
  1. **minimal** — Knapp, nur Fakten (Default)
  2. **professional** — Business, clean, understated
  3. **dev-humor** — Programmierer-Witze, Referenzen
  4. **chaotic** — YOLO, pushing to main, gefährlich
  5. **german** — Deutsche Texte, €-Formatierung, 1.000er-Punkt
  6. **hacker** — Matrix/Terminal-Style, CLI-Ästhetik
  7. **streamer** — Twitch/YouTube-Style, LIVE-Prefix
  8. **weeb** — Anime/Japan-Referenzen, -senpai/-kun Suffixe
- **D-22:** Jeder Preset definiert: singleSessionDetails (per Activity-Icon), singleSessionState, multiSessionMessages (2/3/4/overflow), multiSessionTooltips, buttons (preset-spezifische Links)
- **D-23:** Message-Rotation: Stable-Pick mit 5-Minuten Zeitfenstern (Anti-Flicker, Knuth-Hash)
- **D-24:** Preset-Konfiguration via JSON-Config oder CLI-Flag
- **D-25:** Multi-Session Support: Activity-Counter (edits, commands, searches, reads, thinks), Stats-Zeile ("23 edits · 8 cmds · 1h 15m deep"), dominanter Modus-Detection

### Multi-Session Handling (D-26 bis D-28)
- **D-26:** Komplette Multi-Session-Logik portieren: Session-Registry, Activity-Counter, Stats-Zeile, dominanter Modus, Stale-Check
- **D-27:** 1 Session → direkte Anzeige. 2-4 Sessions → Tier-basierte Messages. 5+ → Overflow mit {n} Platzhalter
- **D-28:** Session-Deduplication by projectPath + pid (unterstützt parallele Instanzen)

### Idle/Stale Detection (D-29 bis D-31)
- **D-29:** Konfigurierbare Timeouts, Defaults: 10min → idle, 30min → remove, 30s PID-Check-Intervall
- **D-30:** PID-Liveness-Check: Windows `tasklist`, Unix `kill -0`
- **D-31:** Timeouts per Config-Datei änderbar (idleTimeout, removeTimeout, staleCheckInterval)

### Buttons (D-32)
- **D-32:** Preset-abhängige Buttons. Jeder Preset definiert eigene Button-Labels und URLs. Max 2 Buttons (Discord-Limit). Nur für andere User sichtbar.

### Config-System (D-33 bis D-35)
- **D-33:** Prioritätskette: CLI-Flags > Env-Vars > JSON-Config > Defaults (Standard Go-Pattern)
- **D-34:** Config-Datei: `~/.claude/discord-presence-config.json`
- **D-35:** Konfigurierbare Werte: preset, port, discordClientId, idleTimeout, removeTimeout, staleCheckInterval, reconnectInterval, buttons

### Statusline-Konflikt (D-36 bis D-38)
- **D-36:** Statusline-Wrapper POSTet an Go HTTP-Server statt in Datei zu schreiben
- **D-37:** Kein File-Intermediär (discord-presence-data.json fällt weg) — Go-Binary empfängt alles über HTTP
- **D-38:** Wrapper leitet stdin durch an nächste Statusline (GSD etc.):
  ```bash
  #!/bin/bash
  read -r json
  echo "$json" | curl -s -X POST -d @- http://127.0.0.1:19460/statusline &
  echo "$json"
  ```

### Startup/Shutdown (D-39 bis D-41)
- **D-39:** Hybrid-Hooks: SessionStart/SessionEnd als Command-Hooks (start.sh/stop.sh), alle Activity-Events als HTTP-Hooks
- **D-40:** start.sh startet Go-Binary im Hintergrund, tracked Sessions per Refcount (Windows) / PID-Files (Unix)
- **D-41:** stop.sh dekrementiert Refcount, stoppt Binary erst wenn 0 Sessions übrig. Matcher: `"startup|clear|compact|resume"`

### Error Handling (D-42 bis D-44)
- **D-42:** Konfigurierbar: Default Exponential Backoff (1s → 2s → 4s → 8s → 16s → 30s max)
- **D-43:** Silent failure bei Activity-Updates wenn Discord nicht verbunden
- **D-44:** Crash-Recovery via start.sh beim nächsten SessionStart

### Icon-Design (D-45 bis D-47)
- **D-45:** AI-generierte Icons via fal.ai MCP (1024x1024px PNG)
- **D-46:** 10 Icons: dsr-code (large), coding, terminal, searching, thinking, reading, idle, starting, multi-session, streaming
- **D-47:** Preset-spezifische Varianten erwägen (z.B. hacker-Style für hacker-Preset). Entscheidung nach Icon-Generierung.

### Modell-Pricing (D-48)
- **D-48:** Statusline liefert `total_cost_usd` fertig berechnet — keine eigene Pricing-Tabelle nötig. Eigene Berechnung nur als JSONL-Fallback. Pricing-Map im Code für Fallback aktualisieren (Opus 4.6: $15/$75, Sonnet 4.6: $3/$15, Haiku 4.5: $1/$5).

### Preset-Wechsel UX (D-49 bis D-50)
- **D-49:** Config Hot-Reload via fsnotify — User editiert Config-Datei, Preset wechselt sofort ohne Neustart
- **D-50:** HTTP-Endpoint `POST /config {"preset":"hacker"}` am bestehenden HTTP-Server für programmatischen Wechsel. Kein eigener CLI-Modus nötig.

### JSONL Fallback (D-51)
- **D-51:** JSONL-Fallback behalten — Zero-Config "Install and forget" Erlebnis. Drei Stufen der Genauigkeit: (1) JSONL only → Projekt/Model/Tokens/Cost, (2) + Statusline HTTP → genauere Daten, (3) + Activity HTTP-Hooks → Echtzeit Activity-Icons

### Logging (D-52 bis D-54)
- **D-52:** Eingebautes Log-File: `~/.claude/discord-presence.log` mit Rotation (5MB max)
- **D-53:** Verbosity-Flags: `-v` Debug (jeder HTTP-Request, jede Discord-Update), `-q` quiet (nur Fehler), Default: Verbindungen + Fehler + Session-Änderungen
- **D-54:** Log-Level auch per Config änderbar (`"logLevel": "debug|info|error"`)

### Security (D-55)
- **D-55:** HTTP-Server Default `127.0.0.1` (localhost only), per Config auf `0.0.0.0` änderbar für WSL/Docker. Kein Auth-Token nötig.

### Discord App Setup Workflow (D-56)
- **D-56:** Discord Application "DSR Code" erstellen als Task im Plan — Schritt-für-Schritt-Anleitung mit Developer Portal, Icon-Upload, Application ID Konfiguration

### Claude's Discretion
- HTTP-Server Implementierung in Go (net/http oder chi/echo)
- Go-Paketstruktur und Modul-Organisation
- Preset-Dateiformat (embedded Go structs vs. externe JSON)
- Exact Exponential Backoff Implementierung
- Windows-spezifische Anpassungen (Named Pipes, PowerShell-Scripts)
- JSONL-Fallback-Pricing Map Pflege
- Log-Rotation Implementierung
- fsnotify Config-Watch Debouncing

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Source Repos (lokal geklont)
- `C:\Users\ktown\Projects\cc-discord-presence\main.go` — Go-Binary Hauptlogik, SessionData struct, Discord IPC, File-Watching
- `C:\Users\ktown\Projects\cc-discord-presence\discord\client.go` — Discord IPC Client, Activity struct (Details, State, SmallImage, SmallText, LargeImage, LargeText, StartTime)
- `C:\Users\ktown\Projects\cc-discord-presence\discord\conn_windows.go` — Windows Named Pipe Verbindung
- `C:\Users\ktown\Projects\cc-discord-presence\scripts\start.sh` — Plugin SessionStart Hook, Binary-Download, Multi-Session-Tracking (Refcount Windows / PID Unix)
- `C:\Users\ktown\Projects\cc-discord-presence\scripts\stop.sh` — SessionEnd Hook, Refcount-Dekrement
- `C:\Users\ktown\Projects\cc-discord-presence\.claude-plugin\plugin.json` — Plugin-Manifest

### Activity-Status Features (zu portieren)
- `C:\Users\ktown\Projects\claude-code-discord-status\src\daemon\sessions.ts` — Session-Registry, Activity-Counter, Stale-Check, PID-Liveness
- `C:\Users\ktown\Projects\claude-code-discord-status\src\daemon\resolver.ts` — Presence-Resolver, stablePick(), Multi-Session-Logik, formatStatsLine(), detectDominantMode()
- `C:\Users\ktown\Projects\claude-code-discord-status\src\daemon\discord.ts` — Reconnect-Logik, Exponential Backoff Pattern
- `C:\Users\ktown\Projects\claude-code-discord-status\src\daemon\server.ts` — HTTP-Server, Zod-Validierung, Session-Endpoints
- `C:\Users\ktown\Projects\claude-code-discord-status\src\presets\types.ts` — MessagePreset Interface (singleSessionDetails, multiSessionMessages etc.)
- `C:\Users\ktown\Projects\claude-code-discord-status\src\presets\*.ts` — Alle 5 Original-Presets (als Inspiration für 8 neue Presets)
- `C:\Users\ktown\Projects\claude-code-discord-status\src\shared\constants.ts` — Icon-Mapping, Timeouts, RECONNECT_INTERVAL
- `C:\Users\ktown\Projects\claude-code-discord-status\src\shared\config.ts` — Config-Loading Pattern (Env > File > Defaults)

### Discord API Dokumentation
- Discord Rich Presence Best Practices: https://docs.discord.com/developers/rich-presence/best-practices — Assets 1024x1024px, Small Image per-player, tooltips
- Discord Activity Object: https://docs.discord.com/developers/events/gateway-events#activity-object — Activity Structure, Buttons (max 2, 1-32 chars label), Assets, Timestamps, Status Display Types
- Discord setActivity Fields: https://docs.discord.com/developers/rich-presence/using-with-the-embedded-app-sdk#setactivity-fields — details, state, assets, timestamps, party, buttons

### Claude Code Hooks
- HTTP Hook Type: `"type": "http"` mit `"url"` Feld — eliminiert Bash-Scripts für Activity-Events
- Command Hook: `"type": "command"` — nötig für SessionStart/SessionEnd (Prozess starten/stoppen)
- Known Bug: Hooks ohne `matcher` Feld → "hook error" (anthropics/claude-plugins-official#1007)
- Hook Performance: HTTP-Hooks 112x schneller als Command-Hooks (anthropics/claude-code#39391)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets (cc-discord-presence)
- `discord.Activity` struct: Details, State, LargeImage, LargeText, SmallImage, SmallText, StartTime — alle Felder bereits vorhanden, SmallImage/SmallText noch ungenutzt
- `fsnotify.Watcher`: File-Watching implementiert — wird für JSONL-Fallback beibehalten, Statusline geht über HTTP
- `readSessionData()`: Priority-basierte Datenquelle (Statusline → JSONL)
- `calculateCost()` / `formatModelName()` / `formatNumber()`: Token/Cost-Formatierung für JSONL-Fallback
- `getGitBranch()`: Git-Branch-Erkennung via exec
- Cross-Platform Discord IPC: Windows Named Pipes (`go-winio`) + Unix Sockets
- GitHub Actions Release Pipeline: 5-Plattform Cross-Compile

### Reusable Patterns (claude-code-discord-status — zu portieren nach Go)
- `SessionRegistry`: In-Memory Session Map mit Lifecycle (start → activity → idle → remove)
- `resolvePresence()`: Single vs Multi-Session Resolver mit Tier-basierter Message-Auswahl
- `stablePick()`: Anti-Flicker Message-Rotation (5-min Buckets + Knuth Hash `2654435761`)
- `ActivityCounts`: Counter pro Activity-Typ (edits, commands, searches, reads, thinks)
- `formatStatsLine()`: "23 edits · 8 cmds · 2 searches · 1h 15m deep" — Singular/Plural, nur Non-Zero
- `detectDominantMode()`: >50% einer Activity → Icon, sonst "mixed"/"multi-session"
- `getMostRecentSession()`: Prefer active over idle, then most-recent lastActivityAt
- `checkStaleSessions()`: PID-Liveness + idle/remove Timeouts
- Config-Loading: Env > File > Defaults Pattern

### Integration Points
- Plugin-Manifest: `.claude-plugin/plugin.json` — SessionStart/SessionEnd als Command-Hooks, Activity als HTTP-Hooks
- Statusline-Wrapper: `~/.claude/statusline-wrapper.sh` — POSTet an Go HTTP-Server, leitet durch an GSD
- Discord Developer Portal: Neue "DSR Code" Application, Asset-Upload (10 Icons 1024x1024)
- fal.ai MCP: Icon-Generierung (konfiguriert in `.mcp.json`)

</code_context>

<specifics>
## Specific Ideas

- Layout D maximiert sichtbare Informationen: Activity + Projekt in Details, Model/Tokens/Cost in State, Small Icon = Activity
- HTTP-Hooks eliminieren alle Bash/Windows-Kompatibilitätsprobleme die wir debuggt haben
- 8 Presets mit je ~50 Messages pro Kategorie für hohe Varianz
- German-Preset mit korrekter €-Formatierung und 1.000er-Punkten
- Hacker-Preset mit Terminal/CLI-Ästhetik (`[root@claude] vim`)
- Streamer-Preset mit LIVE-Prefix und Kamera-Icon
- Weeb-Preset mit -senpai/-kun Suffixe
- Preset-spezifische Buttons (z.B. hacker: "View Terminal Log")
- Statusline-Wrapper als HTTP-POST statt File eliminiert Statusline-Konflikt elegant
- Hybrid-Hooks: nur 2 Command-Hooks (Start/Stop), Rest HTTP — minimale Bash-Exposition
- Exponential Backoff statt fixem Reconnect-Intervall
- Icons per fal.ai generieren, preset-spezifische Varianten möglich

</specifics>

<deferred>
## Deferred Ideas

- PR upstream an tsanva/cc-discord-presence — erst nach stabilem Fork
- Custom Discord App Icons via eigener Design-Iteration nach v1.0
- Twitch/YouTube Stream-Integration (Streaming Activity Type)
- Voice-Channel Rich Presence
- Neues Preset "corporate" für Enterprise-Umgebungen
- Auto-Update-Checker im Go-Binary (npm-Registry Pattern portieren)
- Web-Dashboard für Session-Statistiken
- Preset-Editor CLI/TUI

</deferred>

---

*Phase: 01-discord-rich-presence-activity-status-plugin-merge*
*Context gathered: 2026-04-02*
