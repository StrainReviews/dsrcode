# Phase 3: Fix Discord Presence session count and enhance demo mode - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-06
**Phase:** 03-fix-discord-presence-session-count-and-enhance-demo-mode
**Areas discussed:** Session Dedup Strategie, Demo Mode Erweiterungen, Session Lifecycle, Multi-Session Anzeige, Preview API, Infrastruktur
**MCPs used:** sequential-thinking, exa (web search), context7 (Discord API docs, Go stdlib)

---

## Session Dedup Strategie

### Dedup Key

| Option | Description | Selected |
|--------|-------------|----------|
| Project-Singleton | 1 Session pro Projekt, egal wie viele Quellen/Fenster | |
| Project+PID (2 Fenster erlaubt) | Nur synthetische Duplikate gemerged, echte PIDs getrennt | ✓ |
| Hybrid (Singleton + Config Flag) | Default Singleton, Config fuer Multi-Window | |

**User's choice:** Project+PID
**Notes:** Erlaubt 2 echte Claude Code Fenster auf gleichem Projekt

### Dedup Ort

| Option | Description | Selected |
|--------|-------------|----------|
| Registry-Level | StartSession() prueft ProjectName. Einziger Enforcement-Point | ✓ |
| Bidirektional (HTTP + JSONL) | Spiegelbildliche Checks in beiden Handlern | |
| Middleware Layer | Eigener SessionDeduplicator zwischen Quellen und Registry | |

**User's choice:** Registry-Level
**Notes:** Vorher umfangreiche Vor/Nachteile-Analyse mit sequential-thinking und exa recherchiert. Tabelle mit 8 Dimensionen praesentiert. Registry gewaehlt wegen Single Enforcement Point und keine Code-Duplikation.

### Synthetic ID Erkennung

| Option | Description | Selected |
|--------|-------------|----------|
| Source-Enum (iota) | type SessionSource int mit SourceJSONL, SourceHTTP, SourceClaude | ✓ |
| Prefix-Check | strings.HasPrefix(sessionID, "jsonl-") | |
| PID-basiert (PID=0) | ELIMINIERT: http- Sessions haben echte PIDs | |

**User's choice:** Source-Enum
**Notes:** PID-basiert eliminiert nach Codebase-Analyse (http- Sessions haben echte PIDs via X-Claude-PID Header). Source-Enum gewaehlt fuer Type-Safety und Go-Idiomatik (wie reflect.Kind).

### Upgrade-Verhalten

| Option | Description | Selected |
|--------|-------------|----------|
| Replace in-place | SessionID/PID/Source ersetzen, Counter/StartTime behalten | ✓ |
| Remove + Create | Alte Session entfernen, neue erstellen | |
| Merge mit Counter-Transfer | Neue Session, Counter kopieren | |

**User's choice:** Replace in-place
**Notes:** Nutzt bestehendes copy-before-modify Pattern. Kein Discord-Flicker, kein Datenverlust.

### JSONL-Check Entfernung

| Option | Description | Selected |
|--------|-------------|----------|
| Entfernen | Registry-Level ist einziger Enforcement-Point | ✓ |
| Behalten als Fallback | Defense in depth | |
| Behalten + Warn-Log | Monitoring-Signal wenn Registry versagt | |

**User's choice:** Entfernen
**Notes:** Single source of truth. Redundante Checks verbergen ob Registry-Fix greift.

---

## Demo Mode Erweiterungen

### Demo Scope

| Option | Description | Selected |
|--------|-------------|----------|
| Minimal (nur Skill) | Nur skill .md verbessern | |
| Medium (Go + Skill) | + smallImage, smallText, buttons, largeText | |
| Maximum (Demo Framework) | Medium + Multi-Session + Message Rotation | ✓ |

**User's choice:** Maximum
**Notes:** User wollte zunaechst Option 3 erklaert haben (fakeSessions[]). Nach ausfuehrlicher Erklaerung Option 3 gewaehlt.

### Preview API

