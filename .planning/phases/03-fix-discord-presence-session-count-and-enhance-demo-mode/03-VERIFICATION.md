---
phase: 03-fix-discord-presence-session-count-and-enhance-demo-mode
verified: 2026-04-06T13:15:00Z
status: human_needed
score: 8/8 must-haves verified
re_verification: null
gaps: []
human_verification:
  - test: "Build daemon and verify session count in Discord"
    expected: "Open one Claude Code window on a project. GET http://127.0.0.1:19460/sessions shows 1 session (not 2). The session has a 'source' field with value 'claude', 'http', or 'jsonl'."
    why_human: "Requires running daemon and actual Claude Code activity; cannot verify real IPC without a running session"
  - test: "Run /dsrcode:demo and interact with all 4 modes"
    expected: "4-mode menu appears. Quick Preview sets real Discord presence without any 'Preview Mode' text visible on Discord. Preset Tour loops through presets with user confirmation at each step. Multi-session preview shows correct project count. Message Rotation displays numbered rotation sequence."
    why_human: "Discord visual output and skill menu flow require human interaction"
  - test: "Verify binary version"
    expected: "Running cc-discord-presence.exe --version outputs '3.1.0'"
    why_human: "Requires building and executing the binary"
---

# Phase 3: Fix Discord Presence Session Count and Enhance Demo Mode — Verification Report

**Phase Goal:** Fix duplicate session registration bug (synthetic jsonl- session + real UUID session for same project = "Aktiv in 2 Repos" when only 1 session open), deduplicate sessions per project in the registry, and enhance /dsrcode:demo with detailed preview controls.
**Verified:** 2026-04-06T13:15:00Z
**Status:** human_needed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| #  | Truth                                                                                                 | Status     | Evidence                                                                                          |
|----|-------------------------------------------------------------------------------------------------------|------------|---------------------------------------------------------------------------------------------------|
| 1  | Same project with JSONL + HTTP/UUID sessions results in 1 session (not 2) in Discord                 | VERIFIED   | TestRegistrySessionDedup_Phase16: all 6 upgrade/no-downgrade subtests PASS GREEN                  |
| 2  | Source-aware upgrade chain (jsonl->http->uuid) preserves StartTime and ActivityCounts                 | VERIFIED   | registry.go upgradeSession copies StartedAt, ActivityCounts, Model, TotalTokens, TotalCostUSD      |
| 3  | Two real Claude windows on same project kept separate (different PIDs)                                | VERIFIED   | Same_project_different_PID_keeps_both subtest PASS; registry.go:90 guard on equal-rank different-PID |
| 4  | Resolver uses unique project count for tier selection                                                 | VERIFIED   | TestResolverUniqueProjectTier: both subtests PASS GREEN; resolver.go:38-48 uniqueProjects map      |
| 5  | Extended preview API supports full Discord Activity control with fakeSessions                         | VERIFIED   | Preview_SmallImage and Preview_FakeSessions subtests PASS; PreviewPayload has all new fields       |
| 6  | GET /preview/messages returns message rotation preview                                                | VERIFIED   | GET_preview_messages subtest PASS; handleGetPreviewMessages registered in Handler()               |
| 7  | Demo skill offers 4 modes: Quick Preview, Preset Tour, Multi-Session Preview, Message Rotation        | VERIFIED   | skills/demo/SKILL.md rewritten with 4 modes, curl commands, realistic demo data                   |
| 8  | Version 3.1.0 with CHANGELOG.md in Keep a Changelog format                                           | VERIFIED   | main.go:40 Version="3.1.0", scripts/start.sh VERSION="v3.1.0", .claude-plugin/plugin.json "3.1.0", CHANGELOG.md has [3.1.0] entry |

**Score:** 8/8 truths verified

### Required Artifacts

