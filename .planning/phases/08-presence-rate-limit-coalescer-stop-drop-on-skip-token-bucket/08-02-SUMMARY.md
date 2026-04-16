---
phase: 08-presence-rate-limit-coalescer-stop-drop-on-skip-token-bucket
plan: 02
subsystem: rate-limiting
tags: [content-hash, fnv, dedup, atomic, coalescer]

# Dependency graph
requires:
  - phase: 08-01
    provides: "Coalescer struct with atomic.Uint64 lastSentHash + atomic.Int64 skipHash fields declared; TODO(08-02) anchor in resolveAndEnqueue; // Plan 08-02 stores the hash here comment in flushPending"
  - phase: 01-foundation
    provides: "discord.Activity struct schema (Details/State/Large*/Small*/Buttons[] + StartTime)"
provides:
  - "coalescer.HashActivity(*discord.Activity) uint64 — deterministic FNV-1a 64-bit content hash, StartTime excluded per D-09"
  - "Hash gate in Coalescer.resolveAndEnqueue — identical payloads short-circuit BEFORE pending.Store so the bucket + Discord IPC stay idle"
  - "Hash store in Coalescer.flushPending — c.lastSentHash.Store(HashActivity(a)) only after SetActivity succeeds (T-08-02-05 mitigation: no cache poisoning on IPC failure)"
  - "Invariant I2 proof (TestCoalescer_HashSkip): identical-content burst of N signals -> exactly 1 SetActivity call"
affects: [08-03-hook-dedup, 08-04-release]

# Tech tracking
tech-stack:
  added:
    - "hash/fnv stdlib (new64a FNV-1a 64-bit; zero-dep)"
  patterns:
    - "Field-ordered raw-byte Write into hash.Hash64 (no json.Marshal, no fmt.Sprintf — D-11 Go-version-stable serialization)"
    - "0x1F ASCII Unit Separator between hash fields (RESEARCH.md discrepancy D3, supersedes CONTEXT D-12 \\x00)"
    - "atomic.Uint64.Load compare-then-skip before any state change (lock-free hot path; I7 preserved)"
    - "Hash-store AFTER successful IPC call only (T-08-02-05 cache-poisoning mitigation)"

key-files:
  created:
    - "coalescer/hash.go (68 LOC) — HashActivity + sep constant"
    - ".planning/phases/08-presence-rate-limit-coalescer-stop-drop-on-skip-token-bucket/08-02-SUMMARY.md"
  modified:
    - "coalescer/coalescer.go (+10/-5 net: hash gate replaces TODO(08-02); hash store replaces placeholder comment)"
    - "coalescer/coalescer_test.go (+120/-2: RLC-04/05/06 tests appended; RLC-01 fixed to mutate registry so the hash gate lets the refill test through)"

key-decisions:
  - "Separator = 0x1F (ASCII Unit Separator), not \\x00. Plan embedded both paths — RESEARCH.md discrepancy D3 explicitly resolved to 0x1F because (a) 0x1F is the ASCII-reserved field delimiter and never legitimately appears in user content, (b) \\x00 can break downstream terminal/log handling. Implemented as const sep byte = 0x1F."
  - "StartTime exclusion enforced structurally: the hash function never references a.StartTime. Verified via `grep StartTime coalescer/hash.go` returning matches only inside comment prose (grep -c on the raw token lands zero in code lines)."
  - "Hash gate placed BEFORE pending.Store — identical payloads never consume a token nor pollute the atomic pointer slot. This was Option 1 in the plan's gating-location spectrum; rejected alternatives would have gated AFTER pending.Store (burns atomic state on a payload that gets dropped anyway) or AFTER schedule (burns a bucket reservation)."
  - "Hash store placed AFTER successful SetActivity only — T-08-02-05 (IPC failure does not poison the cache). Explicit code-ordering: error return path does NOT touch lastSentHash; success path Store runs BEFORE sent.Add(1) so the gate sees the fresh value before any telemetry is emitted."
  - "Test-fix for RLC-01 bundled into Task 2 commit — the token-bucket refill test was written in 08-01 before the hash gate existed; its second-signal branch now correctly skips without registry mutation. Added reg.UpdateActivity between signals so RLC-01 still probes the refill path it was designed to exercise, without introducing a false regression."

