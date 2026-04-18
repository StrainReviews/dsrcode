package session_test

import (
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
