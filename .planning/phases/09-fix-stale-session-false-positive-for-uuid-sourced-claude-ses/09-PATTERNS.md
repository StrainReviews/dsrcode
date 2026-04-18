# Phase 9: Fix stale-session false-positive for UUID-sourced Claude sessions — Pattern Map

**Mapped:** 2026-04-18
**Files analyzed:** 10 (8 modify + 2 create)
**Analogs found:** 10 / 10 (100 % — alle Analoga existieren in-repo; keine externen Vorlagen notwendig)
**Target release:** v4.2.1 hotfix

> Downstream-Konsument: `gsd-planner` → `09-01-PLAN.md` (source + tests) und `09-02-PLAN.md` (release artefacts).
> Jede Datei ist mit Rolle, Datenfluss, Analog, konkretem Code-Auszug und Delta versehen. Planner kopiert Pattern-Deltas direkt in Task-Aktionen.

---

## File Classification

| Datei | Typ | Rolle | Datenfluss | Nächstes Analog | Match-Qualität |
|-------|-----|-------|-----------|------------------|-----------------|
| `session/stale.go` | MODIFY | source-code-fix (guard + godoc) | internal control-flow (no data flow — reine Bedingungs-Erweiterung) | `session/stale.go:41` selbst (Phase-7 1-Zeilen-Edit-Präzedenz) | exact (self-analog) |
| `session/stale_test.go` | MODIFY | test (unit, external package `session_test`) | request-response (synchroner `CheckOnce`-Aufruf, Assert auf Registry-State und slog-Buffer) | `session/stale_test.go:10-33` (`TestStaleCheckSkipsPidCheckForHttpSource`, überlebendes Schwester-Pattern) | exact (selbe Datei, selbes Package) |
| `CHANGELOG.md` | MODIFY | release-artefact (Keep-a-Changelog 1.1.0) | document-insert (an Zeile 10, vor `[4.2.0]`) | `CHANGELOG.md:27-36` ( `[4.1.2]` Phase-7 Hotfix-Eintrag) | exact (selbe Voice / selbes Format) |
| `main.go` | MODIFY | manifest (version string) | build-time string replace via `bump-version.sh` | `scripts/bump-version.sh:29` sed-Regel | exact (scripted) |
| `.claude-plugin/plugin.json` | MODIFY | manifest (plugin version) | build-time string replace via `bump-version.sh` | `scripts/bump-version.sh:34` sed-Regel | exact (scripted) |
| `.claude-plugin/marketplace.json` | MODIFY | manifest (marketplace version) | build-time string replace via `bump-version.sh` | `scripts/bump-version.sh:39` sed-Regel | exact (scripted) |
| `scripts/start.sh` | MODIFY | daemon-launcher (VERSION var) | build-time string replace via `bump-version.sh` | `scripts/bump-version.sh:44` sed-Regel | exact (scripted) |
| `scripts/start.ps1` | MODIFY | daemon-launcher (`$Version` var) | build-time string replace via `bump-version.sh` | `scripts/bump-version.sh:49` sed-Regel | exact (scripted) |
| `scripts/phase-09/verify.sh` | CREATE | release-harness (bash live-daemon smoke) | HTTP request-response + log-offset-grep | `scripts/phase-08/verify.sh` (1-153 Zeilen) | exact (strukturelles drop-in-template) |
| `scripts/phase-09/verify.ps1` | CREATE | release-harness (PowerShell parity) | HTTP request-response + log-offset-regex | `scripts/phase-08/verify.ps1` (1-209 Zeilen) | exact (strukturelles drop-in-template) |

**Match-Legende:** `exact` = Analog ist dieselbe Datei / Schwesterdatei im selben Package oder Scripted Template. Alle 10 Dateien treffen `exact` — null Architektur-Neuheiten in dieser Phase (`09-RESEARCH.md §Don't Hand-Roll` bestätigt).

---

## Pattern Assignments

### 1) `session/stale.go` — MODIFY (guard expansion + godoc)

**Rolle:** source-code-fix (Phase-7 Guard ausweiten um `SourceClaude`).
**Datenfluss:** Intern. Keine neue Datenflussrichtung — nur eine zusätzliche Enum-Vergleichsklausel im bestehenden `if`-Ausdruck.
**Analog:** `session/stale.go:41` selbst + Phase-7 SUMMARY (`07-01-SUMMARY.md:28-33`).

#### Aktueller Zustand (Auszug `session/stale.go:38-48`)

```go
        // PID liveness check — skip for sessions with recent hook activity.
        // HTTP hook-based sessions may carry the daemon's parent PID rather
        // than the actual Claude Code process PID, causing false removals.
        if s.PID > 0 && s.Source != SourceHTTP && !IsPidAlive(s.PID) {
            if elapsed > 2*time.Minute {
                slog.Info("removing stale session (PID dead, no recent activity)", "sessionId", s.SessionID, "pid", s.PID, "elapsed", elapsed)
                registry.EndSession(s.SessionID)
                continue
            }
            slog.Debug("PID dead but session has recent activity, skipping removal", "sessionId", s.SessionID, "pid", s.PID, "elapsed", elapsed)
        }
```

#### Delta nach Phase 9

**Zeile 41 — ein zusätzliches Symbol einfügen (D-01):**

```diff
-       if s.PID > 0 && s.Source != SourceHTTP && !IsPidAlive(s.PID) {
+       if s.PID > 0 && s.Source != SourceHTTP && s.Source != SourceClaude && !IsPidAlive(s.PID) {
```

**Zeilen 38-40 — 3-Zeilen-Kommentar durch 15-Zeilen-godoc ersetzen (D-08, wortwörtlich aus `09-CONTEXT.md §D-08`):**

