---
phase: 06-hook-system-overhaul
plan: 01
subsystem: infra
tags: [go, jsonl, bufio, time.Duration, fsnotify, hooks, foundation]

requires:
  - phase: 04-discord-presence-enhanced-analytics
    provides: TokenBreakdown struct, analytics package layout
  - phase: 05-binary-distribution
    provides: dsrcode module path, v4.0.0 baseline
provides:
  - analytics.ParseTranscript(path) for hook-triggered JSONL reads
  - analytics.TranscriptResult struct (Tokens map, CompactionCount, LastModel, ProjectPath)
  - config.Config.ShutdownGracePeriod (time.Duration, 30s default, 0=disabled)
  - CC_DISCORD_SHUTDOWN_GRACE env var
  - shutdownGracePeriod JSON config field (string or int seconds)
  - preset.AllActivityIcons() returns 8 icons including "error" status overlay
affects:
  - Plan 06-03 (hook handlers — will call ParseTranscript from PostToolUse, PreCompact, SessionEnd)
  - Plan 06-04 (JSONL removal + auto-exit goroutine — will consume ShutdownGracePeriod and remove parseJSONLSession)
  - Plan 06-05 (integration wiring + StopFailure presence — will set SmallImage="error")

tech-stack:
  added: []
  patterns:
    - Pure read function in analytics package — no rate limiting (caller's responsibility)
    - 1 MB bufio.Scanner buffer (matches legacy main.go) — must be explicit to avoid silent truncation
    - Status-overlay activity icon (error) with no per-preset message pool
    - durationOrInt-based hot-reloadable config field

key-files:
  created:
    - analytics/transcript.go
    - analytics/transcript_test.go
  modified:
    - config/config.go
    - config/config_test.go
    - preset/types.go
    - preset/preset_test.go

key-decisions:
  - "ParseTranscript empty path returns zero-value (not error) so hook handlers can pass through unset transcript_path without branching"
  - "transcriptMessage struct kept private to analytics — main.go JSONLMessage will be removed in Plan 06-04, no shared type to maintain"
  - "Malformed JSONL lines are skipped (corruption-tolerant) matching legacy parseJSONLSession behavior"
  - "scanner.Err() check after the loop is mandatory — catches ErrTooLong and partial reads (golang/go#26431, github/gh-aw#20028)"
  - "ShutdownGracePeriod hot-reload requires no watcher.go change — Defaults+applyFileConfig+applyEnvVars run on every reload"
  - "Zero ShutdownGracePeriod is the disabled sentinel; auto-exit code in Plan 06-04 will branch on cfg.ShutdownGracePeriod == 0"
  - "AllActivityIcons returns 8 icons but error has no preset messages; preset_test.go skips error in pool-iteration tests"

patterns-established:
  - "Hook-triggered JSONL read pattern: analytics.ParseTranscript(transcriptPath) → TranscriptResult, with throttling owned by caller"
  - "Status-overlay activity icon pattern: SmallImage swap without per-preset message pool (D-19)"
  - "Additive Config field with 30s default + special 0=disabled sentinel + env var override + hot-reload via existing watcher path"

requirements-completed: [D-01, D-02, D-03, D-05, D-15, D-17, D-18, D-19-icon]

duration: 65min
completed: 2026-04-10
---

# Phase 6 / Plan 01: Foundation Summary

**analytics.ParseTranscript() pure-read JSONL helper, Config.ShutdownGracePeriod with hot-reload, and 8th preset activity icon (error) for StopFailure status overlay**

## Performance

- **Duration:** ~65 min
- **Started:** 2026-04-10T13:00Z (approx)
- **Completed:** 2026-04-10T14:20Z (approx)
- **Tasks:** 2
- **Files modified:** 4 (created 2, modified 4)

## Accomplishments

- **analytics.ParseTranscript** — pure read function with 1 MB scanner buffer that hook handlers (PostToolUse, PreCompact, SessionEnd) will call with `transcript_path` from the payload. Returns per-model TokenBreakdown, compaction count, last model ID, and project path. Empty path is a no-op so handlers do not need to branch on missing payload fields.
- **TranscriptResult struct** — replaces the inline parser in main.go (which Plan 06-04 will delete) with a reusable, decoupled API.
- **Config.ShutdownGracePeriod** — new `time.Duration` field, 30s default, hot-reloadable through the existing fsnotify watcher path. Special value 0 disables auto-exit (legacy behavior). Available via `CC_DISCORD_SHUTDOWN_GRACE` env var or `shutdownGracePeriod` JSON field (string `"30s"` or integer seconds).
- **preset.AllActivityIcons** — now returns 8 icons including `error`, the StopFailure status overlay (D-19). Doc comment makes the contract explicit: "error" has no per-preset messages because State and Details continue to show the previous activity.
- **14 new tests** — 10 ParseTranscript tests + 4 ShutdownGracePeriod tests, all passing.

## Task Commits

Each task was committed atomically with full PRE+POST MCP-Verification rounds:

1. **Task 1: analytics.ParseTranscript** — `e47b191` (feat)
2. **Task 2: Config.ShutdownGracePeriod + preset error icon** — `e804e7c` (feat)

## Files Created/Modified

- `analytics/transcript.go` — `ParseTranscript(path)` and `TranscriptResult` struct (created, 117 lines)
- `analytics/transcript_test.go` — 10 unit tests covering empty path, missing/empty file, single message, per-model accumulation, multiple models, compaction counting, malformed-line tolerance, project-path-first-wins, and ignored non-assistant messages (created, 198 lines)
- `config/config.go` — new `ShutdownGracePeriod` field on Config + fileConfig, default in `Defaults()`, file merge in `applyFileConfig`, env var in `applyEnvVars` (modified, +17 lines net)
- `config/config_test.go` — 4 new tests: default 30s, env override "5m", "0s" disabled, file string "2m" (modified, +52 lines)
- `preset/types.go` — `AllActivityIcons()` now returns 8 icons including `error`; new doc comment explaining the status-overlay contract (modified, +9 lines net)
- `preset/preset_test.go` — `TestPresetHasRequiredFields` and `TestPresetMessageCounts` skip the `error` icon since presets do not provide per-icon messages for status overlays (modified, +6 lines)

## Decisions Made

- **transcriptMessage struct kept private to analytics** — Plan 06-04 will delete `main.go.JSONLMessage`, so a shared type would be temporary cruft. Better to duplicate the small struct in `analytics` and let the legacy one disappear cleanly.
- **scanner.Err() check after the Scan loop is mandatory** — without it, oversized lines silently truncate (confirmed via golang/go#26431, github/gh-aw#20028). Added explicit error wrapping.
- **Empty path is a zero-result, not an error** — hooks may pass through with unset `transcript_path`. Per Plan-spec, the convenience-API saves the caller a nil-check.
- **Zero ShutdownGracePeriod = disabled sentinel** — clean, no extra bool field. Plan 06-04's auto-exit code will branch on `cfg.ShutdownGracePeriod == 0`.
- **Hot-reload via existing watcher.go path** — `WatchConfig`'s `runWatchLoop` already calls `Defaults` → `applyFileConfig` → `applyEnvVars` on every change, so the new field flows through automatically without watcher.go edits.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 — Test breakage] preset_test.go updated to skip "error" icon**
- **Found during:** Task 2 (preset/types.go AllActivityIcons update)
- **Issue:** `TestPresetHasRequiredFields` and `TestPresetMessageCounts` iterate every icon returned by `AllActivityIcons()` and assert each has a non-empty `SingleSessionDetails` pool with >= 40 messages. Adding `"error"` to the icon list would break all 8 presets because none provide per-icon messages for `error` (it is a status overlay, not a normal activity per D-19).
- **Fix:** Added `if icon == "error" { continue }` skip in both test loops with a code comment referencing D-19. Doc comments for both tests updated to mention the contract.
- **Files modified:** `preset/preset_test.go`
- **Verification:** Both tests still pass; the contract that "every preset must define every preset-displayable icon" is preserved while the new status-overlay icon is correctly excluded.
- **Committed in:** `e804e7c` (Task 2 commit)

**2. [Rule 2 — Missing critical functionality] 4 ShutdownGracePeriod tests added**
- **Found during:** Task 2 (config/config.go ShutdownGracePeriod implementation)
- **Issue:** Plan 06-01 did not specify tests for the new config field, but D-05 has three observable behaviors that need validation (default 30s, env override, zero=disabled, file override).
- **Fix:** Added `TestConfigShutdownGracePeriodDefault`, `TestConfigShutdownGracePeriodEnvOverride`, `TestConfigShutdownGracePeriodZeroDisables`, `TestConfigShutdownGracePeriodFromFile` following the existing test patterns.
- **Files modified:** `config/config_test.go`
- **Verification:** All 4 new tests pass; existing 18 config tests untouched and still pass.
- **Committed in:** `e804e7c` (Task 2 commit)

---

**Total deviations:** 2 auto-fixed (1 test breakage from architectural cohesion, 1 missing-test coverage)
**Impact on plan:** Both deviations are necessary corollaries of the planned changes — no scope creep, no architectural decisions beyond what D-05 and D-19 already specified.

## Issues Encountered

- **CGO unavailable** — `go test -race` requires `CGO_ENABLED=1`, which is not configured on this system. Per the execution prompt's explicit allowance ("drop -race if CGO unavailable, document in SUMMARY"), tests were run without `-race`. Both `analytics.ParseTranscript` and the config code paths are pure single-goroutine (no shared mutable state, no concurrent access in the test scenarios), so race detection would not have surfaced bugs in this plan's surface area. Future plans (especially 06-04 with the auto-exit goroutine) should re-enable `-race` once a CGO-enabled environment is available.

## MCP Verification Trace

Both tasks completed the full PRE+POST MCP protocol per the Phase-6 mandate. Total MCP calls: **16** (4 per pre-step × 2 tasks + 4 per post-step × 2 tasks).

### Task 1: analytics.ParseTranscript

**PRE-STEP:**
- `sequential-thinking`: 2 thoughts decomposing into 4 sub-steps, choosing direct-portation hypothesis (HYPOTHESE A) over refactor (HYPOTHESE B), tracing D-01/D-02/D-03/D-15/D-17/D-18 alignment
- `context7` (`/websites/pkg_go_dev_go1_25_3`): confirmed `Scanner.Buffer(buf, max)` signature, `Scanner.Err` returns nil on io.EOF, `MaxScanTokenSize=64K`, `Scan` semantics ("scanning stops unrecoverably at EOF, the first I/O error, or a token too large")
- `exa`: 2 queries — confirmed scanner.Buffer + scanner.Err is the correct anti-truncation pattern (github/gh-aw#20028 silent partial processing carry-over from Sergo audit), `t.TempDir()` is the modern test-fixture API since Go 1.15
- `crawl4ai` (`pkg.go.dev/bufio#Scanner.Buffer`): full-text deep read of bufio Scanner docs — verified all method signatures used in implementation

**POST-STEP:**
- `sequential-thinking`: reflected on hypothesis success, confirmed all 6 D-* alignment, identified no missing tests, signed off code quality
- `context7`: cross-checked `fmt.Errorf %w` wrapping pattern for `Unwrap()` chains, confirmed `json.Unmarshal` ignores unknown struct keys by default (no need to mark fields strict)
- `exa`: searched for recent JSON-parser CVEs — confirmed GO-2026-4514 only affects `buger/jsonparser`, not stdlib `encoding/json` used here
- `crawl4ai` (`pkg.go.dev/encoding/json#Unmarshal`): confirmed "unknown keys ignored" behavior for partial-schema parsing

### Task 2: Config.ShutdownGracePeriod + preset error icon

**PRE-STEP:**
- `sequential-thinking`: 2 thoughts decomposing into 8 sub-steps across 4 files, evaluating Hypothesen A (additive) vs B (reorder), discovering and resolving the preset_test.go architectural conflict (Rule 1 fix vs Rule 4 stop)
- `context7` (`/websites/pkg_go_dev_go1_25_3`): confirmed `time.ParseDuration` accepts `"30s"`, `"5m"`, `"1h30m"`, `"0s"`; constants for Nanosecond/Microsecond/Millisecond/Second/Minute/Hour
- `exa`: 2 queries — confirmed `time.Duration(0)` as disabled sentinel is an established pattern (golang/go#15904), `*durationOrInt` pointer + `omitempty` is the standard Go optional-field idiom (Go 1.26 `new(expr)` improvements article)
- `crawl4ai` (`pkg.go.dev/time#ParseDuration`): full-text verification of valid ParseDuration unit suffixes ("ns", "us", "ms", "s", "m", "h")

**POST-STEP:**
- `sequential-thinking`: reflected on all 8 sub-steps completed, confirmed hot-reload works without watcher.go edits, documented both deviations
- `context7`: confirmed `os.Getenv` returns empty string for unset vars, `t.Setenv` from testing.TB is the safe in-test pattern with auto-cleanup
- `exa`: validated additive Config struct field with `*Pointer + omitempty` is the v1 + v2-compatible JSON pattern (jsonv2 experimental note)
- `crawl4ai` (`pkg.go.dev/os#Getenv`): final cross-check that `if v != ""` matches the "Getenv returns empty if not present" semantics, recommendation to use `LookupEnv` only when distinguishing unset-vs-empty matters (not the case here)

## Next Plan Readiness

Plan 06-02 (settings.local.json auto-patch in start.sh + cleanup in stop.sh) is unblocked — it touches scripts only, no Go compile dependency on this plan.

Plan 06-03 (8 hook handlers in server.go) is unblocked — it depends on `analytics.ParseTranscript` (delivered in Task 1).

Plan 06-04 (JSONL removal + auto-exit goroutine) is unblocked — it depends on both `analytics.ParseTranscript` (replaces parseJSONLSession) and `config.ShutdownGracePeriod` (auto-exit timer).

Plan 06-05 (integration + StopFailure presence) is unblocked — it depends on `preset.AllActivityIcons()` returning `"error"` to allow the `SmallImage="error"` swap during StopFailure.

Verification gates passed:
- `go test -v ./analytics/... ./config/... ./preset/...` — 32 tests pass (10 new ParseTranscript + 22 config including 4 new + 10 preset)
- `go build ./...` — clean
- `go vet ./...` — clean
- `gofmt -l <modified files>` — clean (no output)

---
*Phase: 06-hook-system-overhaul-sessionend-posttooluse-precompact-hooks*
*Plan: 01 (Foundation)*
*Completed: 2026-04-10*
