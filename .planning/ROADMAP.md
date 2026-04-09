# cc-discord-presence Roadmap

## Migration Origin

Phases migrated from StrainReviewsScanner on 2026-04-08.

| cc-discord-presence Phase | Original Phase (StrainReviewsScanner) | Status |
|---------------------------|---------------------------------------|--------|
| 1 | 13 | Complete |
| 2 | 15 | Complete |
| 3 | 16 | Complete |
| 4 | 17 | Complete |
| 5 | 18 | Pending |
| 6 | 20 | Context only |

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
- **Status:** Planned
- **Plans:** 8 plans in 4 waves
- **Requirements:** DIST-01 to DIST-50
- **Directory:** `phases/05-binary-distribution/`
- **Goal:** GitHub Releases binary distribution via GoReleaser, cross-platform start.sh rewrite for reliable daemon lifecycle, automated 5-platform build pipeline. Combined with full dsrcode rename (binary, module path, runtime files, skills, docs).

Plans:
- [ ] 05-01-PLAN.md — Go module rename + version variable refactor + runtime file rename
- [ ] 05-02-PLAN.md — GoReleaser config + golangci-lint + .editorconfig + .gitignore
- [ ] 05-03-PLAN.md — CI workflows (release.yml + test.yml)
- [ ] 05-04-PLAN.md — start.sh + start.ps1 complete rewrite (download-first + SHA256)
- [ ] 05-05-PLAN.md — stop.sh + stop.ps1 overhaul + setup-statusline.sh update
- [ ] 05-06-PLAN.md — bump-version.sh + plugin manifests v4.0.0 + delete build.sh
- [ ] 05-07-PLAN.md — Skills update (doctor, update, setup, log)
- [ ] 05-08-PLAN.md — Documentation (CLAUDE.md, CONTRIBUTING.md, README.md, MIGRATION.md)

### Phase 6: Hook System Overhaul
- **Status:** Context only
- **Plans:** 0/0 (not yet planned)
- **Requirements:** TBD
- **Directory:** `phases/06-hook-system-overhaul-sessionend-posttooluse-precompact-hooks/`
- **Goal:** SessionEnd/PostToolUse/PreCompact hooks, JSONL polling removal (hook-triggered reads), binary auto-exit when all sessions end, stale handling event-based, compaction via PreCompact, extended matchers.

## Backlog

- **Discord App Setup** -- Create Discord Application in Developer Portal with custom icons. Deferred from Phase 1 Task 2 until fal.ai icon generation completes. Currently using shared "Clawd Code" app (Client ID 1455326944060248250).

## Progress

| Phase | Plans | Status |
|-------|-------|--------|
| 1 | 11/11 | Complete |
| 2 | 8/8 | Complete |
| 3 | 6/6 | Complete |
| 4 | 9/9 | Complete |
| 5 | 0/8 | Planned |
| 6 | 0/0 | Context only |
| **Total** | **34/42** | **81%** |

---
*Last updated: 2026-04-09*