```go
        // PID liveness check — skip for sessions whose PID is not authoritative.
        //
        // The PID recorded for a session may not be the actual Claude Code process:
        //   - SourceHTTP synthetic IDs ("http-*") always use a wrapper-process PID
        //     from start.sh/start.ps1 — the wrapper exits within seconds.
        //   - SourceClaude UUID IDs also arrive via the HTTP hook path in production
        //     (Claude Code does not spawn dsrcode directly), carrying the same
        //     short-lived wrapper PID via X-Claude-PID header or os.Getppid().
        //
        // On Windows, orphan processes are not reparented to PID 1 — once the
        // wrapper exits, IsPidAlive(wrapperPID) is permanently false even though
        // Claude Code itself is active. On Unix the wrapper parent chain has the
        // same weakness (start.sh exits before Claude does).
        //
        // The removeTimeout backstop (30min default) still reaches truly stale
        // sessions that have stopped sending hooks entirely.
```

**Was bleibt unverändert (Anti-Pattern-Guard aus `09-RESEARCH.md §Anti-Patterns`):**
- `s.PID > 0`-Gate (SourceJSONL hat `PID=0`, bleibt implizit ausgeschlossen).
- 2-Minuten-Grace-Block (`if elapsed > 2*time.Minute`) inkl. `slog.Info` + `slog.Debug` — D-07.
- `removeTimeout`-Backstop (Zeilen 51-55) — unverändert, das ist der einzige verbleibende Löschpfad für SourceHTTP/SourceClaude-Zombies.
- `idleTimeout`-Block (Zeilen 58-61).
- Imports (`context`, `log/slog`, `time`) — kein neuer Import; `SourceClaude` ist im selben Package.

**Ziel-Diff-Größe:** `+15 Zeilen godoc / −3 Zeilen alter Kommentar / +1 Symbol im Guard` = **net +13 Zeilen**. Alles andere ist Über-Modifikation (Pitfall 2 aus RESEARCH).

**Decision refs:** D-01, D-02 (kein GOOS-Branch), D-07 (slog unverändert), D-08 (extended godoc).

---

### 2) `session/stale_test.go` — MODIFY (rename+invert 1 test, add 3 new tests)

**Rolle:** test (unit, external package `session_test`).
**Datenfluss:** request-response — jeder Test baut eine Registry, ruft `CheckOnce(reg, idleTimeout, removeTimeout)` synchron auf, assertet auf `reg.GetSession(id)` sowie optional `buf.String()` (slog-Capture für Test d).
**Analog:** Schwesterfunktion `TestStaleCheckSkipsPidCheckForHttpSource` in derselben Datei (Zeilen 10-33) + `coalescer/coalescer_test.go:267-282` für das slog-bytes.Buffer-Pattern.

#### Imports-Pattern (aus `session/stale_test.go:1-8` — beibehalten, erweitert für Test d)

```go
package session_test

import (
    "testing"
    "time"

    "github.com/StrainReviews/dsrcode/session"
)
```

**Delta:** Nach Phase 9 zusätzliche Imports für Test (d) (`log/slog`-Capture):

```go
import (
    "bytes"
    "log/slog"
    "strings"
    "testing"
    "time"

    "github.com/StrainReviews/dsrcode/session"
)
```

#### Kern-Pattern 1: Überlebender Phase-7-Test (NICHT ANFASSEN — Invariance-Probe)

`TestStaleCheckSkipsPidCheckForHttpSource` (`session/stale_test.go:13-33`) bleibt byteidentisch. Dient als Cross-Source-Invariance-Nyquist-Dimension.

```go
func TestStaleCheckSkipsPidCheckForHttpSource(t *testing.T) {
    reg := session.NewRegistry(func() {})

    req := session.ActivityRequest{
        SessionID: "http-proj-1", // http- prefix => sourceFromID => SourceHTTP
        Cwd:       "/home/user/project",
    }
    // Dead PID: 99999999 is well above any realistic Unix PID and beyond the
    // Windows tasklist allocation ceiling.
    reg.StartSession(req, 99999999)

    // Activity 2.5 minutes ago: past the existing 2-minute grace, well before
    // the 30-minute removeTimeout.
    reg.SetLastActivityForTest("http-proj-1", time.Now().Add(-150*time.Second))

    session.CheckOnce(reg, 10*time.Minute, 30*time.Minute)

    if s := reg.GetSession("http-proj-1"); s == nil {
        t.Fatal("D-01 violation: HTTP-sourced session removed by PID-liveness check despite the SourceHTTP guard")
    }
}
```

#### Kern-Pattern 2: Rename + Assertion-Flip (D-03)

**VORHER (`session/stale_test.go:38-54` — die Phase-7-Fass­ung):**

```go
func TestStaleCheckPreservesPidCheckForPidSource(t *testing.T) {
    reg := session.NewRegistry(func() {})

    req := session.ActivityRequest{
        SessionID: "abcdef12-3456-7890-abcd-ef1234567890", // no prefix => SourceClaude
        Cwd:       "/home/user/project",
    }
    reg.StartSession(req, 99999999) // dead PID

    // Activity 5 minutes ago: well past the 2-minute grace, before removeTimeout.
    reg.SetLastActivityForTest("abcdef12-3456-7890-abcd-ef1234567890", time.Now().Add(-5*time.Minute))

    session.CheckOnce(reg, 10*time.Minute, 30*time.Minute)

    if s := reg.GetSession("abcdef12-3456-7890-abcd-ef1234567890"); s != nil {
        t.Error("D-02 violation: PID-sourced session with dead PID and old activity NOT removed")
    }
}
```

**NACHHER (Phase 9 — Funktionsname, Kommentare und Assertion kippen — Setup byteidentisch):**

```go
// TestStaleCheckSkipsPidCheckForClaudeSource verifies D-01 Phase 9: SourceClaude
// (UUID) sessions with a dead PID are NOT removed by the PID-liveness check in
// production. Their PID is the short-lived wrapper-launcher (start.sh /
// start.ps1), not the Claude Code process itself. Only the removeTimeout
// backstop (30min default) should remove them.
func TestStaleCheckSkipsPidCheckForClaudeSource(t *testing.T) {
    reg := session.NewRegistry(func() {})

    req := session.ActivityRequest{
        SessionID: "abcdef12-3456-7890-abcd-ef1234567890", // no prefix => SourceClaude
        Cwd:       "/home/user/project",
    }
    reg.StartSession(req, 99999999) // dead PID

    // Activity 5 minutes ago: well past the 2-minute grace, before removeTimeout.
    reg.SetLastActivityForTest("abcdef12-3456-7890-abcd-ef1234567890", time.Now().Add(-5*time.Minute))

    session.CheckOnce(reg, 10*time.Minute, 30*time.Minute)

    if s := reg.GetSession("abcdef12-3456-7890-abcd-ef1234567890"); s == nil {
        t.Fatal("D-01 Phase 9 violation: SourceClaude session removed by PID-liveness check despite the SourceClaude guard")
    }
}
```

