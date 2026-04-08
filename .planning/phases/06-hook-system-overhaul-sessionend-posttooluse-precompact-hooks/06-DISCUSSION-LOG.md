# Phase 6: Hook-System Overhaul - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-08
**Phase:** 06-hook-system-overhaul-sessionend-posttooluse-precompact-hooks
**Areas discussed:** Token-Pipeline, Auto-Exit & Lifecycle, Stale-Handling, PreToolUse Matcher

---

## Token-Pipeline

| Option | Description | Selected |
|--------|-------------|----------|
| Hook-Triggered Reads | JSONL-Polling/Watching/Discovery entfernen (~250 Zeilen). Parsing behalten in analytics/. Hooks liefern transcript_path fuer gezielte Reads. ~70% Code-Reduktion. | ✓ |
| StatusLine-Only | Allen JSONL-Code entfernen. Token-Anzeige degradiert zu Total-Only. Verliert Phase 4 Core-Feature. | |
| Nur Non-Token Arbeit | Token-Pipeline unveraendert. Phase 6 fokussiert nur auf andere Aspekte. | |

**User's choice:** Hook-Triggered Reads
**Notes:** Research-backed — Claude Code Hooks liefern KEINE token_usage in Payloads (PostToolUse: session_id, transcript_path, cwd, tool_name, tool_input, tool_result). Token-Breakdown nur via JSONL. Phase 19 hatte dies als Deferred Idea notiert: "Voraussetzung: Claude Code muesste token_usage in Hook-Payloads exponieren."

---

## Auto-Exit & Lifecycle

### Grace-Period Dauer

| Option | Description | Selected |
|--------|-------------|----------|
| 30 Sekunden | Kurz genug fuer Ressourcen-Freigabe, lang genug fuer Session-Wechsel. | |
| 60 Sekunden | Konservativer Puffer. | |
| 5 Minuten | Sehr konservativ, spart start.sh Overhead. | |
| Konfigurierbar | Neues Config-Feld shutdownGracePeriod mit Default. | ✓ |

**User's choice:** Konfigurierbar mit 30s Default

### Grace-Abort Verhalten

| Option | Description | Selected |
|--------|-------------|----------|
| Shutdown abbrechen | SessionStart waehrend Grace setzt Timer zurueck. | ✓ |
| Shutdown durchfuehren | Binary faehrt runter, start.sh startet neu. | |
| Du entscheidest | Claude's Discretion. | |

**User's choice:** Shutdown abbrechen

### Discord-Status vor Shutdown

| Option | Description | Selected |
|--------|-------------|----------|
| Discord Activity clearen | Explizites ClearActivity + Close IPC vor Exit. | ✓ |
| Nichts tun | Discord entfernt Activity automatisch nach ~60s. | |
| Du entscheidest | Claude's Discretion. | |

**User's choice:** Discord Activity clearen

### Auto-Exit Trigger (revidiert nach SessionEnd-Research)

| Option | Description | Selected |
|--------|-------------|----------|
| Dual-Trigger | SessionEnd (instant, ~30%) + Stale-Detection (100% zuverlaessig). Robust. | ✓ |
| Nur Stale-basiert | SessionEnd ignorieren. ~10min Verzoegerung. | |
| SessionEnd + Stop | Stop-Hook als secondary Trigger. | |

**User's choice:** Dual-Trigger
**Notes:** Revidiert nach Research — SessionEnd Hooks sind unzuverlaessig (Issues #33458, #32712, #41577, #17885). Plugin hooks.json SessionEnd feuert NIE (confirmed bug). Timer MUSS primaer bleiben.

---

## Stale-Handling

### Timer-Intervall (revidiert)

| Option | Description | Selected |
|--------|-------------|----------|
| 30s beibehalten | Aktueller Default, einzig zuverlaessiger Mechanismus. | ✓ |
| Dynamisch 30s/2min | 30s aktiv, 2min idle. Marginal weniger CPU. | |
| Du entscheidest | Claude's Discretion. | |

**User's choice:** 30s beibehalten
**Notes:** Urspruenglich war 5min vorgeschlagen (vor Research). Nach SessionEnd-Bug-Research revidiert: Timer ist das einzig Zuverlaessige, darf nicht verlaengsamt werden.

### SessionEnd Verhalten

| Option | Description | Selected |
|--------|-------------|----------|
| Sofort entfernen | SessionEnd → Registry.Remove() nach finalem Token-Read. | ✓ |
| Erst idle dann entfernen | TransitionToIdle() dann nach removeTimeout entfernen. | |
| Du entscheidest | Claude's Discretion. | |

**User's choice:** Sofort entfernen

---

## PreToolUse Matcher

| Option | Description | Selected |
|--------|-------------|----------|
| Wildcard '*' | Alle Tools inkl. MCP. Zero Maintenance. ~1ms HTTP overhead. | ✓ |
| Erweiterte Liste | 10+ explizite Tools. Muss aktualisiert werden. | |
| Zwei Matcher | Wildcard fuer Tracking + spezifisch fuer Aktionen. | |

**User's choice:** Wildcard '*'

---

## PostToolUse Matcher (nach Research hinzugefuegt)

| Option | Description | Selected |
|--------|-------------|----------|
| Wildcard '*' | Jedes Tool-Ende triggert Token-Read. Max Frische. Daemon drosselt intern. | ✓ |
| Nur IO-Tools | Bash\|Edit\|Write\|Read. Weniger Calls, Luecken moeglich. | |
| Kein PostToolUse | PreToolUse fuer alles. Daten 1 Tool-Ausfuehrung alt. | |

**User's choice:** Wildcard '*'

---

## Claude's Discretion

- JSONL ParseTranscript() Refactoring-Details
- PostToolUse/SessionEnd/PreCompact Route Handler Implementation
- Auto-Exit Goroutine Koordination
- Shutdown-Sequenz Timing
- start.sh Auto-Patch Details
- Test-Strategie fuer neue Hooks
- Version Bump

## Deferred Ideas

- Token-Usage in Hook-Payloads (wenn Anthropic das hinzufuegt)
- SessionEnd Fix Monitoring (anthropics/claude-code#33458)
- PostToolUseFailure Hook
- SubagentStart Hook
- Setup Hook
- StatusLine-Only Token-Tracking als spaeteren Fallback
