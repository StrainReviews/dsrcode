# Phase 3: Fix Discord Presence session count and enhance demo mode - Context

**Gathered:** 2026-04-06
**Status:** Ready for planning

<domain>
## Phase Boundary

Fix duplicate session registration bug (synthetic jsonl- session + real UUID/http- session for same project = "Aktiv in 2 Repos" when only 1 session open), deduplicate sessions per project in the registry, enhance /dsrcode:demo with full Discord Activity control, multi-session preview, message rotation preview, and menu-based demo flow.

Repo: `C:\Users\ktown\Projects\cc-discord-presence` (Go, MIT License)
GitHub: `DSR-Labs/cc-discord-presence`

</domain>

<decisions>
## Implementation Decisions

### Session Dedup Strategie (D-01 bis D-05)
- **D-01:** Project+PID Dedup Key — 2 echte Claude Code Fenster auf gleichem Projekt zaehlen separat (verschiedene PIDs). Nur synthetische Duplikate (jsonl- neben http-/UUID) werden gemerged. Kein Project-Singleton.
- **D-02:** Registry-Level Enforcement — SessionRegistry.StartSession() ist der einzige Enforcement-Point. Alle Session-Quellen (JSONL, HTTP, UUID) gehen durch die Registry. Keine bidirektionale oder Middleware-Loesung.
- **D-03:** Source-Enum (iota) — `type SessionSource int` mit `const (SourceJSONL SessionSource = iota; SourceHTTP; SourceClaude)` plus `String()` Methode. Idiomatic Go (wie reflect.Kind), type-safe, erscheint in /status JSON als String (custom MarshalJSON).
- **D-04:** Replace in-place beim Upgrade — SessionID, PID, Source werden ersetzt. StartTime, ActivityCounts, Model, Tokens, Cost bleiben erhalten (addiert). Kein Discord-Flicker, kein Datenverlust. Nutzt bestehendes copy-before-modify Pattern (registry.go:83).
- **D-05:** Bestehender JSONL-Dedup-Check in main.go:489-495 wird ENTFERNT — Registry-Level ist der einzige Enforcement-Point. Single source of truth, keine redundanten Checks.

### Demo Mode Erweiterungen (D-06 bis D-12)
- **D-06:** Maximum Scope — PreviewPayload erweitert um alle Discord Activity Felder + fakeSessions[] fuer Multi-Session-Preview + GET /preview/messages Endpoint fuer Message Rotation Preview. Auto-Cycle ist skill-gesteuert (kein Server-Endpoint).
- **D-07:** PreviewPayload: + smallImage, smallText, largeText, buttons[], startTimestamp, sessionCount, fakeSessions[] — Volle Kontrolle ueber Discord-Anzeige im Demo-Modus. fakeSessions[] erlaubt realistische Multi-Session-Screenshots mit echten Projektnamen, Models, Kosten.
- **D-08:** GET /preview/messages — Neuer Endpoint: `GET /preview/messages?preset=X&activity=Y&count=N&detail=Z`. Antwortet mit vollen Activity-Objekten (details, state, smallImage, smallText). Zeigt StablePick-Rotationen fuer verschiedene Zeitfenster.
- **D-09:** Demo Skill menu-basiert — 4 Modi: 1) Quick Preview, 2) Preset Tour (User-Bestaetigung pro Preset, kein Timer), 3) Multi-Session Preview (mit fakeSessions), 4) Message Rotation (GET /preview/messages). Claude fuehrt durch gewaehlten Modus.
- **D-10:** Demo-Daten realistisch — Default: Projekt "my-saas-app", Model "Opus 4.6", Tokens 250K, Cost $2.50, Branch "feature/auth". Sieht authentisch aus fuer README-Screenshots.
- **D-11:** Preview komplett real — Kein "Preview Mode" Indikator. Screenshots fuer README sollen authentisch wirken. User weiss durch Skill-Instruktionen dass Preview aktiv ist.
- **D-12:** Auto-Cycle ist skill-gesteuert — Claude loopt in der Skill-Logik: POST /preview fuer Preset, fragt User "Screenshot gemacht?", naechster Preset. Kein dedizierter Server-Endpoint fuer Cycling.

### Session Lifecycle (D-13 bis D-16)
- **D-13:** Counter-Handling: Addieren — Konzeptuell korrekt: Session laeuft seit JSONL-Erkennung, HTTP fuegt echte Activity-Daten hinzu. Praktisch identisch zu "zuruecksetzen" da JSONL-Counter immer 0 sind (JSONL trackt keine Tool-Nutzung). StartTime + Model/Tokens von JSONL bleiben erhalten.
- **D-14:** Unique Projects fuer Tier-Auswahl — Resolver zaehlt unique ProjectNames (nicht raw session count) fuer Single/Multi-Session-Tier. 2 Fenster SRS = 1 unique project = Single-Session-Anzeige. SRS + cc-dp = 2 unique = Multi-Session. Industrie-Standard (VSCode, JetBrains Discord Presence).
- **D-15:** getMostRecentSession fuer Session-Daten — Bei 2 Fenstern auf gleichem Projekt: aktuellstes Fenster anzeigen (active > idle, dann neueste lastActivityAt). Bestehendes Pattern, Industrie-Standard.
- **D-16:** JSONL Fallback behalten mit Deprecation-Warning — sync.Once: Nur beim ersten JSONL-Ingest einer Session warnen ("JSONL fallback active — configure HTTP hooks for better accuracy"). Danach still. Kein Log-Spam (JSONL pollt alle 3s).