**Diff-Semantik:** Funktionsname + godoc + Assertion (`!= nil` → `== nil`) + Fehlertext. Setup-Block (Registry / Request / StartSession / SetLastActivityForTest / CheckOnce) byteidentisch → git blame bleibt aussagekräftig.

#### Kern-Pattern 3: Backstop-Regression (D-05 b)

**Neuer Test, nach Kern-Pattern 2 angehängt, gleiches Gerüst, nur zwei Zahlen ändern sich (31 min statt 5 min Backdate + Assertion-Polarität):**

```go
// TestStaleCheckRemovesClaudeSourceAfterRemoveTimeout verifies that the 30-min
// removeTimeout backstop still reaches truly stale SourceClaude sessions —
// zombies that have stopped sending hooks entirely are removed even though
// the PID-liveness check is skipped for them.
func TestStaleCheckRemovesClaudeSourceAfterRemoveTimeout(t *testing.T) {
    reg := session.NewRegistry(func() {})

    req := session.ActivityRequest{
        SessionID: "fedcba98-7654-3210-fedc-ba9876543210", // UUID => SourceClaude
        Cwd:       "/home/user/zombie-project",
    }
    reg.StartSession(req, 99999999) // dead PID (but guard skips the liveness check)

    // Activity 31 minutes ago: past the 30-min removeTimeout backstop.
    reg.SetLastActivityForTest(req.SessionID, time.Now().Add(-31*time.Minute))

    session.CheckOnce(reg, 10*time.Minute, 30*time.Minute)

    if s := reg.GetSession(req.SessionID); s != nil {
        t.Error("D-05(b) violation: SourceClaude session with 31min-old activity NOT removed by removeTimeout backstop")
    }
}
```

#### Kern-Pattern 4: Live-Incident-Mirror (D-05 c)

```go
// TestStaleCheckSurvivesUuidSourcedLongRunningAgent mirrors the live incident
// captured in ~/.claude/dsrcode.log 2026-04-18 11:37-11:44: session
// 2fe1b32a-ea1d-464f-8cac-375a4fe709c9, wrapper PID 5692, removed at elapsed
// 2m25s (150s) while Claude Code was still active. The v4.2.0 daemon auto-exited
// 6 seconds later. This test reproduces the exact elapsed interval (150s) from
// the incident and confirms the v4.2.1 fix keeps the session alive.
func TestStaleCheckSurvivesUuidSourcedLongRunningAgent(t *testing.T) {
    reg := session.NewRegistry(func() {})

    req := session.ActivityRequest{
        SessionID: "2fe1b32a-ea1d-464f-8cac-375a4fe709c9", // live-incident UUID
        Cwd:       "/home/user/long-running-project",
    }
    reg.StartSession(req, 5692) // the actual wrapper PID from the incident log

    // 150 seconds: the exact elapsed from the incident before auto-removal triggered in v4.2.0.
    reg.SetLastActivityForTest(req.SessionID, time.Now().Add(-150*time.Second))

    session.CheckOnce(reg, 10*time.Minute, 30*time.Minute)

    if s := reg.GetSession(req.SessionID); s == nil {
        t.Fatal("D-05(c) live-repro violation: incident session 2fe1b32a-... would have been removed again at 150s elapsed — v4.2.1 fix did not hold")
    }
}
```

#### Kern-Pattern 5: Negative slog-Assertion (D-05 d — Pitfall-1 Auflösung)

**Analog:** `coalescer/coalescer_test.go:267-282` (`TestCoalescer_SummaryIdleSkip`).

```go
// Source-Analog: coalescer/coalescer_test.go:267-282 (Phase-8-Pattern)
func TestCoalescer_SummaryIdleSkip(t *testing.T) {
    var buf bytes.Buffer
    origLogger := slog.Default()
    slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
    defer slog.SetDefault(origLogger)

    synctest.Test(t, func(t *testing.T) {
        c, _, _, _ := newTestCoalescer(t)
        c.EmitSummaryForTest()
    })

    if strings.Contains(buf.String(), "coalescer status") {
        t.Errorf("idle emitSummary produced log line: %q", buf.String())
    }
}
```

**Delta für Phase 9 (`synctest` wird entfernt — D-04 verbietet ihn; `CheckOnce` ist synchron ohne Goroutinen):**

```go
// TestStaleCheckEmitsDebugOnGuardSkip verifies observability: after D-01 widens
// the outer guard to skip SourceClaude, the inner slog.Debug line at
// stale.go:47 MUST NOT fire for SourceClaude + elapsed < 2min. This proves
// the guard skips the entire block (not just the EndSession call) — the
// NEGATIVE form of CONTEXT D-05(d) per Pitfall-1 resolution.
func TestStaleCheckEmitsDebugOnGuardSkip(t *testing.T) {
    var buf bytes.Buffer
    origLogger := slog.Default()
    slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
    defer slog.SetDefault(origLogger)

    reg := session.NewRegistry(func() {})
    req := session.ActivityRequest{SessionID: "uuid-guard-skip-test-abcd", Cwd: "/tmp/p"}
    reg.StartSession(req, 99999999)                                            // dead PID
    reg.SetLastActivityForTest(req.SessionID, time.Now().Add(-30*time.Second)) // inside 2min grace

    session.CheckOnce(reg, 10*time.Minute, 30*time.Minute)

    // NEGATIVE: Debug-Zeile aus stale.go:47 darf für SourceClaude gar nicht feuern,
    // weil D-01 den gesamten umschließenden Block überspringt.
    if strings.Contains(buf.String(), "PID dead but session has recent activity") {
        t.Errorf("D-05(d) violation: inner Debug line fired for SourceClaude, proving guard skip is partial; got: %q", buf.String())
    }

    // Konsistenz: Session überlebt (Guard hat seinen Job gemacht).
    if s := reg.GetSession(req.SessionID); s == nil {
        t.Errorf("D-05(d) consistency violation: SourceClaude session removed despite guard skip expectation")
    }
}
```

