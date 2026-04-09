package analytics_test

import (
	"testing"

	"github.com/StrainReviews/dsrcode/analytics"
)

// TestRecordTool verifies that recording a tool increments the map[string]int counter.
func TestRecordTool(t *testing.T) {
	tests := []struct {
		name  string
		tools []string
		want  map[string]int
	}{
		{
			name:  "single tool",
			tools: []string{"Edit"},
			want:  map[string]int{"Edit": 1},
		},
		{
			name:  "multiple same tool",
			tools: []string{"Edit", "Edit", "Edit"},
			want:  map[string]int{"Edit": 3},
		},
		{
			name:  "mixed tools",
			tools: []string{"Edit", "Bash", "Edit", "Grep", "Edit", "Bash"},
			want:  map[string]int{"Edit": 3, "Bash": 2, "Grep": 1},
		},
		{
			name:  "empty",
			tools: []string{},
			want:  map[string]int{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counter := analytics.NewToolCounter()
			for _, tool := range tt.tools {
				counter.Record(tool)
			}
			got := counter.Counts()
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("tool %q: got %d, want %d", k, got[k], v)
				}
			}
			if len(got) != len(tt.want) {
				t.Errorf("tool count: got %d entries, want %d", len(got), len(tt.want))
			}
		})
	}
}

// TestTopNTools verifies that top 3 tools are returned sorted by count.
func TestTopNTools(t *testing.T) {
	tests := []struct {
		name  string
		tools map[string]int
		n     int
		want  []analytics.ToolEntry
	}{
		{
			name: "top 3 from 5",
			tools: map[string]int{
				"Edit": 42, "Bash": 12, "Grep": 5, "Read": 3, "Write": 1,
			},
			n: 3,
			want: []analytics.ToolEntry{
				{Name: "Edit", Count: 42},
				{Name: "Bash", Count: 12},
				{Name: "Grep", Count: 5},
			},
		},
		{
			name: "fewer tools than N",
			tools: map[string]int{
				"Edit": 10, "Bash": 5,
			},
			n: 3,
			want: []analytics.ToolEntry{
				{Name: "Edit", Count: 10},
				{Name: "Bash", Count: 5},
			},
		},
		{
			name:  "empty tools",
			tools: map[string]int{},
			n:     3,
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counter := analytics.NewToolCounter()
			for name, count := range tt.tools {
				for i := 0; i < count; i++ {
					counter.Record(name)
				}
			}
			got := counter.TopN(tt.n)
			if len(got) != len(tt.want) {
				t.Fatalf("TopN(%d) returned %d entries, want %d", tt.n, len(got), len(tt.want))
			}
			for i, entry := range got {
				if entry.Name != tt.want[i].Name || entry.Count != tt.want[i].Count {
					t.Errorf("TopN[%d] = {%q, %d}, want {%q, %d}",
						i, entry.Name, entry.Count, tt.want[i].Name, tt.want[i].Count)
				}
			}
		})
	}
}

// TestFormatToolsMinimal verifies minimal display: "42 tools".
func TestFormatToolsMinimal(t *testing.T) {
	tests := []struct {
		name          string
		tools         map[string]int
		displayDetail string
		want          string
	}{
		{
			name: "total count",
			tools: map[string]int{
				"Edit": 42, "Bash": 12, "Grep": 5, "Read": 3,
			},
			displayDetail: "minimal",
			want:          "62 tools",
		},
		{
			name: "single tool",
			tools: map[string]int{
				"Edit": 1,
			},
			displayDetail: "minimal",
			want:          "1 tool",
		},
		{
			name:          "no tools",
			tools:         map[string]int{},
			displayDetail: "minimal",
			want:          "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counter := analytics.NewToolCounter()
			for name, count := range tt.tools {
				for i := 0; i < count; i++ {
					counter.Record(name)
				}
			}
			got := analytics.FormatTools(counter, tt.displayDetail)
			if got != tt.want {
				t.Errorf("FormatTools(%q) = %q, want %q", tt.displayDetail, got, tt.want)
			}
		})
	}
}

// TestFormatToolsStandard verifies standard display: "42ed \u00b7 12cmd"
// (using middle dot separator per D-03).
func TestFormatToolsStandard(t *testing.T) {
	tests := []struct {
		name          string
		tools         map[string]int
		displayDetail string
		want          string
	}{
		{
			name: "top tools with abbreviations",
			tools: map[string]int{
				"Edit": 42, "Bash": 12,
			},
			displayDetail: "standard",
			want:          "42ed \u00b7 12cmd",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counter := analytics.NewToolCounter()
			for name, count := range tt.tools {
				for i := 0; i < count; i++ {
					counter.Record(name)
				}
			}
			got := analytics.FormatTools(counter, tt.displayDetail)
			if got != tt.want {
				t.Errorf("FormatTools(%q) = %q, want %q", tt.displayDetail, got, tt.want)
			}
		})
	}
}

// TestFormatToolsVerbose verifies verbose display: "42ed \u00b7 12cmd \u00b7 5grep \u00b7 3read".
func TestFormatToolsVerbose(t *testing.T) {
	tests := []struct {
		name          string
		tools         map[string]int
		displayDetail string
		want          string
	}{
		{
			name: "extended tool list",
			tools: map[string]int{
				"Edit": 42, "Bash": 12, "Grep": 5, "Read": 3,
			},
			displayDetail: "verbose",
			want:          "42ed \u00b7 12cmd \u00b7 5grep \u00b7 3read",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			counter := analytics.NewToolCounter()
			for name, count := range tt.tools {
				for i := 0; i < count; i++ {
					counter.Record(name)
				}
			}
			got := analytics.FormatTools(counter, tt.displayDetail)
			if got != tt.want {
				t.Errorf("FormatTools(%q) = %q, want %q", tt.displayDetail, got, tt.want)
			}
		})
	}
}

// TestToolAbbreviations verifies the abbreviation mapping per D-05:
// Edit->ed, Bash->cmd, Grep->grep, Read->read, Write->write, Agent->agent.
func TestToolAbbreviations(t *testing.T) {
	tests := []struct {
		name string
		tool string
		want string
	}{
		{name: "Edit", tool: "Edit", want: "ed"},
		{name: "Bash", tool: "Bash", want: "cmd"},
		{name: "Grep", tool: "Grep", want: "grep"},
		{name: "Read", tool: "Read", want: "read"},
		{name: "Write", tool: "Write", want: "write"},
		{name: "Agent", tool: "Agent", want: "agent"},
		{name: "Glob", tool: "Glob", want: "glob"},
		{name: "Unknown", tool: "CustomTool", want: "customtool"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := analytics.AbbreviateTool(tt.tool)
			if got != tt.want {
				t.Errorf("AbbreviateTool(%q) = %q, want %q", tt.tool, got, tt.want)
			}
		})
	}
}
