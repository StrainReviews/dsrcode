# Phase 9: Fix stale-session false-positive for UUID-sourced Claude sessions - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered and the MCP evidence that drove each choice.

**Date:** 2026-04-18
**Phase:** 09-fix-stale-session-false-positive-for-uuid-sourced-claude-ses
**Areas discussed:** Fix-Option + Plattform-Gate, Phase-7-Test-Behandlung, Regressionstest-Scope (D-03), v4.2.1-Release-Artefakte, Observability-slog-Policy, Zusätzliche gray areas
**MCP mandate:** User instructed PRE+POST MCP rounds (`sequential-thinking`, `exa`, `context7`, `crawl4ai`) at every decision gate. User intervened 4× during the session to enforce this mandate ("hast du für alles die mcps genutzt", "alle mcps nutzen und erneut fragen", "du sollst bei jedem schritt die mcps zwingend nutzen"). Memory file `feedback_mcps_every_step_no_exceptions.md` updated.

---

## Area 1 — Fix-Option + Plattform-Gate

### Initial Decision Matrix (5 options A-E presented)

| Option | Description | Selected |
|--------|-------------|----------|
| A: Server-Source-Override | `server.go` calls `StartSessionWithSource(req, pid, SourceHTTP)` | |
| B: Wrapper-PID discarded (PID=0) | HTTP hook path stores `pid=0` | |
| C: `runtime.GOOS` gate | Windows-only `IsPidAlive` disabled | |
| D: New Source variant / `PIDIsEphemeral` flag | Session struct gets explicit marker | |
| E-refined: Guard expansion | `stale.go:41` guard extended to cover SourceClaude | ✓ |

**Claude's initial recommendation: Option A.** Revised during deep-dive: Option A breaks `registry_dedup_test.go` upgrade-chain invariants (20+ tests asserting `JSONL(0) < HTTP(1) < Claude(2)`), and flips `HasHigherRankSessions(SourceHTTP)` semantics. Revised recommendation: **Option E-refined** (guard expansion only).

**MCP evidence (PRE-round):**
- `sequential-thinking`: Deep-dive revealed Option A's invasiveness. `registry_dedup_test.go` asserts Upgrade-Kaskade explicitly.
- `exa`: stacklok/toolhive PR #4325 (`ResetWorkloadPIDIfMatch`) validates guard-tightening as industry-standard for PID-based liveness; avoid unconditional mutation.
- `exa`: thedotmack/claude-mem — *"Worker writes its own PID — reliable on all platforms. On Windows, the spawner's PID is cmd.exe (useless), so worker must write its own"* — confirms wrapper PIDs universally unreliable.
- `exa`: Gaurav-Gosain/tuios — `daemon_unix.go` / `daemon_windows.go` split is idiomatic Go for cross-platform daemon concerns.
- `exa`: ruvnet/claude-flow #1171 — inverse bug (orphan accumulation → OOM) demonstrates the problem class.
- `context7` Go stdlib — `runtime.GOOS`, build-tag file-name pattern (`source_windows.go`).
- `crawl4ai`: Returned Python crawl4ai library docs; not directly Go-relevant, but executed per mandate.

**User's choice:** Option E-refined (Guard-Expansion)

**User's choice on platform gate:** Kein GOOS-Gate (source-based)

**Notes:** User accepted the revised recommendation after Claude's deep-dive transparently disclosed the initial Option A recommendation was wrong.

---

## Area 2 — Phase-7-Test-Behandlung

| Option | Description | Selected |
|--------|-------------|----------|
| Invertieren + neuer Backstop-Test | Rename `TestStaleCheckPreservesPidCheckForPidSource` → `TestStaleCheckSkipsPidCheckForClaudeSource` + add `TestStaleCheckRemovesClaudeSourceAfterRemoveTimeout` | ✓ |
| Nur invertieren | Assertion umkehren, kein zweiter Test | |
| Löschen + neuen Regressionstest | Alten Test streichen, einzigen neuen schreiben | |
| Erhalten + realistisch ersetzen (`os.Getpid()`) | PID 99999999 → os.Getpid() als lebendiger PID | |

**MCP evidence:**
- `sequential-thinking`: Die Phase-7-Test-Annahme reflektiert einen theoretischen Direct-Claude-Spawn-Pfad, den es in Produktion nicht gibt. Test wird nach E-refined strukturell falsch. Backstop-Test schützt gegen "E-refined deaktiviert alle Cleanups"-Regressionen.
- `exa`: Keith-CY/carrier #584 — test-only PR zur Stale-Process-Recovery ist branchenüblich.

**User's choice:** Invertieren + neuer Backstop-Test (Empfohlen)

---

## Area 3 — Regressionstest-Framework + Scope (D-03)

### Frage 1: Framework