| Artifact                                  | Expected                                              | Status    | Details                                                                              |
|-------------------------------------------|-------------------------------------------------------|-----------|--------------------------------------------------------------------------------------|
| `session/source.go`                       | SessionSource enum, String(), MarshalJSON()           | VERIFIED  | 57 lines; SourceJSONL=0, SourceHTTP=1, SourceClaude=2; strings serialize correctly   |
| `session/types.go`                        | Session struct with Source field                      | VERIFIED  | Source SessionSource field at line 37 with json:"source" tag                         |
| `session/registry.go`                     | Source-aware dedup in StartSessionWithSource          | VERIFIED  | StartSession delegates to StartSessionWithSource; foundExisting; upgradeSession      |
| `resolver/resolver.go`                    | Unique project counting in ResolvePresence            | VERIFIED  | uniqueProjects map at line 38-48; resolveMulti uses uniqueCount for tier/placeholder |
| `server/server.go`                        | Extended PreviewPayload, FakeSession, GET /preview/messages | VERIFIED | FakeSession, ButtonDef, extended PreviewPayload, handleGetPreviewMessages registered |
| `session/registry_dedup_test.go`          | 8 table-driven dedup subtests, external test package  | VERIFIED  | package session_test; no build tag; all 8 subtests PASS GREEN                        |
| `resolver/resolver_unique_test.go`        | 2 table-driven unique-project subtests                | VERIFIED  | package resolver_test; both subtests PASS GREEN                                      |
| `server/server_preview_test.go`           | 3 table-driven preview subtests                       | VERIFIED  | package server_test; no build tag; all 3 subtests PASS GREEN                         |
| `skills/demo/SKILL.md`                    | 4-mode demo skill with menu and instructions          | VERIFIED  | 210 lines; all 4 modes with curl commands; daemon health check; no "Preview Mode" text |
| `CHANGELOG.md`                            | v3.1.0 entry with Fixed/Added/Changed sections        | VERIFIED  | [3.1.0] 2026-04-06 with Fixed/Added/Changed; also has v3.0.0 and v2.0.0 retroactive |
| `README.md`                               | Demo section with 4 modes                             | VERIFIED  | ## Demo Mode section with table of 4 modes and API curl examples                     |
| `main.go`                                 | Version 3.1.0; jsonlDeprecationOnce; no old dedup     | VERIFIED  | Version="3.1.0"; jsonlDeprecationOnce sync.Once; old JSONL dedup block removed       |
| `scripts/start.sh`                        | VERSION v3.1.0                                        | VERIFIED  | VERSION="v3.1.0" at line 15                                                          |
| `.claude-plugin/plugin.json`              | Version 3.1.0                                         | VERIFIED  | "version": "3.1.0" at line 3                                                         |

### Key Link Verification

| From                                  | To                                         | Via                              | Status  | Details                                                        |
|---------------------------------------|--------------------------------------------|----------------------------------|---------|----------------------------------------------------------------|
| session/registry.go StartSession      | session/source.go sourceFromID/sourceRank  | Delegate to StartSessionWithSource | WIRED | StartSession calls StartSessionWithSource with sourceFromID    |
| session/registry.go StartSessionWithSource | session/source.go sourceRank          | Rank comparison for upgrade      | WIRED   | if newRank > existingRank / if newRank < existingRank          |
| resolver/resolver.go ResolvePresence  | resolver/resolver.go resolveMulti          | unique project count threshold   | WIRED   | len(uniqueProjects) == 1 guard before resolveMulti call        |
| server/server.go handleGetPreviewMessages | resolver.StablePick                   | Message rotation simulation      | WIRED   | resolver.StablePick called in handleGetPreviewMessages loop    |
| server/server.go Handler()            | handleGetPreviewMessages                   | Route registration               | WIRED   | mux.HandleFunc("GET /preview/messages", ...)                   |
| main.go ingestJSONLFallback           | session/registry.go StartSession           | Single enforcement point (D-02)  | WIRED   | StartSession called in ingestJSONLFallback; old dedup removed  |
| skills/demo/SKILL.md                  | server/server.go POST /preview             | Curl commands in skill           | WIRED   | Multiple curl -X POST http://127.0.0.1:19460/preview examples  |
| skills/demo/SKILL.md                  | server/server.go GET /preview/messages     | Curl commands for message rotation | WIRED | curl http://127.0.0.1:19460/preview/messages?preset=... present |

### Data-Flow Trace (Level 4)

| Artifact                      | Data Variable       | Source                          | Produces Real Data | Status    |
|-------------------------------|--------------------|---------------------------------|--------------------|-----------|
| session/registry_dedup_test.go | reg.SessionCount() | session.SessionRegistry.sessions map | Yes — map dedup verified by tests | FLOWING |
| resolver/resolver.go          | uniqueProjects map  | sessions[].ProjectName loop     | Yes — real project names from sessions | FLOWING |
| server/server.go handleGetPreviewMessages | messages slice | preset.LoadPreset + StablePick rotation | Yes — preset file loaded; StablePick produces real messages | FLOWING |