### Infrastruktur (D-17 bis D-25)
- **D-17:** Logging: Info fuer Upgrades, Debug fuer Skips — slog.Info("session upgraded", "from", "jsonl-SRS", "to", uuid). slog.Debug("synthetic session skipped"). Go Best Practice: Info fuer erwartete Lifecycle-Events.
- **D-18:** Version v3.1.0 — SemVer korrekt: Bug-Fix + neue Features (Preview-Endpoints, Source-Enum, GET /messages) = MINOR bump. plugin.json Version = GitHub Release Tag = Binary Version (gekoppelt per Phase 2 D-38).
- **D-19:** CHANGELOG.md im Keep a Changelog Format — Sektionen: ### Fixed (Session Dedup Bug), ### Added (Preview API, Source Enum, GET /messages), ### Changed (Resolver Unique Projects).
- **D-20:** 13 Test-Szenarien — 8 Registry-Dedup-Tests (Upgrade jsonl->http->uuid, No-Downgrade, Same-Project-Different-PID, Source-Enum) + 2 Resolver-Tests (Unique-Project-Tier) + 3 Server-Tests (Preview SmallImage, FakeSessions, GET /messages). Table-driven subtests.
- **D-21:** Keine neuen Config-Flags — Bestehende Config reicht: preset, displayDetail, port, bind, logLevel, timeouts. Source-Enum und Dedup sind intern, Preview ist transient.
- **D-22:** README.md erweitern — Demo-Sektion mit 4 Modi erklaert, Screenshot-Beispiele, POST /preview API-Doku. Marketplace-relevant.
- **D-23:** Worktrees deferred — Phase 2 D-34 respektiert. Worktree-Detection ist ein eigenes Feature fuer eine spaetere Phase. Worktrees verhalten sich wie bisher (eigene Sessions mit cwd-basiertem Namen).
- **D-24:** Source-Enum als String in JSON — `"source": "claude"` / `"http"` / `"jsonl"` im /status Endpoint. Custom MarshalJSON mit String() Methode. Lesbar und selbsterklaerend.
- **D-25:** Externe Test-Packages (black-box) — session_test Package (nicht session) fuer Registry-Dedup-Tests. Bestehendes Pattern aus Phase 1. Testet nur oeffentliche API.

### Claude's Discretion
- Registry map re-keying Strategie beim Replace-in-place
- Concurrent access Pattern waehrend Upgrade (RWMutex Scope)
- FakeSession struct exakte Felder und Defaults
- GET /preview/messages StablePick Zeitfenster-Simulation
- Preview resolver Integration (separate Pfad vs. Registry-Bypass)
- Error Response Format fuer neue Endpoints
- CHANGELOG.md retroaktive Eintraege fuer v3.0.0
- GoReleaser CHANGELOG Extraktion fuer Release Notes
- start.sh VERSION Konstante Update auf v3.1.0

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Go Binary (cc-discord-presence) — Session Dedup
- `C:\Users\ktown\Projects\cc-discord-presence\session\registry.go` — StartSession dedup logic (line 37-68), UpdateActivity counter increment (line 109), immutable copy pattern (line 83)
- `C:\Users\ktown\Projects\cc-discord-presence\session\types.go` — Session struct (line 32-48), ActivityCounts (line 17-28), EmptyActivityCounts (line 26-28)
- `C:\Users\ktown\Projects\cc-discord-presence\main.go` — ingestJSONLFallback (line 478-515), JSONL dedup check to remove (line 489-495), jsonlFallbackWatcher (line 422), synthetic ID generation (line 487)
- `C:\Users\ktown\Projects\cc-discord-presence\server\server.go` — handleHook session_id fallback (line 269-278), HTTP synthetic ID "http-" prefix (line 276)

### Go Binary — Resolver (Unique Projects)
- `C:\Users\ktown\Projects\cc-discord-presence\resolver\resolver.go` — ResolvePresence tier selection (line 26-38), resolveMulti (line 228-288), buildMultiPlaceholderValues with projectCounts (line 146-226), getMostRecentSession (line 290+)

### Go Binary — Preview/Demo
- `C:\Users\ktown\Projects\cc-discord-presence\server\server.go` — PreviewPayload struct (line 167-174), handlePostPreview (line 380-441), PreviewState (line 176-181), onPreview callback (line 432-434)

### Plugin — Demo Skill
- `C:\Users\ktown\Projects\cc-discord-presence\skills\demo\SKILL.md` — Current /dsrcode:demo skill (to be expanded with 4 modes)

