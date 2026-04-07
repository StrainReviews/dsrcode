package analytics

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// SubagentEntry holds the state of a single subagent in the hierarchy tree.
// Per D-07: ParentID enables tree structure, Status tracks working/completed.
type SubagentEntry struct {
	ID            string     `json:"id"`
	Type          string     `json:"type"`
	Description   string     `json:"description,omitempty"`
	Status        string     `json:"status"`
	SpawnTime     time.Time  `json:"spawnTime"`
	CompletedTime *time.Time `json:"completedTime,omitempty"`
	ParentID      string     `json:"parentId,omitempty"`
	Prompt        string     `json:"prompt,omitempty"`
}

// SubagentStopEvent holds fields from a SubagentStop hook payload.
type SubagentStopEvent struct {
	AgentID     string `json:"agent_id"`
	Description string `json:"description"`
	AgentType   string `json:"agent_type"`
	Prompt      string `json:"prompt"`
}

// SubagentTree manages a collection of SubagentEntry items with hierarchy
// support and 4-strategy matching for completion per D-08.
type SubagentTree struct {
	agents []SubagentEntry
}

// NewSubagentTree creates an empty SubagentTree.
func NewSubagentTree() *SubagentTree {
	return &SubagentTree{}
}

// Spawn adds a new subagent to the tree. If the entry has no Status set,
// it defaults to "working". Prompt is truncated to 500 characters.
func (st *SubagentTree) Spawn(entry SubagentEntry) {
	if entry.Status == "" {
		entry.Status = "working"
	}
	if len(entry.Prompt) > 500 {
		entry.Prompt = entry.Prompt[:500]
	}
	st.agents = append(st.agents, entry)
}

// All returns a copy of all subagent entries.
func (st *SubagentTree) All() []SubagentEntry {
	result := make([]SubagentEntry, len(st.agents))
	copy(result, st.agents)
	return result
}

// CompleteByAgentID implements strategy 1: exact match on agent_id.
// Returns the matched agent's ID, or "" if not found.
func (st *SubagentTree) CompleteByAgentID(agentID string) string {
	if agentID == "" {
		return ""
	}
	for i := range st.agents {
		if st.agents[i].Status == "working" && st.agents[i].ID == agentID {
			now := time.Now()
			st.agents[i].Status = "completed"
			st.agents[i].CompletedTime = &now
			return st.agents[i].ID
		}
	}
	return ""
}

// CompleteByDescriptionPrefix implements strategy 2: 57-char prefix match
// on description. Returns the matched agent's ID, or "" if not found.
func (st *SubagentTree) CompleteByDescriptionPrefix(description string) string {
	if description == "" {
		return ""
	}
	eventPrefix := truncateStr(description, 57)
	for i := range st.agents {
		if st.agents[i].Status != "working" {
			continue
		}
		agentPrefix := truncateStr(st.agents[i].Description, 57)
		if agentPrefix != "" && agentPrefix == eventPrefix {
			now := time.Now()
			st.agents[i].Status = "completed"
			st.agents[i].CompletedTime = &now
			return st.agents[i].ID
		}
	}
	return ""
}

// CompleteByAgentType implements strategy 3: match first working agent
// with the given type. Returns the matched agent's ID, or "" if not found.
func (st *SubagentTree) CompleteByAgentType(agentType string) string {
	if agentType == "" {
		return ""
	}
	for i := range st.agents {
		if st.agents[i].Status == "working" && st.agents[i].Type == agentType {
			now := time.Now()
			st.agents[i].Status = "completed"
			st.agents[i].CompletedTime = &now
			return st.agents[i].ID
		}
	}
	return ""
}

// CompleteOldestWorking implements strategy 4: fallback to the oldest working
// subagent by SpawnTime. Returns the matched agent's ID, or "" if none working.
func (st *SubagentTree) CompleteOldestWorking() string {
	oldestIdx := -1
	var oldestTime time.Time
	for i := range st.agents {
		if st.agents[i].Status != "working" {
			continue
		}
		if oldestIdx == -1 || st.agents[i].SpawnTime.Before(oldestTime) {
			oldestIdx = i
			oldestTime = st.agents[i].SpawnTime
		}
	}
	if oldestIdx == -1 {
		return ""
	}
	now := time.Now()
	st.agents[oldestIdx].Status = "completed"
	st.agents[oldestIdx].CompletedTime = &now
	return st.agents[oldestIdx].ID
}

// ChildrenOf returns all agents whose ParentID matches the given ID.
func (st *SubagentTree) ChildrenOf(parentID string) []SubagentEntry {
	var children []SubagentEntry
	for _, a := range st.agents {
		if a.ParentID == parentID {
			children = append(children, a)
		}
	}
	return children
}

// FormatSubagents formats the subagent tree for display based on displayDetail.
//
// Levels per D-03:
//
//	minimal:  "3 agents"
//	standard: "3 agents (2 done)"
//	verbose:  "3: 2xExplore 1xreviewer"
//	private:  ""
//
// Empty tree returns "" per D-04.
func FormatSubagents(tree *SubagentTree, displayDetail string) string {
	agents := tree.All()
	count := len(agents)
	if count == 0 {
		return ""
	}

	switch displayDetail {
	case "private":
		return ""
	case "verbose":
		// Count by type
		typeCounts := make(map[string]int)
		for _, a := range agents {
			typeCounts[a.Type]++
		}
		// Sort types by count desc, then name asc
		type typeEntry struct {
			name  string
			count int
		}
		var entries []typeEntry
		for name, c := range typeCounts {
			entries = append(entries, typeEntry{name, c})
		}
		sort.Slice(entries, func(i, j int) bool {
			if entries[i].count != entries[j].count {
				return entries[i].count > entries[j].count
			}
			return entries[i].name < entries[j].name
		})
		var parts []string
		for _, e := range entries {
			parts = append(parts, fmt.Sprintf("%dx%s", e.count, e.name))
		}
		return fmt.Sprintf("%d: %s", count, strings.Join(parts, " "))
	case "standard":
		completed := 0
		for _, a := range agents {
			if a.Status == "completed" {
				completed++
			}
		}
		if completed > 0 {
			return fmt.Sprintf("%d %s (%d done)", count, pluralize(count, "agent", "agents"), completed)
		}
		return fmt.Sprintf("%d %s", count, pluralize(count, "agent", "agents"))
	default: // minimal
		return fmt.Sprintf("%d %s", count, pluralize(count, "agent", "agents"))
	}
}

// truncateStr returns the first n characters of s, or s itself if shorter.
func truncateStr(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

// pluralize returns singular if count == 1, otherwise plural.
func pluralize(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}
