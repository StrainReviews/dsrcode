package coalescer_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"golang.org/x/time/rate"

	"github.com/StrainReviews/dsrcode/coalescer"
	"github.com/StrainReviews/dsrcode/config"
	"github.com/StrainReviews/dsrcode/discord"
	"github.com/StrainReviews/dsrcode/preset"
	"github.com/StrainReviews/dsrcode/session"
)

// mockDiscord captures every SetActivity call on a buffered channel.
// Follows discord/client_test.go:14-47 mockConn shape — one method, one
// channel, optional failErr for the disconnect path.
type mockDiscord struct {
	calls   chan discord.Activity
	failErr atomic.Value // error or nil
}

func newMockDiscord() *mockDiscord {
	return &mockDiscord{calls: make(chan discord.Activity, 100)}
}

func (m *mockDiscord) SetActivity(a discord.Activity) error {
	if v := m.failErr.Load(); v != nil {
		if err, ok := v.(error); ok && err != nil {
			return err
		}
	}
	m.calls <- a
	return nil
}

// testPreset returns a minimal valid MessagePreset so resolver.ResolvePresence
// returns a non-nil Activity for a single session.
func testPreset() *preset.MessagePreset {
	return &preset.MessagePreset{
		Label:                        "test",
		SingleSessionDetails:         map[string][]string{},
		SingleSessionDetailsFallback: []string{"Working on {project}"},
		SingleSessionState:           []string{"{model}"},
		MultiSessionMessages:         map[string][]string{},
		MultiSessionOverflow:         []string{"n sessions"},
		MultiSessionTooltips:         []string{"multi"},
	}
}

// newTestCoalescer builds a Coalescer with a buffered updateChan and an
// empty registry/preset. Returns everything needed to drive the pipeline.
// t.Helper() so failure lines point at the test, not this helper.
func newTestCoalescer(t *testing.T) (*coalescer.Coalescer, chan struct{}, *mockDiscord, *session.SessionRegistry) {
	t.Helper()
	updateCh := make(chan struct{}, 1)
	reg := session.NewRegistry(func() {})
	md := newMockDiscord()
	var pmu sync.RWMutex
	p := testPreset()
	cp := &p // **preset.MessagePreset
	detailGet := func() config.DisplayDetail { return config.DetailMinimal }
	c := coalescer.New(updateCh, reg, &pmu, cp, md, detailGet, nil)
	return c, updateCh, md, reg
}

// --- RLC-15 probe (MUST RUN FIRST — gates the rest of the suite) ---
//
// This test verifies that testing/synctest (Go 1.25 GA) virtualizes
// time.Now() across the boundary into golang.org/x/time/rate. If it
// passes, the rest of the coalescer test suite can rely on virtualized
// time for deterministic rate-limit behaviour. If it FAILS, the entire
// test strategy must fall back to injecting a ClockFunc into rate.Limiter
// (see 08-RESEARCH.md Assumption A4).

func TestSynctest_RateLimiterProbe(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		lim := rate.NewLimiter(rate.Every(time.Second), 1)

		// First reservation: bucket has 1 token -> Delay() == 0.
		r1 := lim.Reserve()
		if !r1.OK() {
			t.Fatal("first reservation !OK")
		}
		if d := r1.Delay(); d != 0 {
			t.Fatalf("first Reserve Delay = %v, want 0", d)
		}

		// Second reservation IMMEDIATELY: bucket empty -> Delay() > 0.
		r2 := lim.Reserve()
		if !r2.OK() {
			t.Fatal("second reservation !OK")
		}
		if d := r2.Delay(); d == 0 || d > 1100*time.Millisecond {
			t.Fatalf("second Reserve Delay = %v, want 0<d<=1s", d)
		}

		// Cancel r2 so it returns its token to the bucket — we only wanted
		// to probe the "bucket drained" path. Without cancel the Limiter
		// would treat r2's future token as consumed at t=1s, and any later
		// Reserve() would schedule against t=2s (see pkg.go.dev/golang.org/x/time/rate#Reservation.Cancel).
		r2.Cancel()

		// Advance virtual clock 1.1 s — bucket has 1 fresh token available.
		time.Sleep(1100 * time.Millisecond)
		synctest.Wait()

		// Third reservation: after refill + r2 cancel, Delay() == 0.
		r3 := lim.Reserve()
		if d := r3.Delay(); d != 0 {
			t.Fatalf("after sleep Reserve Delay = %v, want 0 (synctest+rate.Limiter INCOMPATIBLE — fall back to ClockFunc injection per 08-RESEARCH.md A4)", d)
		}
	})
}