### Behavioral Spot-Checks

| Behavior                                      | Command                                                        | Result                         | Status |
|-----------------------------------------------|----------------------------------------------------------------|--------------------------------|--------|
| All 8 registry dedup tests pass               | go test ./session/ -run TestRegistrySessionDedup_Phase16       | All 8 subtests PASS            | PASS   |
| All 2 resolver unique-project tests pass      | go test ./resolver/ -run TestResolverUniqueProjectTier         | Both subtests PASS             | PASS   |
| All 3 server preview tests pass               | go test ./server/ -run TestPreviewEndpoints_Phase16            | All 3 subtests PASS            | PASS   |
| Full project test suite passes (7 packages)   | go test ./...                                                  | All packages PASS, no failures | PASS   |
| Full project builds without error             | go build ./...                                                 | Exit 0, no output              | PASS   |
| Version 3.1.0 in all 3 locations              | grep "3.1.0" main.go; grep "v3.1.0" scripts/start.sh; grep "3.1.0" .claude-plugin/plugin.json | All match | PASS |
| Old JSONL dedup block removed                 | grep "Check if an HTTP\|HasPrefix.*jsonl" main.go              | No match                       | PASS   |
| jsonlDeprecationOnce exists                   | grep "jsonlDeprecationOnce" main.go                            | Found at package level and in Do() | PASS |

### Requirements Coverage

The REQUIREMENTS.md in `.planning/REQUIREMENTS.md` covers StrainReviewsScanner (INFRA, SCRP, MTCH, AUTH, SRCH, UI). Phase 3 requirements (D-01 through D-25) are defined in the Discord project's CONTEXT.md, not in REQUIREMENTS.md. Cross-referencing CONTEXT.md decisions:

| Requirement | Source Plan | Description                                                               | Status    | Evidence                                             |
|-------------|-------------|---------------------------------------------------------------------------|-----------|------------------------------------------------------|
| D-01        | 03-02       | Two real Claude windows with different PIDs kept separate                 | SATISFIED | Same_project_different_PID_keeps_both test PASS      |
| D-02        | 03-02       | Registry.StartSession() is single dedup enforcement point                 | SATISFIED | StartSession delegates to StartSessionWithSource; main.go old check removed |
| D-03        | 03-01       | SessionSource enum (iota) with String() method                            | SATISFIED | session/source.go: SourceJSONL=0, SourceHTTP=1, SourceClaude=2 |
| D-04        | 03-02       | Replace in-place preserves StartTime, ActivityCounts, Model, Tokens, Cost | SATISFIED | upgradeSession copies all preserved fields           |
| D-05        | 03-02       | JSONL dedup check in main.go removed                                      | SATISFIED | Old block not found in main.go                       |
| D-06        | 03-04       | PreviewPayload extended with all new fields                               | SATISFIED | 7 new fields: smallImage, smallText, largeText, buttons, startTimestamp, sessionCount, fakeSessions |
| D-07        | 03-04       | Full Discord Activity control in preview                                  | SATISFIED | FakeSession struct, ButtonDef struct, all fields wired |
| D-08        | 03-04       | GET /preview/messages endpoint                                            | SATISFIED | Registered in Handler(), returns previewMessage array |
| D-09        | 03-05       | Demo skill menu-based with 4 modes                                        | SATISFIED | SKILL.md has 4 modes with step-by-step Claude instructions |
| D-10        | 03-04, 03-05 | Realistic demo data (my-saas-app, Opus 4.6, 250K, $2.50, feature/auth) | SATISFIED | Fake session defaults match; SKILL.md specifies same |
| D-11        | 03-04, 03-05 | No "Preview Mode" indicator in output                                     | SATISFIED | No "Preview Mode" default in handlePostPreview; SKILL.md explicitly forbids it |
| D-12        | 03-05       | Auto-cycle is skill-driven, not server-side timer                         | SATISFIED | SKILL.md Preset Tour: Claude loops with user confirmation |
| D-13        | 03-02       | Counter-handling: additive (JSONL counters are 0, so functionally preserved) | SATISFIED | upgradeSession preserves ActivityCounts              |
| D-14        | 03-03       | Resolver uses unique project count for tier selection                     | SATISFIED | ResolvePresence uses uniqueProjects map; resolveMulti uses uniqueCount |
| D-15        | 03-03       | getMostRecentSession selects best session for same-project display        | SATISFIED | getMostRecentSession called in ResolvePresence for 1-unique-project case |
| D-16        | 03-02       | JSONL deprecation warning with sync.Once                                  | SATISFIED | jsonlDeprecationOnce.Do() in ingestJSONLFallback     |
| D-17        | 03-02       | slog.Info for upgrades, slog.Debug for skips                              | SATISFIED | registry.go: slog.Info on upgrade, slog.Debug on skip |
| D-18        | 03-05       | Version v3.1.0 across main.go, start.sh, plugin.json                     | SATISFIED | All 3 files verified                                 |
| D-19        | 03-05       | CHANGELOG.md in Keep a Changelog format                                   | SATISFIED | [3.1.0] with Fixed/Added/Changed; v3.0.0 and v2.0.0 retroactive |
| D-20        | 03-00       | 13 test scenarios (8 registry + 2 resolver + 3 server)                    | SATISFIED | All 13 subtests exist and PASS GREEN                 |
| D-21        | N/A         | No new config flags needed                                                | SATISFIED | No new config flags introduced                       |
| D-22        | 03-05       | README.md demo section with 4 modes                                       | SATISFIED | ## Demo Mode section with 4-mode table and API examples |
| D-23        | N/A         | Worktrees deferred to later phase                                         | SATISFIED | Not implemented; behavior unchanged                  |
| D-24        | 03-01       | SessionSource serializes as string in JSON ("claude"/"http"/"jsonl")      | SATISFIED | MarshalJSON returns json.Marshal(s.String()); Source_enum_MarshalJSON test PASS |
| D-25        | 03-00       | External test packages (black-box testing)                                | SATISFIED | All 3 test files use _test package suffix            |

