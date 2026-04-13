# Phase 7 Discussion Log

**Date:** 2026-04-13
**Mode:** discuss (interactive, 4-question batches)
**MCPs used:** sequential-thinking (×4), exa.web_search_exa (×2), context7.resolve-library-id (×2)

## Context Triggers

- Live incident report (user): "ich habe aktuell eine session offen aber trotzdem hat sich dsrcode.exe nach einer gewissen zeit geschlossen. jetzt läuft es wieder weil wir hier eine neue session haben."
- State evidence collected pre-discussion:
  - `~/.claude/dsrcode.refcount` = 20 (wildly drifted)
  - `~/.claude/dsrcode.log` = 478 bytes (overwritten on last daemon start)
  - `~/.claude/dsrcode.log.err` = no crash markers
  - `~/.claude/settings.local.json`: `PreToolUse` / `PostToolUse` already use `matcher: "*"` (initial hypothesis about MCP matcher was wrong)
  - `hooks/hooks.json` (plugin): no SessionEnd command hook → stop.sh/ps1 never fires

## Questions & Answers

### Gray Area 1 — PID-Check

**Q:** Wie fixen wir den PID-Dead-Check für HTTP-Sessions (Bug #1)?

Options presented:
- Skip für HTTP-Sessions (Recommended)
- 2min-Schwelle konfigurierbar
- PID-Tracking nur Unix
- X-Claude-PID erzwingen

**A:** Skip für HTTP-Sessions (Recommended) → D-01

Follow-ups: none (option self-contained, locked D-01/02/03)

### Gray Area 2 — Activity

**Q:** Wie fixen wir fehlendes LastActivityAt-Update bei Tool-Calls (Bug #2)?

Options presented:
- handlePostToolUse updated LastActivityAt (Recommended)
- Eigener /hooks/heartbeat Endpoint
- Daemon-interner Self-Heartbeat
- Alle Handler updaten LastActivityAt

**A:** handlePostToolUse updated LastActivityAt (Recommended) → D-04

Follow-up Q: Was soll handlePostToolUse im Session-Update setzen?
- Options: Nur LastActivityAt (Recommended) / +Icon 'coding' / Touch()-Methode

**A:** Nur LastActivityAt (Recommended) → D-05 (no UI side effects)

### Gray Area 3 — Refcount

**Q:** Wie fixen wir die Refcount-Drift (Bug #3)?

Options presented:
- SessionEnd command hook (Recommended)
- Refcount abschaffen
- Auto-sync beim Start
- Ignorieren

**A:** SessionEnd command hook (Recommended) → D-07

Follow-up Q: Wo wird der SessionEnd-command-hook registriert?
- Options: Plugin hooks.json (Recommended) / settings.local.json via start.sh-patch / Beide Orte

**A:** Plugin hooks.json (Recommended) → D-07 location locked

### Gray Area 4 — Scope

**Q:** Welchen Scope soll Phase 7 haben?

Options presented:
- Alle 4 Bugs + Log-Append (Recommended)
- Nur Windows-Hotfix
- Minimal: nur Bug #1+#2
- Maximal: + Monitoring/Observability

**A:** Alle 4 Bugs + Log-Append (Recommended) → D-13/14/15 (cross-platform, v4.1.2)

Follow-up Q: Wie handhaben wir das Log-File im start.ps1/sh?
- Options: Append + Size-Cap 10MB (Recommended) / Pure Append / Date-based Rotation / Keep-Simple

**A:** Append + Size-Cap 10MB (Recommended) → D-11

## Deferred (scope-creep candidates flagged)

- Monitoring/Observability (Prometheus /metrics)
- Refcount abolition
- X-Claude-PID enforcement (upstream dependency)
- Heartbeat-based liveness-probe architecture (from Exa research — kept as backstop idea)

## MCP Research Used

- **sequential-thinking (×4):** decomposed root-cause hypothesis, corrected initial wrong MCP-matcher theory after reading settings.local.json, validated PID-dead-check + handlePostToolUse as true root causes, structured gray-area trade-offs for discussion.
- **exa.web_search_exa (×2):** surfaced heartbeat patterns (Kubernetes liveness probes, Go heartbeat article) — informed "Daemon-interner Self-Heartbeat" option for Bug #2; user chose simpler fix.
- **context7 (×2):** resolved fsnotify and claude-code-hooks library IDs for reference — docs not needed in final CONTEXT.md because bugs are code-local not library-usage issues.
- **crawl4ai:** not invoked — no specific URL needed beyond Exa search results.

## Anti-patterns Avoided

- Acting on first hypothesis (MCP-matcher whitelist) without verifying settings.local.json state — would have shipped a non-fix.
- Bundling Observability into Phase 7 (scope creep per scope_guardrail).
- Adding config knobs for the PID-check skip (complexity without benefit per D-03).