patterns-established:
  - "Deterministic content hash pattern: fnv.New64a + fixed-order field writes + 0x1F separator + nil-guard"
  - "Atomic-Uint64 lock-free gate: Load()==stored? → skip+counter. No sync.Mutex touches; hot path completes in ~3 ns (atomic Load + cmp + branch)"
  - "Conditional store idiom for two-atomic state: gate uses lastSentHash.Load(); flush uses lastSentHash.Store(HashActivity(a)). Store exclusive to success path guarantees gate never has a stale-post-error value"

requirements-completed: [RLC-04, RLC-05, RLC-06]

# Metrics
duration: 25min
completed: 2026-04-16
---

# Phase 08 Plan 02: Content-hash change detection Summary

**Deterministic FNV-1a 64-bit hash of user-visible Activity fields gates the Coalescer's flush pipeline; identical payloads never reach the token bucket, and 3 new synctest-driven tests (RLC-04/05/06) prove invariant I2 "identical-content burst -> 1 SetActivity call".**

## Performance

- **Duration:** ~25 min
- **Started:** 2026-04-16T22:00:00Z (approx, following 08-01 handoff)
- **Completed:** 2026-04-16T22:21:28Z
- **Tasks:** 3
- **Files:** 2 modified, 1 created (production); 1 modified (test)

## Accomplishments

- Created `coalescer/hash.go` (68 LOC) implementing the FNV-1a 64-bit `HashActivity(*discord.Activity) uint64` — deterministic across process restarts, field-ordered raw bytes (no `json.Marshal`, no `fmt.Sprintf`), `0x1F` separator (discrepancy D3), `StartTime` excluded (D-09), nil-guard at entry.
- Wired the hash gate into `Coalescer.resolveAndEnqueue` — identical payloads short-circuit BEFORE `pending.Store` so neither the atomic slot nor the token bucket see the redundant update. Gate is lock-free: `atomic.Uint64.Load` + uint64 compare + branch.
- Wired the hash store into `Coalescer.flushPending` — `c.lastSentHash.Store(HashActivity(a))` runs only after `SetActivity` returns nil, so IPC failures do NOT poison the cache (T-08-02-05 mitigation).
- Added 3 RLC tests that all pass under `synctest` bubble: `TestHashActivity_Deterministic` (purity + nil-guard + non-zero for non-nil), `TestHashActivity_ExcludesStartTime` (StartTime-only differ → equal hash; Details differ → distinct hash), `TestCoalescer_HashSkip` (1 flush on first signal, 0 flushes on identical-content second signal even after 5+ s of virtual time).
- Fixed a pre-existing test from 08-01 (`TestCoalescer_TokenBucketRate` / RLC-01) that implicitly relied on the absence of a hash gate. Added a single-line registry mutation between the two probe signals so the test continues to probe token-bucket refill behavior (its original intent) while being hash-gate-aware. Bundled into Task 2's commit so the gate and its test fallout land together.
- Full suite green: `go test ./... -count=1` across all 11 packages (`dsrcode`, `analytics`, `coalescer`, `config`, `discord`, `preset`, `resolver`, `server`, `session`, no-test-file packages).

## Task Commits

Each task was committed atomically:

1. **Task 1: Create `coalescer/hash.go` with `HashActivity`** — `f5a3eb3` (feat)
2. **Task 2: Wire hash gate into `resolveAndEnqueue` + hash store into `flushPending`** — `307a245` (feat, bundles Rule-1 RLC-01 test fixup)
3. **Task 3: Add RLC-04/05/06 tests** — `a73b302` (test)

**Plan metadata:** captured by the final docs commit below.

## Files Created/Modified

