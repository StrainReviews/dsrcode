package analytics

import "sync"

// Features controls which analytics features are active per D-34.
type Features struct {
	Analytics bool `json:"analytics"`
}

// TrackerConfig holds configuration for the analytics Tracker.
type TrackerConfig struct {
	Features Features
}

// TrackerState holds the current analytics state for a single session.
type TrackerState struct {
	Tokens          map[string]TokenBreakdown `json:"tokens,omitempty"`
	Baselines       map[string]TokenBreakdown `json:"baselines,omitempty"`
	ToolCounts      map[string]int            `json:"toolCounts,omitempty"`
	Subagents       []SubagentEntry           `json:"subagents,omitempty"`
	CompactionCount int                       `json:"compactionCount"`
	ContextPct      *float64                  `json:"contextPct"`
}

// sessionState is the internal per-session state held by the Tracker.
type sessionState struct {
	tokens      map[string]TokenBreakdown
	baselines   map[string]TokenBreakdown
	tools       *ToolCounter
	subagents   *SubagentTree
	compactions *CompactionTracker
	contextPct  *float64
}

func newSessionState() *sessionState {
	return &sessionState{
		tokens:      make(map[string]TokenBreakdown),
		baselines:   make(map[string]TokenBreakdown),
		tools:       NewToolCounter(),
		subagents:   NewSubagentTree(),
		compactions: NewCompactionTracker(),
	}
}

// Tracker is the central analytics coordinator. It dispatches events to
// domain-specific handlers and manages per-session state.
type Tracker struct {
	mu       sync.RWMutex
	sessions map[string]*sessionState
	config   TrackerConfig
}

// NewTracker creates a new Tracker with the given configuration.
func NewTracker(cfg TrackerConfig) *Tracker {
	return &Tracker{
		sessions: make(map[string]*sessionState),
		config:   cfg,
	}
}

// getOrCreate returns the sessionState for the given sessionID, creating
// it if it doesn't exist yet.
func (t *Tracker) getOrCreate(sessionID string) *sessionState {
	state, ok := t.sessions[sessionID]
	if !ok {
		state = newSessionState()
		t.sessions[sessionID] = state
	}
	return state
}

// RecordTool records a tool invocation for the given session.
// No-op when analytics feature is disabled per D-34.
func (t *Tracker) RecordTool(sessionID, toolName string) {
	if !t.config.Features.Analytics {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	state := t.getOrCreate(sessionID)
	state.tools.Record(toolName)
}

// RecordSubagentSpawn records a new subagent spawn for the given session.
// No-op when analytics feature is disabled per D-34.
func (t *Tracker) RecordSubagentSpawn(sessionID string, entry SubagentEntry) {
	if !t.config.Features.Analytics {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	state := t.getOrCreate(sessionID)
	state.subagents.Spawn(entry)
}

// RecordSubagentComplete records a subagent completion for the given session.
// Uses 4-strategy matching per D-08.
// No-op when analytics feature is disabled per D-34.
func (t *Tracker) RecordSubagentComplete(sessionID string, event SubagentStopEvent) {
	if !t.config.Features.Analytics {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	state := t.getOrCreate(sessionID)

	// Strategy 1: exact agent_id match
	if event.AgentID != "" {
		if matched := state.subagents.CompleteByAgentID(event.AgentID); matched != "" {
			return
		}
	}
	// Strategy 2: description prefix match (57 chars)
	if event.Description != "" {
		if matched := state.subagents.CompleteByDescriptionPrefix(event.Description); matched != "" {
			return
		}
	}
	// Strategy 3: agent_type match
	if event.AgentType != "" {
		if matched := state.subagents.CompleteByAgentType(event.AgentType); matched != "" {
			return
		}
	}
	// Strategy 4: fallback to oldest working
	state.subagents.CompleteOldestWorking()
}

// UpdateTokens updates the token breakdown for a specific model in the given session.
// Handles baseline computation for compaction recovery per D-19.
// No-op when analytics feature is disabled per D-34.
func (t *Tracker) UpdateTokens(sessionID, model string, tokens TokenBreakdown) {
	if !t.config.Features.Analytics {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	state := t.getOrCreate(sessionID)

	// Check for compaction baseline
	if stored, ok := state.tokens[model]; ok {
		newBaseline := ComputeBaseline(stored, tokens)
		if newBaseline.Total() > 0 {
			existing := state.baselines[model]
			state.baselines[model] = EffectiveTotal(existing, newBaseline)
		}
	}
	state.tokens[model] = tokens
}

// RecordCompaction records a compaction event with UUID deduplication.
// No-op when analytics feature is disabled per D-34.
func (t *Tracker) RecordCompaction(sessionID, uuid string) {
	if !t.config.Features.Analytics {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	state := t.getOrCreate(sessionID)
	state.compactions.Record(uuid)
}

// UpdateContextUsage updates the context usage percentage for the given session.
// No-op when analytics feature is disabled per D-34.
func (t *Tracker) UpdateContextUsage(sessionID string, pct float64) {
	if !t.config.Features.Analytics {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	state := t.getOrCreate(sessionID)
	state.contextPct = &pct
}

// GetState returns a snapshot of the analytics state for the given session.
// Returns a zero-value TrackerState if the session doesn't exist.
func (t *Tracker) GetState(sessionID string) TrackerState {
	t.mu.RLock()
	defer t.mu.RUnlock()
	state, ok := t.sessions[sessionID]
	if !ok {
		return TrackerState{}
	}
	return TrackerState{
		Tokens:          copyTokenMap(state.tokens),
		Baselines:       copyTokenMap(state.baselines),
		ToolCounts:      state.tools.Counts(),
		Subagents:       state.subagents.All(),
		CompactionCount: state.compactions.Count(),
		ContextPct:      state.contextPct,
	}
}

// copyTokenMap creates a shallow copy of a map[string]TokenBreakdown.
func copyTokenMap(m map[string]TokenBreakdown) map[string]TokenBreakdown {
	if len(m) == 0 {
		return nil
	}
	result := make(map[string]TokenBreakdown, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}