**Deltas vom coalescer-Analog:**
1. `synctest.Test(...)` entfällt (D-04 verbietet `testing/synctest` für `CheckOnce`).
2. `strings.Contains`-Polarität negativ (coalescer-Fassung war ebenfalls negativ — passend).
3. Zusätzliche positive Konsistenz-Assertion auf `reg.GetSession`.

**Validation-Framework (D-04):**
- `session.NewRegistry(func() {})` — vorhandener Konstruktor.
- `reg.StartSession(req, pid)` — `registry.go:42-44` dispatcht nach `sourceFromID` in `SourceClaude`.
- `reg.SetLastActivityForTest(id, t)` — `registry.go:451-465` (Phase-7-Helper, unverändert).
- `session.CheckOnce(reg, idle, remove)` — `stale.go:31` exportiert für externe Tests.
- `reg.GetSession(id)` — Read-after-check.

**Test-File-Layout-Entscheidung (Discretion):** Alle 4 neuen/invertierten Tests in **derselben** `session/stale_test.go` (55 → ~200 Zeilen, well below 800-line cap).

**Decision refs:** D-03, D-04, D-05 (a/b/c/d).

---

### 3) `CHANGELOG.md` — MODIFY (insert `[4.2.1]` section)

**Rolle:** release-artefact (Keep-a-Changelog 1.1.0).
**Datenfluss:** Dokument-Einschub zwischen Zeilen 8 und 10 (nach `## [Unreleased]`, vor `## [4.2.0]`).
**Analog:** `[4.1.2]`-Eintrag bei `CHANGELOG.md:27-36` (Phase-7 Hotfix-Voice) + Sections-Ordnung aus `[4.2.0]` bei Zeilen 10-25.

#### Analog-Auszug (`CHANGELOG.md:27-36`, `[4.1.2]` Phase-7-Hotfix)

```markdown
## [4.1.2] - 2026-04-13

### Fixed
- **Daemon self-termination during long MCP-heavy sessions** — `handlePostToolUse` now updates `LastActivityAt` on every tool call (Bug #2, D-04/D-05 Phase 7). Previously, MCP-heavy sessions could idle the activity clock past the 30-minute remove-timeout even while Claude Code was actively calling tools, causing the daemon to remove the session and eventually self-exit. New `registry.Touch()` method updates the activity timestamp without firing a Discord presence update.
- **PID-liveness check false-positives for HTTP-sourced sessions** — `session/stale.go` now skips the PID liveness check when `Source == SourceHTTP` (Bug #1, D-01/D-02 Phase 7). The daemon's recorded PID for HTTP-sourced sessions is the short-lived `start.sh` wrapper process on Windows, which exits within seconds even while the Claude Code session is alive. The 30-minute `removeTimeout` remains as the backstop.
- ...

### Changed
- ...
```

#### Voice-Konventionen (aus `[4.1.2]` + `[4.2.0]` verifiziert):
- **Fett-geführter Lead-Satz** beschreibt die benutzer-sichtbare Verhaltensänderung.
- **Dedizierter en-dash** (` — `) trennt Lead-Satz von Ursache/Mechanik.
- **Inline-Code** für Symbole (`handlePostToolUse`, `SourceHTTP`, `session/stale.go`).
- **D-NN/Phase-N-Zitate** am Satzende (`Phase 7, D-01/D-02`).
- **Sektionen-Reihenfolge:** `Fixed` → `Changed` → `Added` (`[4.2.0]` nutzt alle drei, `[4.1.2]` nur `Fixed` + `Changed`).

#### Delta für `[4.2.1]` (Insert-Ziel: Zeile 10, direkt nach `## [Unreleased]`)

```markdown
## [4.2.1] - 2026-04-18

### Fixed
- **Daemon no longer auto-exits during long-running UUID-sourced Claude Code sessions** — `session/stale.go:41` guard now also skips the PID-liveness check when `Source == SourceClaude` (Phase 9, D-01). In production, UUID session IDs arrive via the HTTP hook path carrying the wrapper-launcher PID (`start.sh`/`start.ps1`) which exits within seconds, making `IsPidAlive(wrapperPID)` a false-negative for Claude-process liveness. Live incident `~/.claude/dsrcode.log` 2026-04-18 11:37-11:44: session `2fe1b32a-ea1d-464f-8cac-375a4fe709c9`, wrapper PID 5692, 8× `"PID dead but session has recent activity, skipping removal"` debug lines before removal at 2m25s → daemon auto-exit. The Phase-7 guard (Bug #1) covered only `SourceHTTP`; Phase 9 extends it to the second hook-delivered source. The 30-minute `removeTimeout` backstop still reaches zombie sessions.

### Changed
- **Extended godoc at `session/stale.go:38-52`** explains why the PID-liveness skip covers both `SourceHTTP` and `SourceClaude`, notes Windows orphan-reparenting absence and Unix `start.sh` parent-chain weakness, and documents the `removeTimeout` backstop contract (Phase 9, D-08).

### Added
- **Three regression tests in `session/stale_test.go`** (Phase 9, D-05): `TestStaleCheckSkipsPidCheckForClaudeSource` (D-03 inverted rename of the Phase-7 PID-source test), `TestStaleCheckRemovesClaudeSourceAfterRemoveTimeout` (30-min backstop preservation), `TestStaleCheckSurvivesUuidSourcedLongRunningAgent` (live-incident mirror: session `2fe1b32a-...`, PID 5692, elapsed 150s), plus `TestStaleCheckEmitsDebugOnGuardSkip` (negative slog assertion: inner debug line MUST NOT fire for `SourceClaude`, proving the guard skips the entire block).
- **Cross-platform verify harness** at `scripts/phase-09/verify.sh` (bash) and `scripts/phase-09/verify.ps1` (PowerShell) — mirrors the Phase-8 T1..Tn pattern with log-offset-cleanup-on-exit idiom (Phase 9, D-06).
```

