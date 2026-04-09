# Phase 6: Hook System Overhaul - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-09 (updated from 2026-04-08)
**Phase:** 06-hook-system-overhaul-sessionend-posttooluse-precompact-hooks
**Areas discussed:** SessionEnd-Strategie, Neue Hook-Events, StopFailure Display, Matcher-Strategie, Hook-Deployment-Split, FileChanged, SubagentStart, PostCompact, Setup-Hook, stop.sh Cleanup

---

## SessionEnd-Deployment

| Option | Description | Selected |
|--------|-------------|----------|
| Dual Registration (Empfohlen) | SessionEnd in BEIDEN: plugin hooks.json + settings.local.json via Auto-Patch. Hooks werden gemerged+dedupliziert. | ✓ |
| Nur settings.local.json | NUR in settings.local.json. Plugin hooks.json SessionEnd streichen. | |
| SessionEnd komplett streichen | Kein SessionEnd-Hook. Nur Timer-basierte Stale-Detection. | |
| Claude entscheidet | Downstream-Agents waehlen. | |

**User's choice:** Dual Registration
**Notes:** Research bestaetigt: Plugin hooks.json SessionEnd feuert NIE (#33458 still open). settings.local.json funktioniert als Workaround. Dual Registration ist zukunftssicher.

---

## SessionEnd Timeout

| Option | Description | Selected |
|--------|-------------|----------|
| HTTP-Hook blitzschnell (<100ms) | Handler antwortet sofort mit HTTP 200. 1.5s Timeout ist kein Problem. | ✓ |
| Env-Var Timeout erhoehen | CLAUDE_CODE_SESSIONEND_HOOKS_TIMEOUT_MS=5000 | |
| Beides | Schnell + Env-Var als Safety-Net | |

**User's choice:** HTTP-Hook blitzschnell (<100ms)
**Notes:** HTTP-Handler antwortet in <10ms. Background-Goroutine hat kein Zeitlimit. Env-Var nicht noetig.

---

## Auto-Patch settings.local.json

| Option | Description | Selected |
|--------|-------------|----------|
| Ja, idempotent patchen (Empfohlen) | start.sh liest, fuegt hinzu, schreibt zurueck. Atomisch. Bestehende User-Hooks erhalten. | ✓ |
| Nur wenn Datei nicht existiert | Respektiert User-Aenderungen komplett. | |
| Claude entscheidet | | |

**User's choice:** Ja, idempotent patchen

---

## SessionEnd Final JSONL-Read

| Option | Description | Selected |
|--------|-------------|----------|
| HTTP 200 sofort + Background-Read (Empfohlen) | Handler antwortet HTTP 200 (<10ms). Goroutine: JSONL-Read + Registry.Remove + Refcount-Check. Go-idiomatic. | ✓ |
| Nur Remove, kein Read | Kein JSONL-Read. Letzte Daten vom PostToolUse. | |
| Claude entscheidet | | |

**User's choice:** HTTP 200 sofort + Background-Read
**Notes:** Da dsrcode HTTP-Server nutzt (nicht Script), ist der 1.5s Timeout irrelevant. Daemon lebt weiter nach Response.

---

## SessionEnd Reason-Handling

| Option | Description | Selected |
|--------|-------------|----------|
| Einheitlich behandeln (Empfohlen) | Alle Reasons gleich. Reason wird geloggt (slog.Info). | ✓ |
| Differenziert behandeln | clear/resume vs logout/prompt_input_exit unterschiedlich. | |
| Claude entscheidet | | |

**User's choice:** Einheitlich behandeln

---

## Env-Var CLAUDE_CODE_SESSIONEND_HOOKS_TIMEOUT_MS

| Option | Description | Selected |
|--------|-------------|----------|
| Nicht noetig (Empfohlen) | HTTP <10ms. Timeout = non-blocking error. start.sh KANN Env-Var nicht setzen (Claude laeuft schon). | ✓ |
| In README dokumentieren | Fuer User mit Command-Hooks. | |
| In /dsrcode:setup Wizard | Setup traegt Env-Var in Shell-Profile ein. | |

**User's choice:** Nicht noetig

---

## Neue Hook-Events Scope

| Option | Description | Selected |
|--------|-------------|----------|
| StopFailure + PostCompact | 2 neue Events. Minimal. | |
| Nur StopFailure | 1 neues Event. | |
| StopFailure + PostCompact + CwdChanged | 3 neue Events. | |
| Alle relevanten (5+) | StopFailure, PostCompact, CwdChanged, PostToolUseFailure, Setup. | |
| 7 Events (gewaehlt) | + SubagentStart + FileChanged | ✓ |

**User's choice:** 7 Events: StopFailure, PostCompact, CwdChanged, PostToolUseFailure, Setup, SubagentStart, FileChanged
**Notes:** Spaeter reduziert auf 6: FileChanged gestrichen (fsnotify reicht), Setup gestrichen (start.sh reicht).

---

## StopFailure Display

| Option | Description | Selected |
|--------|-------------|----------|
| Neues 'error' Icon + SmallText (Empfohlen) | 8. Icon. SmallText zeigt Error-Typ. State/Details unveraendert. Temporaer. | ✓ |
| Bestehendes 'idle' Icon + SmallText | Kein neues Asset. Weniger visuell distinkt. | |
| Nur SmallText aendern | Icon bleibt. Subtiler. | |
| Claude entscheidet | | |

**User's choice:** Neues 'error' Icon + SmallText

---

## Matcher-Strategie neue Events

| Option | Description | Selected |
|--------|-------------|----------|
| Wildcard * ueberall + FileChanged spezifisch (Empfohlen) | Konsistent mit D-11/D-12. CwdChanged kein Matcher. | ✓ |
| Alles spezifisch | Weniger HTTP-Overhead, mehr Maintenance. | |
| Claude entscheidet | | |

**User's choice:** Wildcard * ueberall (FileChanged spaeter gestrichen)

---

## Hook-Deployment-Split

| Option | Description | Selected |
|--------|-------------|----------|
| Nur SessionEnd in settings.local.json | 15 Events Plugin, 1 in Local. | |
| SessionEnd + kritische in Local | 3-4 in Local, Rest Plugin. | |
| Alles in settings.local.json (Empfohlen) | Plugin minimal, 13 HTTP-Hooks in Local. Zukunftssicherste Option. | ✓ |
| Claude entscheidet | | |

**User's choice:** Alles in settings.local.json
**Notes:** User fragte ob Option 3 zukunftssicherste ist. Research bestaetigt: Ja, wegen multipler Plugin-Bugs (#33458, #34573, #36456). /hooks Menue zeigt trotzdem alle Hooks.

---

## FileChanged

| Option | Description | Selected |
|--------|-------------|----------|
| Streichen — fsnotify reicht (Empfohlen) | Redundant mit bestehendem fsnotify-Watcher. | ✓ |
| Nutzen als fsnotify-Ersatz | Weniger eigener Code, aber Abhaengigkeit von Claude Code. | |
| Claude entscheidet | | |

**User's choice:** Streichen

---

## SubagentStart Display

| Option | Description | Selected |
|--------|-------------|----------|
| SmallImage 'thinking' + SmallText 'Subagent: {type}' (Empfohlen) | Sofortige Anzeige bei Subagent-Start. SubagentTree.Spawn() aufrufen. | ✓ |
| Nur intern tracken, kein Presence-Update | Presence erst bei PreToolUse des Subagents. | |
| Claude entscheidet | | |

**User's choice:** SmallImage 'thinking' + SmallText 'Subagent: {type}'

---

## PostCompact + PreCompact Interaktion

| Option | Description | Selected |
|--------|-------------|----------|
| PreCompact Baseline + PostCompact Counter (Empfohlen) | PreCompact: Baseline snappen. PostCompact: Counter + Logging + Verifizierung. | ✓ |
| Nur PreCompact | PostCompact-Wert marginal. 14 statt 15 Events. | |
| Claude entscheidet | | |

**User's choice:** PreCompact Baseline + PostCompact Counter

---

## Setup-Hook

| Option | Description | Selected |
|--------|-------------|----------|
| Streichen — start.sh reicht (Empfohlen) | Setup feuert NUR bei --init/--maintenance. start.sh (SessionStart) reicht. | ✓ |
| Setup fuer einmalige Init-Tasks | Ergaenzend zu start.sh. | |
| Claude entscheidet | | |

**User's choice:** Streichen

---

## stop.sh Cleanup + Plugin hooks.json

| Option | Description | Selected |
|--------|-------------|----------|
| Plugin: SessionStart + SessionEnd. Local: 13 HTTP-Hooks. stop.sh: URL-Match Cleanup (Empfohlen) | Plugin behaelt Daemon-Start + Dual Registration. stop.sh entfernt dsrcode-Hooks. | ✓ |
| Plugin: nur SessionStart. Local: 14 inkl. SessionEnd | Verliert Dual Registration. | |
| Claude entscheidet | | |

**User's choice:** Plugin: SessionStart + SessionEnd. Local: 13 HTTP-Hooks. stop.sh: URL-Match Cleanup.

---

## Claude's Discretion

- Exakte Drosselungs-Logik, Route Handler Implementierungen, Auto-Exit Goroutine, Shutdown-Timing, Test-Strategie, CHANGELOG, Icon Design, start.sh/stop.sh JSON-Manipulation Details

## Deferred Ideas

- Token-Usage in Hook-Payloads (Issue #11008)
- SessionEnd Fix Monitoring (#33458)
- PostCompact Token-Counts (Issue #40492)
- FileChanged als fsnotify-Ersatz (evaluiert, verworfen)
- Setup-Hook (evaluiert, verworfen)
- Agent Teams Events (experimentell)
