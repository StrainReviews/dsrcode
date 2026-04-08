# Phase 1: Discord Rich Presence + Activity Status Plugin Merge - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-02
**Phase:** 01-discord-rich-presence-activity-status-plugin-merge
**Areas discussed:** Merge-Strategie, Activity Source, Discord Layout, Distribution, Presets, Discord App, Multi-Session, Idle/Stale, Buttons, Config, Statusline-Konflikt, Startup/Shutdown, Error Handling, Icons, Pricing, Preset UX, JSONL Fallback, Logging, Security, Discord App Workflow

---

## 1. Merge-Strategie

| Option | Description | Selected |
|--------|-------------|----------|
| A) Go-only | Activity-Logik komplett in Go | |
| B) Hybrid | Go-Binary + minimale Hooks schreiben Activity-Daten | ✓ |
| C) Node.js beibehalten | Separater Daemon neben Go-Binary | |

**User's choice:** B) Hybrid
**Notes:** Go-Binary hat bereits fsnotify, SmallImage/SmallText ungenutzt — perfekt für Activity

---

## 2. Activity Status Quelle

| Option | Description | Selected |
|--------|-------------|----------|
| A) Hook → JSON-Datei | Bash-Hook schreibt JSON, Go pollt | |
| A+) HTTP-Hook → Go HTTP-Server | Claude Code HTTP-Hooks direkt an Go | ✓ |
| B) Nur Statusline | Aus Statusline-Daten ableiten | |
| C) JSONL Parsing | Aus Session-JSONL ableiten | |

**User's choice:** A+) HTTP-Hooks
**Notes:** 112x schneller als Command-Hooks (Issue #39391), keine Bash-Probleme

---

## 3. Discord Display Layout

| Option | Description | Selected |
|--------|-------------|----------|
| A) Activity als Small Icon | Nur Icon, Text nur auf Hover | |
| B) Activity in State-Zeile | Activity + Model + Cost in einer Zeile | |
| C) Activity als Details-Zeile | Activity prominent, Rest in State | |
| D) Maximum Output | Activity + Projekt in Details, Model/Tokens/Cost in State, Small Icon = Activity | ✓ |

**User's choice:** D) Maximum Output
**Notes:** Zeigt alles sichtbar ohne Hover

---

## 4. Plugin-Verteilung

| Option | Description | Selected |
|--------|-------------|----------|
| A) Fork + eigene Releases | DSR-Labs/cc-discord-presence | ✓ |
| B) PR upstream | Beitrag an tsanva/cc-discord-presence | |
| C) Lokaler Build | Immer go build aus Source | |
| D) Fork + PR | Beides parallel | |

**User's choice:** A) Fork

---

## 5. Output Presets

| Option | Description | Selected |
|--------|-------------|----------|
| A) Bestehende 5 erweitern | Mehr Messages pro Preset | |
| B) 5 + neue Presets | Bestehende behalten + neue | |
| C) Komplett neu designen | Alle from scratch, 8 Presets | ✓ |

**User's choice:** C) 8 Presets: minimal, professional, dev-humor, chaotic, german, hacker, streamer, weeb

---

## 6. Discord App Name

| Option | Description | Selected |
|--------|-------------|----------|
| Clawd Code | Original-Name | |
| Claude Code | Möglicherweise geblockt (Trademark) | |
| DSR Code | Eigener Brand | ✓ |
| Andere Vorschläge | Codewright, Agentic, Pair Pilot etc. | |

**User's choice:** DSR Code

---

## 7. Multi-Session Handling

| Option | Description | Selected |
|--------|-------------|----------|
| A) Alles übernehmen | Komplette Multi-Session-Logik portieren | ✓ |
| B) Vereinfacht | Nur letzte aktive Session | |
| C) Hybrid | Detection + Stats ohne Tier-Messages | |

**User's choice:** A) Komplett portieren

---

## 8. Idle/Stale Detection

| Option | Description | Selected |
|--------|-------------|----------|
| A) Gleiche Timeouts | 10min/30min/30s fix | |
| B) Kürzere Timeouts | 5min/15min aggressiver | |
| C) Konfigurierbar | Defaults wie A, per Config änderbar | ✓ |

**User's choice:** C) Konfigurierbar

---

## 9. Buttons

| Option | Description | Selected |
|--------|-------------|----------|
| A) Dynamisch pro Projekt | GitHub Repo URL | |
| B) Statisch konfigurierbar | User setzt Links in Config | |
| C) Preset-abhängig | Jeder Preset eigene Buttons | ✓ |
| D) Keine Buttons | Schlank halten | |
| E) Kombination A+B | Dynamisch + Config Override | |

**User's choice:** C) Preset-abhängig

---

## 10. Config-System

| Option | Description | Selected |
|--------|-------------|----------|
| A) JSON-Config | Nur Datei | |
| B) JSON + Env-Vars | Env überschreibt Config | |
| C) JSON + CLI-Flags | Flags überschreiben Config | |
| D) CLI > Env > JSON > Defaults | Maximum Flexibilität | ✓ |

