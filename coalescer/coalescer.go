// Package coalescer owns the single-goroutine loop that serializes
// Discord presence updates. It replaces the drop-on-skip presenceDebouncer
// with a pending-state buffer + token-bucket limiter so updates queued
// during a rate-limit cooldown are flushed exactly once when the limiter
// permits — never discarded.
//
// The design is lock-free on the hot path: the pending Activity lives in
// an atomic.Pointer slot, counters are atomic.Int64, and the last-sent
// hash is atomic.Uint64. Only Coalescer.Run touches the rate.Limiter —
// all other producers push struct{}{} tokens into updateChan. See Phase
// 8 (CONTEXT.md D-05..D-33) for the full decision record.
package coalescer

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"

	"github.com/StrainReviews/dsrcode/config"
	"github.com/StrainReviews/dsrcode/discord"
	"github.com/StrainReviews/dsrcode/preset"
	"github.com/StrainReviews/dsrcode/resolver"
	"github.com/StrainReviews/dsrcode/session"
)

const (
	// DiscordRateInterval is the minimum cadence between Discord SetActivity
	// calls. Empirical: Discord RPC accepts ~5 updates per 20 s; 4 s is the
	// conservative-but-snappy middle ground. See 08-CONTEXT.md D-05.
	DiscordRateInterval = 4 * time.Second

	// DiscordRateBurst is how many tokens the bucket starts with. Burst=2
	// lets the first two coalesced updates after daemon start flush
	// immediately so the presence lands on Discord with <100 ms perceived
	// latency on cold boot. See 08-CONTEXT.md D-05 + D-25.
	DiscordRateBurst = 2

	// DebounceDelay is the front-aggregator window — rapid bursts of
	// signals from different producers collapse into one resolver run
	// before the bucket is touched. See 08-CONTEXT.md D-19.
	DebounceDelay = 100 * time.Millisecond

	// SummaryInterval is the cadence for the structured INFO summary log.
	// Skipped entirely when all counters are zero (D-27 + Claude's Discretion).
	SummaryInterval = 60 * time.Second
)

// discordSetter is the minimum surface the Coalescer needs from the
// Discord client. Accept interfaces, return structs — this lets tests
// supply a capturing mock without wiring a real IPC connection.
type discordSetter interface {
	SetActivity(a discord.Activity) error
}

// Coalescer buffers the latest pending *discord.Activity in a lock-free
// atomic slot and flushes it to the Discord client when the token-bucket
// limiter permits. A single long-running goroutine owns the limiter +
// timers; external callers only push signals into updateChan.
type Coalescer struct {
	updateChan    <-chan struct{}
	registry      *session.SessionRegistry
	presetMu      *sync.RWMutex
	currentPreset **preset.MessagePreset
	discord       discordSetter
	detailGetter  func() config.DisplayDetail
	getDedupCount func() int64 // injected by 08-03; default returns 0

	limiter    *rate.Limiter
	debounce   *time.Timer // 100ms front-aggregator (Stop-replace idiom)
	flushTimer *time.Timer // bucket-reservation flusher (Stop-replace idiom)

	pending      atomic.Pointer[discord.Activity]
	lastSentHash atomic.Uint64 // written by Plan 08-02; read-only here

	sent     atomic.Int64
	skipRate atomic.Int64
	skipHash atomic.Int64 // incremented by Plan 08-02's hash gate
}

// New constructs a Coalescer. getDedupCount may be nil; callers without a
// dedup middleware should pass nil — a no-op will be substituted so the
// 60 s summary log reports zero dedups. Plan 08-03 supplies the real
// getter.
func New(
	updateChan <-chan struct{},
	registry *session.SessionRegistry,
	presetMu *sync.RWMutex,
	currentPreset **preset.MessagePreset,
	discordClient discordSetter,
	detailGetter func() config.DisplayDetail,
	getDedupCount func() int64,
) *Coalescer {
	if getDedupCount == nil {
		getDedupCount = func() int64 { return 0 }
	}
	return &Coalescer{
		updateChan:    updateChan,
		registry:      registry,
		presetMu:      presetMu,
		currentPreset: currentPreset,
		discord:       discordClient,
		detailGetter:  detailGetter,
		getDedupCount: getDedupCount,
		limiter:       rate.NewLimiter(rate.Every(DiscordRateInterval), DiscordRateBurst),
	}
}

// Run is the main loop. Blocks until ctx is cancelled. Must run in its
// own goroutine. Defer-recovers per Phase 6 D-09 so a panic doesn't kill
// the daemon — it logs + calls Shutdown to clean up timers.
func (c *Coalescer) Run(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("coalescer panic", "panic", r)
		}
		c.Shutdown()
	}()

	summary := time.NewTicker(SummaryInterval)
	defer summary.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.updateChan:
			c.onSignal()
		case <-summary.C:
			c.emitSummary()
		}
	}
}

// onSignal is the entry for every updateChan tick. Applies the 100 ms
// front-aggregator: a rapid burst of N signals in <100 ms collapses into
// a single resolveAndEnqueue call.
func (c *Coalescer) onSignal() {
	if c.debounce != nil {
		c.debounce.Stop()
	}
	c.debounce = time.AfterFunc(DebounceDelay, c.resolveAndEnqueue)
}

