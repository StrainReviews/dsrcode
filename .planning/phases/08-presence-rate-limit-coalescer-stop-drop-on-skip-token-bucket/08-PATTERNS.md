# Phase 8: Presence Rate-Limit Coalescer — Pattern Map

**Mapped:** 2026-04-16
**Files analyzed:** 11 (2 new Go source, 1 new Go test, 1 new middleware, 1 new middleware test, 2 verify scripts, 3 modified Go/metadata, 1 modified CHANGELOG)
**Analogs found:** 11 / 11 — every new file has a production analog inside this repo
**Package path prefix:** `github.com/StrainReviews/dsrcode`

All line numbers verified against the working tree on 2026-04-16.

---

## Repo-wide Conventions

Extract these once, apply to every file under Phase 8.

### Package Layout
- **One concern per file.** `discord/client.go` houses `Client` + `Activity`, `discord/conn_unix.go` / `conn_windows.go` split by OS build tag. `session/registry.go` is the struct; `session/stale.go` is the stale checker; `session/types.go` is field definitions. Use the same split for `coalescer/coalescer.go` vs. `coalescer/hash.go`.
- **Test files sibling to source.** `*_test.go` lives next to `*.go` in the same directory. External test packages (`package coalescer_test`, `package session_test`, `package server_test`, `package resolver_test`) are the dominant style; use them when exercising only the public API. Only switch to internal `package coalescer` when you must touch unexported state (e.g. `discord/client_test.go:1 package discord`).
- **File size budget.** 200-400 LOC typical, 800 max (per `common/coding-style.md`). `main.go` is already 624 LOC — the new Coalescer MUST live in a new package, not inline, to stay inside budget.
- **No emoji in source, comments, docs, or CHANGELOG.** Verified across `main.go`, `server/server.go`, `CHANGELOG.md` — strictly prose.

### Import Grouping
Three blocks, separated by blank lines, in this exact order:
1. stdlib
2. third-party (non-project)
3. project imports (`github.com/StrainReviews/dsrcode/...`)

Reference: `main.go:1-26` and `server/server.go:5-28`.

### Error Handling
- Library/boundary errors are wrapped: `fmt.Errorf("handshake failed: %w", err)` (see `discord/client.go:69`).
- Background-goroutine errors are logged, not returned: `slog.Debug("discord SetActivity failed", "error", err)` (see `main.go:545`).
- Never silently swallow. Every `err != nil` either returns-with-wrap or `slog`s.
- `slog` key convention: short snake_case keys (`"session"`, `"tool"`, `"elapsed"`, `"retry_in"`). No CamelCase.

### Goroutine Lifecycle
- `for { select { case <-ctx.Done(): return; case <-signal: ... } }` is the canonical loop (see `main.go:409-460 autoExitLoop`, `main.go:516-555 presenceDebouncer`, `main.go:466-499 discordConnectionLoop`).
- **Defer-recover is mandatory** for handler-spawned background goroutines (Phase 6 D-09 precedent — see `server/server.go:604-610`):
  ```go
  go func() {
      defer func() {
          if r := recover(); r != nil {
              slog.Error("<thing> panic", "session", sessionID, "panic", r)
          }
      }()
      ...
  }()
  ```
  Apply this to `coalescer.Run(ctx)`, the dedup `cleanupLoop`, and any `time.AfterFunc` callbacks that touch non-trivial state.
- **Idempotent callbacks.** `time.AfterFunc` callbacks MUST re-check state on entry (grace ctx check in `main.go:433`, our `pending.Swap(nil)` returns-nil-if-already-flushed idiom).

### Channel Signalling
- Non-blocking signal bus via `chan struct{}` of capacity 1 — single producer/consumer.
  ```go
  select {
  case updateChan <- struct{}{}:
  default: // non-blocking
  }
  ```
  Reference: `main.go:143-150` (dual-target fan-out to `updateChan` + `autoExitChan`).

### Testing Style
- **Go stdlib `testing` + table-driven** (required per `golang/testing.md`). Example: `discord/client_test.go:100-160` (named struct table + `t.Run(tt.name, …)`).
- **External package** `package foo_test` for public-API testing. Seen in `resolver/resolver_test.go:1`, `session/registry_test.go:1`, `server/server_test.go:1`.
- **Mock via interface.** `discord/client_test.go:14-47` defines a `mockConn` satisfying the `Conn` interface with buffered read/write. For Coalescer, mock `discord.Client` via a small interface inside `coalescer` (`SetActivity(Activity) error`).
- **`httptest`** for HTTP middleware tests: `server/server_test.go:44-92` via `httptest.NewRequest` + `httptest.NewRecorder` + `handler.ServeHTTP`.
- **`t.Helper()`** inside test helpers so failures point at the calling test line, not the helper.
- **`-race` mandatory.** CI already runs `go test -race ./... -count=1 -v` (see CONTEXT.md discrepancy D1 — the flag is already present in `.github/workflows/test.yml`, so the D-32 task is a verification step, not a change).
- **`testing/synctest`** (Go 1.25 GA) for virtualized time. Import path: `testing/synctest`. See RESEARCH.md §Pattern 1 and §Pitfall 6. No `GOEXPERIMENT` flag needed on Go 1.25.

### Constants
- Named constants at package/file top, `camelCase` (unexported) or `PascalCase` (exported). No magic literals. Reference: `main.go:36-47` (`ClientID`, `discordRateLimit`, `debounceDelay`).
- 500ms dedup TTL + 60s summary interval MUST be named constants, never inline literals (per CONTEXT.md §specifics).

### Immutability
- Copy-before-modify on structs (Phase 6 D-09 precedent). Example: `session/registry.go:123-134 upgradeSession` does `upgraded := *existing` before mutating.
- `atomic.Pointer[T].Store(&newValue)` is the lock-free equivalent — never mutate the pointee in place; always Store a freshly-built value.

