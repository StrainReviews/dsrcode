// Package session defines the shared data types for Claude Code session tracking.
// These types are used by the registry, resolver, server, and discord packages.
package session

import "time"

// SessionStatus represents the current activity state of a session.
type SessionStatus string

const (
	// StatusActive indicates the session has recent tool activity.
	StatusActive SessionStatus = "active"
	// StatusIdle indicates the session has not had activity within the idle timeout.
	StatusIdle SessionStatus = "idle"
)

// ActivityCounts tracks the number of tool invocations by category.
type ActivityCounts struct {
	Edits    int `json:"edits"`
	Commands int `json:"commands"`
	Searches int `json:"searches"`
	Reads    int `json:"reads"`
	Thinks   int `json:"thinks"`
}

// EmptyActivityCounts returns an ActivityCounts with all counters at zero.
func EmptyActivityCounts() ActivityCounts {
	return ActivityCounts{}
}

// Session holds the full state of a single Claude Code session.
type Session struct {
	SessionID      string         `json:"sessionId"`
	ProjectPath    string         `json:"projectPath"`
	ProjectName    string         `json:"projectName"`
	PID            int            `json:"pid"`
	Source         SessionSource  `json:"source"`
	Model          string         `json:"model"`
	Branch         string         `json:"branch"`
	Details        string         `json:"details"`
	SmallImageKey  string         `json:"smallImageKey"`
	SmallText      string         `json:"smallText"`
	TotalTokens    int64          `json:"totalTokens"`
	TotalCostUSD   float64        `json:"totalCostUsd"`
	ActivityCounts ActivityCounts `json:"activityCounts"`
	Status         SessionStatus  `json:"status"`
	StartedAt      time.Time      `json:"startedAt"`
	LastActivityAt time.Time      `json:"lastActivityAt"`
	LastFile       string         `json:"lastFile"`
	LastFilePath   string         `json:"lastFilePath"`
	LastCommand    string         `json:"lastCommand"`
	LastQuery      string         `json:"lastQuery"`
}

// ActivityRequest is the JSON body sent by the Claude Code hook to report
// tool usage. The server parses this and feeds it to the registry.
type ActivityRequest struct {
	SessionID     string `json:"session_id"`
	Cwd           string `json:"cwd"`
	ToolName      string `json:"tool_name,omitempty"`
	SmallImageKey string `json:"smallImageKey,omitempty"`
	SmallText     string `json:"smallText,omitempty"`
	Details       string `json:"details,omitempty"`
	LastFile      string `json:"lastFile,omitempty"`
	LastFilePath  string `json:"lastFilePath,omitempty"`
	LastCommand   string `json:"lastCommand,omitempty"`
	LastQuery     string `json:"lastQuery,omitempty"`
}

// CounterMap maps a SmallImageKey value to the corresponding ActivityCounts
// field name. Used by the registry to increment the correct counter.
var CounterMap = map[string]string{
	"coding":    "edits",
	"terminal":  "commands",
	"searching": "searches",
	"reading":   "reads",
	"thinking":  "thinks",
}