**User's choice:** D) Volle Prioritätskette

---

## 11. Statusline-Konflikt

| Option | Description | Selected |
|--------|-------------|----------|
| A) Wrapper-Kette | Global-Wrapper leitet an Projekt weiter | |
| B-mod) HTTP statt File | Wrapper POSTet an Go HTTP-Server | ✓ |
| C) Projekt-Wrapper | Projekt-statusLine als Wrapper | |

**User's choice:** B-modified — Konsistent mit HTTP-first Ansatz
**Notes:** Eliminiert discord-presence-data.json, Wrapper leitet durch an GSD

---

## 12. Startup/Shutdown

| Option | Description | Selected |
|--------|-------------|----------|
| A) Hybrid-Hooks | Start/Stop Command, Activity HTTP | ✓ |
| B) Systemd/Service | Immer laufender Daemon | |
| C) Manueller Start | User startet selbst | |

**User's choice:** A) Hybrid
**Notes:** Nur 2 Command-Hooks, Rest HTTP — minimale Bash-Exposition

---

## 13. Error Handling

| Option | Description | Selected |
|--------|-------------|----------|
| A) Gleiche Strategie | Fix 5s Reconnect | |
| B) Exponential Backoff | 1s→2s→4s→...→30s | |
| C) Konfigurierbar | Default Backoff, Intervall per Config | ✓ |

**User's choice:** C) Konfigurierbar

---

## 14. Icon-Design

| Option | Description | Selected |
|--------|-------------|----------|
| A) Minimalistisch | Einfarbige Linie-Icons | |
| B) Farbig | Jede Activity eigene Farbe | |
| C) Pixel-Art | Retro 8-Bit | |
| D) AI-generiert | Via fal.ai MCP | ✓ |

**User's choice:** D) AI-generiert via fal.ai, preset-spezifische Varianten

---

## 15. Modell-Pricing

| Option | Description | Selected |
|--------|-------------|----------|
| A) Hardcoded + Fallback | Pricing-Map im Code | |
| B) Auto-Detect Statusline | total_cost_usd von Claude Code | ✓ |
| C) Konfigurierbar | Pricing in Config | |

**User's choice:** B) Statusline liefert fertige Kosten

---

## 16. Preset-Wechsel UX

| Option | Description | Selected |
|--------|-------------|----------|
| A) Nur Config-Datei | Hot-Reload via fsnotify | |
| A+C) Config + HTTP | Hot-Reload + POST /config Endpoint | ✓ |
| B) CLI-Command | Eigener CLI-Modus | |
| D) Alles | Config + CLI + HTTP | |

**User's choice:** A+C — Config Hot-Reload + HTTP Endpoint

---

## 17. JSONL Fallback

| Option | Description | Selected |
|--------|-------------|----------|
| A) Behalten | Zero-Config Fallback | ✓ |
| B) Droppen | Weniger Komplexität | |
| C) Optional | Per Config abschaltbar | |

**User's choice:** A) Behalten — drei Stufen der Genauigkeit

---

## 18. Logging

| Option | Description | Selected |
|--------|-------------|----------|
| A) Wie bisher | stdout, start.sh redirect | |
| B) Eingebautes Log-File | Direkt in Datei, Rotation | |
| C) Eingebaut + Verbosity | -v Debug, -q quiet, Log-Rotation | ✓ |

**User's choice:** C) Eingebaut mit Verbosity-Flags

---

## 19. Security

| Option | Description | Selected |
|--------|-------------|----------|
| A) Nur localhost | 127.0.0.1, kein Remote | |
| B) Konfigurierbar | Default localhost, 0.0.0.0 per Config | ✓ |
| C) Localhost + Auth | Plus Bearer-Token | |

**User's choice:** B) Konfigurierbar

---

## 20. Discord App Workflow

| Option | Description | Selected |
|--------|-------------|----------|
| A) Manuell vorab | Vor Implementierung erstellen | |
| B) Als Task im Plan | Schritt-für-Schritt-Anleitung | ✓ |
| C) Temporär Original-ID | Erst vor Release eigene App | |

**User's choice:** B) Als Task im Plan

---

## Claude's Discretion

- HTTP-Server Implementierung (net/http vs chi/echo)
- Go-Paketstruktur und Modul-Organisation
- Preset-Dateiformat (embedded Go structs vs. externe JSON)
- Exponential Backoff Implementierung
- Windows-spezifische Anpassungen
- JSONL-Fallback-Pricing Map Pflege
- Log-Rotation Implementierung
- fsnotify Config-Watch Debouncing

## Deferred Ideas

- PR upstream an tsanva/cc-discord-presence
- Custom Discord App Icons via eigener Design-Iteration
- Twitch/YouTube Stream-Integration (Streaming Activity Type)
- Voice-Channel Rich Presence
- "corporate" Preset für Enterprise
- Auto-Update-Checker
- Web-Dashboard für Session-Statistiken
- Preset-Editor CLI/TUI

---

*Phase: 01-discord-rich-presence-activity-status-plugin-merge*
*Discussion date: 2026-04-02*