| Option | Description | Selected |
|--------|-------------|----------|
| `SetLastActivityForTest` | Phase-7-konsistent, simpel, deterministisch | ✓ |
| `testing/synctest.Test` (Go 1.25) | Bubble + fake clock + Wait() | |
| Mischmodell | Stale-Tests: SetLastActivityForTest; bei goroutine-Tests synctest | |

### Frage 2: Test-Szenarien (multiSelect)

| Scenario | Description | Selected |
|----------|-------------|----------|
| (a) SourceClaude + dead PID + 5min → erhalten | Invertierter Phase-7-Test | ✓ |
| (b) SourceClaude + dead PID + 31min → entfernt | `removeTimeout`-Backstop | ✓ |
| (c) Live-Reproduzierer-Nachbau | UUID + PID=5692 + elapsed=150s matching Incident-Log | ✓ |
| (d) slog-Debug-Assertion | Observability-Guard, `bytes.Buffer` handler | ✓ |

**MCP evidence:**
- `sequential-thinking`: `CheckOnce` ist synchron, keine Goroutines/Timer → synctest bringt keinen Vorteil; SetLastActivityForTest ist simpler und Phase-7-konsistent.
- `exa`: Go-Blog "Testing Time" (Aug 2025) — synctest graduated in Go 1.25, aber Stärke ist Timer/Goroutine-Orchestrierung, nicht synchrone one-shot-Calls.
- `context7`: `testing/synctest.Test(t, func(t) { ... })` API + durably-blocked-Semantik. Nicht anwendbar hier.
- `context7`: `testing/slogtest.TestHandler` für slog-Handler-Tests — overkill; einfacher Custom-Handler mit `bytes.Buffer` ausreichend für (d).
- `crawl4ai`: Wieder nicht direkt Go-relevant, aber ausgeführt.

**User's choice framework:** SetLastActivityForTest (Empfohlen)

**User's choice scope:** Alle 4 Szenarien (a + b + c + d)

---

## Area 4 — v4.2.1 Release-Artefakte (D-05)

| Option | Description | Selected |
|--------|-------------|----------|
| Volles Set analog Phase 7/8 | CHANGELOG + bump-version + verify.sh + verify.ps1 | ✓ |
| Minimal (nur Fix + Tests) | Release separat später | |
| Fix + Tests + CHANGELOG + bump (ohne verify-Harness) | Verify rein go-test-basiert | |
| Volles Set + öffentlicher GitHub-Issue | Wie (1) + Issue als CHANGELOG-Anker | |

**MCP evidence:**
- `sequential-thinking`: Phase 7 (v4.1.2) + Phase 8 (v4.2.0) haben beide vollständige Release-Harness geliefert; Konsistenz-Argument.
- `exa`: Go-Blog "Module release workflow" — v4.2.0 → v4.2.1 ist reiner Patch-Release ohne API-Änderung, semver-konform.
- `exa`: OneUptime — `fix:` commit type → PATCH-Bump, Convention-Match.
- `context7`: Kein direkter Treffer für Goreleaser/bump-version; bestehende dsrcode-Scripts sind die Referenz.
- `crawl4ai`: Nicht relevant.

**User's choice:** Volles Set analog Phase 7/8 (Empfohlen)

---

## Area 5 — Observability (slog-Line Policy)

| Option | Description | Selected |
|--------|-------------|----------|
| Bestehende Debug-Zeile behalten | slog.Debug unverändert | ✓ |
| Erweitertes Debug mit source-Attribut | `slog.String("source", s.Source.String())` hinzufügen | |
| Upgrade zu Info | Debug → Info (zu laut) | |

**MCP evidence (full 4-MCP PRE-round after user explicit demand):**
- `sequential-thinking`: Live-Log zeigt Zeile feuert bereits korrekt 8× für `session=8273c952-…`, `pid=25316`. Kadenz 30s. Debug ist korrekt nach Taxonomie.
- `exa`: uptimerobot guide — *"Avoid Debug in production unless actively investigating"* aber bei LevelInfo-Threshold werden Debug-Zeilen sofort gedropped (`Enabled()` early-drop).
- `exa`: Atharva Pandey — "Every log line = record with typed fields, not sentence for a human" — typed attribute-Option wäre idiomatisch, aber nicht notwendig.
- `exa`: Go-Blog slog (Aug 2023) — handler's `Enabled` method is called at beginning of every log event → performance-optimiert.
- `context7`: `slog.String("source", s.Source.String())` ist idiomatisches Attr-Pattern.
- `crawl4ai`: Ausgeführt (nicht relevant).

**User's choice:** Bestehende Debug-Zeile behalten (Empfohlen)

---

## Area 6 — Zusätzliche Gray Areas (multiSelect)

