package session_test

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/StrainReviews/dsrcode/session"
)

// TestStaleCheckSkipsPidCheckForHttpSource verifies D-01 Phase 7: HTTP-sourced
// sessions with a dead PID are NOT removed by the PID-liveness check. They are
// only removed via the removeTimeout path (the 30m backstop).
func TestStaleCheckSkipsPidCheckForHttpSource(t *testing.T) {
	reg := session.NewRegistry(func() {})

	req := session.ActivityRequest{
		SessionID: "http-proj-1", // http- prefix => sourceFromID => SourceHTTP
		Cwd:       "/home/user/project",
	}
	// Dead PID: 99999999 is well above any realistic Unix PID and beyond the
	// Windows tasklist allocation ceiling.
	reg.StartSession(req, 99999999)

	// Activity 2.5 minutes ago: past the existing 2-minute grace, well before
	// the 30-minute removeTimeout.
	reg.SetLastActivityForTest("http-proj-1", time.Now().Add(-150*time.Second))

	session.CheckOnce(reg, 10*time.Minute, 30*time.Minute)

	if s := reg.GetSession("http-proj-1"); s == nil {
		t.Fatal("D-01 violation: HTTP-sourced session removed by PID-liveness check despite the SourceHTTP guard")
	}
}

// TestStaleCheckSkipsPidCheckForClaudeSource verifies D-01 Phase 9: SourceClaude
// (UUID) sessions with a dead PID are NOT removed by the PID-liveness check in
// production. Their PID is the short-lived wrapper-launcher (start.sh /
// start.ps1), not the Claude Code process itself. Only the removeTimeout
// backstop (30min default) should remove them.
//
// This test inverts the Phase-7 TestStaleCheckPreservesPidCheckForPidSource
// (which reflected a theoretical direct-spawn path production never took).
// Per Phase-9 CONTEXT D-03.
func TestStaleCheckSkipsPidCheckForClaudeSource(t *testing.T) {
	reg := session.NewRegistry(func() {})

	req := session.ActivityRequest{
		SessionID: "abcdef12-3456-7890-abcd-ef1234567890", // no prefix => SourceClaude
		Cwd:       "/home/user/project",
	}
	reg.StartSession(req, 99999999) // dead PID

	// Activity 5 minutes ago: well past the 2-minute grace, before removeTimeout.
	reg.SetLastActivityForTest("abcdef12-3456-7890-abcd-ef1234567890", time.Now().Add(-5*time.Minute))

	session.CheckOnce(reg, 10*time.Minute, 30*time.Minute)

	if s := reg.GetSession("abcdef12-3456-7890-abcd-ef1234567890"); s == nil {
		t.Fatal("D-01 Phase 9 violation: SourceClaude session removed by PID-liveness check despite the SourceClaude guard")
	}
}

// TestStaleCheckRemovesClaudeSourceAfterRemoveTimeout verifies D-05(b): the
// 30-min removeTimeout backstop still reaches truly stale SourceClaude
// sessions. Zombies that have stopped sending hooks entirely are removed
// even though the PID-liveness check is skipped for them.
func TestStaleCheckRemovesClaudeSourceAfterRemoveTimeout(t *testing.T) {
	reg := session.NewRegistry(func() {})

	req := session.ActivityRequest{
		SessionID: "fedcba98-7654-3210-fedc-ba9876543210", // UUID => SourceClaude
		Cwd:       "/home/user/zombie-project",
	}
	reg.StartSession(req, 99999999) // dead PID (but guard skips the liveness check)

	// Activity 31 minutes ago: past the 30-min removeTimeout backstop.
	reg.SetLastActivityForTest(req.SessionID, time.Now().Add(-31*time.Minute))

	session.CheckOnce(reg, 10*time.Minute, 30*time.Minute)

	if s := reg.GetSession(req.SessionID); s != nil {
		t.Error("D-05(b) violation: SourceClaude session with 31min-old activity NOT removed by removeTimeout backstop")
	}
}

// TestStaleCheckSurvivesUuidSourcedLongRunningAgent mirrors the live incident
// captured in ~/.claude/dsrcode.log 2026-04-18 11:37-11:44: session
// 2fe1b32a-ea1d-464f-8cac-375a4fe709c9, wrapper PID 5692, removed at elapsed
// 2m25s (150s) while Claude Code was still active. The v4.2.0 daemon auto-exited
// 6 seconds later. This test reproduces the exact elapsed interval (150s) from
// the incident and confirms the v4.2.1 fix keeps the session alive.
// Per Phase-9 CONTEXT §specifics (live-log anchor).
func TestStaleCheckSurvivesUuidSourcedLongRunningAgent(t *testing.T) {
	reg := session.NewRegistry(func() {})

	req := session.ActivityRequest{
		SessionID: "2fe1b32a-ea1d-464f-8cac-375a4fe709c9", // live-incident UUID
		Cwd:       "/home/user/long-running-project",
	}
	reg.StartSession(req, 5692) // the actual wrapper PID from the incident log

	// 150 seconds: the exact elapsed from the incident before auto-removal triggered in v4.2.0.
	reg.SetLastActivityForTest(req.SessionID, time.Now().Add(-150*time.Second))

	session.CheckOnce(reg, 10*time.Minute, 30*time.Minute)

	if s := reg.GetSession(req.SessionID); s == nil {
		t.Fatal("D-05(c) live-repro violation: incident session 2fe1b32a-... would have been removed again at 150s elapsed — v4.2.1 fix did not hold")
	}
}

// TestStaleCheckEmitsDebugOnGuardSkip verifies D-05(d) observability:
// after D-01 widens the outer guard to skip SourceClaude, the inner
// slog.Debug line at stale.go:47 MUST NOT fire for SourceClaude +
// elapsed < 2min. (If it did fire, the guard skip would only be partial —
// the test proves the guard skips the entire block, not just the
// EndSession call.)
//
// This is the NEGATIVE form of the CONTEXT D-05(d) assertion, per the
// RESEARCH Pitfall-1 resolution (Example 6). Positive-form "Debug fires
// for guard skip" is semantically impossible after D-01 because the
// Debug line is INSIDE the block the guard now skips.
//
// slog-capture pattern borrowed from coalescer/coalescer_test.go:267-282.
func TestStaleCheckEmitsDebugOnGuardSkip(t *testing.T) {
	var buf bytes.Buffer
	origLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(origLogger)

	reg := session.NewRegistry(func() {})
	req := session.ActivityRequest{SessionID: "uuid-guard-skip-test-abcd", Cwd: "/tmp/p"}
	reg.StartSession(req, 99999999)                                            // dead PID
	reg.SetLastActivityForTest(req.SessionID, time.Now().Add(-30*time.Second)) // inside 2min grace

	session.CheckOnce(reg, 10*time.Minute, 30*time.Minute)

	// NEGATIVE: the "PID dead but session has recent activity" debug line
	// lives INSIDE the guarded block. After D-01, SourceClaude sessions do
	// not enter that block at all — so the line MUST NOT appear.
	if strings.Contains(buf.String(), "PID dead but session has recent activity") {
		t.Errorf("D-05(d) violation: inner Debug line fired for SourceClaude, proving guard skip is partial; got: %q", buf.String())
	}

	// Positive: the session IS preserved (guard did its job).
	if s := reg.GetSession(req.SessionID); s == nil {
		t.Errorf("D-05(d) consistency violation: SourceClaude session removed despite guard skip expectation")
	}
}