// --- RLC-01 Token bucket coalescing ---
//
// 10 updateChan signals in <100 ms must collapse into a single coalesced
// flush (the 100 ms debounce + single-slot pending eat the extras). After
// the initial burst (2 tokens), further flushes must wait 4 s per token.

func TestCoalescer_TokenBucketRate(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c, updateCh, md, reg := newTestCoalescer(t)
		// Seed a session so resolver returns non-nil.
		reg.StartSession(session.ActivityRequest{SessionID: "s1", Cwd: "/tmp"}, 1234)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go c.Run(ctx)

		// Fire 10 signals; updateChan is buffered(1), so extra sends drop —
		// that is the intended front-aggregator. onSignal's 100 ms debounce
		// then collapses any signals the channel did deliver into one
		// resolveAndEnqueue call.
		for i := 0; i < 10; i++ {
			select {
			case updateCh <- struct{}{}:
			default:
			}
		}
		// Wait for debounce (100 ms) + burst flush.
		time.Sleep(200 * time.Millisecond)
		synctest.Wait()

		// After 10 signals, exactly 1 coalesced SetActivity call.
		assertCalls(t, md.calls, 1, "10 signals in <100 ms -> 1 coalesced flush")

		// No new signals -> no new flush even after 4 s advance.
		time.Sleep(5 * time.Second)
		synctest.Wait()
		assertCalls(t, md.calls, 0, "no new signals -> no new flush after refill")

		// Fire another signal; expect flush within 4 s (token available).
		updateCh <- struct{}{}
		time.Sleep(5 * time.Second)
		synctest.Wait()
		assertCalls(t, md.calls, 1, "after 4 s refill + signal, expect 1 flush")
	})
}

// --- RLC-02 atomic.Pointer pending slot ---
//
// resolveAndEnqueue called twice with different registry state must result
// in pending.Load() returning the second Activity (Store overwrites).

func TestCoalescer_PendingSlot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c, _, _, reg := newTestCoalescer(t)
		reg.StartSession(session.ActivityRequest{SessionID: "s1", Cwd: "/a"}, 1234)

		// Drain both burst tokens first so the next schedule() defers the
		// flush via time.AfterFunc instead of running it inline. Without
		// this, the Delay==0 fast path would flush-and-clear pending inside
		// resolveAndEnqueue and PendingForTest would always see nil.
		c.DrainBucketForTest()

		// Drive resolveAndEnqueue twice — second Store must replace first.
		c.ResolveAndEnqueueForTest()
		first := c.PendingForTest()
		if first == nil {
			t.Fatal("first resolveAndEnqueue did not store pending")
		}
		// Mutate registry so the resolver produces a different Activity.
		reg.UpdateActivity("s1", session.ActivityRequest{
			SessionID:     "s1",
			SmallImageKey: "coding",
			SmallText:     "Editing",
		})
		c.ResolveAndEnqueueForTest()
		second := c.PendingForTest()
		if second == nil {
			t.Fatal("second resolveAndEnqueue did not store pending")
		}
		if second == first {
			t.Fatal("second Store did not replace first (same pointer)")
		}
	})
}

// --- RLC-03 AfterFunc flush scheduling ---
//
// After draining the burst budget, the next signal must schedule a deferred
// flush (time.AfterFunc) that fires exactly once at Delay(). A second
// flushPending call after Swap must be a no-op (idempotent per D-23).