---

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `coalescer/coalescer.go` (new) | new-package: coalescing rate-limiter struct + goroutine | signal-driven (`updateChan` → debounce → resolve → hash-check → reserve → flush → SetActivity) | `main.go:501-555 presenceDebouncer` (function being replaced) + `session/registry.go:16-37` (struct + constructor + notification shape) | exact + role-match |
| `coalescer/hash.go` (new) | new-package: deterministic 64-bit content hash | pure function `Activity → uint64` | No repo uses `hash/fnv` yet; reference `discord/client.go:24-34 Activity` for field list + `resolver/resolver.go:164-173` for which fields the resolver actually populates | no-analog (stdlib only) |
| `coalescer/coalescer_test.go` (new) | unit test (external package + optional internal) | synctest bubble → signal → assert `SetActivity` calls | `discord/client_test.go:14-47` (mock via interface), `resolver/resolver_test.go:1-60` (external pkg + table-driven), `session/registry_test.go:13-61` (onChange-counter idiom) | exact |
| `server/hook_dedup.go` (new) | middleware: body-preserving HTTP dedup + TTL cache + cleanup goroutine | request → hash → sync.Map lookup → HTTP 200 (drop) or next (pass) + background `time.Ticker` GC | `server/server.go:234 lastTranscriptRead sync.Map` + `server/server.go:349-370 handleHook` (body read + json.Unmarshal) + `main.go:409-460 autoExitLoop` (select loop + defer-cleanup) | role-match (sync.Map + goroutine lifecycle from repo; HTTP middleware is new surface) |
| `server/hook_dedup_test.go` (new) | integration test | `httptest.NewRequest` → middleware → downstream handler → assert | `server/server_test.go:17-92 TestHookEndpoints` (httptest pattern) | exact |
| `scripts/phase-08/verify.sh` (new) | shell script: T1..T6 end-to-end smoke | curl/grep/version-check | `scripts/test-rotate-log.sh:1-94` (set -euo pipefail, PASS/FAIL assertions, cleanup trap) | role-match (no phase-07 verify.sh exists; reuse the test-rotate-log.sh idiom) |
| `scripts/phase-08/verify.ps1` (new) | PowerShell: T1..T6 cross-platform smoke | Invoke-WebRequest / Test-Path / Get-Content matching | `scripts/phase-06.1/verify.ps1:1-233` (`Invoke-Test`, `Read-ManualAssertion`, results-to-json pattern) | exact (direct precedent) |
| `main.go` constants + wiring (modified) | integration-glue: constants 40-46 + goroutine wiring 160 + shutdown sequence 348-355 | wiring-only | `main.go:160 go presenceDebouncer(...)` call site; `main.go:348-355` Discord shutdown sequence | self (surgical edit) |
| `server/server.go` Handler() wrap (modified) | integration-glue: wrap returned `http.Handler` | middleware composition | `server/server.go:320-347 Handler()` currently does `return mux` verbatim — no middleware exists today | no-analog (first middleware in the codebase) |
| `go.mod` (modified) | metadata | `require golang.org/x/time v0.15.0` | `go.mod:5-11` existing `require` block | exact |
| `CHANGELOG.md` v4.2.0 entry (modified) | metadata: release notes | prose | `CHANGELOG.md:10-19 [4.1.2]` entry | exact |

---

## Pattern Assignments

### `coalescer/coalescer.go` (new-package, signal-driven)

**Role:** Single-goroutine coalescing rate-limiter that consumes `updateChan`, debounces, content-hashes, reserves a token, and flushes the latest pending `*discord.Activity` to `discord.Client.SetActivity`. Replaces the drop-on-skip `presenceDebouncer`.

**Closest analog:** `main.go:501-555` (the function it replaces) + `session/registry.go:16-37` (struct + ctor + notification pattern).

**Key patterns to replicate:**
- Function signature parameters (dependencies): `updateChan`, `registry`, `presetMu`, `currentPreset`, `discordClient`, `displayDetailGetter`. The struct's constructor takes these as direct fields — no functional-options overhead; matches `session.NewRegistry(onChange func())` simplicity.
- `for { select { case <-ctx.Done(): ... case <-ch: ... } }` top-level loop. Cleanup on `ctx.Done()` stops timers, then returns.
- `time.AfterFunc` for 100ms debounce, Stop-replace-on-new idiom (lifted verbatim from `main.go:525-528`).
- `resolver.ResolvePresence(sessions, p, detail, time.Now())` call shape with `presetMu.RLock()` wrap (lifted verbatim from `main.go:535-541`).
- `slog.Debug("presence update skipped", "reason", "<reason>")` event keys (extend the existing `"presence update skipped (rate limit)"` wording; switch to structured field per `slog` convention already used elsewhere).
- `defer-recover` on the goroutine entry (Phase 6 D-09 pattern from `server/server.go:604-610`).

**Core code excerpt — current `presenceDebouncer` structure (`main.go:501-555`, verbatim):**
```go
// presenceDebouncer listens for registry change signals and updates Discord
// presence with rate limiting (15s) and debouncing (100ms).
// displayDetailGetter returns the current DisplayDetail level from config.
func presenceDebouncer(
    ctx context.Context,
    updateChan <-chan struct{},
    registry *session.SessionRegistry,
    presetMu *sync.RWMutex,
    currentPreset **preset.MessagePreset,
    discordClient *discord.Client,
    displayDetailGetter func() config.DisplayDetail,
) {
    var lastUpdate time.Time
    var debounceTimer *time.Timer

    for {
        select {
        case <-ctx.Done():
            if debounceTimer != nil {
                debounceTimer.Stop()
            }
            return
        case <-updateChan:
            // Debounce: wait 100ms after last signal
            if debounceTimer != nil {
                debounceTimer.Stop()
            }
            debounceTimer = time.AfterFunc(debounceDelay, func() {
                // Rate limit: skip if < 15s since last SetActivity
                if time.Since(lastUpdate) < discordRateLimit {
                    slog.Debug("presence update skipped (rate limit)")
                    return
                }

                sessions := registry.GetAllSessions()
                presetMu.RLock()
                p := *currentPreset
                presetMu.RUnlock()

                detail := displayDetailGetter()
                activity := resolver.ResolvePresence(sessions, p, detail, time.Now())

                if activity != nil {
                    if err := discordClient.SetActivity(*activity); err != nil {
                        slog.Debug("discord SetActivity failed", "error", err)
                        return
                    }
                }

                lastUpdate = time.Now()
                slog.Debug("presence updated", "sessions", len(sessions))
            })
        }
    }
}
```

**Struct-with-state analog (`session/registry.go:16-37`, verbatim):**
```go
// SessionRegistry manages active Claude Code sessions in a thread-safe map.
// All mutation methods call the onChange callback (if non-nil) after the state
// changes, allowing the caller to trigger a Discord presence update.
type SessionRegistry struct {
    mu       sync.RWMutex
    sessions map[string]*Session // key: sessionID
    onChange func()              // called after any mutation
}

// NewRegistry creates a new SessionRegistry with the given onChange callback.
// The callback is invoked after every mutation (start, update, end, idle transition).
func NewRegistry(onChange func()) *SessionRegistry {
    return &SessionRegistry{
        sessions: make(map[string]*Session),
        onChange: onChange,
    }
}
```

**Grace-timer-with-callback-idempotency analog (`main.go:423-438`, verbatim):**
```go
graceCtx, graceCancel := context.WithCancel(ctx)
slog.Info("all sessions ended, starting shutdown grace period",
    "duration", gracePeriod)

timer := time.AfterFunc(gracePeriod, func() {
    // Double-check the grace context before firing cancel().
    // If a new session arrived during the window between the
    // timer firing and this callback being scheduled, the
    // grace context will already be cancelled and we must
    // not terminate the daemon.
    if graceCtx.Err() != nil {
        return
    }
    slog.Info("grace period expired, initiating auto-exit")
    cancel()
})
```
This establishes the repo-approved idempotent-callback pattern. The Coalescer flush callback adapts it as `pending.Swap(nil) == nil → return`.