#### Footer-Link-Update (Datei-Ende, Zeilen 186-195)

Links-Block verlangt eine neue `[4.2.1]`-Zeile + Update der `[Unreleased]`-Compare-Range:

```markdown
[Unreleased]: https://github.com/DSR-Labs/cc-discord-presence/compare/v4.2.1...HEAD
[4.2.1]: https://github.com/DSR-Labs/cc-discord-presence/compare/v4.2.0...v4.2.1
[4.2.0]: https://github.com/DSR-Labs/cc-discord-presence/compare/v4.1.2...v4.2.0
...
```

(Phase-7 hat die Links fürchterlicherweise vergessen — `CHANGELOG.md:186` springt von `v4.1.0...HEAD` auf `v4.1.0`. Phase 9 korrigiert das NICHT, bleibt scope-eng. Nur `[4.2.1]`-Link wird hinzugefügt.)

**Acceptance-Grep (aus `09-VALIDATION.md §09-02-03/04`):**
- `grep -c '## \[4.2.1\]' CHANGELOG.md == 1`
- `grep -c '2fe1b32a-ea1d-464f-8cac-375a4fe709c9' CHANGELOG.md >= 1`
- Reihenfolge: `awk '/## \[/{print NR": "$0}' CHANGELOG.md | head -3` muss `[Unreleased] < [4.2.1] < [4.2.0]` zeigen.

**Decision refs:** D-06, D-09.

---

### 4-8) Version-Manifeste (`main.go`, `plugin.json`, `marketplace.json`, `start.sh`, `start.ps1`) — MODIFY via `bump-version.sh`

**Rolle:** manifest (Versions-String).
**Datenfluss:** build-time string replace durch `scripts/bump-version.sh 4.2.1`. Kein Handwerk.
**Analog:** `scripts/bump-version.sh:29-51` — jede der 5 sed-Regeln ist bereits für die richtige Datei parametriert.

#### sed-Propagation (analog aus `scripts/bump-version.sh:29-51`, wortwörtlich):

```bash
# 1. main.go: version = "dev" -> version = "X.Y.Z" (inside grouped var block)
sed -i.bak "s/^\(\s*\)version = \"[^\"]*\"/\1version = \"${NEW_VERSION}\"/" "$REPO_ROOT/main.go"
rm -f "$REPO_ROOT/main.go.bak"

# 2. plugin.json: "version": "X.Y.Z"
sed -i.bak "s/\"version\": \"[^\"]*\"/\"version\": \"${NEW_VERSION}\"/" "$REPO_ROOT/.claude-plugin/plugin.json"
rm -f "$REPO_ROOT/.claude-plugin/plugin.json.bak"

# 3. marketplace.json: "version": "X.Y.Z"
sed -i.bak "s/\"version\": \"[^\"]*\"/\"version\": \"${NEW_VERSION}\"/" "$REPO_ROOT/.claude-plugin/marketplace.json"
rm -f "$REPO_ROOT/.claude-plugin/marketplace.json.bak"

# 4. start.sh: VERSION="vX.Y.Z"
sed -i.bak "s/VERSION=\"v[^\"]*\"/VERSION=\"v${NEW_VERSION}\"/" "$REPO_ROOT/scripts/start.sh"
rm -f "$REPO_ROOT/scripts/start.sh.bak"

# 5. start.ps1: $Version = "vX.Y.Z"
sed -i.bak "s/\\\$Version = \"v[^\"]*\"/\$Version = \"v${NEW_VERSION}\"/" "$REPO_ROOT/scripts/start.ps1"
rm -f "$REPO_ROOT/scripts/start.ps1.bak"
```

**Delta für Phase 9:** KEINS. `bash ./scripts/bump-version.sh 4.2.1` im Repo-Root → alle 5 Dateien propagiert in einem Schritt. Script ist idempotent (Phase 7 + Phase 8 erfolgreich so ausgeführt; Präzedenz: `CLAUDE.md §Releasing` Schritt 1).

**Acceptance-Grep (aus `09-VALIDATION.md §09-02-01`):**
```bash
for f in main.go .claude-plugin/plugin.json .claude-plugin/marketplace.json scripts/start.sh scripts/start.ps1; do
    grep -c '4\.2\.1' "$f"     # each must return >= 1
done
```

**Anti-Pattern:** Handwerkliche sed-Schleifen in execute-plan tasks. VERBOTEN — `bump-version.sh` ist der Ein-Pass-Weg.

**Decision refs:** D-06.

---

### 9) `scripts/phase-09/verify.sh` — CREATE (bash live-daemon harness)

**Rolle:** release-harness.
**Datenfluss:** HTTP request-response an `http://127.0.0.1:19460` + Log-Offset-Scan auf `~/.claude/dsrcode.log`.
**Analog:** `scripts/phase-08/verify.sh:1-152` (strukturelles drop-in-template).

#### Imports / Header-Pattern (aus `scripts/phase-08/verify.sh:16-29`):

```bash
#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOG_FILE="${HOME}/.claude/dsrcode.log"
DAEMON_URL="http://127.0.0.1:19460"

pass() { echo "PASS: $1"; }
fail() { echo "FAIL: $1" >&2; exit 1; }

# Record log offset at start so T3..T6 only look at lines written during this run.
if [[ ! -f "$LOG_FILE" ]]; then
    fail "log file $LOG_FILE not found — daemon not running?"
fi
LOG_OFFSET=$(wc -c < "$LOG_FILE")
```

**Delta für Phase 9:** Header-Kommentar swappen (`Phase 8 live-daemon…` → `Phase 9 live-daemon…`), Version `4.2.0` → `4.2.1`. Rest byteidentisch.

#### Cleanup-Trap-Pattern (aus `scripts/phase-08/verify.sh:36-63`):