func TestCoalescer_FlushSchedule(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c, updateCh, md, reg := newTestCoalescer(t)
		reg.StartSession(session.ActivityRequest{SessionID: "s1", Cwd: "/a"}, 1234)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go c.Run(ctx)

		// Drain both burst tokens. Each signal is separated enough that the
		// debounce window closes and resolveAndEnqueue/schedule runs.
		updateCh <- struct{}{}
		time.Sleep(200 * time.Millisecond)
		synctest.Wait()
		// Mutate registry so the second signal resolves to a distinct
		// Activity (otherwise the single-slot pending makes this a no-op
		// once Plan 08-02 lands, but here we only care about scheduling).
		reg.UpdateActivity("s1", session.ActivityRequest{SessionID: "s1", SmallImageKey: "coding"})
		updateCh <- struct{}{}
		time.Sleep(200 * time.Millisecond)
		synctest.Wait()

		beforeLen := len(md.calls)
		// Third signal must schedule a deferred flush, not fire immediately.
		reg.UpdateActivity("s1", session.ActivityRequest{SessionID: "s1", SmallImageKey: "thinking"})
		updateCh <- struct{}{}
		time.Sleep(200 * time.Millisecond)
		synctest.Wait()
		if len(md.calls) != beforeLen {
			t.Fatalf("third signal fired immediately — expected deferred; got %d new calls", len(md.calls)-beforeLen)
		}
		// Advance ~4 s so the deferred flush fires.
		time.Sleep(5 * time.Second)
		synctest.Wait()
		if len(md.calls) != beforeLen+1 {
			t.Fatalf("after 4 s wait, want %d calls, got %d", beforeLen+1, len(md.calls))
		}
	})
}

// --- RLC-11 Summary skipped when idle ---
//
// Forcing emitSummary() with zero counters must NOT emit a "coalescer
// status" INFO line. Captured via slog text handler into a buffer.

func TestCoalescer_SummaryIdleSkip(t *testing.T) {
	var buf bytes.Buffer
	origLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(origLogger)

	synctest.Test(t, func(t *testing.T) {
		c, _, _, _ := newTestCoalescer(t)
		// No signals; force emitSummary with all counters at 0.
		c.EmitSummaryForTest()
	})

	if strings.Contains(buf.String(), "coalescer status") {
		t.Errorf("idle emitSummary produced log line: %q", buf.String())
	}
}

// --- RLC-12 Shutdown idempotent ---
//
// Storing a pending Activity, then calling Shutdown twice, must not panic
// and must leave pending.Load() == nil with all timers stopped.

func TestCoalescer_ShutdownIdempotent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c, _, _, reg := newTestCoalescer(t)
		reg.StartSession(session.ActivityRequest{SessionID: "s1", Cwd: "/a"}, 1234)
		// Drain burst tokens so resolveAndEnqueue's pending.Store sticks
		// (otherwise the inline flush path clears it immediately).
		c.DrainBucketForTest()
		c.ResolveAndEnqueueForTest()
		if c.PendingForTest() == nil {
			t.Fatal("no pending after resolveAndEnqueue")
		}
		c.Shutdown()
		c.Shutdown() // must not panic
		if c.PendingForTest() != nil {
			t.Fatal("pending not cleared after Shutdown")
		}
	})
}

// --- RLC-13 Reconnect discards pending on IPC error ---
//
// When SetActivity returns an error (simulating a closed IPC), flushPending
// must atomically Swap the pending slot to nil so no stale Activity is
// retried. Per D-33 the coalescer only logs + discards; connectionLoop
// in main.go is responsible for re-triggering a fresh updateChan signal.

func TestCoalescer_ReconnectDiscard(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c, updateCh, md, reg := newTestCoalescer(t)
		md.failErr.Store(error(errIPCClosed))
		reg.StartSession(session.ActivityRequest{SessionID: "s1", Cwd: "/a"}, 1234)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go c.Run(ctx)

		updateCh <- struct{}{}
		time.Sleep(200 * time.Millisecond)
		synctest.Wait()

		if c.PendingForTest() != nil {
			t.Fatal("pending not discarded after SetActivity error")
		}
	})
}

// --- helpers ---

var errIPCClosed = discordSetError("ipc closed")

type discordSetError string

func (e discordSetError) Error() string { return string(e) }

// assertCalls drains all currently-buffered calls from ch and asserts
// count == want. synctest.Wait() must have been called by the caller
// before invoking this, so the buffered channel reflects final state.
func assertCalls(t *testing.T, ch chan discord.Activity, want int, msg string) {
	t.Helper()
	got := 0
	for {
		select {
		case <-ch:
			got++
		default:
			if got != want {
				t.Fatalf("%s: got %d calls, want %d", msg, got, want)
			}
			return
		}
	}
}