**Specific adaptations required for Phase 8:**
- Rename to `Coalescer` struct; hold `limiter *rate.Limiter`, `pending atomic.Pointer[discord.Activity]`, `lastSentHash atomic.Uint64`, `sent/skipRate/skipHash/deduped atomic.Int64`, `debounce *time.Timer`, `flushTimer *time.Timer`, `summaryTicker *time.Ticker`.
- Replace the inner `if time.Since(lastUpdate) < discordRateLimit { return }` (root cause at `main.go:530`) with: (1) compute hash; skip if equal to `lastSentHash.Load()`; (2) `pending.Store(a)`; (3) `Reserve()`; (4) if `r.Delay() == 0` flush now else `time.AfterFunc(r.Delay(), flushPending)`.
- Add `emitSummary()` method that atomically Swaps all four counters to 0 and emits `slog.Info("coalescer status", …)` when any counter was non-zero (D-27 + Claude's Discretion: skip when idle).
- Add `Shutdown()` method: `pending.Store(nil)`, `debounce.Stop()`, `flushTimer.Stop()`, `summaryTicker.Stop()`. Called from `Run` defer + from `main.go` shutdown sequence before `SetActivity(Activity{})`.
- Expose a `IncrementDeduped()` method (or accept a `dedupCounter *atomic.Int64` in the constructor) so `HookDedupMiddleware` can push the `deduped` count into the Coalescer's 60s summary — this resolves discrepancy D4 from RESEARCH.md (hook-dedup counter lives in `server` package but the summary aggregates it).
- Use `slog.Debug("presence update skipped", "reason", "rate_limit")` and `"reason", "content_hash"` — structured, not the current interpolated-string wording (D-26).
- Defer-recover at `Run` entry (Phase 6 D-09).

---

### `coalescer/hash.go` (new-package, pure function)

**Role:** Deterministic 64-bit FNV-1a hash over user-visible `Activity` fields, excluding `StartTime`.

**Closest analog:** No existing `hash/fnv` user in repo. Reference `discord/client.go:24-34 Activity` struct (the input schema) and `resolver/resolver.go:164-173` (which fields the resolver actually populates — this is the ground truth for what the hash must cover).

**Key patterns to replicate:**
- Single exported function per file (hash.go contains only `func HashActivity(*discord.Activity) uint64`).
- Import grouping (stdlib block separate from project block).
- Inline field-by-field `hasher.Write([]byte(a.Field))` — NO `json.Marshal`, NO `fmt.Sprintf`. Each field followed by a separator byte (use `0x1F` Unit Separator per RESEARCH.md discrepancy D3).
- Nil-guard at entry: `if a == nil { return 0 }`.
- Buttons loop preserves slice order (`resolver/resolver.go:172` uses `convertButtons(p.Buttons)` which also preserves order — deterministic by construction).
- Package-level named constant for the separator byte.

**Activity schema analog (`discord/client.go:18-34`, verbatim):**
```go
// Button represents a clickable button on Discord Rich Presence (max 2 per activity).
type Button struct {
    Label string `json:"label"`
    URL   string `json:"url"`
}

// Activity represents Discord Rich Presence activity
type Activity struct {
    Details    string     `json:"details,omitempty"`
    State      string     `json:"state,omitempty"`
    LargeImage string     `json:"large_image,omitempty"`
    LargeText  string     `json:"large_text,omitempty"`
    SmallImage string     `json:"small_image,omitempty"`
    SmallText  string     `json:"small_text,omitempty"`
    StartTime  *time.Time `json:"-"`
    Buttons    []Button   `json:"-"` // Custom serialization in SetActivity
}
```

**Resolver output analog (`resolver/resolver.go:164-173`, verbatim) — the fields that actually end up in the Activity:**
```go
return &discord.Activity{
    Details:    truncate(details, maxFieldLength),
    State:      truncate(state, maxFieldLength),
    LargeImage: largeImageKey,
    LargeText:  largeText,
    SmallImage: s.SmallImageKey,
    SmallText:  s.SmallText,
    StartTime:  &startTime,
    Buttons:    convertButtons(p.Buttons),
}
```

**Specific adaptations required for Phase 8:**
- Use `hash/fnv.New64a()` — stdlib, deterministic across Go versions + process restarts (D-08).
- Include exactly: `Details`, `State`, `LargeImage`, `LargeText`, `SmallImage`, `SmallText`, then range over `Buttons` writing `Label` + `URL` per entry.
- EXCLUDE `StartTime` — code comment MUST explain why (D-09, and verified deterministic per `resolver_test.go:333-339`).
- Use `const sep byte = 0x1F` at file top (RESEARCH.md discrepancy D3: 0x1F is the ASCII Unit Separator; never appears in user content).
- Return `h.Sum64()` as `uint64` (matches `atomic.Uint64.Store/Load` signature for `Coalescer.lastSentHash`).
- Exported name `HashActivity` so `coalescer_test.go` (external package `coalescer_test`) can call it.

---

### `coalescer/coalescer_test.go` (new, unit test)

**Role:** Unit tests for Coalescer coalescing, hash skip, token-bucket scheduling, disconnect discards, shutdown clears. Uses `testing/synctest` bubbles for virtualized time.

**Closest analog:** Three exemplars.

1. **Mock via interface** — `discord/client_test.go:14-47` (the `mockConn` pattern):
```go
// mockConn implements the Conn interface for testing
type mockConn struct {
    writeBuffer bytes.Buffer
    readBuffer  bytes.Buffer
    readErr     error
    writeErr    error
    closed      bool
}

func (m *mockConn) Read(b []byte) (n int, err error) { ... }
func (m *mockConn) Write(b []byte) (n int, err error) { ... }
func (m *mockConn) Close() error { ... }
```

2. **External package + table-driven** — `resolver/resolver_test.go:1-52` (verbatim):
```go
package resolver_test

import (
    "fmt"
    "strings"
    "testing"
    "time"

    "github.com/StrainReviews/dsrcode/config"
    "github.com/StrainReviews/dsrcode/preset"
    "github.com/StrainReviews/dsrcode/resolver"
    "github.com/StrainReviews/dsrcode/session"
)

func TestStablePick(t *testing.T) {
    pool := []string{"alpha", "bravo", "charlie", "delta", "echo"}
    seed := int64(42)
    now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

    result1 := resolver.StablePick(pool, seed, now)
    if result1 == "" {
        t.Fatal("StablePick returned empty string for non-empty pool")
    }
    // ... more assertions
}
```

3. **onChange-counter idiom** — `session/registry_test.go:13-51` (verbatim):
```go
func TestRegistryStartSession(t *testing.T) {
    changed := 0
    reg := session.NewRegistry(func() { changed++ })

    req := session.ActivityRequest{
        SessionID: "sess-1",
        Cwd:       "/home/user/project",
    }

    s := reg.StartSession(req, 1234)
    if s == nil {
        t.Fatal("StartSession returned nil")
    }
    // ... assertions on registry state and counter
    if changed != 1 {
        t.Errorf("onChange called %d times, want 1", changed)
    }
}
```

**Key patterns to replicate:**
- `package coalescer_test` (external).
- Tiny interface mocking: define `type discordSetter interface { SetActivity(discord.Activity) error }` inside the Coalescer package and accept it in the constructor (Accept interfaces, return structs — `golang/coding-style.md`). The test supplies a `mockDiscord` that captures calls on a buffered channel.
- Table-driven subtests via `t.Run(tt.name, ...)`.
- Counter-based assertion ("`skipped_hash == N-1` after 60s summary").
- `synctest.Test(t, func(t *testing.T) { … })` wrapper with `time.Sleep` + `synctest.Wait` for virtual-time advancement.
- `t.Helper()` in shared setup helpers.
- First test: the **RLC-15 probe** per RESEARCH.md assumption A4 — verify `synctest` virtualizes `rate.Limiter` correctly before writing dependent tests.

**Specific adaptations required for Phase 8:**
- Import `testing/synctest` (Go 1.25 stdlib; no GOEXPERIMENT needed on Go 1.25).
- Define `mockDiscord` with `calls chan discord.Activity` (buffered ~100) — same shape as `mockConn` but one method.
- Tests to write (one per RLC-ID):
  - `TestCoalescer_TokenBucketRate` (RLC-01)
  - `TestCoalescer_PendingSlot` (RLC-02)
  - `TestCoalescer_FlushSchedule` (RLC-03)
  - `TestHashActivity_Deterministic` (RLC-04) — asserts `HashActivity(a) == HashActivity(a)` across calls + identical bytes.
  - `TestHashActivity_ExcludesStartTime` (RLC-05) — builds two Activities differing ONLY in `StartTime`, asserts hashes equal.
  - `TestCoalescer_HashSkip` (RLC-06)
  - `TestCoalescer_SummaryIdleSkip` (RLC-11)
  - `TestCoalescer_ShutdownIdempotent` (RLC-12)
  - `TestCoalescer_ReconnectDiscard` (RLC-13)
  - `TestSynctest_RateLimiterProbe` (RLC-15 probe)
- All tests MUST pass under `go test -race`.

---

### `server/hook_dedup.go` (new, middleware)

**Role:** HTTP middleware wrapping `s.Handler()`. For `/hooks/*` routes, computes FNV-64a dedup key over `route | session_id | tool_name | body`, drops duplicates within 500ms TTL via `sync.Map`-backed cache, body-preserving via `io.ReadAll` + `io.NopCloser(bytes.NewReader(...))`. Background `time.Ticker(60s)` GC loop.

**Closest analog (three combined):**

1. **`sync.Map` precedent in same package** — `server/server.go:234` field and `server/server.go:589-603` usage (verbatim):
```go
// lastTranscriptRead throttles PostToolUse-triggered JSONL reads per D-02:
// max 1 read per 10 seconds per session. Keys are session IDs, values are
// time.Time of the last read. Must not be copied.
lastTranscriptRead sync.Map
```
```go
// Throttled background JSONL read per D-02: max 1 read per 10 seconds per session.
if s.tracker != nil && transcriptPath != "" {
    now := time.Now()
    shouldRead := true
    if lastVal, ok := s.lastTranscriptRead.Load(sessionID); ok {
        if lastTime, ok := lastVal.(time.Time); ok {
            if now.Sub(lastTime) < postToolUseThrottleInterval {
                shouldRead = false
            }
        }
    }
    if shouldRead {
        s.lastTranscriptRead.Store(sessionID, now)
        // ... background work
    }
}
```
The `Load`→type-assert-to-`time.Time`→compare-elapsed idiom maps 1-to-1 onto the dedup cache.

2. **Body read + JSON probe** — `server/server.go:349-370 handleHook` (verbatim):
```go
func (s *Server) handleHook(w http.ResponseWriter, r *http.Request) {
    hookType := r.PathValue("hookType")

    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "read error", http.StatusBadRequest)
        return
    }

    var payload HookPayload
    if err := json.Unmarshal(body, &payload); err != nil {
        // Retry with sanitized JSON: Claude Code on Windows may send
        // unescaped backslashes in paths (known bug anthropics/claude-code#20015).
        sanitized := sanitizeWindowsJSON(body)
        if err2 := json.Unmarshal(sanitized, &payload); err2 != nil {
            slog.Debug("hook: invalid JSON body", "error", err2)
            http.Error(w, "invalid JSON", http.StatusBadRequest)
            return
        }
    }
    // ...
}
```
Note: the dedup middleware sits BEFORE this handler and must re-wrap the body so the handler's `io.ReadAll` still returns bytes. Fail-open if body read fails (pass through to handler, which returns the 400 itself).

3. **Cleanup goroutine lifecycle** — `main.go:409-460 autoExitLoop` (verbatim select-loop shape):
```go
for {
    select {
    case <-ctx.Done():
        return
    case <-autoExitChan:
        count := registry.SessionCount()
        // ... per-signal work
    }
}
```

**Key patterns to replicate:**
- Struct in `server` package (same package as Server — internal cross-access permitted; path filter is package-private detail).
- `sync.Map` with `key string` (hex-encoded FNV sum) → `value time.Time` (last-seen).
- `Load`+`Sub(lastTime) < TTL` idiom directly from `server.go:597`.
- Constructor kicks off `go m.cleanupLoop(ctx)` — accepts `ctx` so the loop exits on daemon shutdown (`main.go`-driven).
- Path filter: `if !strings.HasPrefix(r.URL.Path, "/hooks/") { next.ServeHTTP(w, r); return }`.
- Body re-wrap: `r.Body = io.NopCloser(bytes.NewReader(body))` AFTER `io.ReadAll`.
- Fail-open: `io.ReadAll` error → pass through to next, do not fail request.
- Silent-200 on dedup hit: `w.WriteHeader(http.StatusOK)` + `slog.Debug("hook deduped", "route", …, "key", keyHex[:8])`.
- Named constants at top: `hookDedupTTL = 500 * time.Millisecond`, `hookDedupCleanupPeriod = 60 * time.Second`.
- Defer-recover on `cleanupLoop` goroutine.

**Specific adaptations required for Phase 8:**
- Extract `session_id` and `tool_name` via tiny probe struct (`json.Unmarshal` tolerating extra fields); fallbacks-to-empty-string if decode fails (fail-open).
- Hash key = FNV-64a of `route + 0x1F + session_id + 0x1F + tool_name + 0x1F + body` (use same 0x1F separator as `HashActivity` for consistency; RESEARCH.md discrepancy D3 supersedes CONTEXT D-12's "\x00").
- Expose `DedupedCount() int64` reader for Coalescer's 60s summary.
- Optional: `http.MaxBytesReader(r.Body, 64<<10)` defence-in-depth per RESEARCH.md §Security (64KB cap; hook payloads are <10KB).
- Wire in `server.go Handler()` return: `return m.Wrap(mux)` instead of bare `return mux`.
- Lifecycle: `cleanupLoop(ctx)` reads `ctx.Done()` and `ticker.C`; `ticker.Stop()` in `defer`. No separate `stopCh` needed — the `server` package already threads `ctx` through `Start(ctx, ...)`.

---

### `server/hook_dedup_test.go` (new, integration test)

**Role:** Integration tests exercising the middleware through `httptest.NewRequest` + assert downstream handler decoded the body + assert duplicate is dropped.

**Closest analog:** `server/server_test.go:17-92` (verbatim, first 76 lines of relevance):
```go
package server_test

import (
    "encoding/json"
    "fmt"
    "net/http"
    "net/http/httptest"
    "strings"
    "sync"
    "testing"
    "time"

    "github.com/StrainReviews/dsrcode/server"
    "github.com/StrainReviews/dsrcode/session"
)

func defaultTestConfig() server.ServerConfig {
    return server.ServerConfig{
        Preset:        "minimal",
        DisplayDetail: "minimal",
        Port:          19460,
        BindAddr:      "127.0.0.1",
    }
}

func newTestServer(onConfig func(server.ConfigUpdatePayload)) (*server.Server, *session.SessionRegistry) {
    registry := session.NewRegistry(func() {})
    srv := server.NewServer(
        registry,
        onConfig,
        "2.0.0",
        func() server.ServerConfig { return defaultTestConfig() },
        func() bool { return false },
        nil,
        nil,
    )
    return srv, registry
}

func TestHookEndpoints(t *testing.T) {
    srv, registry := newTestServer(nil)
    handler := srv.Handler()

    body := `{"tool_name":"Edit","session_id":"test-1","cwd":"/tmp/myproject"}`
    req := httptest.NewRequest(http.MethodPost, "/hooks/pre-tool-use", strings.NewReader(body))
    req.Header.Set("Content-Type", "application/json")

    w := httptest.NewRecorder()
    handler.ServeHTTP(w, req)

    if w.Code != http.StatusOK {
        t.Fatalf("expected status 200, got %d", w.Code)
    }

    // Verify session was created
    if registry.SessionCount() != 1 {
        t.Fatalf("expected 1 session, got %d", registry.SessionCount())
    }
    // ... more assertions on session state
}
```

**Key patterns to replicate:**
- `package server_test` (external).
- `newTestServer` helper returning `(*server.Server, *session.SessionRegistry)` — replicate the same factory and add a variant that returns the full middleware-wrapped handler: `func newTestHandler(t *testing.T) (http.Handler, *session.SessionRegistry, *server.HookDedupMiddleware)`.
- `httptest.NewRequest(http.MethodPost, "/hooks/pre-tool-use", strings.NewReader(body))` + `httptest.NewRecorder()` + `handler.ServeHTTP(w, req)`.
- Assertions on `w.Code == http.StatusOK` and `registry.SessionCount()`.
- For body-preservation: send a valid hook body, verify the underlying handler decoded `payload.ToolName == "Edit"` (by observing the registry state — same style as existing `TestHookEndpoints`).

**Specific adaptations required for Phase 8:**
- Tests to write (one per RLC-ID):
  - `TestHookDedup_PathFilter` (RLC-07) — POST to `/health`, `/status`, `/preview`, `/statusline` pass through; only `/hooks/*` goes via dedup.
  - `TestHookDedup_KeyComposition` (RLC-08) — same route+body but different session-id → distinct keys.
  - `TestHookDedup_TTLCleanup` (RLC-09) — use `synctest` to advance 60s and verify `sync.Map` entry count drops.
  - `TestHookDedup_BodyPreserved` (RLC-10) — POST valid hook, assert downstream registry received the `tool_name` — identical to `TestHookEndpoints` but routed through middleware.
  - `TestPreview_BypassesCoalescer` (RLC-14) — POST `/preview`, verify middleware did NOT touch it (no entry in dedup cache).
- Use `-race` compatible patterns (no shared mutable state without mutex/atomic).
- Duplicate-detection test: POST identical request twice within 100ms, assert second returns 200 AND downstream registry state did not change twice.

---

### `scripts/phase-08/verify.sh` (new, bash smoke test)

**Role:** T1..T6 end-to-end smoke for Phase 8 on Unix / Git Bash. Checks version, health, burst-coalesce rate, identical-content burst, hook dedup, 60s summary.

**Closest analog:** `scripts/test-rotate-log.sh:1-94` (verbatim) — the only `.sh` test harness in the repo using set -euo pipefail + PASS/FAIL assertions. No `phase-07/verify.sh` exists.

**Key patterns to replicate:**
- Shebang `#!/bin/bash` followed by `set -euo pipefail`.
- `SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"` for portable path resolution.
- `mktemp -d` + `trap 'rm -rf "$TMPDIR"' EXIT` for cleanup.
- `echo "PASS: <desc>"` / `echo "FAIL: <desc>" >&2; exit 1` per test.
- Final banner: `echo ""; echo "All Phase 8 verify tests passed."`.

**Excerpt (`scripts/test-rotate-log.sh:1-30`, verbatim):**
```bash
#!/bin/bash
# Test harness for Phase 7 Bug #4 rotate_log function.
# Validates: 11MB file gets renamed to .log.1; second 11MB run overwrites .log.1.
# Usage: bash scripts/test-rotate-log.sh
# Exits 0 on success, non-zero on any assertion failure.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
START_SH="$SCRIPT_DIR/start.sh"

if [[ ! -f "$START_SH" ]]; then
    echo "FAIL: $START_SH not found" >&2
    exit 1
fi
```

**Specific adaptations required for Phase 8:**
- T1: `dsrcode --version | grep -q "4.2.0"` — version check.
- T2: `curl -fsS http://127.0.0.1:19460/health` — HTTP 200.
- T3: Burst 10 `curl -X POST` to `/hooks/pre-tool-use` (distinct session IDs) within 1s → grep log for `"presence updated"` → assert count ≤ 3 (burst 2 + one coalesced flush).
- T4: Burst 10 identical `curl -X POST` → grep `"presence update skipped"` with `reason=content_hash` ≥ 9 times.
- T5: Two identical POSTs to `/hooks/pre-tool-use` within 500ms → grep `"hook deduped"` == 1.
- T6: `sleep 60` → grep `"coalescer status"` ≥ 1 within the preceding 60s (sentinel-based: record log offset at start, tail from there).
- Log file path: `~/.claude/dsrcode.log` (matches `scripts/start.sh` convention).
- Exit 0 on all-pass, non-zero otherwise (matches `test-rotate-log.sh`).

---

### `scripts/phase-08/verify.ps1` (new, PowerShell smoke test)

**Role:** T1..T6 smoke for Phase 8 on Windows PowerShell 5.1+. Mirrors `verify.sh` behavior.

**Closest analog:** `scripts/phase-06.1/verify.ps1:1-232` (verbatim patterns; adapt tests T1..T6 for Phase 8).

**Key patterns to replicate (`scripts/phase-06.1/verify.ps1:1-70`, verbatim):**
```powershell
#Requires -Version 5.1
<#
.SYNOPSIS
    Phase 6.1 post-rename verification — runs T1-T8 smoke tests.
...
#>

[CmdletBinding()]
param(
    [switch]$SkipOptional,
    [switch]$SkipManual
)

$ErrorActionPreference = 'Continue'

# Test result tracking
$script:results = [ordered]@{}
$script:passCount = 0
$script:failCount = 0
$script:skipCount = 0

function Invoke-Test {
    param(
        [string]$Id,
        [string]$Description,
        [scriptblock]$Test
    )
    Write-Host -NoNewline "[$Id] $Description ... "
    try {
        $result = & $Test
        if ($result -is [bool] -and $result) {
            Write-Host "PASS" -ForegroundColor Green
            $script:passCount++
            $script:results[$Id] = 'PASS'
            return $true
        } else {
            Write-Host "FAIL" -ForegroundColor Red
            $script:failCount++
            $script:results[$Id] = 'FAIL'
            return $false
        }
    } catch {
        Write-Host "ERROR: $($_.Exception.Message)" -ForegroundColor Red
        $script:failCount++
        $script:results[$Id] = "ERROR: $($_.Exception.Message)"
        return $false
    }
}
```

**Version/health test excerpt (`scripts/phase-06.1/verify.ps1:99-115`, verbatim):**
```powershell
# T1: Binary version
Invoke-Test -Id 'T1' -Description 'dsrcode --version returns 4.1.0' -Test {
    $dsrcodeBin = Get-Command -Name 'dsrcode' -ErrorAction SilentlyContinue
    if (-not $dsrcodeBin) { return $false }
    $output = & dsrcode --version 2>&1 | Out-String
    return ($output -match 'dsrcode\s+4\.1\.0')
}

# T2: Health endpoint
Invoke-Test -Id 'T2' -Description 'HTTP 200 from http://127.0.0.1:19460/health' -Test {
    try {
        $resp = Invoke-WebRequest -Uri 'http://127.0.0.1:19460/health' -UseBasicParsing -TimeoutSec 2 -ErrorAction Stop
        return ($resp.StatusCode -eq 200)
    } catch {
        return $false
    }
}
```

**Results export pattern (`scripts/phase-06.1/verify.ps1:211-232`, verbatim):**
```powershell
# Export results to JSON for Plan 05 SUMMARY consumption
$summaryPath = Join-Path $PSScriptRoot 'verify-results.json'
$summary = @{
    phase       = '6.1'
    timestamp   = (Get-Date).ToString('o')
    passCount   = $passCount
    failCount   = $failCount
    skipCount   = $skipCount
    results     = $results
} | ConvertTo-Json -Depth 4
Set-Content -Path $summaryPath -Value $summary -Encoding UTF8
```

**Specific adaptations required for Phase 8:**
- Update version regex to `'dsrcode\s+4\.2\.0'` for T1.
- T3-T4: Use `Invoke-WebRequest -Method Post -Body '...'` in a for-loop to burst requests; tail `$env:USERPROFILE\.claude\dsrcode.log` via `Get-Content -Tail N` and regex-match.
- T5: Fire two identical POSTs via `Invoke-WebRequest`; then `Get-Content -Tail 50 $logPath | Select-String -Pattern 'hook deduped'` and assert count == 1.
- T6: `Start-Sleep -Seconds 60` then `Select-String -Pattern 'coalescer status'` on the log tail.
- `$phase = '8'` in the JSON export block.
- No `-SkipOptional` / `-SkipManual` flags needed if all six tests are automated (CONTEXT.md §specifics suggests T5+T6 are fully scriptable via log-tail).

---

### `main.go` (modified, integration-glue)

**Role:** Replace `presenceDebouncer` wiring with `Coalescer.Run`, update rate constants, inject Coalescer.Shutdown() into the D-07 shutdown sequence.

**Closest analog:** itself — surgical edits at three existing sites.

**Current constants block (`main.go:36-47`, verbatim):**
```go
const (
    // Discord Application ID for "DSR Code"
    ClientID = "1489600745295708160"

    // discordRateLimit is the minimum interval between Discord SetActivity calls.
    // Discord rate-limits activity updates to once every 15 seconds.
    discordRateLimit = 15 * time.Second

    // debounceDelay is the time to wait after the last registry change before
    // resolving a new presence. Collapses rapid-fire tool events.
    debounceDelay = 100 * time.Millisecond
)
```

**Current wiring (`main.go:158-164`, verbatim):**
```go
// 11. Presence update debouncer (goroutine)
go presenceDebouncer(ctx, updateChan, registry, &presetMu, &currentPreset, discordClient, func() config.DisplayDetail {
    cfgMu.RLock()
    defer cfgMu.RUnlock()
    return cfg.DisplayDetail
})
```

**Current shutdown sequence (`main.go:339-357`, verbatim):**
```go
// D-07 shutdown sequence:
//   1. Clear Discord Rich Presence activity (empty SetActivity over live IPC)
//   2. Close Discord IPC
//   3. Cancel context -> triggers server.Server.Start to call
//      httpServer.Shutdown(5s timeout) for in-flight request drain, stops
//      stale checker, presence debouncer, config watcher, and auto-exit
//      goroutine.
// cancel() is idempotent (context.CancelFunc), so calling it after
// ctx.Done() already fired is safe.
slog.Debug("clearing Discord activity")
if err := discordClient.SetActivity(discord.Activity{}); err != nil {
    slog.Debug("clear activity failed", "error", err)
}
slog.Debug("closing Discord IPC")
if err := discordClient.Close(); err != nil {
    slog.Debug("discord close failed", "error", err)
}
cancel()
slog.Info("shutdown complete")
```

**Specific adaptations required for Phase 8:**
- **Constants block**: DELETE `discordRateLimit` (replaced by coalescer package). Keep `debounceDelay` OR move it into the coalescer package — RESEARCH.md recommends moving it, but keeping it exported at `main.go` is also acceptable (D-19 says 100ms debounce stays as a front-aggregator; Coalescer consumes it internally). Planner decides.
- **Wiring**: replace `go presenceDebouncer(...)` with:
  ```go
  coalescer := coalescer.New(updateChan, registry, &presetMu, &currentPreset, discordClient, displayDetailGetter, dedupMiddleware.DedupedCount)
  go coalescer.Run(ctx)
  ```
  Where `dedupMiddleware.DedupedCount` is the counter-getter created at the HTTP server wiring step (see `server.go` section below).
- **Shutdown**: BEFORE `discordClient.SetActivity(discord.Activity{})`, add `coalescer.Shutdown()` call. Order per D-23: (1) `coalescer.Shutdown()` halts new SetActivity calls, (2) direct-to-client `SetActivity(clearActivity)` pushes the empty frame, (3) `discordClient.Close()`. This preserves Phase 6.04 D-07 semantics.
- Delete the entire `presenceDebouncer` function body (`main.go:501-555`).
- Add `import "github.com/StrainReviews/dsrcode/coalescer"` to the project-imports block.

---

### `server/server.go` Handler() wrap (modified, integration-glue)

**Role:** Wrap the bare `mux` return with `HookDedupMiddleware.Wrap(...)`.

**Closest analog:** itself — the current `return mux` line.

**Current code (`server/server.go:320-347`, verbatim):**
```go
// Handler returns the http.Handler with all routes registered.
// Uses Go 1.22+ ServeMux method routing patterns.
func (s *Server) Handler() http.Handler {
    mux := http.NewServeMux()

    // Dedicated hook handlers MUST be registered BEFORE the wildcard route
    // so that Go's method routing picks the specific handler first.
    mux.HandleFunc("POST /hooks/subagent-stop", s.handleSubagentStop)
    mux.HandleFunc("POST /hooks/session-end", s.handleSessionEnd)
    mux.HandleFunc("POST /hooks/post-tool-use", s.handlePostToolUse)
    mux.HandleFunc("POST /hooks/stop-failure", s.handleStopFailure)
    mux.HandleFunc("POST /hooks/pre-compact", s.handlePreCompact)
    mux.HandleFunc("POST /hooks/post-compact", s.handlePostCompact)
    mux.HandleFunc("POST /hooks/subagent-start", s.handleSubagentStart)
    mux.HandleFunc("POST /hooks/post-tool-use-failure", s.handlePostToolUseFailure)
    mux.HandleFunc("POST /hooks/cwd-changed", s.handleCwdChanged)
    mux.HandleFunc("POST /hooks/{hookType}", s.handleHook)
    mux.HandleFunc("POST /statusline", s.handleStatusline)
    mux.HandleFunc("POST /config", s.handleConfigUpdate)
    mux.HandleFunc("GET /health", s.handleHealth)
    mux.HandleFunc("GET /sessions", s.handleSessions)
    mux.HandleFunc("GET /presets", s.handleGetPresets)
    mux.HandleFunc("GET /status", s.handleGetStatus)
    mux.HandleFunc("POST /preview", s.handlePostPreview)
    mux.HandleFunc("GET /preview/messages", s.handleGetPreviewMessages)

    return mux
}
```

**Specific adaptations required for Phase 8:**
- Add a `*HookDedupMiddleware` field to the `Server` struct (next to `lastTranscriptRead sync.Map` at `server.go:234`).
- Initialize it in `NewServer` — constructor signature gains no new required param if the middleware is package-internal and lazily created by `Handler()` (recommended).
- Last line becomes `return s.hookDedup.Wrap(mux)` instead of bare `return mux`. Preserve the registration-order comment block; preserve `http.ServeMux` semantics (no route-ordering change).
- The middleware instance must be shared (single sync.Map + single ticker goroutine), so create ONCE in `NewServer` or lazily on first `Handler()` call — do NOT create inside `Handler()` (it's called once at `Start` via `net/http`, but defensive coding prefers once-in-ctor).
- `Start(ctx, ...)` already passes `ctx` — thread it into the middleware so `cleanupLoop(ctx)` can exit on daemon shutdown. Either (a) change `NewHookDedupMiddleware` to accept no ctx and start the cleanup loop on first `Wrap` call with the incoming `http.Request`'s context (brittle), or (b) expose an explicit `StartCleanup(ctx)` that `Server.Start` invokes. Option (b) is closer to the existing `autoExitLoop` pattern.

---

### `go.mod` (modified, metadata)

**Role:** Add `golang.org/x/time v0.15.0` to `require`.

**Closest analog:** itself.

**Current content (`go.mod:1-14`, verbatim):**
```go
module github.com/StrainReviews/dsrcode

go 1.25.0

require (
    github.com/Microsoft/go-winio v0.6.2
    github.com/fsnotify/fsnotify v1.9.0
    github.com/nicksnyder/go-i18n/v2 v2.6.1
    golang.org/x/text v0.35.0
    gopkg.in/natefinch/lumberjack.v2 v2.2.1
)

require golang.org/x/sys v0.13.0 // indirect
```

**Specific adaptations required for Phase 8:**
- Add `golang.org/x/time v0.15.0` inside the primary `require` block (alphabetic order between `golang.org/x/text` and `gopkg.in/natefinch/...`).
- Run `go mod tidy` — this lets Go recompute `go.sum` and may shift `golang.org/x/sys` to a newer indirect version (x/time depends on x/sys). Don't hand-edit `go.sum`.
- Command: `go get golang.org/x/time@v0.15.0 && go mod tidy`.

---

### `CHANGELOG.md` v4.2.0 entry (modified, metadata)

**Role:** Insert a Keep-a-Changelog 1.1.0 entry for v4.2.0 above the `[4.1.2]` block.

**Closest analog:** `CHANGELOG.md:10-19` (the `[4.1.2]` block immediately above the new entry's target position).

**Current v4.1.2 entry (`CHANGELOG.md:10-19`, verbatim):**
```markdown
## [4.1.2] - 2026-04-13

### Fixed
- **Daemon self-termination during long MCP-heavy sessions** — `handlePostToolUse` now updates `LastActivityAt` on every tool call (Bug #2, D-04/D-05 Phase 7). Previously, MCP-heavy sessions could idle the activity clock past the 30-minute remove-timeout even while Claude Code was actively calling tools, causing the daemon to remove the session and eventually self-exit. New `registry.Touch()` method updates the activity timestamp without firing a Discord presence update.
- **PID-liveness check false-positives for HTTP-sourced sessions** — `session/stale.go` now skips the PID liveness check when `Source == SourceHTTP` (Bug #1, D-01/D-02 Phase 7). The daemon's recorded PID for HTTP-sourced sessions is the short-lived `start.sh` wrapper process on Windows, which exits within seconds even while the Claude Code session is alive. The 30-minute `removeTimeout` remains as the backstop.
- **Refcount drift via missing SessionEnd command hook** — plugin `hooks/hooks.json` now registers `SessionEnd` as a `type: command` hook invoking `scripts/stop.sh` / `scripts/stop.ps1` (Bug #3, D-07/D-09 Phase 7). ...
- **Log overwrite on daemon restart** — `start.sh` and `start.ps1` now rotate `dsrcode.log` at 10 MB to `dsrcode.log.1` ... (Bug #4, D-10/D-11/D-12 Phase 7). ...

### Changed
- Unix log redirection (`scripts/start.sh`) now splits stdout and stderr into separate files (`~/.claude/dsrcode.log` and `~/.claude/dsrcode.log.err`) and uses append mode (`>>`) so crash traces survive daemon restarts — matching Windows behavior established in v4.1.1.
```

**Key patterns to replicate:**
- Header: `## [X.Y.Z] - YYYY-MM-DD`.
- Section order: `### Fixed` → `### Changed` → `### Added` (Keep-a-Changelog 1.1.0 — matches `[4.1.2]` except v4.2.0 has an Added section).
- Past-tense, user-benefit-first prose (see CONTEXT.md §specifics: "Headline angle: 'Fix presence stalling during long MCP-heavy sessions'").
- **Bold** phrase at the start of each bullet to front-load the user-visible pain point.
- End-of-bullet parenthetical citing the Phase number / D-IDs / RLC-IDs (e.g. `(Phase 8, RLC-01/RLC-02)`).
- No emoji.
- No `[Unreleased]` banner consumption — that block stays as-is on line 8.

**Specific adaptations required for Phase 8:**
- Insert at line 10 (above the existing `[4.1.2]` block). Date: use `date -u +%Y-%m-%d` on release day.
- Follow the exact content outlined in CONTEXT.md §D-34:
  ```markdown
  ## [4.2.0] - 2026-MM-DD

  ### Fixed
  - Presence updates no longer dropped during Discord rate-limit cooldown — latest update is coalesced and flushed when the limiter permits.
  - Race condition on presence-update shared state eliminated via atomic.Pointer.
  - Duplicate hook events (observed on PreToolUse firing 2x in 30-130ms) are now deduped server-side.

  ### Changed
  - Presence debouncer replaced with a token-bucket coalescer (4s cadence, burst 2) matching Discord RPC empirical limits.
  - FNV-64 content-hash now gates SetActivity calls — identical payloads are never resent.

  ### Added
  - Hook-dedup middleware wrapping /hooks/* routes, 500ms TTL.
  - Periodic 60s INFO summary with coalescer counters.
  ```
- Reformat to match the **bold-lead-phrase** style of `[4.1.2]` — e.g. `**Presence updates no longer dropped during Discord rate-limit cooldown** — latest update is coalesced and flushed when the limiter permits (Phase 8, RLC-01/RLC-02).`
- Run `./scripts/bump-version.sh 4.2.0` FIRST (per `CLAUDE.md §Releasing` + CONTEXT.md §D-34) — this updates `main.go`, `.claude-plugin/plugin.json`, `.claude-plugin/marketplace.json`, `scripts/start.sh`, `scripts/start.ps1` in a single pass. The CHANGELOG edit is the one manual step.

---

## Shared Patterns (Cross-Cutting)

### Authentication
**Not applicable.** All HTTP endpoints are localhost-only (`127.0.0.1` bind per DIST-55, enforced at `server/server.go:1528-1530`). No auth middleware exists; none required.

### Error Handling (applied to every new file)
**Source:** `main.go:544-547` + `server/server.go:604-610`.
```go
// Background goroutine error: log + continue
if err := discordClient.SetActivity(*a); err != nil {
    slog.Debug("discord SetActivity failed", "error", err)
    return
}

// Goroutine panic recovery
defer func() {
    if r := recover(); r != nil {
        slog.Error("post-tool-use background panic",
            "session", sessionID, "panic", r)
    }
}()
```
**Apply to:** Coalescer.Run (panic recovery), `cleanupLoop` (panic recovery), any `time.AfterFunc` callback that touches >1 atomic or calls external code.

### sync.Map TTL Cache (applied to hook dedup + anywhere else needing per-key TTL)
**Source:** `server/server.go:234 field + 589-603 usage`.
```go
lastTranscriptRead sync.Map  // map[sessionID]time.Time

// Lookup-and-gate idiom:
now := time.Now()
shouldRead := true
if lastVal, ok := s.lastTranscriptRead.Load(sessionID); ok {
    if lastTime, ok := lastVal.(time.Time); ok {
        if now.Sub(lastTime) < postToolUseThrottleInterval {
            shouldRead = false
        }
    }
}
if shouldRead {
    s.lastTranscriptRead.Store(sessionID, now)
    // ... do work
}
```
**Apply to:** `HookDedupMiddleware` — same Load-then-Store idiom with `hookDedupTTL = 500 * time.Millisecond`. Add a `cleanupLoop` goroutine for TTL eviction since this cache grows on every request (unlike `lastTranscriptRead` which is bounded by session count).

### Constants at File Top (applied to every new file with magic numbers)
**Source:** `main.go:36-47`.
**Apply to:**
- `coalescer/coalescer.go` — `DiscordRateInterval`, `DiscordRateBurst`, `DebounceDelay`, `SummaryInterval`.
- `coalescer/hash.go` — `const sep byte = 0x1F` (with comment: "ASCII Unit Separator — reserved for field delimiters, never appears in user content").
- `server/hook_dedup.go` — `hookDedupTTL`, `hookDedupCleanupPeriod`.

### Named Constant Separator (applied to FNV hashing + dedup key composition)
**Source:** RESEARCH.md discrepancy D3 (supersedes CONTEXT D-12's "\x00").
Use `0x1F` (ASCII Unit Separator) in BOTH:
- `coalescer/hash.go` (Activity field delimiter)
- `server/hook_dedup.go` (dedup key composition)

Define as a package-level `const sep byte = 0x1F` in each file (small duplication acceptable — avoids cross-package coupling).

### Defer-Recover on Background Goroutines (applied to every `go func` in new code)
**Source:** `server/server.go:604-610` (Phase 6 D-09 established precedent).
**Apply to:** `Coalescer.Run`, `HookDedupMiddleware.cleanupLoop`, any `time.AfterFunc` closure that allocates > trivial state.

### Context-Driven Shutdown (applied to every new goroutine)
**Source:** `main.go:409-460 autoExitLoop`.
```go
for {
    select {
    case <-ctx.Done():
        // cleanup (Stop timers, clear atomics)
        return
    case <-signal:
        // work
    }
}
```
**Apply to:** `Coalescer.Run`, `HookDedupMiddleware.cleanupLoop`. No `sync.WaitGroup` needed (CONTEXT.md §deferred confirms: `ctx.Done()` + idempotent cancel is sufficient).

### Build-Tag File Split (optional, not needed Phase 8)
**Source:** `discord/conn_unix.go` + `discord/conn_windows.go` + `session/stale_unix.go` + `session/stale_windows.go`.
Phase 8 does NOT need this — all new code is platform-agnostic. Only `scripts/phase-08/verify.sh` vs. `verify.ps1` represent the platform split, and those are shell/PowerShell, not Go build-tag files.

---

## No Analog Found

The following file has no direct Go analog in the repo — use RESEARCH.md patterns + stdlib docs instead:

| File | Role | Data Flow | Reason |
|------|------|-----------|--------|
| `coalescer/hash.go` | pure-function hash helper | pure `Activity → uint64` | No existing repo code uses `hash/fnv`. Follow RESEARCH.md §Pattern 4 (field-ordered `hasher.Write([]byte(a.Field))`) + stdlib `pkg.go.dev/hash/fnv`. |
| `server/server.go` Handler() wrap | first middleware in codebase | HTTP composition | No middleware exists yet. RESEARCH.md §Pattern 5 + §Pattern 7 cover body-preserving middleware. The `Handler()` return change itself is trivial (`return m.Wrap(mux)` instead of `return mux`). |

Everything else has a 1-to-1 or role-match analog in this repository.

---

## Metadata

**Analog search scope:** `main.go`, `discord/`, `server/`, `session/`, `resolver/`, `config/`, `scripts/`, `.github/workflows/`, `CHANGELOG.md`, `go.mod`.
**Files scanned:** 38 Go source/test + 10 scripts + 3 CI/CHANGELOG.
**Pattern extraction date:** 2026-04-16.
**Line-number freshness:** verified against the working tree on 2026-04-16; any planner edit landing before Phase 8 execution MUST re-verify by re-reading the cited line ranges.

*Phase: 08-presence-rate-limit-coalescer-stop-drop-on-skip-token-bucket*
*Pattern mapping complete: 2026-04-16*
