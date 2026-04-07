package session_test

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/tsanva/cc-discord-presence/session"
)

// TestUpdateTranscriptPath verifies that UpdateTranscriptPath stores the path
// on the session using the immutable copy pattern.
func TestUpdateTranscriptPath(t *testing.T) {
	changed := 0
	reg := session.NewRegistry(func() { changed++ })

	req := session.ActivityRequest{
		SessionID: "sess-tp",
		Cwd:       "/home/user/project",
	}
	reg.StartSession(req, 1234)
	changed = 0 // reset after start

	reg.UpdateTranscriptPath("sess-tp", "/home/user/.claude/projects/abc/session.jsonl")

	s := reg.GetSession("sess-tp")
	if s == nil {
		t.Fatal("GetSession returned nil")
	}
	if s.TranscriptPath != "/home/user/.claude/projects/abc/session.jsonl" {
		t.Errorf("TranscriptPath = %q, want %q", s.TranscriptPath, "/home/user/.claude/projects/abc/session.jsonl")
	}
	if changed != 1 {
		t.Errorf("onChange called %d times, want 1", changed)
	}
}

// TestUpdateTranscriptPathNonexistent verifies that UpdateTranscriptPath
// on a nonexistent session is a no-op and does not call onChange.
func TestUpdateTranscriptPathNonexistent(t *testing.T) {
	changed := 0
	reg := session.NewRegistry(func() { changed++ })

	reg.UpdateTranscriptPath("nonexistent", "/some/path")

	if changed != 0 {
		t.Errorf("onChange called %d times for nonexistent session, want 0", changed)
	}
}

// TestUpdateTranscriptPathOverwrite verifies that a second call overwrites the path.
func TestUpdateTranscriptPathOverwrite(t *testing.T) {
	reg := session.NewRegistry(func() {})

	req := session.ActivityRequest{
		SessionID: "sess-tp2",
		Cwd:       "/home/user/project",
	}
	reg.StartSession(req, 1234)

	reg.UpdateTranscriptPath("sess-tp2", "/path/one.jsonl")
	reg.UpdateTranscriptPath("sess-tp2", "/path/two.jsonl")

	s := reg.GetSession("sess-tp2")
	if s == nil {
		t.Fatal("GetSession returned nil")
	}
	if s.TranscriptPath != "/path/two.jsonl" {
		t.Errorf("TranscriptPath = %q, want %q", s.TranscriptPath, "/path/two.jsonl")
	}
}