- `coalescer/hash.go` (CREATED, 68 LOC): `const sep byte = 0x1F` + `func HashActivity(a *discord.Activity) uint64`. Single exported symbol per file (follows the 08-PATTERNS `one concern per file` rule).
- `coalescer/coalescer.go` (MODIFIED, +10/-5 net): replaced the `TODO(08-02)` placeholder block in `resolveAndEnqueue` with the real hash gate (`HashActivity(a) == c.lastSentHash.Load()` → `skipHash.Add(1)` + `slog.Debug("presence update skipped", "reason","content_hash")` + `return`); replaced the `// Plan 08-02 stores the hash here.` comment in `flushPending` with `c.lastSentHash.Store(HashActivity(a))` BEFORE `c.sent.Add(1)`.
- `coalescer/coalescer_test.go` (MODIFIED, +120/-2): appended 3 RLC tests at end (`TestHashActivity_Deterministic`, `TestHashActivity_ExcludesStartTime`, `TestCoalescer_HashSkip`); added a 6-line registry mutation in `TestCoalescer_TokenBucketRate` so RLC-01's second-signal branch remains hash-gate-aware.

## Decisions Made

- **Separator = `0x1F`, not `\x00`.** 08-CONTEXT.md D-12 originally specified `\x00` for Hook-Dedup's key composition; 08-RESEARCH.md §Pattern 4 + discrepancy D3 explicitly resolve to `0x1F` for HashActivity. `0x1F` is the ASCII Unit Separator (reserved-for-field-delimiting control char); `\x00` is a legitimate byte in some UTF-8 paths and can break terminals / log tailers. Implemented as a package-level `const sep byte = 0x1F` with a comment citing the discrepancy.
- **StartTime exclusion is structural, not conditional.** The hash function never references `a.StartTime` at all — no `if` branch, no zero-guard. Verified with `grep StartTime coalescer/hash.go` → only comment lines. Avoids any possibility of a future regression where someone flips a branch and quietly re-includes StartTime.
- **Hash gate BEFORE `pending.Store`.** Three gating-location alternatives existed: (1) before `pending.Store` [chosen], (2) after `pending.Store` but before `schedule()`, (3) inside `flushPending` after `Swap(nil)`. Option 1 wins on two axes: (a) lock-free single atomic read, no pending-slot mutation at all on skip, (b) matches RESEARCH.md §Code Examples reference implementation.
- **Hash store AFTER successful `SetActivity` only.** T-08-02-05 threat register entry explicitly specifies this ordering. Structurally enforced by putting `lastSentHash.Store` inside the branch reached only if `err == nil` — the error path returns earlier without touching the hash cache. Verified by reading lines 212-220 of `coalescer.go`.
- **RLC-01 test fix bundled into Task 2's commit.** The plan's acceptance criteria for Task 2 said "go build ./... && go vet ./...", but running the existing test suite after the gate wire-up showed RLC-01 (TokenBucketRate) now failing — not a regression, a test that implicitly depended on the hash gate's absence. Rule 1 (auto-fix bugs) applied: the test was still meaningful (probes token-bucket refill) but needed a registry mutation between its two signals for the refill path to be observable through the gate. Committed alongside the gate changes because they are mechanically coupled.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Test bug] `TestCoalescer_TokenBucketRate` (RLC-01) assertion failed after hash gate landed**

- **Found during:** Task 2 post-edit verification (`go test ./coalescer -count=1 -v`).
- **Issue:** The test fires `updateCh <- struct{}{}` at line 163 without mutating registry state. With the Plan 08-02 hash gate now live, the resolver produces the SAME `*discord.Activity` as the previous flush → hash matches `lastSentHash` → skip → 0 SetActivity calls. The test expected 1.
- **Why this counts as Rule 1, not Rule 4:** The test's original intent was to probe token-bucket refill (the "after 4 s + signal, expect 1 flush" assertion). It was written in 08-01 before the hash gate existed, so it never had to consider content identity. The correctness contract of the system shifted (now "identical content never flushes" is a feature, not a bug), and the test needs to cooperate with it. No architectural change — one-line fix to mutate the registry.
- **Fix:** Added `reg.UpdateActivity("s1", session.ActivityRequest{SessionID: "s1", SmallImageKey: "coding"})` between the two test signals (same pattern as RLC-03 `TestCoalescer_FlushSchedule` at line 232). Now the second resolver output differs from the first, the hash gate lets it through, and RLC-01 continues to observe the token-bucket refill path.
- **Files modified:** `coalescer/coalescer_test.go` (+6/-0)
- **Verification:** `go test ./coalescer/... -count=1 -v` — all 7 prior tests (including fixed RLC-01) PASS.
- **Committed in:** `307a245` (Task 2).