```bash
CLEANUP_IDS=()
CLEANUP_DONE=false
register_cleanup() {
    CLEANUP_IDS+=("$1")
}
cleanup() {
    local rc=$?
    if $CLEANUP_DONE; then
        return $rc
    fi
    CLEANUP_DONE=true
    set +e
    for id in "${CLEANUP_IDS[@]+"${CLEANUP_IDS[@]}"}"; do
        curl -sS -X POST \
            -H "Content-Type: application/json" \
            -d "{\"session_id\":\"$id\",\"reason\":\"verify-sh-cleanup\"}" \
            "${DAEMON_URL}/hooks/session-end" \
            >/dev/null 2>&1 || true
    done
    return $rc
}
trap cleanup EXIT
```

**Delta für Phase 9:** Vollständig wortwörtlich übernehmen. Cleanup-ID-Präfixe bleiben `t3-s$i`, `t4-s1`, `t5-s1`; bei Bedarf durch Phase-9-spezifische IDs ergänzen (siehe T-Swap unten).

#### log_tail-Pattern (aus `scripts/phase-08/verify.sh:65-68`):

```bash
log_tail() {
    # Print log bytes written after LOG_OFFSET.
    dd if="$LOG_FILE" bs=1 skip="$LOG_OFFSET" 2>/dev/null
}
```

**Delta:** Byteidentisch.

#### T-Assertion-Swap (das ist das einzige echte Neue für Phase 9)

| Phase-8 T | Phase-8 Assertion | → Phase-9 T (swap-in) | Phase-9 Assertion |
|-----------|-------------------|------------------------|-------------------|
| T1 | `dsrcode --version` enthält `4.2.0` | T1 (unverändert strukturell) | `dsrcode --version` enthält `4.2.1` |
| T2 | `/health` → HTTP 200 | T2 (unverändert) | `/health` → HTTP 200 |
| T3 | Burst 10 → ≤3 SetActivity | T3 (STREICHEN) | — (coalescer-Phase-8-Spezifik, nicht Phase-9-Scope) |
| T4 | 10 identische → ≥9 content_hash skips | T4 (STREICHEN) | — |
| T5 | 2 identische Hooks → 1 dedup | T5 (STREICHEN) | — |
| T6 | 60s summary log | T6 (STREICHEN) | — |
| — | — | **T3 (NEU)** | UUID-Hook auf `/hooks/pre-tool-use` mit synthetischer Session-ID `verify-09-uuid-$(uuidgen or fallback)`, dann 0 weitere Hooks für 10 s; `log_tail \| grep -c '"removing stale session"' == 0`. Beweis: SourceClaude überlebt den PID-Liveness-Check trotz Wrapper-exit. |
| — | — | **T4 (NEU, optional/documented)** | Manueller MCP-heavy Repro-Hinweis im Script-Header — `scripts/phase-09/verify.sh` druckt am Ende eine Anweisung: "For manual repro: start Claude Code, MCP-heavy flow >2min silence, tail dsrcode.log, assert no 'removing stale session' on UUID." |

**Decision-Rationale für T-Swap:** Phase-8-T3/T4/T5/T6 testen coalescer/dedup, nicht stale-detection. Phase 9 ersetzt sie mit einem stale-detection-spezifischen T3. T4 bleibt bewusst manuell (Wall-clock ≥2 min Reproduktion, passt nicht in ein einmaliges Smoke-Script).

#### Vollständiges Phase-9-T3-Template (abgeleitet aus Phase-8 T3-Schema, Zeilen 88-104):

```bash
# --- T3: SourceClaude guard does NOT remove dead-PID UUID session ---
UUID_ID="verify09-$(date +%s)-abc-def-1234-5678-90ab-cdef"
register_cleanup "$UUID_ID"
curl -fsS -o /dev/null -X POST \
    -H "Content-Type: application/json" \
    -d "{\"session_id\":\"$UUID_ID\",\"tool_name\":\"Edit\",\"cwd\":\"/tmp/verify09\"}" \
    "${DAEMON_URL}/hooks/pre-tool-use"

# Wait > 2-minute grace to give the stale-check ticker a full sweep, but < 30min
# so the removeTimeout backstop does not fire. 150s mirrors the live-incident elapsed.
echo "T3 waiting 150s for stale-check ticker to sweep..."
sleep 150

T3_REMOVE_COUNT=$(log_tail | grep -c "\"removing stale session\".*\"$UUID_ID\"" || true)
if [[ "$T3_REMOVE_COUNT" -eq 0 ]]; then
    pass "T3 SourceClaude UUID session survived PID-liveness sweep (0 removals observed)"
else
    fail "T3 SourceClaude UUID session was removed $T3_REMOVE_COUNT time(s) — v4.2.1 guard regression"
fi
```

**Hinweis zum Wartezeitaufwand:** 150 s Sleep macht verify.sh zu einem ~3-minütigen Lauf. Alternative (für schnelle CI-Läufe) ist das Senken des stale-check-Intervalls per env-var, aber das ist außerhalb des Phase-9-Scope. Planner sollte den 150 s Sleep als Baseline akzeptieren; User kann mit `bash -x scripts/phase-09/verify.sh` nachverfolgen.

**Decision refs:** D-06, D-09.

---

### 10) `scripts/phase-09/verify.ps1` — CREATE (PowerShell parity)

**Rolle:** release-harness (Windows-Parity).
**Datenfluss:** HTTP request-response via `Invoke-WebRequest` + Log-Regex auf `~\.claude\dsrcode.log`.
**Analog:** `scripts/phase-08/verify.ps1:1-209`.

#### Imports / Header-Pattern (aus `scripts/phase-08/verify.ps1:1-32`):

```powershell
#Requires -Version 5.1
<#
.SYNOPSIS
    Phase 8 live-daemon verification harness ...
#>

[CmdletBinding()]
param()

$ErrorActionPreference = 'Continue'

$script:results  = [ordered]@{}
$script:passCount = 0
$script:failCount = 0

$DaemonUrl = 'http://127.0.0.1:19460'
$LogFile   = Join-Path $env:USERPROFILE '.claude\dsrcode.log'
```

**Delta für Phase 9:** `Phase 8` → `Phase 9` im Synopsis-Header, `4.2.0` → `4.2.1` in T1-Matcher.

#### Invoke-Test Helper (aus `scripts/phase-08/verify.ps1:34-60`):

