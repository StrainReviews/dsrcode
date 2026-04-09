package analytics_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/StrainReviews/dsrcode/analytics"
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

// TestTrackerRecordToolPersists verifies that RecordTool persists tool counts
// to disk via write-on-event (D-31). Reads persisted file back to verify.
func TestTrackerRecordToolPersists(t *testing.T) {
	dir := t.TempDir()
	tracker := analytics.NewTracker(analytics.TrackerConfig{
		Features: analytics.Features{Analytics: true},
		DataDir:  dir,
	})

	tracker.RecordTool("sess-1", "Edit")
	tracker.RecordTool("sess-1", "Bash")

	// Verify persisted file exists and contains correct data
	data, err := analytics.LoadToolAnalytics(dir, "sess-1")
	if err != nil {
		t.Fatalf("LoadToolAnalytics error: %v", err)
	}
	if data.Tools["Edit"] != 1 {
		t.Errorf("persisted Edit = %d, want 1", data.Tools["Edit"])
	}
	if data.Tools["Bash"] != 1 {
		t.Errorf("persisted Bash = %d, want 1", data.Tools["Bash"])
	}
}

// TestTrackerUpdateTokensPersists verifies that UpdateTokens persists token
// data to disk via write-on-event (D-31).
func TestTrackerUpdateTokensPersists(t *testing.T) {
	dir := t.TempDir()
	tracker := analytics.NewTracker(analytics.TrackerConfig{
		Features: analytics.Features{Analytics: true},
		DataDir:  dir,
	})

	tracker.UpdateTokens("sess-1", "claude-opus-4-6", analytics.TokenBreakdown{
		Input: 100_000, Output: 40_000,
	})

	data, err := analytics.LoadAnalytics(dir, "sess-1")
	if err != nil {
		t.Fatalf("LoadAnalytics error: %v", err)
	}
	if data.Tokens == nil {
		t.Fatal("persisted Tokens is nil")
	}
	if data.Tokens["claude-opus-4-6"].Input != 100_000 {
		t.Errorf("persisted Input = %d, want 100000", data.Tokens["claude-opus-4-6"].Input)
	}
}

// TestTrackerRecordSubagentSpawnPersists verifies that RecordSubagentSpawn
// persists subagent data to disk via write-on-event (D-31).
func TestTrackerRecordSubagentSpawnPersists(t *testing.T) {
	dir := t.TempDir()
	tracker := analytics.NewTracker(analytics.TrackerConfig{
		Features: analytics.Features{Analytics: true},
		DataDir:  dir,
	})

	tracker.RecordSubagentSpawn("sess-1", analytics.SubagentEntry{
		ID:          "agent-001",
		Type:        "Explore",
		Description: "scan project",
		SpawnTime:   time.Now(),
	})

	data, err := analytics.LoadSubagentAnalytics(dir, "sess-1")
	if err != nil {
		t.Fatalf("LoadSubagentAnalytics error: %v", err)
	}
	if len(data.Agents) != 1 {
		t.Fatalf("persisted agents = %d, want 1", len(data.Agents))
	}
	if data.Agents[0].Type != "Explore" {
		t.Errorf("persisted agent type = %q, want %q", data.Agents[0].Type, "Explore")
	}
}

