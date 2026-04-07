package analytics_test

import (
	"testing"
	"time"

	"github.com/tsanva/cc-discord-presence/analytics"
)

// TestSpawnSubagent verifies that spawning adds a SubagentEntry with status "working".
func TestSpawnSubagent(t *testing.T) {
	tree := analytics.NewSubagentTree()
	tree.Spawn(analytics.SubagentEntry{
		ID:          "agent-001",
		Type:        "Explore",
		Description: "scan project structure for architecture patterns",
		SpawnTime:   time.Now(),
	})

	agents := tree.All()
	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}
	if agents[0].Status != "working" {
		t.Errorf("status = %q, want %q", agents[0].Status, "working")
	}
}

// TestCompleteByAgentID verifies strategy 1: match by agent_id (D-08).
func TestCompleteByAgentID(t *testing.T) {
	tests := []struct {
		name        string
		subagents   []analytics.SubagentEntry
		event       analytics.SubagentStopEvent
		wantMatched string
	}{
		{
			name: "exact agent_id match",
			subagents: []analytics.SubagentEntry{
				{ID: "agent-001", Type: "Explore", Description: "scan project", Status: "working", SpawnTime: time.Now()},
				{ID: "agent-002", Type: "reviewer", Description: "review code", Status: "working", SpawnTime: time.Now()},
			},
			event: analytics.SubagentStopEvent{
				AgentID: "agent-002",
			},
			wantMatched: "agent-002",
		},
		{
			name: "agent_id not found - falls through",
			subagents: []analytics.SubagentEntry{
				{ID: "agent-001", Type: "Explore", Description: "scan project", Status: "working", SpawnTime: time.Now()},
			},
			event: analytics.SubagentStopEvent{
				AgentID: "agent-999",
			},
			wantMatched: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := analytics.NewSubagentTree()
			for _, sa := range tt.subagents {
				tree.Spawn(sa)
			}
			matched := tree.CompleteByAgentID(tt.event.AgentID)
			if matched != tt.wantMatched {
				t.Errorf("CompleteByAgentID() = %q, want %q", matched, tt.wantMatched)
			}
		})
	}
}

// TestCompleteByDescriptionPrefix verifies strategy 2: 57-char prefix match.
func TestCompleteByDescriptionPrefix(t *testing.T) {
	tests := []struct {
		name        string
		subagents   []analytics.SubagentEntry
		description string
		wantMatched string
	}{
		{
			name: "description prefix match (57 chars)",
			subagents: []analytics.SubagentEntry{
				{
					ID:          "agent-001",
					Type:        "Explore",
					Description: "scan project structure for architecture patterns and conventions used",
					Status:      "working",
					SpawnTime:   time.Now(),
				},
			},
			description: "scan project structure for architecture patterns and conventions used throughout the codebase",
			wantMatched: "agent-001",
		},
		{
			name: "no prefix match",
			subagents: []analytics.SubagentEntry{
				{
					ID:          "agent-001",
					Type:        "Explore",
					Description: "totally different description about something else entirely",
					Status:      "working",
					SpawnTime:   time.Now(),
				},
			},
			description: "scan project structure for architecture patterns",
			wantMatched: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := analytics.NewSubagentTree()
			for _, sa := range tt.subagents {
				tree.Spawn(sa)
			}
			matched := tree.CompleteByDescriptionPrefix(tt.description)
			if matched != tt.wantMatched {
				t.Errorf("CompleteByDescriptionPrefix() = %q, want %q", matched, tt.wantMatched)
			}
		})
	}
}

// TestCompleteByAgentType verifies strategy 3: agent_type match.
func TestCompleteByAgentType(t *testing.T) {
	tests := []struct {
		name        string
		subagents   []analytics.SubagentEntry
		agentType   string
		wantMatched string
	}{
		{
			name: "agent type match",
			subagents: []analytics.SubagentEntry{
				{ID: "agent-001", Type: "Explore", Description: "scan project", Status: "working", SpawnTime: time.Now()},
				{ID: "agent-002", Type: "reviewer", Description: "review code", Status: "working", SpawnTime: time.Now()},
			},
			agentType:   "reviewer",
			wantMatched: "agent-002",
		},
		{
			name: "no type match",
			subagents: []analytics.SubagentEntry{
				{ID: "agent-001", Type: "Explore", Description: "scan project", Status: "working", SpawnTime: time.Now()},
			},
			agentType:   "reviewer",
			wantMatched: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := analytics.NewSubagentTree()
			for _, sa := range tt.subagents {
				tree.Spawn(sa)
			}
			matched := tree.CompleteByAgentType(tt.agentType)
			if matched != tt.wantMatched {
				t.Errorf("CompleteByAgentType() = %q, want %q", matched, tt.wantMatched)
			}
		})
	}
}