Wortwörtlich übernehmen — generisches Test-Framework für alle Tests.

#### Log-Offset-Reader (aus `scripts/phase-08/verify.ps1:62-85`):

```powershell
$script:logOffset = (Get-Item $LogFile).Length

function Get-NewLogBytes {
    $fs = [System.IO.File]::Open($LogFile, 'Open', 'Read', 'ReadWrite')
    try {
        $null = $fs.Seek($script:logOffset, 'Begin')
        $sr = New-Object System.IO.StreamReader($fs)
        return $sr.ReadToEnd()
    } finally {
        $fs.Close()
    }
}

function Count-LogMatches {
    param([string]$Pattern)
    $text = Get-NewLogBytes
    if ([string]::IsNullOrEmpty($text)) { return 0 }
    return ([regex]::Matches($text, $Pattern)).Count
}
```

**Delta:** Byteidentisch.

#### T-Block-Pattern (Invoke-Test + Regex count, aus `scripts/phase-08/verify.ps1:107-123`):

```powershell
Invoke-Test -Id 'T1' -Description 'dsrcode --version reports 4.2.0' -Test {
    $cmd = Get-Command -Name 'dsrcode' -ErrorAction SilentlyContinue
    if (-not $cmd) { return $false }
    $output = & dsrcode --version 2>&1 | Out-String
    return ($output -match '4\.2\.0')
}
```

**Delta für Phase 9:** Version `4.2.0` → `4.2.1` in T1.

#### Finally-Cleanup-Pattern (aus `scripts/phase-08/verify.ps1:191-208`):