// TestTrackerLoadSession verifies that LoadSession restores all analytics
// from persisted files on startup (D-02).
func TestTrackerLoadSession(t *testing.T) {
	dir := t.TempDir()

	// Pre-populate persisted files using Save functions
	if err := analytics.SaveToolAnalytics(dir, "sess-restore", analytics.ToolAnalytics{
		Tools: map[string]int{"Edit": 5, "Bash": 3},
	}); err != nil {
		t.Fatal(err)
	}
	if err := analytics.SaveAnalytics(dir, "sess-restore", analytics.SessionAnalytics{
		Tokens: map[string]analytics.TokenBreakdown{
			"claude-opus-4-6": {Input: 200_000, Output: 80_000},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if err := analytics.SaveSubagentAnalytics(dir, "sess-restore", analytics.SubagentAnalytics{
		Agents: []analytics.SubagentEntry{
			{ID: "a1", Type: "Explore", Status: "completed"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	// Create tracker and load session
	tracker := analytics.NewTracker(analytics.TrackerConfig{
		Features: analytics.Features{Analytics: true},
		DataDir:  dir,
	})

	if err := tracker.LoadSession("sess-restore"); err != nil {
		t.Fatalf("LoadSession error: %v", err)
	}

	state := tracker.GetState("sess-restore")
	if state.ToolCounts["Edit"] != 5 {
		t.Errorf("restored Edit = %d, want 5", state.ToolCounts["Edit"])
	}
	if state.ToolCounts["Bash"] != 3 {
		t.Errorf("restored Bash = %d, want 3", state.ToolCounts["Bash"])
	}
	if state.Tokens["claude-opus-4-6"].Input != 200_000 {
		t.Errorf("restored Input = %d, want 200000", state.Tokens["claude-opus-4-6"].Input)
	}
	if len(state.Subagents) != 1 {
		t.Fatalf("restored subagents = %d, want 1", len(state.Subagents))
	}
	if state.Subagents[0].Type != "Explore" {
		t.Errorf("restored subagent type = %q, want %q", state.Subagents[0].Type, "Explore")
	}
}

// TestTrackerLoadSessionNoFiles verifies that LoadSession for a session with
// no persisted files returns no error and creates an empty session (D-21 lazy init).
func TestTrackerLoadSessionNoFiles(t *testing.T) {
	dir := t.TempDir()
	tracker := analytics.NewTracker(analytics.TrackerConfig{
		Features: analytics.Features{Analytics: true},
		DataDir:  dir,
	})

	if err := tracker.LoadSession("nonexistent"); err != nil {
		t.Fatalf("LoadSession error for missing files: %v", err)
	}

	state := tracker.GetState("nonexistent")
	if len(state.ToolCounts) != 0 {
		t.Errorf("ToolCounts should be empty, got %v", state.ToolCounts)
	}
	if len(state.Tokens) != 0 {
		t.Errorf("Tokens should be empty, got %v", state.Tokens)
	}
}

// TestTrackerNoPersistWhenDisabled verifies that when analytics feature is
// disabled, no files are written to disk (D-34).
func TestTrackerNoPersistWhenDisabled(t *testing.T) {
	dir := t.TempDir()
	tracker := analytics.NewTracker(analytics.TrackerConfig{
		Features: analytics.Features{Analytics: false},
		DataDir:  dir,
	})

	tracker.RecordTool("sess-1", "Edit")
	tracker.UpdateTokens("sess-1", "claude-opus-4-6", analytics.TokenBreakdown{Input: 1000})
	tracker.RecordSubagentSpawn("sess-1", analytics.SubagentEntry{ID: "a1"})

	// Check no files were created
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		t.Errorf("unexpected file in data dir: %s", e.Name())
	}
}

// TestTrackerPersistNoDataDir verifies that the tracker gracefully handles
// an empty DataDir (no persistence, in-memory only).
func TestTrackerPersistNoDataDir(t *testing.T) {
	tracker := analytics.NewTracker(analytics.TrackerConfig{
		Features: analytics.Features{Analytics: true},
		DataDir:  "", // no data dir = no persistence
	})

	// Should not panic
	tracker.RecordTool("sess-1", "Edit")
	tracker.UpdateTokens("sess-1", "claude-opus-4-6", analytics.TokenBreakdown{Input: 1000})

	state := tracker.GetState("sess-1")
	if state.ToolCounts["Edit"] != 1 {
		t.Errorf("Edit count = %d, want 1", state.ToolCounts["Edit"])
	}
}

// TestTrackerMultipleSessionsPersist verifies that persistence is per-session
// and does not mix data between sessions.
func TestTrackerMultipleSessionsPersist(t *testing.T) {
	dir := t.TempDir()
	tracker := analytics.NewTracker(analytics.TrackerConfig{
		Features: analytics.Features{Analytics: true},
		DataDir:  dir,
	})

	tracker.RecordTool("sess-a", "Edit")
	tracker.RecordTool("sess-b", "Bash")

	dataA, _ := analytics.LoadToolAnalytics(dir, "sess-a")
	dataB, _ := analytics.LoadToolAnalytics(dir, "sess-b")

	if dataA.Tools["Edit"] != 1 {
		t.Errorf("sess-a Edit = %d, want 1", dataA.Tools["Edit"])
	}
	if _, exists := dataA.Tools["Bash"]; exists {
		t.Error("sess-a should not have Bash")
	}
	if dataB.Tools["Bash"] != 1 {
		t.Errorf("sess-b Bash = %d, want 1", dataB.Tools["Bash"])
	}

	// Verify files are separate
	_, errA := os.Stat(filepath.Join(dir, "tools-sess-a.json"))
	_, errB := os.Stat(filepath.Join(dir, "tools-sess-b.json"))
	if errA != nil {
		t.Errorf("tools-sess-a.json missing: %v", errA)
	}
	if errB != nil {
		t.Errorf("tools-sess-b.json missing: %v", errB)
	}
}