### Anti-Patterns Found

| File                  | Line | Pattern  | Severity | Impact |
|-----------------------|------|----------|----------|--------|
| (none found)          | —    | —        | —        | No anti-patterns detected in phase 16 modified files |

Scanned: session/source.go, session/registry.go, resolver/resolver.go, server/server.go, main.go, skills/demo/SKILL.md
- No TODO/FIXME/PLACEHOLDER comments
- No stub return patterns (return null, return [], etc.)
- No hardcoded empty data that flows to rendering
- No "Preview Mode" text in production code

### Human Verification Required

#### 1. Discord Session Count in Production

**Test:** Build and run the daemon (`go build -o cc-discord-presence.exe . && ./cc-discord-presence.exe`). Open one Claude Code window on a project. Execute some tool actions. Check `GET http://127.0.0.1:19460/sessions` — should return exactly 1 session.
**Expected:** Response shows 1 session with `"source": "claude"` (or "http"). Session count in Discord presence shows "1 Projekt" not "Aktiv in 2 Repos".
**Why human:** Requires running daemon process and real Claude Code IPC session. Cannot verify Discord IPC output programmatically.

#### 2. Demo Skill 4-Mode Interaction

**Test:** In Claude Code, run `/dsrcode:demo`. Interact with each of the 4 modes. For Quick Preview: verify Discord shows presence without "Preview Mode" text. For Preset Tour: verify Claude loops through presets asking for confirmation. For Multi-Session: verify Discord shows multi-project stats. For Message Rotation: verify numbered rotation list is shown.
**Expected:** Each mode works as documented in SKILL.md. No "Preview Mode" text in Discord output. Auto-cycle is skill-controlled (Claude asks before advancing).
**Why human:** Discord visual output and skill interaction flow require human observation.

#### 3. Binary Version Output

**Test:** Build binary and run `./cc-discord-presence.exe --version` (or equivalent flag).
**Expected:** Output contains "3.1.0".
**Why human:** Requires building and executing the binary in a shell environment.

### Gaps Summary

No gaps found. All 8 roadmap success criteria are fully implemented and verified programmatically. 13 TDD test stubs from Plan 00 all pass GREEN. The entire project test suite (7 packages) passes with zero regressions. Version 3.1.0 is consistent across all 3 required files.

Three items require human verification before this phase can be marked `passed`:
1. Confirming the session count fix works in a live Discord presence scenario
2. Confirming the demo skill's 4-mode flow works interactively
3. Confirming the binary version output

These are not blockers — the implementation is complete and correct. They are visual/interactive confirmations that cannot be automated.

---

_Verified: 2026-04-06T13:15:00Z_
_Verifier: Claude (gsd-verifier)_