| Option | Description | Selected |
|--------|-------------|----------|
| Alle 6 Discord-Felder | smallImage, smallText, largeText, buttons, startTimestamp + sessionCount | |
| Nur smallImage + sessionCount | Minimaler Ansatz | |
| Alle + fakeSessions[] | Volle Kontrolle + realistische Multi-Session-Screenshots | ✓ |

**User's choice:** Alle + fakeSessions[]
**Notes:** fakeSessions[] ermoeglicht "Aktiv in 2 Repos: SRS, cc-dp" mit echten Daten statt Platzhaltern.

### Auto-Cycle

| Option | Description | Selected |
|--------|-------------|----------|
| Skill-gesteuert | Claude loopt in Skill-Logik | ✓ |
| Server-Endpoint | POST /preview/cycle mit Preset-Liste | |
| Kein Auto-Cycle | Manuell wie bisher | |

**User's choice:** Skill-gesteuert

### Preset Tour Timing

| Option | Description | Selected |
|--------|-------------|----------|
| 30 Sekunden | Timer-basiert | |
| User-Bestaetigung | Kein Timer, Claude fragt nach Screenshot | ✓ |
| Konfigurierbar | Default 30s, ueber duration steuerbar | |

**User's choice:** User-Bestaetigung

### Demo Modi

| Option | Description | Selected |
|--------|-------------|----------|
| Quick Preview | 1 Preset + Detail + Activity → Screenshot | ✓ |
| Preset Tour | Alle 8 Presets mit User-Bestaetigung | ✓ |
| Multi-Session Preview | fakeSessions konfigurieren | ✓ |
| Message Rotation | GET /preview/messages zeigt Variationen | ✓ |

**User's choice:** Alle 4 Modi
**Notes:** Menu-basierter Skill mit 4 Modi als Hauptmenu.

### Demo-Daten

| Option | Description | Selected |
|--------|-------------|----------|
| Realistische Beispiele | my-saas-app, Opus 4.6, 250K tokens, $2.50 | ✓ |
| DSR-Labs Branding | DSR Code Projekt-Daten | |
| Claude waehlt | Downstream-Planner entscheidet | |

**User's choice:** Realistische Beispiele

---

## Session Lifecycle

### Counter-Handling

| Option | Description | Selected |
|--------|-------------|----------|
| Addieren | JSONL Counter (immer 0) + HTTP Counter | ✓ |
| Zuruecksetzen | Counter auf 0 beim Upgrade | |
| JSONL verwerfen | Nur HTTP Counter zaehlen | |

**User's choice:** Addieren
**Notes:** Alle 3 Optionen produzieren identisches Ergebnis (JSONL Counter sind immer 0). Addieren ist konzeptuell korrekt.

### JSONL Fallback Zukunft

| Option | Description | Selected |
|--------|-------------|----------|
| Behalten | Keine Aenderung | |
| Deprecation-Warning | sync.Once Warn pro Session | ✓ |
| Entfernen | Nur HTTP + Statusline | |

**User's choice:** Deprecation-Warning

---

## Multi-Session Anzeige

### Tier Count

| Option | Description | Selected |
|--------|-------------|----------|
| Unique Projects | 2 Fenster SRS = Single-Session | ✓ |
| Raw Session Count | 2 Fenster SRS = Multi-Session | |
| Hybrid Fenster-Info | Unique Projects + "(2 Fenster)" im State | |

**User's choice:** Unique Projects
**Notes:** Recherche mit exa + context7: VSCode (774K Installs) und JetBrains zeigen nur aktives Projekt. Industrie-Standard.

### Session Pick

| Option | Description | Selected |
|--------|-------------|----------|
| Aktuellstes Fenster | getMostRecentSession (active > idle) | ✓ |
| Aggregiert | Counter summieren, aelteste StartTime | |

**User's choice:** Aktuellstes Fenster

### Worktrees

| Option | Description | Selected |
|--------|-------------|----------|
| Deferred lassen | Phase 2 D-34 respektieren | ✓ |
| Jetzt fixen | Git-Detection einbauen (~40 Zeilen) | |

