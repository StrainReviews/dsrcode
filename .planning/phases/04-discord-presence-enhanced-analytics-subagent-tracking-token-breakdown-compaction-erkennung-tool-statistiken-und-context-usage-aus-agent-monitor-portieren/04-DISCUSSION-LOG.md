# Phase 4: Discord Presence Enhanced Analytics - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md -- this log preserves the alternatives considered.

**Date:** 2026-04-07
**Phase:** 04-discord-presence-enhanced-analytics
**Areas discussed:** JSONL-Discovery, Hook-Registrierung, Testing-Strategie, cmd/ Struktur, Compaction-Baselines, Hook-Performance, Backward Compatibility, Multi-Session Analytics, GET /status Response, /dsrcode:status Ausgabe, Model-Erkennung, Preset-Message Sprache, Analytics Lifecycle, settings.json Permissions, Preset-Dateiformat, Config-Schema, Graceful Shutdown, German Preset Migration, Translation Workflow, Feature Toggle
**Deep-Scan Agents:** 5 parallel (Token/JSONL, Subagents, Tool-Stats, Full Features, cc-discord-presence)

---

## JSONL-Discovery (D-13)

| Option | Description | Selected |
|--------|-------------|----------|
| transcript_path aus Hooks | Hook-Events liefern Pfad direkt mit | Yes |
| Bestehenden Watcher erweitern | findMostRecentJSONL() um Token-Parsing erweitern | |
| Beides kombiniert | transcript_path bevorzugt, findMostRecentJSONL() als Fallback | |

**User's choice:** transcript_path aus Hooks

---

## Hook-Registrierung (D-14)

| Option | Description | Selected |
|--------|-------------|----------|
| Automatisch beim Start | start.sh patcht hooks.json | Yes |
| Statisch in hooks.json | Direkt aktualisieren | |
| Beides | Statisch + start.sh Reparatur | |

**User's choice:** Automatisch beim Start

---

## Testing-Strategie (D-15)

| Option | Description | Selected |
|--------|-------------|----------|
| External Test Package | analytics_test (Black-Box) | |
| Internal + External | Internal fuer Internals + External fuer API | Yes |
| Du entscheidest | Claude waehlt | |

**User's choice:** Internal + External

---

## Kosten-Tracking Pricing-Quelle (D-16)

| Option | Description | Selected |
|--------|-------------|----------|
| Hardcoded in Go | pricing.go mit 6 Modellen + Wildcard | Yes |
| Config-Datei (JSON) | pricing.json mit Hot-Reload | |

**User's choice:** Hardcoded in Go
**Notes:** User fragte nach Opus 4.6 1M Context. Research: GA seit 13.03.2026, $5/$25 flat, kein Aufpreis.

---

## Kosten-Platzhalter (D-17)

| Option | Description | Selected |
|--------|-------------|----------|
| {cost} + {cost_detail} | Cache-aware + verbose Detail | Yes |
| Nur {cost} verbessern | Nur interne Verbesserung | |

**User's choice:** {cost} + {cost_detail}

---

## Neue Features aus Deep-Scan

| Option | Description | Selected |
|--------|-------------|----------|
| Kosten-Tracking | Cache-aware Pricing Engine | Yes |
| transcript_path speichern | In Session struct fuer spaetere Nutzung | Yes |
| Alles wie geplant | Nur D-01 bis D-12 | |

**User's choice:** Kosten-Tracking + transcript_path speichern

---

## Compaction-Baselines (D-19)

| Option | Description | Selected |
|--------|-------------|----------|
| In Analytics-JSON persistieren | Baseline-Werte im Token-File | Yes |
| Nur In-Memory | Verliert Daten bei Restart | |
| Keine Baselines | Ungenau nach Compaction | |

**User's choice:** In Analytics-JSON persistieren

---

## Hook-Performance (D-20)

| Option | Description | Selected |
|--------|-------------|----------|
| Getrennte Verantwortung | Hooks O(1), JSONL-Watcher async | Yes |
| Async Goroutine im Hook | Fire-and-forget | |
| Sync mit Incremental Cache | Wie Agent Monitor | |

**User's choice:** Getrennte Verantwortung
**Notes:** Research: Go Goroutinen 2KB, fsnotify Write-Event-Detection, bestehender Watcher in main.go

---

## Backward Compatibility (D-21)

| Option | Description | Selected |
|--------|-------------|----------|
| Lazy Init | os.IsNotExist(), erst bei Event schreiben | Yes |
| Leere Dateien beim Start | Daemon erstellt leere JSONs | |
| Migration Script | Separates Script | |

**User's choice:** Lazy Init

---

## Multi-Session Analytics (D-22)

| Option | Description | Selected |
|--------|-------------|----------|
| Per-Session only | Neue Platzhalter nur Single-Session | |
| Selektive Aggregation | totalCost cache-aware, andere von aktivster Session | Yes |
| Volle Aggregation | Alle auch als Multi-Variante | |

**User's choice:** Selektive Aggregation

---

## GET /status Response (D-24)