// TestCompleteOldestWorking verifies strategy 4: fallback to oldest working subagent.
func TestCompleteOldestWorking(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		subagents   []analytics.SubagentEntry
		wantMatched string
	}{
		{
			name: "oldest working selected",
			subagents: []analytics.SubagentEntry{
				{ID: "agent-002", Type: "reviewer", Description: "review code", Status: "working", SpawnTime: now.Add(1 * time.Second)},
				{ID: "agent-001", Type: "Explore", Description: "scan project", Status: "working", SpawnTime: now},
			},
			wantMatched: "agent-001",
		},
		{
			name: "skips completed agents",
			subagents: []analytics.SubagentEntry{
				{ID: "agent-001", Type: "Explore", Description: "scan project", Status: "completed", SpawnTime: now},
				{ID: "agent-002", Type: "reviewer", Description: "review code", Status: "working", SpawnTime: now.Add(1 * time.Second)},
			},
			wantMatched: "agent-002",
		},
		{
			name:        "no working agents",
			subagents:   []analytics.SubagentEntry{},
			wantMatched: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := analytics.NewSubagentTree()
			for _, sa := range tt.subagents {
				tree.Spawn(sa)
			}
			matched := tree.CompleteOldestWorking()
			if matched != tt.wantMatched {
				t.Errorf("CompleteOldestWorking() = %q, want %q", matched, tt.wantMatched)
			}
		})
	}
}

// TestSubagentTree verifies parent-child hierarchy via ParentID.
func TestSubagentTree(t *testing.T) {
	tree := analytics.NewSubagentTree()
	tree.Spawn(analytics.SubagentEntry{
		ID:        "agent-001",
		Type:      "Explore",
		SpawnTime: time.Now(),
	})
	tree.Spawn(analytics.SubagentEntry{
		ID:        "agent-002",
		Type:      "reviewer",
		ParentID:  "agent-001",
		SpawnTime: time.Now(),
	})

	children := tree.ChildrenOf("agent-001")
	if len(children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(children))
	}
	if children[0].ID != "agent-002" {
		t.Errorf("child ID = %q, want %q", children[0].ID, "agent-002")
	}
}

// TestFormatSubagentsMinimal verifies minimal display: "3 agents".
func TestFormatSubagentsMinimal(t *testing.T) {
	tests := []struct {
		name          string
		count         int
		displayDetail string
		want          string
	}{
		{name: "three agents", count: 3, displayDetail: "minimal", want: "3 agents"},
		{name: "one agent", count: 1, displayDetail: "minimal", want: "1 agent"},
		{name: "zero agents", count: 0, displayDetail: "minimal", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree := analytics.NewSubagentTree()
			for i := 0; i < tt.count; i++ {
				tree.Spawn(analytics.SubagentEntry{
					ID:        "agent-" + string(rune('0'+i)),
					Type:      "Explore",
					SpawnTime: time.Now(),
				})
			}
			got := analytics.FormatSubagents(tree, tt.displayDetail)
			if got != tt.want {
				t.Errorf("FormatSubagents(%q) = %q, want %q", tt.displayDetail, got, tt.want)
			}
		})
	}
}

// TestFormatSubagentsVerbose verifies verbose display: "3: 2xExplore 1xreviewer".
func TestFormatSubagentsVerbose(t *testing.T) {
	tree := analytics.NewSubagentTree()
	tree.Spawn(analytics.SubagentEntry{ID: "a1", Type: "Explore", SpawnTime: time.Now()})
	tree.Spawn(analytics.SubagentEntry{ID: "a2", Type: "Explore", SpawnTime: time.Now()})
	tree.Spawn(analytics.SubagentEntry{ID: "a3", Type: "reviewer", SpawnTime: time.Now()})

	got := analytics.FormatSubagents(tree, "verbose")
	// Should contain count and type breakdown
	if got == "" {
		t.Error("FormatSubagents(verbose) returned empty string for 3 agents")
	}
	// Verify it starts with the count
	if got[:1] != "3" {
		t.Errorf("FormatSubagents(verbose) should start with count, got %q", got)
	}
}
