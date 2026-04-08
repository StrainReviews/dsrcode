# Phase 2: DSRCodePresence Setup Wizard - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-03
**Phase:** 02-dsrcodepresence-setup-wizard
**Areas discussed:** Architektur, Commands, Setup-Wizard, Hooks, Presets, Endpoints, First-Run, Distribution, Doctor, Multi-Session, README

---

## Architektur-Entscheidung

### "Von Grundauf" Analyse — 3 Optionen untersucht

| Option | Ansatz | Bewertung |
|--------|--------|-----------|
| MCP Server (TypeScript) | Discord IPC als MCP Server | Nicht machbar (IPC Singleton, stirbt mit Session) |
| Stack-Wechsel TypeScript | Go Binary komplett ersetzen | Nicht sinnvoll (11 Plans verworfen, kein maintained discord-rpc) |
| **Claude-as-Wizard + Go Daemon** | Slash-Commands steuern Go Binary via HTTP | **Gewaehlt** |

**Research durchgefuehrt:**
- Context7: Claude Code Plugin Architecture (commands/, hooks/, .mcp.json, skills/)
- Exa: Discord RPC Node.js libraries (alle abandoned), MCP Server Lifecycle, Claude Code Plugin Guides
- GitHub Issues: #37339 (session_id), #30613 (HTTP hooks ECONNREFUSED), #37559 (Hook capabilities)
- Sequential Thinking: 7+ Thought-Chains fuer Architektur, Hooks, Presets, Multi-Session

**User's choice:** Claude-as-Wizard + Go Daemon
**Rationale:** "so koennten wir das fest in claude code integrieren" — Slash-Commands bieten native Integration

---

## Command-Design

**Frage:** Welche Slash-Commands sollen ins Plugin?
**Optionen:** 4 Basis (setup/preset/status/log) + optionale (doctor/update/demo)
**User's choice:** Alle 7 Commands
**Notes:** User wollte maximalen Umfang

---

## Setup-Wizard Flow

**Frage:** Was soll /dsrcode:setup Schritt fuer Schritt konfigurieren?

| Phase | Inhalt |
|-------|--------|
| A | Voraussetzungen (Binary, Discord, Port, Daemon) |
| B | Discord-Verbindungstest (IPC) |
| C | Basis-Konfiguration (Preset, Discord App ID) |
| D | Erweiterte Einstellungen optional |
| E | Hook-Diagnose |
| F | Config schreiben + Daemon (re)starten |
| G | Verifikation + Zusammenfassung |

**User's choice:** "ja" — nach ausfuehrlicher Darstellung
**Notes:** User wollte ausfuehrlichere Aufstellung bevor er zustimmte

---

## Hook-Migration

**Frage:** Was genau umstellen?

| Hook | Vorher | Nachher |
|------|--------|---------|
| PreToolUse | command (bash hook-forward.sh) | http (localhost:19460) |
| UserPromptSubmit | command (bash hook-forward.sh) | http (localhost:19460) |
| Stop | command (bash hook-forward.sh) | http (localhost:19460) |
| Notification | nicht vorhanden | http (localhost:19460) NEU |
| PostToolUse | nicht vorhanden | bewusst NICHT |