### Prior Phase Context
- `.planning/phases/01-discord-rich-presence-activity-status-plugin-merge/01-CONTEXT.md` — D-28 (Session dedup by ProjectPath+PID), D-51 (JSONL Fallback tiers), D-55 (localhost only)
- `.planning/phases/02-dsrcodepresence-setup-wizard/02-CONTEXT.md` — D-31 (session_id fallback "http-{projectName}"), D-32 (JSONL-Dedup stays), D-34 (Worktree deferred), D-38 (coupled versioning)

### Claude Code Hook Issues
- anthropics/claude-code#37339 — session_id missing in UserPromptSubmit, Stop, PostCompact (OPEN)
- anthropics/claude-code#27994 — bare-repo worktree root detection issue (relevant for D-23 deferral)

### Testing Patterns
- `C:\Users\ktown\Projects\cc-discord-presence\session\registry_test.go` — Existing external test package pattern (Phase 1)
- `C:\Users\ktown\Projects\cc-discord-presence\resolver\resolver_test.go` — TestMultiSessionProjectsDuplicate (line 695), TestMultiSessionProjects (line 676)

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `session.SessionRegistry`: Thread-safe session management with RWMutex and onChange callback — extend StartSession with Source-aware dedup
- `resolver.StablePick()`: Anti-flicker message rotation (Knuth hash, 5-min windows) — reuse for GET /preview/messages simulation
- `resolver.buildMultiPlaceholderValues()`: Already groups sessions by ProjectName with counts — extend for unique-project tier
- `server.PreviewPayload`: Existing struct to extend with new Discord Activity fields
- `server.handlePostPreview()`: Existing preview handler with timer cleanup — extend for fakeSessions and full Activity control

### Established Patterns
- Immutable Session Updates: Copy-before-modify in Registry (registry.go:83) — use for replace-in-place upgrade
- Activity Counter Map: SmallImageKey → Counter field mapping (types.go:63-69) — unchanged
- PID Detection: X-Claude-PID header with os.Getppid() fallback (server.go:149-157) — used for real PID in session upgrade
- Config Priority Chain: CLI > Env > JSON > Defaults (config.go) — no new config needed
- External test packages: session_test, resolver_test for black-box testing (Phase 1 pattern)
- Synthetic ID prefixes: "jsonl-" (main.go:487), "http-" (server.go:276) — to be augmented with Source enum

### Integration Points
- Registry.StartSession() — Central entry point for all session creation. Dedup logic lives here.
- resolver.ResolvePresence() — Needs unique-project counting instead of raw session count
- server.handlePostPreview() — Needs extended payload handling + resolver bypass for fakeSessions
- skills/demo/SKILL.md — Complete rewrite for 4-mode menu-based flow
- CHANGELOG.md — New file at repo root
- README.md — Demo section expansion

### Known Bug (Root Cause Confirmed)
- main.go:489-495: JSONL dedup checks for HTTP sessions, but server.go handleHook does NOT check for JSONL sessions
- session/registry.go:37-68: Dedup uses ProjectPath+PID — JSONL has PID=0, HTTP has real PID → registry can't prevent both
- Race condition: If JSONL session starts before first HTTP hook → both exist → "2 Projekte" in Discord

</code_context>

<specifics>
## Specific Ideas

- Session quality hierarchy: SourceClaude (UUID) > SourceHTTP (http-prefix) > SourceJSONL (jsonl-prefix)
- Upgrade chain: jsonl- → http- → UUID, each replace-in-place with data preservation
- Resolver uses `len(uniqueProjects)` not `len(sessions)` for tier — matches VSCode/JetBrains behavior
- fakeSessions for demo: POST /preview with realistic project data (my-saas-app, Opus 4.6, 250K tokens, $2.50)
- JSONL deprecation: sync.Once warning per session, not per ingest (avoids 20 warns/minute)
- Source enum appears in GET /status JSON as human-readable string ("claude", "http", "jsonl")
- CHANGELOG.md follows keepachangelog.com format with Fixed/Added/Changed sections

</specifics>

<deferred>
## Deferred Ideas

- Worktree-Agent Zuordnung zum Parent-Projekt (Phase 2 D-34) — eigene Phase mit Git-Detection
- Per-Session Presets (Phase 2 deferred) — verschiedene Presets fuer verschiedene Projekte
- Activity-abhaengige State-Messages (Phase 2 deferred) — State variiert je nach Activity-Typ
- PR upstream an tsanva/cc-discord-presence — nach stabilem Fork
- Web-Dashboard fuer Session-Statistiken — eigene Phase
- Auto-Update-Checker im Go-Binary (statt in start.sh) — spaetere Iteration
- POST /preview/cycle Server-Endpoint — skill-gesteuert reicht aktuell

</deferred>

---

*Phase: 03-fix-discord-presence-session-count-and-enhance-demo-mode*
*Context gathered: 2026-04-06*
