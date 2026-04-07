package analytics

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// SessionAnalytics holds the token and compaction analytics for a session.
// Persisted as JSON per D-06 (separate files per feature).
type SessionAnalytics struct {
	Tokens          map[string]TokenBreakdown `json:"tokens,omitempty"`
	Baseline        TokenBreakdown            `json:"baseline,omitempty"`
	CompactionCount int                       `json:"compactionCount,omitempty"`
}

// ToolAnalytics holds the tool invocation counts for a session.
type ToolAnalytics struct {
	Tools map[string]int `json:"tools,omitempty"`
}

// SubagentAnalytics holds the subagent tree for a session.
type SubagentAnalytics struct {
	Agents []SubagentEntry `json:"agents,omitempty"`
}

// SaveAnalytics writes token/compaction analytics to a JSON file.
// Per D-31: write-on-event. Per D-21: creates directory on first write.
// Per T-17-01-01: file permissions 0644.
func SaveAnalytics(dir, sessionID string, data SessionAnalytics) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "analytics-"+sessionID+".json")
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, content, 0644)
}

// LoadAnalytics reads token/compaction analytics from a JSON file.
// Returns zero-value SessionAnalytics if file doesn't exist per D-21.
func LoadAnalytics(dir, sessionID string) (SessionAnalytics, error) {
	path := filepath.Join(dir, "analytics-"+sessionID+".json")
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return SessionAnalytics{}, nil
		}
		return SessionAnalytics{}, err
	}
	var data SessionAnalytics
	if err := json.Unmarshal(content, &data); err != nil {
		return SessionAnalytics{}, err
	}
	return data, nil
}

// SaveToolAnalytics writes tool invocation counts to a JSON file.
// Separate file per D-06.
func SaveToolAnalytics(dir, sessionID string, data ToolAnalytics) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "tools-"+sessionID+".json")
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, content, 0644)
}

// LoadToolAnalytics reads tool invocation counts from a JSON file.
// Returns zero-value ToolAnalytics if file doesn't exist per D-21.
func LoadToolAnalytics(dir, sessionID string) (ToolAnalytics, error) {
	path := filepath.Join(dir, "tools-"+sessionID+".json")
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ToolAnalytics{}, nil
		}
		return ToolAnalytics{}, err
	}
	var data ToolAnalytics
	if err := json.Unmarshal(content, &data); err != nil {
		return ToolAnalytics{}, err
	}
	return data, nil
}

// SaveSubagentAnalytics writes the subagent tree to a JSON file.
// Separate file per D-06.
func SaveSubagentAnalytics(dir, sessionID string, data SubagentAnalytics) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "subagents-"+sessionID+".json")
	content, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, content, 0644)
}

// LoadSubagentAnalytics reads the subagent tree from a JSON file.
// Returns zero-value SubagentAnalytics if file doesn't exist per D-21.
func LoadSubagentAnalytics(dir, sessionID string) (SubagentAnalytics, error) {
	path := filepath.Join(dir, "subagents-"+sessionID+".json")
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return SubagentAnalytics{}, nil
		}
		return SubagentAnalytics{}, err
	}
	var data SubagentAnalytics
	if err := json.Unmarshal(content, &data); err != nil {
		return SubagentAnalytics{}, err
	}
	return data, nil
}