// resolveAndEnqueue runs the resolver once, stores the result in the
// pending slot, and asks schedule() to book a token.
func (c *Coalescer) resolveAndEnqueue() {
	sessions := c.registry.GetAllSessions()
	c.presetMu.RLock()
	p := *c.currentPreset
	c.presetMu.RUnlock()

	a := resolver.ResolvePresence(sessions, p, c.detailGetter(), time.Now())
	if a == nil {
		return
	}

	// TODO(08-02): content-hash skip inserted here.
	// if hashActivity(a) == c.lastSentHash.Load() {
	//     c.skipHash.Add(1)
	//     slog.Debug("presence update skipped", "reason", "content_hash")
	//     return
	// }

	c.pending.Store(a)
	c.schedule()
}

// schedule acquires a reservation from the token bucket and either
// flushes inline (Delay==0, fast path) or schedules a single-shot
// AfterFunc. The previous flushTimer is always Stop()ed first so only
// the latest-scheduled flush ever runs.
func (c *Coalescer) schedule() {
	if c.flushTimer != nil {
		c.flushTimer.Stop()
	}
	r := c.limiter.Reserve()
	if !r.OK() {
		// Only happens if burst==0; we configured 2. Defensive.
		slog.Warn("coalescer reservation !ok", "burst", c.limiter.Burst())
		return
	}
	if d := r.Delay(); d == 0 {
		c.flushPending()
	} else {
		c.skipRate.Add(1) // deferred flush == "rate-limit in effect"
		c.flushTimer = time.AfterFunc(d, c.flushPending)
	}
}

// flushPending atomically takes the pending slot and flushes it to
// Discord. Idempotent: if another flush already ran, Swap returns nil
// and this is a no-op — matches the main.go:427 graceCtx.Err() pattern
// for time.AfterFunc idempotency.
func (c *Coalescer) flushPending() {
	a := c.pending.Swap(nil)
	if a == nil {
		return
	}
	if err := c.discord.SetActivity(*a); err != nil {
		// D-33: discord IPC error -> debug log + discard. Don't retry.
		// connectionLoop in main.go will detect disconnect and reconnect,
		// triggering a fresh updateChan signal.
		slog.Debug("discord SetActivity failed", "error", err)
		return
	}
	// Plan 08-02 stores the hash here.
	c.sent.Add(1)
}

// emitSummary flushes and resets the four counters. Skips the log if
// all counters are zero — avoids noise on idle daemons (D-27 Claude's
// Discretion). Reads dedup counter from the injected getter.
func (c *Coalescer) emitSummary() {
	sent := c.sent.Swap(0)
	sr := c.skipRate.Swap(0)
	sh := c.skipHash.Swap(0)
	dd := c.getDedupCount()
	if sent+sr+sh+dd == 0 {
		return
	}
	slog.Info("coalescer status",
		"sent", sent,
		"skipped_rate", sr,
		"skipped_hash", sh,
		"deduped", dd,
	)
}

// Shutdown clears pending and stops all timers. Idempotent. Called by
// Run's defer on ctx.Done() and also directly from main.go's D-07
// shutdown sequence (BEFORE the direct-to-Discord clear-activity call,
// so no new SetActivity from the Coalescer can race with the clear).
func (c *Coalescer) Shutdown() {
	c.pending.Store(nil)
	if c.debounce != nil {
		c.debounce.Stop()
	}
	if c.flushTimer != nil {
		c.flushTimer.Stop()
	}
}

// --- Test-only accessors ---
//
// The ForTest suffix signals "do not use in production code paths".
// These exist because coalescer_test.go lives in the external package
// coalescer_test and cannot reach unexported fields or methods.

// PendingForTest returns the current pending *discord.Activity WITHOUT
// clearing it. Intended for tests only — exposes a read-only view of the
// atomic slot so tests can assert Store/Swap semantics.
func (c *Coalescer) PendingForTest() *discord.Activity {
	return c.pending.Load()
}

// ResolveAndEnqueueForTest exposes the internal resolve+enqueue step so
// tests can drive the pipeline without relying on a real signal bus.
func (c *Coalescer) ResolveAndEnqueueForTest() {
	c.resolveAndEnqueue()
}

// ScheduleForTest exposes schedule() so tests can drive the token-bucket
// path directly with a pre-set pending value.
func (c *Coalescer) ScheduleForTest() {
	c.schedule()
}

// EmitSummaryForTest forces the 60 s summary log path to run once.
func (c *Coalescer) EmitSummaryForTest() {
	c.emitSummary()
}

// DrainBucketForTest consumes all tokens currently in the limiter bucket so
// subsequent schedule() calls see Delay() > 0 and defer the flush via
// time.AfterFunc. Tests that want to observe the pending slot or deferred
// scheduling must call this before driving resolveAndEnqueue, otherwise the
// initial burst flushes inline and PendingForTest returns nil.
func (c *Coalescer) DrainBucketForTest() {
	for i := 0; i < DiscordRateBurst; i++ {
		c.limiter.Reserve()
	}
}
