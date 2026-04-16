# cc-discord-presence Roadmap

## Migration Origin

Phases migrated from StrainReviewsScanner on 2026-04-08.

| cc-discord-presence Phase | Original Phase (StrainReviewsScanner) | Status |
|---------------------------|---------------------------------------|--------|
| 1 | 13 | Complete |
| 2 | 15 | Complete |
| 3 | 16 | Complete |
| 4 | 17 | Complete |
| 5 | 18 | Complete |
| 6 | 20 | Complete |

## Phases

### Phase 1: Discord Rich Presence + Activity Status Plugin Merge
- **Status:** Complete
- **Plans:** 11/11 complete
- **Requirements:** D-01 to D-56
- **Directory:** `phases/01-discord-rich-presence-activity-status-plugin-merge/`
- **Summary:** Single Go binary with HTTP hooks, 8 display presets (200+ messages each), multi-session tracking, config hot-reload, presence debouncer, JSONL fallback, 5-platform release pipeline.

### Phase 2: DSRCodePresence Setup Wizard
- **Status:** Complete
- **Plans:** 8/8 complete
- **Requirements:** DSR-01 to DSR-42
- **Directory:** `phases/02-dsrcodepresence-setup-wizard/`
- **Summary:** 7-phase guided setup wizard, 4 display detail levels (minimal/standard/verbose/private), preview/demo mode for screenshot generation, HTTP hooks with idle detection.

### Phase 3: Fix Discord Presence Session Count + Demo Mode
- **Status:** Complete
- **Plans:** 6/6 complete
- **Requirements:** D-01 to D-25
- **Directory:** `phases/03-fix-discord-presence-session-count-and-enhance-demo-mode/`
- **Summary:** PID-based session tracking (Unix) with refcount fallback (Windows), enhanced demo mode with 4 modes (quick preview, preset tour, multi-session, message rotation).

### Phase 4: Discord Presence Enhanced Analytics
- **Status:** Complete
- **Plans:** 9/9 complete
- **Requirements:** DPA-01 to DPA-30
- **Directory:** `phases/04-discord-presence-enhanced-analytics-subagent-tracking-token-breakdown-compaction-erkennung-tool-statistiken-und-context-usage-aus-agent-monitor-portieren/`
- **Summary:** Subagent tracking, token breakdown by model, compaction detection, tool statistics, context usage display, bilingual message presets (EN/DE).

### Phase 5: Binary Distribution Pipeline + Full dsrcode Rename
- **Status:** Complete
- **Plans:** 8/8 complete
- **Requirements:** DIST-01 to DIST-50
- **Directory:** `phases/05-binary-distribution/`
- **Goal:** GitHub Releases binary distribution via GoReleaser, cross-platform start.sh rewrite for reliable daemon lifecycle, automated 5-platform build pipeline. Combined with full dsrcode rename (binary, module path, runtime files, skills, docs).

Plans:
- [x] 05-01-PLAN.md — Go module rename + version variable refactor + runtime file rename
- [x] 05-02-PLAN.md — GoReleaser config + golangci-lint + .editorconfig + .gitignore
- [x] 05-03-PLAN.md — CI workflows (release.yml + test.yml)
- [x] 05-04-PLAN.md — start.sh + start.ps1 complete rewrite (download-first + SHA256)
- [x] 05-05-PLAN.md — stop.sh + stop.ps1 overhaul + setup-statusline.sh update
- [x] 05-06-PLAN.md — bump-version.sh + plugin manifests v4.0.0 + delete build.sh
- [x] 05-07-PLAN.md — Skills update (doctor, update, setup, log)
- [x] 05-08-PLAN.md — Documentation (CLAUDE.md, CONTRIBUTING.md, README.md, MIGRATION.md)

### Phase 6: Hook System Overhaul
- **Status:** Complete
- **Plans:** 5/5 complete
- **Requirements:** D-01 to D-24
- **Directory:** `phases/06-hook-system-overhaul-sessionend-posttooluse-precompact-hooks/`
- **Goal:** 8 new hook handlers (SessionEnd, PostToolUse, PreCompact, PostCompact, StopFailure, SubagentStart, PostToolUseFailure, CwdChanged), JSONL polling removal (~250 lines), binary auto-exit with grace period, settings.local.json hook deployment, wildcard matchers.
- **Summary:** 15 hook events deployed (13 HTTP in settings.local.json + 2 plugin hooks), analytics sync bridge from tracker to registry, ~250 LOC JSONL polling removed, auto-exit with configurable grace period (30s default, 0=disabled), "error" status overlay icon, CHANGELOG v4.1.0 shipped. 14 commits across 5 plans, 100+ new tests.

Plans:
- [x] 06-01-PLAN.md — Foundation: analytics.ParseTranscript + config.ShutdownGracePeriod + error icon
- [x] 06-02-PLAN.md — Scripts: settings.local.json auto-patch (start.sh) + cleanup (stop.sh)
- [x] 06-03-PLAN.md — 8 new hook handlers in server.go
- [x] 06-04-PLAN.md — JSONL removal from main.go + auto-exit goroutine + shutdown sequence
- [x] 06-05-PLAN.md — Integration wiring + CHANGELOG + verification checkpoint