| Option | Description | Selected |
|--------|-------------|----------|
| Per-Session (Option C) | Jede Session eigene Analytics | |
| Per-Session + Aggregate | C + Top-Level Summen | Yes |
| Nested Top-Level (Option A) | Nur globale Analytics | |

**User's choice:** Per-Session + Aggregate
**Notes:** Detaillierter Pro/Contra mit Previews aller 3 Response-Strukturen

---

## /dsrcode:status Ausgabe (D-23)

| Option | Description | Selected |
|--------|-------------|----------|
| Einrueckung + Status-Icons | Check/Spin, cross-platform | Yes |
| Unicode Box-Drawing | Kann in Terminals brechen | |

**User's choice:** Einrueckung + Status-Icons

---

## Model-Erkennung (D-25)

| Option | Description | Selected |
|--------|-------------|----------|
| Per-Model aus JSONL | msg.model + msg.usage pro Eintrag | Yes |
| Session.Model (einfach) | Ein Preis fuer alles | |

**User's choice:** Per-Model aus JSONL

---

## Preset-Message Sprache (D-27)

| Option | Description | Selected |
|--------|-------------|----------|
| Englisch | Konsistent mit bestehenden | |
| Deutsch | Passend fuer User | |
| Bilingual EN+DE | Alle Presets beide Sprachen | Yes |

**User's choice:** Bilingual EN+DE mit globaler Config. German Preset entfaellt.

---

## Analytics Lifecycle (D-26)

| Option | Description | Selected |
|--------|-------------|----------|
| Nie auto-loeschen | Files bleiben | Yes |
| Nach Daemon-Restart | Frischer Start | |
| Keep N Sessions | Auto-Cleanup | |

**User's choice:** Nie auto-loeschen

---

## Hook-Installation / settings.json (D-28)

| Option | Description | Selected |
|--------|-------------|----------|
| Plugin hooks.json | Standard Marketplace-Plugins | Yes |
| Dual Install | hooks.json + settings.json | |
| Nur settings.json | Wie Agent Monitor | |

**User's choice:** Plugin hooks.json
**Notes:** Cowork-Bug #27398/#40495 betrifft alle Mechanismen. Plugin-Standard korrekt.

---

## Preset-Dateiformat (D-29)

| Option | Description | Selected |
|--------|-------------|----------|
| Top-Level Sprach-Wrapper | messages.en/de Sub-Objekte | Yes |
| Suffix-Keys | singleSessionDetails_de | |
| Separate Dateien | coding.en.json / coding.de.json | |

**User's choice:** Top-Level Sprach-Wrapper

---

## Config-Schema / i18n (D-30)

| Option | Description | Selected |
|--------|-------------|----------|
| ISO 639-1 | Einfaches lang Feld | |
| go-i18n Library | BCP-47, Bundle/Localizer | Yes |
| BCP-47 ohne Library | Manueller Fallback | |

**User's choice:** go-i18n Library (multilingual-ready)

---

## Graceful Shutdown (D-31, D-32)

| Option | Description | Selected |
|--------|-------------|----------|
| Write-on-Event | Sofort auf Disk, crash-proof | Yes |
| Batched + Flush | RAM sammeln, periodisch flushen | |

**User's choice:** Write-on-Event
**Notes:** Performance: ~1ms/write, ~1-10/s, SSD 0.01%. Stabilitaet: crash-proof. Aktualitaet: immer aktuell.

---

## German Preset Migration (D-33)

| Option | Description | Selected |
|--------|-------------|----------|
| Auto-Migration | german -> professional + lang=de | Yes |
| Legacy beibehalten | german.json bleibt | |

**User's choice:** Auto-Migration

---

## Feature Toggle (D-34)

| Option | Description | Selected |
|--------|-------------|----------|
| Feature Map | features: { analytics: true } | Yes |
| Einzelner Boolean | analytics: true | |
| Granulare Toggles | track_tokens etc. | |

**User's choice:** Feature Map (zukunftssicher)

---

## Translation Workflow (D-35)

| Option | Description | Selected |
|--------|-------------|----------|
| Hybrid | go-i18n Code + Preset-JSON kreativ | Yes |
| go-i18n fuer alles | Einheitlich, Kontext-Verlust | |
| Manuell | Kein Tooling | |

**User's choice:** Hybrid
**Notes:** 5 Gruende warum go-i18n fuer Presets falsch: Kontext-Verlust, Array-Index-Kopplung, StablePick-Inkompatibilitaet, kreative vs. funktionale Uebersetzung, Plural-Overhead ohne Nutzen

---

## Claude's Discretion

JSONL Watcher Refactoring-Scope, SubagentEntry struct, Tracker Lifecycle, File-Persist Pfade, Token/Tool Abbreviations, CHANGELOG, Error Handling, Debouncing, go-i18n Message-IDs, Preset DE Qualitaet, Feature Map Defaults, Concurrent Access

## Deferred Ideas

Worktree-Agent Zuordnung, Per-Session Presets, Web-Dashboard, Tool-Flow Transitions, SQLite Persistenz, Workflow Patterns, Agent Co-occurrence, Session Complexity Scoring, Crowdin Integration, CONTRIBUTING.md