**User's choice:** Deferred lassen
**Notes:** Ausfuehrliche Vor/Nachteile-Analyse. Edge Cases (bare repos, Windows-Pfade) und Regressions-Risiko (Prettier #18872, Claude Code #27994) sprachen dagegen.

---

## Infrastruktur

### Version

| Option | Description | Selected |
|--------|-------------|----------|
| v3.1.0 mit Changelog | SemVer korrekt + CHANGELOG.md | ✓ |
| v3.0.1 | Bug-Fix Kommunikation | |

**User's choice:** v3.1.0 mit Changelog

### Logging Level

| Option | Description | Selected |
|--------|-------------|----------|
| Info Upgrades, Debug Skips | Erwartete Events = Info | ✓ |
| Alles Info | Maximale Sichtbarkeit | |
| Warn fuer Upgrades | Ungewoehnlich = Warn | |

**User's choice:** Info Upgrades, Debug Skips

### Testing

| Option | Description | Selected |
|--------|-------------|----------|
| 13 Szenarien | 8 Registry + 2 Resolver + 3 Server | ✓ |
| 8 Registry-Tests | Nur kritische Dedup-Szenarien | |
| Claude entscheidet | Freie Hand | |

**User's choice:** 13 Szenarien

### Config

| Option | Description | Selected |
|--------|-------------|----------|
| Keine neuen Flags | Bestehende Config reicht | ✓ |
| previewDefaults Sektion | Default-Preview-Werte | |
| sessionDedup Flag | Backward-compat Escape Hatch | |

**User's choice:** Keine neuen Flags

### Preview Look

| Option | Description | Selected |
|--------|-------------|----------|
| Komplett real | Kein "Preview Mode" Indikator | ✓ |
| Mit Indikator | [PREVIEW] Prefix | |
| Nur im Tooltip | Hover-only Indikator | |

**User's choice:** Komplett real

### README

| Option | Description | Selected |
|--------|-------------|----------|
| Demo-Sektion erweitern | 4 Modi + Screenshots + API Doku | ✓ |
| Nur CHANGELOG | README-Update spaeter | |
| Minimal Bugfix Note | Nur Bug-Fix erwaehnen | |

**User's choice:** Demo-Sektion erweitern

### GET /preview/messages Response

| Option | Description | Selected |
|--------|-------------|----------|
| Volle Activity-Objekte | details, state, smallImage, smallText | ✓ |
| Nur Text | details + state | |
| Mit Zeitfenster-Metadaten | + Timestamp-Range | |

**User's choice:** Volle Activity-Objekte

### Source-Enum JSON Format

| Option | Description | Selected |
|--------|-------------|----------|
| String | "source": "claude" | ✓ |
| Int | "source": 2 | |
| Beides | String + Int | |

**User's choice:** String

### Deprecation Warning Frequenz

| Option | Description | Selected |
|--------|-------------|----------|
| Einmal pro Session (sync.Once) | Kein Log-Spam | ✓ |
| Einmal pro Minute | Regelmaessige Erinnerung | |
| Jedes Mal | Maximale Sichtbarkeit | |

**User's choice:** Einmal pro Session

### Test-Packages

| Option | Description | Selected |
|--------|-------------|----------|
| Extern (black-box) | session_test Package | ✓ |
| Intern (white-box) | session Package | |
| Mix | Je nach Test-Bedarf | |

**User's choice:** Extern (black-box)

### CHANGELOG Format

| Option | Description | Selected |
|--------|-------------|----------|
| Keep a Changelog | Fixed/Added/Changed Sektionen | ✓ |
| Nur GitHub Release Notes | Kein lokales File | |
| Beides | CHANGELOG + Release Notes | |

**User's choice:** Keep a Changelog

---

## Claude's Discretion

- Registry map re-keying, concurrent access, FakeSession struct
- GET /preview/messages StablePick simulation
- Error response format, preview resolver integration
- CHANGELOG retroaktive Eintraege, GoReleaser Extraktion
- start.sh VERSION Konstante Update

## Deferred Ideas

- Worktree-Agent Parent-Projekt-Zuordnung — eigene Phase
- Per-Session Presets — eigene Phase
- POST /preview/cycle Endpoint — skill-gesteuert reicht
