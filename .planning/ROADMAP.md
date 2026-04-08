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

### Phase 5: Binary Distribution Pipeline
- **Status:** Pending
- **Plans:** 0/3 executed
- **Requirements:** DIST-01 to DIST-10
- **Directory:** `phases/05-binary-distribution/`
- **Goal:** GitHub Releases binary distribution via goreleaser, cross-platform start.sh rewrite for reliable daemon lifecycle, automated 5-platform build pipeline.

### Phase 6: Hook System Overhaul
- **Status:** Context only
- **Plans:** 0/0 (not yet planned)
- **Requirements:** TBD
- **Directory:** `phases/06-hook-system-overhaul-sessionend-posttooluse-precompact-hooks/`
- **Goal:** SessionEnd/PostToolUse/PreCompact hooks, JSONL polling removal (hook-triggered reads), binary auto-exit when all sessions end, stale handling event-based, compaction via PreCompact, extended matchers.

### Phase 7: GitHub Repo Transfer [To be planned]
- **Status:** Not started
- **Plans:** TBD
- **Requirements:** TBD
- **Goal:** Transfer GitHub repository from StrainReviews/dsrcode to DSR-Labs/cc-discord-presence. Update CI/CD, download URLs, redirects, plugin install command.

## Backlog

- **Discord App Setup** -- Create Discord Application in Developer Portal with custom icons. Deferred from Phase 1 Task 2 until fal.ai icon generation completes. Currently using shared "Clawd Code" app (Client ID 1455326944060248250).

## Progress

| Phase | Plans | Status |
|-------|-------|--------|
| 1 | 11/11 | Complete |
| 2 | 8/8 | Complete |
| 3 | 6/6 | Complete |
| 4 | 9/9 | Complete |
| 5 | 0/3 | Pending |
| 6 | 0/0 | Context only |
| 7 | TBD | Not started |
| **Total** | **34/37** | **91%** |

---
*Last updated: 2026-04-08*