### Phase 6.1: Project Folder Rename + Claude Code Memory Migration (INSERTED)
- **Status:** Not planned
- **Plans:** 0 plans
- **Depends on:** Phase 6
- **Directory:** `phases/06.1-project-folder-rename-claude-code-memory-migration/`
- **Goal:** Rename local project folder `C:\Users\ktown\Projects\cc-discord-presence` → `C:\Users\ktown\Projects\dsrcode` and migrate the corresponding Claude Code memory directory (`C--Users-ktown-Projects-cc-discord-presence` → `C--Users-ktown-Projects-dsrcode`). Deferred manual step from Phase 5 / v4.0.0 release per `05-CONTEXT.md` lines 93 + 193. No code changes required — binary name, Go module path, and runtime files already renamed in Phase 5. Scope: stop daemon, filesystem rename, Claude memory migration, update external path references (shell/IDE/git).

Plans:
- [ ] TBD (run /gsd-plan-phase 6.1 to break down)

## Backlog

- **Discord App Setup** -- Create Discord Application in Developer Portal with custom icons. Deferred from Phase 1 Task 2 until fal.ai icon generation completes. Currently using shared "Clawd Code" app (Client ID 1455326944060248250).

## Progress

| Phase | Plans | Status |
|-------|-------|--------|
| 1 | 11/11 | Complete |
| 2 | 8/8 | Complete |
| 3 | 6/6 | Complete |
| 4 | 9/9 | Complete |
| 5 | 8/8 | Complete |
| 6 | 5/5 | Complete |
| 6.1 | 0/? | Inserted (not planned) |
| **Total** | **47/47+** | **100% (through Phase 6)** |

### Phase 7: Fix daemon auto-exit bugs: PID-dead check, MCP activity tracking, refcount drift, log overwrite

**Goal:** Fix four daemon lifecycle bugs causing self-termination during active MCP-heavy Claude Code sessions: (1) PID-liveness-check skips for HTTP-sourced sessions in stale.go, (2) handlePostToolUse updates LastActivityAt (server.go), (3) SessionEnd command hook added to plugin hooks.json so stop.sh/ps1 decrements refcount, (4) start.sh/ps1 append-to-log with 10 MB rotation instead of truncate. Cross-platform hotfix targeting v4.1.2.
**Requirements**: See `07-CONTEXT.md` §decisions (D-01..D-15)
**Depends on:** Phase 6
**Status:** Complete
**Plans:** 5/5 plans complete

Plans:
- [x] 07-01-PLAN.md — Bug #1: Skip PID-liveness check for HTTP-sourced sessions (session/stale.go + tests) — Wave 1
- [x] 07-02-PLAN.md — Bug #2: registry.Touch() method + handlePostToolUse activity-clock update — Wave 1
- [x] 07-03-PLAN.md — Bug #3: SessionEnd command hook + dual-registration to settings.local.json (plugin hooks.json + start.sh/start.ps1 + stop.sh cleanup) — Wave 1
- [x] 07-04-PLAN.md — Bug #4: Cross-platform log rotation (10 MB cap, .log.1 backup) + start.ps1 stderr-split fix — Wave 2 (depends on 07-03)
- [x] 07-05-PLAN.md — Release v4.1.2: bump-version.sh + CHANGELOG + VALIDATION.md finalization — Wave 2 (depends on 07-01..07-04)

**Summary:** All 4 daemon lifecycle bugs fixed in hotfix v4.1.2 (released 2026-04-13). Bug #1 guard expansion in `session/stale.go`, Bug #2 `registry.Touch()` + `handlePostToolUse` activity-clock, Bug #3 SessionEnd command hook with dual-registration (hooks.json + settings.local.json) per upstream issues #17885/#33458/#35892, Bug #4 portable log rotation (10 MB cap) + start.ps1 stderr-split fix. 11 commits, new test harness `test-rotate-log.sh/ps1`, GoReleaser CI green, 5-platform binaries published.

### Phase 8: Presence Rate-Limit Coalescer: Stop Drop-on-Skip (token bucket + pending-state buffer + FNV-64 change detection + hook dedup + race-free mutex)

**Goal:** Replace the drop-on-skip `presenceDebouncer` with a coalescing token-bucket rate-limiter so presence updates queued during a rate-limit cooldown are flushed exactly once when the limiter permits — never discarded. Five bundled fixes: (1) pending-state buffer + flusher via `atomic.Pointer[Activity]`, (2) `golang.org/x/time/rate` token bucket (4 s cadence, burst 2), (3) FNV-64a content-hash change detection (StartTime excluded), (4) hook-dedup middleware for duplicate POST `/hooks/*` requests (500 ms TTL, `sync.Map` + 60 s ticker cleanup), (5) race-free shared state via `atomic.Pointer` / `atomic.Uint64` / `atomic.Int64`. Tests use Go 1.25 `testing/synctest` bubbles. Ship as v4.2.0.
**Requirements:** RLC-01, RLC-02, RLC-03, RLC-04, RLC-05, RLC-06, RLC-07, RLC-08, RLC-09, RLC-10, RLC-11, RLC-12, RLC-13, RLC-14, RLC-15, RLC-16, RLC-17
**Depends on:** Phase 7
**Plans:** 4 plans

Plans:
- [x] 08-01-PLAN.md — Coalescer core: token bucket + pending-buffer + atomic state + Run/Shutdown (Wave 1)
- [x] 08-02-PLAN.md — FNV-64a content-hash change detection + hash gate in flush path (Wave 2, depends on 08-01)
- [x] 08-03-PLAN.md — HookDedupMiddleware + http.MaxBytesReader + Server wiring + dedup getter injection (Wave 2, depends on 08-01)
- [ ] 08-04-PLAN.md — Release v4.2.0: CHANGELOG + bump-version.sh + verify.sh/ps1 T1-T6 + human-verify checkpoint (Wave 3, depends on 08-01/02/03)

---
*Last updated: 2026-04-16 (Phase 8 planned — 4 plans, 17 requirements, v4.2.0 target)*