**Research:** Context7 (alle 9 Hook-Typen), Exa (Hook guides), GitHub Issues (#30613, #37559), claudedirectory.org (Notification hook Beschreibung)
**User's choice:** Alle 4 HTTP hooks, Notification dazu, PostToolUse nicht
**Notes:** User wollte ausfuehrliche MCP-Recherche vor der Frage

---

## displayDetail System

**Frage:** Wie erweiterte Daten in Discord anzeigen?

| Level | {file} wird zu | {command} wird zu |
|-------|---------------|------------------|
| minimal | Projektname | "..." |
| standard | Dateiname | gekuerzt 20 Zeichen |
| verbose | voller Pfad | voll |
| private | "file" | "..." |

**Research:** GitHub Issues (#37339 session_id, #25642 CLAUDE_SESSION_ID), Discord Rich Presence Privacy, Hook Payload Format (Context7)
**User's choice:** "ja"
**Notes:** Resolver mappt Werte statt Fallback-Syntax

---

## Preset-Erweiterung

**Frage:** Wie Presets erweitern fuer mehr Variation?

**Erster Vorschlag:** Conditional Fallback Syntax `{file|fallback}`
**User's Korrektur:** "kein fallback. nur die presets in doppelter variation und die alten platzhalter ersetzen. keine ueberbleibsel"
**Finaler Ansatz:** Resolver mappt Platzhalter-WERTE je nach displayDetail. EIN Message-Set, verschiedene Ergebnisse.

| Aspect | Vorher | Nachher |
|--------|--------|---------|
| Messages pro Activity | 20 | 40 |
| Platzhalter | {project}, {branch}, {model}, {tokens}, {cost} | + {file}, {filepath}, {command}, {query}, {activity}, {sessions} |
| Variation | Nur Message-Rotation | Message-Rotation + wechselnde Daten |

---

## Go Binary Endpoints

**Frage:** Response-Format fuer GET /presets und GET /status?

| Endpoint | Response |
|----------|----------|
| GET /presets | current, displayDetail, presets[name, description, example] |
| GET /status | version, uptime, discord{connected, appId}, config, sessions[mit LastFile/Command], hooks{total, byType, lastReceivedAt} |
| POST /preview | Setzt Presence auf exakten Text, duration Parameter |

**User's choice:** "ja"

---

## First-Run UX

**Frage:** Was passiert beim ersten Start?

**Options:** Hint bei jeder Session / einmaliger Hint / kein Hint
**User's choice:** Einzeilige Ausgabe in start.sh wenn Config nicht existiert: "DSR Code gestartet (Preset: minimal) — /dsrcode:setup fuer Anpassungen"

---

## userConfig

**Frage:** Werte bei Plugin-Installation abfragen?

**Options:** Kein userConfig / Nur Preset / Preset + Discord App ID
**User's choice:** Kein userConfig — alles ueber /dsrcode:setup

---

## Versionierung & Distribution

**Frage:** Plugin-Version vs. Binary-Version?

**User's choice:** Gekoppelt. Eine Version fuer beides. Auto-Update via start.sh Version-Check.

---

## /dsrcode:doctor

**Frage:** Was diagnostizieren?

**Research:** Exa (flutter doctor best practices), Discord IPC Detection (Named Pipes, Unix Sockets), GitHub Issues (session_id), Codebase-Analyse (server.go silent rejection Bug)
**User's choice:** 7 Kategorien mit Status-Icons, konkrete Fix-Vorschlaege
**Notes:** User wollte ausfuehrliche MCP-Recherche. flutter doctor als Design-Referenz.

---

## Multi-Session UX

**Frage:** Wie Sessions identifizieren und anzeigen?

**Research:** Exa (VS Code Discord, MultiRPC, Discord IPC Singleton), GitHub Issues (#25642, #18629, #27299, #20132, #28720 — alle fordern CLAUDE_SESSION_ID), Codebase Deep-Dive (server.go handleHook, registry.go StartSession, main.go JSONL-Dedup)
**User's choice:** Self-triggering Hook + cwd+PID Match
**Notes:** User wollte gründliche MCP-Recherche. Kritischer Bug gefunden: server.go:126-129 silent rejection bei leerer session_id.

---

## README & Marketplace

**Frage:** Sprache und Screenshots?
**User's choice:** Englisch. Echte Discord Screenshots via /dsrcode:demo.
**Notes:** Fuehrte zum 7. Command (/dsrcode:demo) mit POST /preview Endpoint.

---

## Claude's Discretion

- Exakte Message-Texte fuer 40 Messages pro Activity pro Preset
- Go-Paketstruktur fuer erweiterten Payload-Parser
- Slash-Command .md Datei-Inhalte
- POST /preview Timeout-Mechanismus
- start.sh Version-Check Implementierung

## Deferred Ideas

- Per-Session Presets
- Activity-abhaengige State-Messages
- Worktree-Agent Zuordnung
- charmbracelet/huh TUI als Non-Claude-Code Alternative
