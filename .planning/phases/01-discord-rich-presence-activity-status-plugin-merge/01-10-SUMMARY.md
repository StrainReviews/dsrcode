---
phase: 01-discord-rich-presence-activity-status-plugin-merge
plan: 10
subsystem: infra
tags: [bash, discord, statusline, curl, shell-script]

# Dependency graph
requires:
  - phase: 01-08
    provides: HTTP server with /statusline endpoint at port 19460
provides:
  - scripts/statusline-wrapper.sh — stdin→HTTP→stdout passthrough for Claude Code statusline chain
affects: [discord-app-setup, icon-generation]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Statusline chain passthrough: read stdin → POST HTTP fire-and-forget → echo stdout"
    - "curl --max-time 1 background (&) for non-blocking side-channel delivery"

key-files:
  created:
    - scripts/statusline-wrapper.sh
  modified: []

key-decisions:
  - "Task 2 (Discord Application setup) deferred: icons will be generated via fal.ai MCP first, then Discord App set up properly"
  - "curl runs in background with 1s timeout so statusline chain is never blocked by HTTP server availability"

patterns-established:
  - "Statusline wrapper pattern: always pass stdin through to stdout regardless of HTTP success/failure"

requirements-completed: [D-36, D-37, D-38]

# Metrics
duration: ~5min
completed: 2026-04-02
---

# Phase 1 Plan 10: Statusline Wrapper + Discord App Setup Summary

**Statusline wrapper script created (curl fire-and-forget passthrough); Discord Application setup deferred pending fal.ai icon generation**

## Performance

- **Duration:** ~5 min
- **Started:** 2026-04-02
- **Completed:** 2026-04-02
- **Tasks:** 1/2 (Task 2 deferred by user decision)
- **Files modified:** 1

## Accomplishments
- `scripts/statusline-wrapper.sh` created in cc-discord-presence — reads JSON from stdin, POSTs to `http://127.0.0.1:19460/statusline` via curl in background (fire-and-forget, 1s timeout), echoes original JSON to stdout
- Script passes bash syntax check (`bash -n`) and is marked executable (`chmod +x`)
- Discord Application setup deferred intentionally — icons will be generated via fal.ai MCP first to ensure quality assets before portal registration

## Task Commits

1. **Task 1: Create statusline wrapper script** - `a529e93` (feat) — in cc-discord-presence repo
2. **Task 2: Discord App setup** - SKIPPED (user decision — deferred until fal.ai icons ready)

**Plan metadata:** (pending — created after deferral)

## Files Created/Modified
- `scripts/statusline-wrapper.sh` (cc-discord-presence) — Statusline chain wrapper; reads stdin, POSTs to HTTP server at 19460/statusline, passes through to stdout. Per D-36/D-37/D-38.

## Decisions Made
- Task 2 deferred by explicit user decision: Discord Application "DSR Code" and icon uploads will happen after fal.ai MCP generates the 10 icon assets (1024x1024 PNG). This is the correct order — icons first, then portal registration.
- curl uses `--max-time 1` and runs in background (`&`) so statusline chain is non-blocking even if the Go server is not running.

## Deviations from Plan

### Known Deferrals (User Decision)

**Task 2: Discord Application "DSR Code" setup — deferred**
- **Decision made by:** User (explicit instruction before plan execution)
- **Reason:** Icon assets must be generated via fal.ai MCP first; creating the Discord App without final icons would require a second upload pass
- **Deferred until:** fal.ai icon generation is complete (10 icons: dsr-code, coding, terminal, searching, thinking, reading, idle, starting, multi-session, streaming)
- **Requirements affected:** D-45, D-46, D-47, D-56 (not yet completed)
- **Impact:** No code changes blocked — the Go server already uses a configurable `discordClientId`; the Application ID simply needs to be set in `~/.claude/discord-presence-config.json` once available

---

**Total deviations:** 0 auto-fixed; 1 known deferral (user decision, not a failure)
**Impact on plan:** Task 1 complete and functional. Task 2 is a manual human-action step (portal + icon upload) gated on asset generation. No code work was skipped.

## Issues Encountered
None during Task 1.

## User Setup Required

**Task 2 remains pending.** When fal.ai icons are ready:

1. Generate 10 icons at 1024x1024 PNG via fal.ai MCP with a dark background and green (#22c55e) accent:
   - `dsr-code`, `coding`, `terminal`, `searching`, `thinking`, `reading`, `idle`, `starting`, `multi-session`, `streaming`
2. Create Discord Application at https://discord.com/developers/applications — name it **"DSR Code"** (per D-16)
3. Copy the Application ID into `~/.claude/discord-presence-config.json` as `discordClientId`
4. Upload all 10 PNGs to **Rich Presence → Art Assets** with the exact asset key names listed above

## Known Stubs

None — the wrapper script is fully functional. The `discordClientId` in config is the only missing piece, and that is a runtime configuration value, not a code stub.

## Next Phase Readiness

- Statusline chain is wired: `statusline-wrapper.sh` → HTTP POST → Go server → Discord presence update
- Once Discord Application is set up and Application ID is configured, the full Rich Presence pipeline will be operational end-to-end
- Phase 1 has no further planned code work — all remaining items are the deferred Discord App setup (Task 2)

---
*Phase: 01-discord-rich-presence-activity-status-plugin-merge*
*Completed: 2026-04-02 (partial — Task 2 deferred)*