// TestUpdateAnalytics verifies that UpdateAnalytics sets all analytics fields
// on the session using the immutable copy pattern.
func TestUpdateAnalytics(t *testing.T) {
	changed := 0
	reg := session.NewRegistry(func() { changed++ })

	req := session.ActivityRequest{
		SessionID: "sess-analytics",
		Cwd:       "/home/user/project",
	}
	reg.StartSession(req, 1234)
	changed = 0

	ctxPct := 78.5
	update := session.AnalyticsUpdate{
		TokenBreakdownRaw: json.RawMessage(`{"opus":{"input":1000,"output":500}}`),
		TokenBaselinesRaw: json.RawMessage(`{"opus":{"input":200,"output":100}}`),
		ToolCounts:        map[string]int{"Edit": 42, "Bash": 12, "Grep": 5},
		SubagentTreeRaw:   json.RawMessage(`[{"id":"sub-1","type":"Explore","status":"done"}]`),
		CompactionCount:   2,
		ContextUsagePct:   &ctxPct,
		CostBreakdownRaw:  json.RawMessage(`{"total":1.20,"input":0.80,"output":0.35,"cache":0.05}`),
	}

	reg.UpdateAnalytics("sess-analytics", update)

	s := reg.GetSession("sess-analytics")
	if s == nil {
		t.Fatal("GetSession returned nil")
	}

	// Verify TokenBreakdownRaw
	if string(s.TokenBreakdownRaw) != `{"opus":{"input":1000,"output":500}}` {
		t.Errorf("TokenBreakdownRaw = %s, want opus breakdown", string(s.TokenBreakdownRaw))
	}

	// Verify TokenBaselinesRaw
	if string(s.TokenBaselinesRaw) != `{"opus":{"input":200,"output":100}}` {
		t.Errorf("TokenBaselinesRaw = %s, want opus baselines", string(s.TokenBaselinesRaw))
	}

	// Verify ToolCounts
	if s.ToolCounts == nil {
		t.Fatal("ToolCounts is nil")
	}
	if s.ToolCounts["Edit"] != 42 {
		t.Errorf("ToolCounts[Edit] = %d, want 42", s.ToolCounts["Edit"])
	}
	if s.ToolCounts["Bash"] != 12 {
		t.Errorf("ToolCounts[Bash] = %d, want 12", s.ToolCounts["Bash"])
	}
	if s.ToolCounts["Grep"] != 5 {
		t.Errorf("ToolCounts[Grep] = %d, want 5", s.ToolCounts["Grep"])
	}

	// Verify SubagentTreeRaw
	if string(s.SubagentTreeRaw) != `[{"id":"sub-1","type":"Explore","status":"done"}]` {
		t.Errorf("SubagentTreeRaw = %s, want subagent array", string(s.SubagentTreeRaw))
	}

	// Verify CompactionCount
	if s.CompactionCount != 2 {
		t.Errorf("CompactionCount = %d, want 2", s.CompactionCount)
	}

	// Verify ContextUsagePct
	if s.ContextUsagePct == nil {
		t.Fatal("ContextUsagePct is nil")
	}
	if *s.ContextUsagePct != 78.5 {
		t.Errorf("ContextUsagePct = %f, want 78.5", *s.ContextUsagePct)
	}

	// Verify CostBreakdownRaw
	if string(s.CostBreakdownRaw) != `{"total":1.20,"input":0.80,"output":0.35,"cache":0.05}` {
		t.Errorf("CostBreakdownRaw = %s, want cost breakdown", string(s.CostBreakdownRaw))
	}

	// Verify onChange was called
	if changed != 1 {
		t.Errorf("onChange called %d times, want 1", changed)
	}
}

// TestUpdateAnalyticsPartial verifies that UpdateAnalytics only updates
// non-nil fields, preserving existing values for nil fields.
func TestUpdateAnalyticsPartial(t *testing.T) {
	reg := session.NewRegistry(func() {})

	req := session.ActivityRequest{
		SessionID: "sess-partial",
		Cwd:       "/home/user/project",
	}
	reg.StartSession(req, 1234)

	// First update: set all fields
	ctxPct := 50.0
	reg.UpdateAnalytics("sess-partial", session.AnalyticsUpdate{
		TokenBreakdownRaw: json.RawMessage(`{"opus":{"input":1000}}`),
		ToolCounts:        map[string]int{"Edit": 10},
		CompactionCount:   1,
		ContextUsagePct:   &ctxPct,
	})

	// Second update: only update ToolCounts and CompactionCount
	reg.UpdateAnalytics("sess-partial", session.AnalyticsUpdate{
		ToolCounts:      map[string]int{"Edit": 20, "Bash": 5},
		CompactionCount: 3,
	})

	s := reg.GetSession("sess-partial")
	if s == nil {
		t.Fatal("GetSession returned nil")
	}

	// TokenBreakdownRaw should be preserved (not overwritten by nil)
	if string(s.TokenBreakdownRaw) != `{"opus":{"input":1000}}` {
		t.Errorf("TokenBreakdownRaw = %s, want preserved value", string(s.TokenBreakdownRaw))
	}

	// ToolCounts should be updated
	if s.ToolCounts["Edit"] != 20 {
		t.Errorf("ToolCounts[Edit] = %d, want 20", s.ToolCounts["Edit"])
	}
	if s.ToolCounts["Bash"] != 5 {
		t.Errorf("ToolCounts[Bash] = %d, want 5", s.ToolCounts["Bash"])
	}

	// CompactionCount always updates (not nil-guarded)
	if s.CompactionCount != 3 {
		t.Errorf("CompactionCount = %d, want 3", s.CompactionCount)
	}

	// ContextUsagePct should be preserved (nil in update)
	if s.ContextUsagePct == nil {
		t.Fatal("ContextUsagePct should be preserved, got nil")
	}
	if *s.ContextUsagePct != 50.0 {
		t.Errorf("ContextUsagePct = %f, want 50.0 (preserved)", *s.ContextUsagePct)
	}
}

