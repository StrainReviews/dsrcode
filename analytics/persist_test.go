package analytics_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tsanva/cc-discord-presence/analytics"
)

// TestSaveAndLoad verifies round-trip write/read of SessionAnalytics JSON (D-31).
func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()

	data := analytics.SessionAnalytics{
		Tokens: map[string]analytics.TokenBreakdown{
			"claude-opus-4-6": {
				Input: 250_000, Output: 80_000, CacheRead: 156_000, CacheWrite: 14_000,
			},
		},
		Baseline: analytics.TokenBreakdown{
			Input: 100_000, Output: 40_000,
		},
		CompactionCount: 2,
	}

	err := analytics.SaveAnalytics(dir, "test-session-1", data)
	if err != nil {
		t.Fatalf("SaveAnalytics() error: %v", err)
	}

	loaded, err := analytics.LoadAnalytics(dir, "test-session-1")
	if err != nil {
		t.Fatalf("LoadAnalytics() error: %v", err)
	}

	// Verify tokens round-trip
	opus := loaded.Tokens["claude-opus-4-6"]
	if opus.Input != 250_000 {
		t.Errorf("loaded Input = %d, want 250000", opus.Input)
	}
	if opus.Output != 80_000 {
		t.Errorf("loaded Output = %d, want 80000", opus.Output)
	}
	if loaded.Baseline.Input != 100_000 {
		t.Errorf("loaded Baseline.Input = %d, want 100000", loaded.Baseline.Input)
	}
	if loaded.CompactionCount != 2 {
		t.Errorf("loaded CompactionCount = %d, want 2", loaded.CompactionCount)
	}
}

// TestLazyInit verifies that no file is created until the first event (D-21).
func TestLazyInit(t *testing.T) {
	dir := t.TempDir()

	// Before any save, no analytics files should exist
	files, err := filepath.Glob(filepath.Join(dir, "analytics-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Errorf("expected no analytics files before first event, got %d", len(files))
	}

	// After first save, file should exist
	data := analytics.SessionAnalytics{
		Tokens: map[string]analytics.TokenBreakdown{
			"claude-sonnet-4-6": {Input: 1000, Output: 500},
		},
	}
	err = analytics.SaveAnalytics(dir, "lazy-session", data)
	if err != nil {
		t.Fatalf("SaveAnalytics() error: %v", err)
	}

	// Now check file exists
	_, err = analytics.LoadAnalytics(dir, "lazy-session")
	if err != nil {
		t.Errorf("LoadAnalytics() after save returned error: %v", err)
	}
}

// TestSeparateFiles verifies that tokens, tools, subagents each get their own file (D-06).
func TestSeparateFiles(t *testing.T) {
	dir := t.TempDir()

	tokenData := analytics.SessionAnalytics{
		Tokens: map[string]analytics.TokenBreakdown{
			"claude-opus-4-6": {Input: 100_000},
		},
	}
	toolData := analytics.ToolAnalytics{
		Tools: map[string]int{"Edit": 42, "Bash": 12},
	}
	subagentData := analytics.SubagentAnalytics{
		Agents: []analytics.SubagentEntry{
			{ID: "agent-001", Type: "Explore", Status: "completed"},
		},
	}

	tests := []struct {
		name     string
		saveFunc func() error
		wantFile string
	}{
		{
			name: "token analytics file",
			saveFunc: func() error {
				return analytics.SaveAnalytics(dir, "session-1", tokenData)
			},
			wantFile: "analytics-session-1.json",
		},
		{
			name: "tool analytics file",
			saveFunc: func() error {
				return analytics.SaveToolAnalytics(dir, "session-1", toolData)
			},
			wantFile: "tools-session-1.json",
		},
		{
			name: "subagent analytics file",
			saveFunc: func() error {
				return analytics.SaveSubagentAnalytics(dir, "session-1", subagentData)
			},
			wantFile: "subagents-session-1.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.saveFunc(); err != nil {
				t.Fatalf("save error: %v", err)
			}
			path := filepath.Join(dir, tt.wantFile)
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Errorf("expected file %s to exist", tt.wantFile)
			}
		})
	}
}

// TestConcurrentWrites verifies that two sessions can write simultaneously
// without corruption (separate files per session).
func TestConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	done := make(chan error, 2)

	go func() {
		data := analytics.SessionAnalytics{
			Tokens: map[string]analytics.TokenBreakdown{
				"claude-opus-4-6": {Input: 100_000},
			},
		}
		done <- analytics.SaveAnalytics(dir, "session-A", data)
	}()

	go func() {
		data := analytics.SessionAnalytics{
			Tokens: map[string]analytics.TokenBreakdown{
				"claude-sonnet-4-6": {Input: 200_000},
			},
		}
		done <- analytics.SaveAnalytics(dir, "session-B", data)
	}()

	for i := 0; i < 2; i++ {
		if err := <-done; err != nil {
			t.Errorf("concurrent write %d failed: %v", i, err)
		}
	}

	// Verify both sessions are independently readable
	a, err := analytics.LoadAnalytics(dir, "session-A")
	if err != nil {
		t.Fatalf("load session-A error: %v", err)
	}
	if a.Tokens["claude-opus-4-6"].Input != 100_000 {
		t.Error("session-A data corrupted")
	}

	b, err := analytics.LoadAnalytics(dir, "session-B")
	if err != nil {
		t.Fatalf("load session-B error: %v", err)
	}
	if b.Tokens["claude-sonnet-4-6"].Input != 200_000 {
		t.Error("session-B data corrupted")
	}
}

// TestLoadNonExistent verifies os.IsNotExist returns zero-value SessionAnalytics.
func TestLoadNonExistent(t *testing.T) {
	dir := t.TempDir()

	data, err := analytics.LoadAnalytics(dir, "nonexistent-session")
	if err != nil {
		t.Fatalf("LoadAnalytics(nonexistent) should not error, got: %v", err)
	}
	if data.Tokens != nil && len(data.Tokens) > 0 {
		t.Errorf("expected empty Tokens map, got %+v", data.Tokens)
	}
	if data.CompactionCount != 0 {
		t.Errorf("expected CompactionCount=0, got %d", data.CompactionCount)
	}
}
