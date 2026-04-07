package analytics_test

import (
	"testing"
	"time"

	"github.com/tsanva/cc-discord-presence/analytics"
)

// TestTrackerRecordTool verifies that the tracker dispatches tool events to the tools counter.
func TestTrackerRecordTool(t *testing.T) {
	tracker := analytics.NewTracker(analytics.TrackerConfig{
		Features: analytics.Features{Analytics: true},
	})

	tracker.RecordTool("test-session", "Edit")
	tracker.RecordTool("test-session", "Edit")
	tracker.RecordTool("test-session", "Bash")

	state := tracker.GetState("test-session")
	if state.ToolCounts["Edit"] != 2 {
		t.Errorf("Edit count = %d, want 2", state.ToolCounts["Edit"])
	}
	if state.ToolCounts["Bash"] != 1 {
		t.Errorf("Bash count = %d, want 1", state.ToolCounts["Bash"])
	}
}

// TestTrackerRecordSubagentSpawn verifies that the tracker dispatches subagent spawn events.
func TestTrackerRecordSubagentSpawn(t *testing.T) {
	tracker := analytics.NewTracker(analytics.TrackerConfig{
		Features: analytics.Features{Analytics: true},
	})

	tracker.RecordSubagentSpawn("test-session", analytics.SubagentEntry{
		ID:          "agent-001",
		Type:        "Explore",
		Description: "scan project",
		SpawnTime:   time.Now(),
	})

	state := tracker.GetState("test-session")
	if len(state.Subagents) != 1 {
		t.Fatalf("expected 1 subagent, got %d", len(state.Subagents))
	}
	if state.Subagents[0].Type != "Explore" {
		t.Errorf("subagent type = %q, want %q", state.Subagents[0].Type, "Explore")
	}
}

// TestTrackerRecordSubagentComplete verifies that the tracker dispatches subagent completion.
func TestTrackerRecordSubagentComplete(t *testing.T) {
	tracker := analytics.NewTracker(analytics.TrackerConfig{
		Features: analytics.Features{Analytics: true},
	})

	tracker.RecordSubagentSpawn("test-session", analytics.SubagentEntry{
		ID:          "agent-001",
		Type:        "Explore",
		Description: "scan project structure for patterns",
		SpawnTime:   time.Now(),
	})

	tracker.RecordSubagentComplete("test-session", analytics.SubagentStopEvent{
		AgentID: "agent-001",
	})

	state := tracker.GetState("test-session")
	if len(state.Subagents) != 1 {
		t.Fatalf("expected 1 subagent, got %d", len(state.Subagents))
	}
	if state.Subagents[0].Status != "completed" {
		t.Errorf("subagent status = %q, want %q", state.Subagents[0].Status, "completed")
	}
}

// TestTrackerUpdateTokens verifies that the tracker dispatches token updates
// with baseline checking for compaction recovery.
func TestTrackerUpdateTokens(t *testing.T) {
	tracker := analytics.NewTracker(analytics.TrackerConfig{
		Features: analytics.Features{Analytics: true},
	})

	tracker.UpdateTokens("test-session", "claude-opus-4-6", analytics.TokenBreakdown{
		Input: 100_000, Output: 40_000,
	})

	state := tracker.GetState("test-session")
	tokens := state.Tokens["claude-opus-4-6"]
	if tokens.Input != 100_000 {
		t.Errorf("Input = %d, want 100000", tokens.Input)
	}
	if tokens.Output != 40_000 {
		t.Errorf("Output = %d, want 40000", tokens.Output)
	}
}

// TestTrackerRecordCompaction verifies that the tracker increments the compaction count.
func TestTrackerRecordCompaction(t *testing.T) {
	tracker := analytics.NewTracker(analytics.TrackerConfig{
		Features: analytics.Features{Analytics: true},
	})

	tracker.RecordCompaction("test-session", "compaction-uuid-1")
	tracker.RecordCompaction("test-session", "compaction-uuid-2")

	state := tracker.GetState("test-session")
	if state.CompactionCount != 2 {
		t.Errorf("CompactionCount = %d, want 2", state.CompactionCount)
	}
}

// TestTrackerUpdateContextUsage verifies that the tracker updates context%.
func TestTrackerUpdateContextUsage(t *testing.T) {
	tracker := analytics.NewTracker(analytics.TrackerConfig{
		Features: analytics.Features{Analytics: true},
	})

	tracker.UpdateContextUsage("test-session", 78.5)

	state := tracker.GetState("test-session")
	if state.ContextPct == nil {
		t.Fatal("ContextPct is nil")
	}
	if *state.ContextPct != 78.5 {
		t.Errorf("ContextPct = %f, want 78.5", *state.ContextPct)
	}
}

// TestTrackerFeatureDisabled verifies that when features.analytics=false,
// all record methods are no-ops (D-34).
func TestTrackerFeatureDisabled(t *testing.T) {
	tracker := analytics.NewTracker(analytics.TrackerConfig{
		Features: analytics.Features{Analytics: false},
	})

	// All these should be no-ops
	tracker.RecordTool("test-session", "Edit")
	tracker.RecordSubagentSpawn("test-session", analytics.SubagentEntry{
		ID:   "agent-001",
		Type: "Explore",
	})
	tracker.RecordSubagentComplete("test-session", analytics.SubagentStopEvent{
		AgentID: "agent-001",
	})
	tracker.UpdateTokens("test-session", "claude-opus-4-6", analytics.TokenBreakdown{
		Input: 100_000,
	})
	tracker.RecordCompaction("test-session", "uuid-1")
	tracker.UpdateContextUsage("test-session", 78.5)

	state := tracker.GetState("test-session")

	// Everything should be zero/nil/empty
	if len(state.ToolCounts) != 0 {
		t.Errorf("ToolCounts should be empty, got %v", state.ToolCounts)
	}
	if len(state.Subagents) != 0 {
		t.Errorf("Subagents should be empty, got %v", state.Subagents)
	}
	if len(state.Tokens) != 0 {
		t.Errorf("Tokens should be empty, got %v", state.Tokens)
	}
	if state.CompactionCount != 0 {
		t.Errorf("CompactionCount should be 0, got %d", state.CompactionCount)
	}
	if state.ContextPct != nil {
		t.Errorf("ContextPct should be nil, got %f", *state.ContextPct)
	}
}