// TestUpdateAnalyticsNonexistent verifies that UpdateAnalytics on a
// nonexistent session is a no-op and does not call onChange.
func TestUpdateAnalyticsNonexistent(t *testing.T) {
	changed := 0
	reg := session.NewRegistry(func() { changed++ })

	reg.UpdateAnalytics("nonexistent", session.AnalyticsUpdate{
		CompactionCount: 5,
	})

	if changed != 0 {
		t.Errorf("onChange called %d times for nonexistent session, want 0", changed)
	}
}

// TestUpdateAnalyticsConcurrent verifies that concurrent UpdateAnalytics,
// UpdateTranscriptPath, and GetSession calls do not cause data races.
func TestUpdateAnalyticsConcurrent(t *testing.T) {
	reg := session.NewRegistry(func() {})

	req := session.ActivityRequest{
		SessionID: "sess-concurrent-analytics",
		Cwd:       "/home/user/project",
	}
	reg.StartSession(req, 1234)

	var wg sync.WaitGroup
	const goroutines = 50

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			sid := "sess-concurrent-analytics"

			switch n % 4 {
			case 0:
				ctxPct := float64(n)
				reg.UpdateAnalytics(sid, session.AnalyticsUpdate{
					ToolCounts:      map[string]int{"Edit": n},
					CompactionCount: n,
					ContextUsagePct: &ctxPct,
				})
			case 1:
				reg.UpdateTranscriptPath(sid, "/path/to/session.jsonl")
			case 2:
				reg.GetSession(sid)
			case 3:
				reg.GetAllSessions()
			}
		}(i)
	}

	wg.Wait()
	// If we reach here without race detector panic, the test passes
}

// TestUpdateAnalyticsImmutability verifies that the copy-before-modify pattern
// prevents mutations from affecting previously retrieved session copies.
func TestUpdateAnalyticsImmutability(t *testing.T) {
	reg := session.NewRegistry(func() {})

	req := session.ActivityRequest{
		SessionID: "sess-immutable",
		Cwd:       "/home/user/project",
	}
	reg.StartSession(req, 1234)

	// Get a copy before analytics update
	before := reg.GetSession("sess-immutable")
	if before == nil {
		t.Fatal("GetSession returned nil")
	}

	// Update analytics
	ctxPct := 90.0
	reg.UpdateAnalytics("sess-immutable", session.AnalyticsUpdate{
		ToolCounts:      map[string]int{"Edit": 99},
		CompactionCount: 7,
		ContextUsagePct: &ctxPct,
	})

	// The "before" copy should NOT be affected
	if before.CompactionCount != 0 {
		t.Errorf("before.CompactionCount = %d, want 0 (immutable copy should be unchanged)", before.CompactionCount)
	}
	if before.ContextUsagePct != nil {
		t.Errorf("before.ContextUsagePct should be nil (immutable copy should be unchanged)")
	}
	if before.ToolCounts != nil {
		t.Errorf("before.ToolCounts should be nil (immutable copy should be unchanged)")
	}

	// The new copy should have the updated values
	after := reg.GetSession("sess-immutable")
	if after.CompactionCount != 7 {
		t.Errorf("after.CompactionCount = %d, want 7", after.CompactionCount)
	}
}