**2. [Env limitation, carries over from 08-01] Local `-race` unavailable on Windows**

- **Found during:** setup (pre-Task 1).
- **Issue:** `go test -race` requires CGO which needs a C compiler. None of gcc/clang/mingw on PATH on this Windows workstation.
- **Fix:** Run tests without `-race` locally (all 10 tests pass). CI keeps `-race` on `ubuntu-latest` per `.github/workflows/test.yml:28` where CGO works — atomic correctness of `lastSentHash` gets race-detector coverage on every push. Invariant I7 (no `sync.Mutex` in coalescer) is structurally enforced via code-inspection, independent of the race detector.
- **Files modified:** None (env-only).
- **Committed in:** N/A (no code change; inherited deviation from 08-01).

---

**Total deviations:** 2 (1 auto-fixed test bug scoped to Task 2; 1 inherited env limitation).

**Impact on plan:** Zero scope creep. All plan decisions (D-08/D-09/D-10/D-11 + RESEARCH discrepancy D3) implemented as specified. No new API surface beyond the plan's two symbols (`HashActivity` export + RLC-04/05/06 tests). No new dependencies.

## Authentication Gates

No authentication gates encountered. Task was pure local Go development.

## Issues Encountered

- Context7 CLI fallback still has the Windows Git-Bash path-munging bug that strips forward slashes from library IDs (`golang/go` becomes `C:/Program Files/Git/golang/go`). Worked around by using local `go doc hash/fnv.New64a` + `go doc hash.Hash` + `go doc hash.Hash64` — the stdlib introspection CLI gives verbatim function signatures and type contracts without any network dependency.
- Exa CLI (`exa-mcp-cli`) is not published on npm under that name in this env. Cross-checked implementation patterns against in-repo artifacts: RESEARCH.md §Pattern 4 (verbatim hash code), RESEARCH.md §Code Examples lines 767-819 (verbatim resolveAndEnqueue + flushPending reference), and the 08-01-SUMMARY handoff notes.
- No other issues. The TODO anchor from 08-01 made Task 2 a surgical edit (2 replacements); the `atomic.Uint64` field was already declared so no struct-layout change; the constructor signature was unchanged.

## Threat Flags

No new threat flags discovered. All 5 relevant STRIDE entries in the plan's threat model are mitigated as specified:

- **T-08-02-01** (Tampering — adversarial Activity collision): accepted. Resolver input is fully in-process; FNV-1a 64-bit ~2^32 birthday bound; worst case is one legitimate skip that auto-recovers on next distinct resolve.
- **T-08-02-02** (Info Disclosure via logs): mitigated. `slog.Debug("presence update skipped", "reason", "content_hash")` carries no hash value; the uint64 lives only in the atomic slot in memory.
- **T-08-02-03** (DoS — frequently-changing-but-equivalent Activity): mitigated. StartTime exclusion (D-09) is the primary mitigation; field list frozen by code-ordered raw-byte writes.
- **T-08-02-04** (Tampering — `json.Marshal` map-ordering drift): mitigated. Hash uses field-ordered raw bytes via `hasher.Write`. Grep-enforceable: `grep json.Marshal coalescer/hash.go` returns comment matches only.
- **T-08-02-05** (DoS — IPC failure poisons cache): mitigated. `lastSentHash.Store` executes ONLY inside the `err == nil` branch of `flushPending`; error path returns earlier untouched.
- **T-08-02-06** (Elevation): n/a. No new access surface.

## Handoff Notes

### For Plan 08-03 (HookDedupMiddleware)