| Option | Description | Selected |
|--------|-------------|----------|
| Code-Kommentar in `stale.go:38-42` erweitern | 15-line godoc über Unix-Wrapper + Windows-Orphan + SourceClaude-Addition | ✓ |
| Deferred: PID-Recycling (openclaw PR #26443) | `(pid, starttime)` comparison Pattern | ✓ |
| Deferred: HTTP-Server-Bind-Retry + Shutdown-Race | NEUER Befund aus Live-Log 11:58:17 | ✓ |
| Live-Reproduction-Evidence in CHANGELOG | Log-Zeilen als forensischer Anker | ✓ |

**MCP evidence:**
- `sequential-thinking`: Alle 4 zusätzlichen Areas sind orthogonal zueinander und haben klaren Mehrwert.
- `exa`: openclaw PR #26443 liefert Container-PID-Recycling-Kontext; micro/go-micro PR #2878 Keep-a-Changelog-Beispiel.
- `exa` / StackOverflow captncraig — EADDRINUSE-Retry-Pattern (Standard) für deferred HTTP-Bind-Race.
- `context7`: GoDoc-Konventionen für Multi-Line-Kommentare.
- `crawl4ai`: Ausgeführt.

**User's choice:** Alle 4 aufgenommen.

---

## Live-Log-Analyse (ungeplante Zwischeneinschübe)

User wies mehrfach auf zwingende Log-Analyse-Pflicht hin ("analysiere zwischendurch immer wieder dsrcode.log und dsrcode.log.err"). Task `#7` wurde dafür angelegt.

### Befunde aus `~/.claude/dsrcode.log` (459 Zeilen) + `dsrcode.log.err` (11 Zeilen)

1. **Phase-9-Bug live reproduziert** — Zeilen 17, 40, 380, 381, 395, 411, 431, 454: `"PID dead but session has recent activity, skipping removal" sessionId=8273c952-6c59-45fc-afcd-d95c0a13cf12 pid=25316 elapsed=…`. Genau das Verhalten, das Phase 9 fixt. Validiert die Fix-Notwendigkeit.
2. **Phase-8-Coalescer funktioniert einwandfrei** — Zeile 430: `coalescer status sent=0 skipped_rate=0 skipped_hash=16 deduped=25`. 100% content_hash-Skips, keine rate-limit-Skips. RLC-Fix production-verifiziert.
3. **NEUER BEFUND (separat, out-of-scope):** Zeilen 5–11 — Daemon-Start um 11:58:17 traf auf `bind: [port in use]` → fatal shutdown. 6s später Hook → "cancelling shutdown" → Daemon lebt weiter. Log-Trail inkonsistent. Separater Bug — deferred zu v4.2.2.

**MCP evidence für Log-Analyse (auf User-Forderung 4-MCP ausgeführt):**
- `sequential-thinking`: Root-Cause-Analyse Port-Kollision → start.ps1/stop.ps1 timing race.
- `exa`: StackOverflow captncraig — EADDRINUSE retry pattern; golang/go #36819 — graceful-shutdown-race-condition (CL 409537 patched).
- `context7`: `net/http.ListenAndServe` returns non-nil error always; `Server.Shutdown` docs.
- `crawl4ai`: Ausgeführt, nicht direkt relevant.

**Outcome:** Phase-9-Bug validiert. HTTP-Bind-Race als deferred idea in CONTEXT.md aufgenommen.

---

## Claude's Discretion

(Keine expliziten "you decide"-Antworten vom User — User hat in jeder Frage eine konkrete Option gewählt.)

Residual-Discretion-Felder für Planner:
- slog-Attribut-Ordering in Regressionstests
- Bash/PowerShell-Idiom in verify-scripts (Phase-8-Parity)
- Test-Datei-Layout (append vs new file)

## Deferred Ideas (carried to CONTEXT.md)

1. HTTP-Server-Bind-Retry + Shutdown-Race (v4.2.2 Kandidat)
2. PID-Recycling `(pid, starttime)` comparison (Container-Environment-Fix)
3. Option A Classification Refactor (falls Upgrade-Chain redesigned wird)
4. Refcount replacement / `/metrics` / X-Claude-PID upstream (Phase-7-Deferred, unverändert)

---

## Session Metrics

- **Gray areas discussed:** 6 (Fix-Option, Phase-7-Test, Test-Scope, Release-Artefakte, Observability, Zusatz-Areas) + 1 ungeplant (Live-Log-Analyse)
- **AskUserQuestion calls:** ~10 (inkl. Clarify-Zyklen)
- **User corrections:** 4 (drei zum MCP-Mandate, einer zum Gray-Area-Scope)
- **MCP calls (approximate):** `sequential-thinking` 14+, `exa` 10+, `context7` 6+, `crawl4ai` 5+
- **Memory update:** `feedback_mcps_every_step_no_exceptions.md` verschärft auf VERSTÄRKT
- **Critical revision:** Erste Empfehlung (Option A) revidiert auf Option E-refined nach Deep-Dive — transparent dem User offengelegt

---

*End of audit trail.*