```powershell
finally {
    foreach ($id in $script:CleanupIds) {
        try {
            $body = "{`"session_id`":`"$id`",`"reason`":`"verify-ps1-cleanup`"}"
            Invoke-WebRequest -Method Post -Uri ($DaemonUrl + '/hooks/session-end') `
                -ContentType 'application/json' -Body $body `
                -UseBasicParsing -TimeoutSec 3 -ErrorAction Stop | Out-Null
        } catch {
            # Swallow — cleanup must never abort.
        }
    }
}
```

**Delta:** Byteidentisch — das ist die Windows-Parity zum bash EXIT-Trap.

#### T-Swap für Phase 9 (dasselbe Swap-Schema wie `verify.sh`):

```powershell
Invoke-Test -Id 'T3' -Description 'SourceClaude UUID session survives PID-liveness sweep' -Test {
    $uuid = "verify09-$(Get-Date -Format 'yyyyMMddHHmmss')-abc-def-1234-5678-90ab-cdef"
    $script:CleanupIds.Add($uuid)
    Invoke-HookPost -Path '/hooks/pre-tool-use' `
        -Body "{`"session_id`":`"$uuid`",`"tool_name`":`"Edit`",`"cwd`":`"/tmp/verify09`"}"

    Write-Verbose 'T3 waiting 150s for stale-check ticker to sweep...'
    Start-Sleep -Seconds 150

    $removes = Count-LogMatches ('"removing stale session".*"' + [regex]::Escape($uuid) + '"')
    Write-Verbose "T3 observed $removes removals for $uuid (expect 0)"
    return ($removes -eq 0)
}
```

**Decision refs:** D-06, D-09.

---

## Shared Patterns

### Shared Pattern A — External test package `session_test`

**Source:** `session/stale_test.go:1` (`package session_test`).
**Apply to:** Alle Tests in `session/stale_test.go` (Kern-Patterns 2-5).

```go
package session_test

import (
    "testing"
    "time"

    "github.com/StrainReviews/dsrcode/session"
)
```

**Warum:** Externe Tests verifizieren nur die öffentliche API (`session.NewRegistry`, `session.CheckOnce`, `session.ActivityRequest`, `session.SessionSource`). Keine Tests gegen interne Hilfsfunktionen — das bleibt Phase-7/8-Konvention.

### Shared Pattern B — Dead-PID `99999999` + `SetLastActivityForTest`

**Source:** `session/stale_test.go:22,26,45,48`.
**Apply to:** Tests a, b, c, d.

```go
reg.StartSession(req, 99999999)                                           // dead PID
reg.SetLastActivityForTest(id, time.Now().Add(-<backdate>*time.<unit>))   // deterministic activity
session.CheckOnce(reg, 10*time.Minute, 30*time.Minute)                    // idle=10m, remove=30m
```

**Warum:** `99999999` übersteigt jeden realistischen Linux-PID und die Windows `tasklist`-Obergrenze → deterministisches `IsPidAlive()==false` cross-platform. `SetLastActivityForTest` entkoppelt Tests von `time.Sleep`.

### Shared Pattern C — slog-Capture via `bytes.Buffer` + `defer SetDefault`

**Source:** `coalescer/coalescer_test.go:267-272`.
**Apply to:** Nur Test d (`TestStaleCheckEmitsDebugOnGuardSkip`).

```go
var buf bytes.Buffer
origLogger := slog.Default()
slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
defer slog.SetDefault(origLogger)
```

**Warum:** In-repo idiom. Erlaubt Content-Grep auf Log-Ausgabe via `strings.Contains(buf.String(), ...)`. `slogtest.TestHandler` ist hier falsch (validiert Handler-Implementierungen, nicht Log-Inhalt).

### Shared Pattern D — Phase-NN sed-Idempotenz

**Source:** `scripts/bump-version.sh:29-51`.
**Apply to:** Alle 5 Version-Manifeste (main.go, plugin.json, marketplace.json, start.sh, start.ps1).

```bash
bash ./scripts/bump-version.sh 4.2.1
```

**Warum:** Phase 7 + Phase 8 haben das Skript produktiv verwendet. Jede sed-Regel ist auf die jeweilige Datei zugeschnitten (`version = "..."`, `"version": "..."`, `VERSION="v..."`, `$Version = "v..."`). Handwerkliche Alternativen sind ein Anti-Pattern.

### Shared Pattern E — Phase-NN verify-Harness Struktur

**Source:** `scripts/phase-08/verify.sh` (1-152) + `verify.ps1` (1-209).
**Apply to:** `scripts/phase-09/verify.sh` + `verify.ps1`.

Struktur-Contract:
1. `#!/bin/bash` + `set -euo pipefail` (bash) / `#Requires -Version 5.1` + `$ErrorActionPreference = 'Continue'` (pwsh)
2. `DaemonUrl`, `LogFile`, `pass()`/`fail()` (bash) / `Invoke-Test` (pwsh) Helpers
3. Log-Offset-Capture (`wc -c` bash / `.Length` pwsh)
4. `CLEANUP_IDS` + `trap cleanup EXIT` (bash) / `$CleanupIds` + `finally` (pwsh)
5. T1 Binary-Version → T2 `/health` → T3+ Phase-spezifische Assertions
6. Exit code 0 auf all-pass, non-zero auf fail

**Warum:** Cross-platform Parity. Bash-Windows-Brüche (kein `uuidgen` auf vanilla Git-Bash) werden mit `date +%s`-Fallbacks abgefangen.

---

## No Analog Found

**Keine.** Alle 10 Dateien haben einen exakten oder strukturell passenden Analog im Repo. Phase 9 führt keine net-neue Infrastruktur ein — das ist der kritische Unterschied zur Phase 8 (coalescer-Paket, rate-Limiter-Abhängigkeit). Jede Zeile Code/Script, die Phase 9 schreibt, hat einen byteidentischen oder 1-Symbol-entfernten Prior-Art-Anker im Repo.

---

## Metadata

**Analog search scope:**
- `session/*.go` — `stale.go`, `stale_test.go`, `source.go`, `registry.go` (registrierte Sibling-Analogs)
- `coalescer/coalescer_test.go:250-282` (slog+bytes.Buffer-Pattern)
- `scripts/bump-version.sh` (sed-Propagation-Pattern)
- `scripts/phase-08/verify.{sh,ps1}` (release-harness-Template)
- `CHANGELOG.md:1-195` (Keep-a-Changelog-Voice + [4.1.2]/[4.2.0]-Präzedenz)
- `.planning/phases/07-*/07-01-SUMMARY.md` (1-Zeilen-Diff-Präzedenz)

**Files scanned:** 9 source/test Dateien + 2 verify-Skripte + 1 CHANGELOG + 1 bump-Skript + 1 Phase-7-SUMMARY = **14 Dateien**.

**Pattern extraction date:** 2026-04-18.

**Nyquist coverage check (gegen `09-VALIDATION.md §Nyquist Dimensions Covered`):**

| Dimension | Pattern-Anker |
|-----------|----------------|
| Positive path (D-01) | Kern-Pattern 2 (Rename+Invert) |
| Backstop (removeTimeout) | Kern-Pattern 3 (31min-Test) |
| Live-incident regression | Kern-Pattern 4 (2fe1b32a/5692/150s) |
| Observability (negative slog) | Kern-Pattern 5 (bytes.Buffer) |
| Cross-source invariance (HTTP-Guard) | Kern-Pattern 1 (überlebender Phase-7-Test) |
| SourceJSONL-Schutz | `s.PID > 0`-Gate unverändert in stale.go (§1 Was bleibt unverändert) |
| Release artefact integrity | §3 CHANGELOG + §4-8 bump-version + §9/10 verify-harness |
| Manual E2E | Dokumentiert in §9 T4-Header + `09-VALIDATION.md §Manual-Only Verifications` |

**Alle 8 Dimensionen haben einen konkreten Pattern-Anker.** Nyquist-Floor erreicht.

---

## PATTERN MAPPING COMPLETE

**Phase:** 9 - fix-stale-session-false-positive-for-uuid-sourced-claude-ses
**Files classified:** 10 (8 MODIFY + 2 CREATE)
**Analogs found:** 10 / 10 (100 % exact-match)

### Coverage
- Files with exact analog: 10
- Files with role-match analog: 0
- Files with no analog: 0

### Key Patterns Identified
- **1-Zeilen-Guard-Erweiterung im selben `if`-Ausdruck** — Phase-7 `s.Source != SourceHTTP` wird zu Phase-9 `s.Source != SourceHTTP && s.Source != SourceClaude`; Enum im selben Package, kein neuer Import.
- **Test-Rename + Assertion-Flip mit byteidentischem Setup** — Git-Blame bleibt aussagekräftig (Funktionsname + 1-Symbol-Assertion-Flip die einzigen Diffs in Kern-Pattern 2).
- **slog-Capture via `bytes.Buffer` + `slog.NewTextHandler` + `defer slog.SetDefault`** — exaktes in-repo Pattern aus `coalescer/coalescer_test.go:267-282`, synctest entfällt (D-04).
- **`bump-version.sh` 5-File-Propagation in einem Pass** — handwerkliche sed-Schleifen sind Anti-Pattern.
- **Phase-NN verify-Harness als strukturelles drop-in** — Phase-8 Template wortwörtlich übernehmen, nur T-Assertions swappen auf Phase-9-Scope.
- **Keep-a-Changelog 1.1.0 Voice (Fett-Lead-Satz + en-dash + D-NN-Zitat + inline-code-Symbole)** — `[4.1.2]` ist die exakte Phase-7-Hotfix-Präzedenz.

### File Created
`.planning/phases/09-fix-stale-session-false-positive-for-uuid-sourced-claude-ses/09-PATTERNS.md`

### Ready for Planning
Pattern-Mapping abgeschlossen. Planner kann jetzt 2 Pläne erstellen:
- `09-01-PLAN.md` — stale.go guard + godoc + 4 Regression-Tests (Pattern-Anker §1 + §2; deckt D-01, D-02, D-03, D-04, D-05, D-07, D-08)
- `09-02-PLAN.md` — v4.2.1 Release-Artefakte + human-verify-Checkpoint (Pattern-Anker §3 + §4-8 + §9/10; deckt D-06, D-09)

Git tag + push bleibt User-Step per `CLAUDE.md §Releasing` und `3-tag-push-limit`-Memory.

*Phase: 09-fix-stale-session-false-positive-for-uuid-sourced-claude-ses*
*Pattern-Extraction Date: 2026-04-18*
*Target release: v4.2.1 hotfix*
