package analytics

import (
	"fmt"
	"sort"
	"strings"
)

// toolAbbreviations maps Claude Code tool names to short display abbreviations
// per D-05.
var toolAbbreviations = map[string]string{
	"Edit":      "ed",
	"Write":     "write",
	"Bash":      "cmd",
	"Read":      "read",
	"Grep":      "grep",
	"Glob":      "glob",
	"WebSearch": "web",
	"WebFetch":  "fetch",
	"Task":      "think",
	"Agent":     "agent",
}

// AbbreviateTool returns the short form of a tool name. Known tools use the
// abbreviation map; unknown tools fall back to the full name in lowercase.
func AbbreviateTool(name string) string {
	if abbr, ok := toolAbbreviations[name]; ok {
		return abbr
	}
	return strings.ToLower(name)
}

// ToolEntry holds a tool name and its invocation count, used for sorting.
type ToolEntry struct {
	Name  string
	Count int
}

// ToolCounter tracks tool invocation counts using map[string]int.
type ToolCounter struct {
	counts map[string]int
}

// NewToolCounter creates a new ToolCounter with an empty counter map.
func NewToolCounter() *ToolCounter {
	return &ToolCounter{
		counts: make(map[string]int),
	}
}

// Record increments the counter for the given tool name.
func (tc *ToolCounter) Record(toolName string) {
	tc.counts[toolName]++
}

// Counts returns a copy of the internal counter map.
func (tc *ToolCounter) Counts() map[string]int {
	result := make(map[string]int, len(tc.counts))
	for k, v := range tc.counts {
		result[k] = v
	}
	return result
}

// TopN returns the top n tools sorted by count descending, then name ascending.
// Returns nil for an empty counter.
func (tc *ToolCounter) TopN(n int) []ToolEntry {
	if len(tc.counts) == 0 {
		return nil
	}
	var entries []ToolEntry
	for name, count := range tc.counts {
		entries = append(entries, ToolEntry{Name: name, Count: count})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Count != entries[j].Count {
			return entries[i].Count > entries[j].Count
		}
		return entries[i].Name < entries[j].Name
	})
	if n > len(entries) {
		n = len(entries)
	}
	return entries[:n]
}

// totalCount returns the sum of all tool invocations.
func (tc *ToolCounter) totalCount() int {
	total := 0
	for _, c := range tc.counts {
		total += c
	}
	return total
}

// FormatTools formats tool statistics for display based on displayDetail.
//
// Levels per D-03:
//
//	minimal:  total count ("42 tools")
//	standard: top 3 abbreviated with middot separator ("42ed · 12cmd")
//	verbose:  top 4 abbreviated ("42ed · 12cmd · 5grep · 3read")
//	private:  ""
//
// Empty counter returns "" per D-04.
func FormatTools(counter *ToolCounter, displayDetail string) string {
	if counter.totalCount() == 0 {
		return ""
	}

	switch displayDetail {
	case "private":
		return ""
	case "verbose":
		return formatToolList(counter, 4)
	case "standard":
		return formatToolList(counter, 3)
	default: // minimal
		total := counter.totalCount()
		return fmt.Sprintf("%d %s", total, pluralize(total, "tool", "tools"))
	}
}

// formatToolList returns an abbreviated tool list with middot separators.
func formatToolList(counter *ToolCounter, n int) string {
	entries := counter.TopN(n)
	if len(entries) == 0 {
		return ""
	}
	var parts []string
	for _, e := range entries {
		parts = append(parts, fmt.Sprintf("%d%s", e.Count, AbbreviateTool(e.Name)))
	}
	return strings.Join(parts, " \u00b7 ")
}