- The hash gate in this plan is INDEPENDENT of the HookDedupMiddleware. 08-03 adds a DIFFERENT dedup surface (HTTP duplicate POSTs against `/hooks/*`), with its own FNV-64a hash over `route + sep + session_id + sep + tool_name + sep + body`. The two hash pipelines do not share state and cannot interfere.
- **Reuse the same `0x1F` separator convention in 08-03.** 08-PATTERNS.md §"Named Constant Separator" mandates `0x1F` in BOTH files (small duplication preferred over cross-package coupling). Define as `const sep byte = 0x1F` at the top of `server/hook_dedup.go` with a comment mirroring the one in `coalescer/hash.go`.
- The Coalescer's `getDedupCount func() int64` constructor parameter is still `nil` at `main.go`'s `coalescer.New(...)` call (from 08-01). Plan 08-03 swaps the `nil` for `srv.HookDedup().DedupedCount` (or equivalent accessor) — no other coalescer-side changes needed.
- No new atomic state on the Coalescer struct for 08-03. The 60 s summary log already reads `dd := c.getDedupCount()` — the wiring is in place.

### For Plan 08-04 (Release v4.2.0)

- `main.go` `version` var still at `"4.1.2"`. 08-04 will run `./scripts/bump-version.sh 4.2.0`.
- CHANGELOG v4.2.0 "Changed" section should mention "FNV-64 content-hash now gates SetActivity calls — identical payloads are never resent" per D-34. This plan provides the factual basis.

### General

- `hash.go` is a standalone pure-function file; no new test file needed — the three tests live in `coalescer_test.go` which already exists from 08-01. No coupling to resolver internals; resolver code unchanged this plan.
- Probe arithmetic note from 08-01 (`r2.Cancel()` before sleep) is unaffected by this plan — the hash gate has no interaction with `rate.Reservation` semantics.

## Self-Check: PASSED

Files verified to exist on disk:

- `coalescer/hash.go` FOUND
- `coalescer/coalescer.go` FOUND (modified)
- `coalescer/coalescer_test.go` FOUND (modified)
- `.planning/phases/08-presence-rate-limit-coalescer-stop-drop-on-skip-token-bucket/08-02-SUMMARY.md` FOUND (this file)

Commits verified to exist:

- `f5a3eb3` FOUND — feat(08-02): add coalescer/hash.go with FNV-1a 64-bit HashActivity
- `307a245` FOUND — feat(08-02): wire content-hash gate into Coalescer flush pipeline
- `a73b302` FOUND — test(08-02): add RLC-04/05/06 hash-gate coverage

Grep-based acceptance gates (all from 08-02-PLAN.md success_criteria):

- `grep -c 'fnv.New64a' coalescer/hash.go` = 1 PASS
- `grep -c 'HashActivity' coalescer/coalescer.go` = 2 PASS (>= 1)
- `grep -c 'c.lastSentHash.Store' coalescer/coalescer.go` = 1 PASS (>= 1)
- `grep -c 'c.lastSentHash.Load' coalescer/coalescer.go` = 1 PASS (>= 1)
- `grep -c 'TODO(08-02)' coalescer/coalescer.go` = 0 PASS
- `grep -c '0x1[Ff]' coalescer/hash.go` = 3 PASS (>= 1)
- `grep StartTime coalescer/hash.go` — matches only in comment prose (excluded from code) PASS
- `grep -n 'HashActivity(a) == c.lastSentHash.Load()' coalescer/coalescer.go` = line 167 PASS
- `grep -n 'c.lastSentHash.Store(HashActivity(a))' coalescer/coalescer.go` = line 218 PASS
- `grep -c 'content_hash' coalescer/coalescer.go` = 1 PASS
- `grep -c 'c.skipHash.Add(1)' coalescer/coalescer.go` = 1 PASS

Test gates:

- `go test ./coalescer/... -count=1 -v` — **10/10** PASS (7 prior + 3 new: TestHashActivity_Deterministic, TestHashActivity_ExcludesStartTime, TestCoalescer_HashSkip)
- `go test ./... -count=1` — full suite green across all 11 packages
- `go build ./...` — exit 0
- `go vet ./...` — exit 0

## Next Phase Readiness

- Plan 08-03 (HookDedupMiddleware): **READY**. Coalescer surface unchanged; `getDedupCount` still nil; hash discipline + `0x1F` separator convention documented for reuse.
- Plan 08-04 (Release v4.2.0): **READY once 08-03 lands**. No version strings touched; CHANGELOG entry deferred per D-34.
- No blockers. No concerns.

---
*Phase: 08-presence-rate-limit-coalescer-stop-drop-on-skip-token-bucket*
*Plan: 02*
*Completed: 2026-04-16*
