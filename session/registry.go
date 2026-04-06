// Package session provides the in-memory session registry for tracking
// concurrent Claude Code sessions with thread-safe access.
package session

import (
	"path/filepath"
	"sync"
	"time"
)

// SessionRegistry manages active Claude Code sessions in a thread-safe map.
// All mutation methods call the onChange callback (if non-nil) after the state
// changes, allowing the caller to trigger a Discord presence update.
type SessionRegistry struct {
	mu       sync.RWMutex
	sessions map[string]*Session // key: sessionID
	onChange func()              // called after any mutation
}

// NewRegistry creates a new SessionRegistry with the given onChange callback.
// The callback is invoked after every mutation (start, update, end, idle transition).
func NewRegistry(onChange func()) *SessionRegistry {
	return &SessionRegistry{
		sessions: make(map[string]*Session),
		onChange: onChange,
	}
}

// notifyChange calls the onChange callback if it is set.
// Must be called with mu held (write lock).
func (r *SessionRegistry) notifyChange() {
	if r.onChange != nil {
		r.onChange()
	}
}

// StartSession registers a new session or returns an existing one if a session
// with the same ProjectPath and PID already exists (dedup per D-28).
func (r *SessionRegistry) StartSession(req ActivityRequest, pid int) *Session {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Dedup: check if a session with the same ProjectPath+PID already exists
	for _, existing := range r.sessions {
		if existing.ProjectPath == req.Cwd && existing.PID == pid {
			return existing
		}
	}

	now := time.Now()
	s := &Session{
		SessionID:      req.SessionID,
		ProjectPath:    req.Cwd,
		ProjectName:    filepath.Base(req.Cwd),
		PID:            pid,
		Source:         sourceFromID(req.SessionID),
		Details:        "Starting session...",
		SmallImageKey:  "starting",
		SmallText:      "Starting up...",
		Status:         StatusActive,
		ActivityCounts: EmptyActivityCounts(),
		StartedAt:      now,
		LastActivityAt: now,
	}

	r.sessions[req.SessionID] = s
	r.notifyChange()
	return s
}

// StartSessionWithSource registers a new session with an explicit source.
// If an existing lower-ranked session for the same project exists, it is upgraded
// (re-keyed) to the new ID/PID/Source. If a same-or-higher ranked session exists
// for the project, the behavior depends on rank:
//   - Lower incoming rank (downgrade attempt): returns existing session unchanged
//   - Equal or higher incoming rank with same PID: returns existing unchanged
//   - Equal rank with different PID: creates new session (separate window per D-01)
//
// Returns the active session for the project.
func (r *SessionRegistry) StartSessionWithSource(req ActivityRequest, pid int, source SessionSource) *Session {
	r.mu.Lock()
	defer r.mu.Unlock()

	projectName := filepath.Base(req.Cwd)
	newRank := sourceRank(source)

	// Search for upgrade candidates: existing sessions for the same project
	for _, existing := range r.sessions {
		if existing.ProjectName != projectName {
			continue
		}
		existingRank := sourceRank(existing.Source)

		if newRank > existingRank {
			// Upgrade: higher-ranked source replaces lower-ranked one
			return r.upgradeSession(existing, req.SessionID, pid, source)
		}
		if newRank < existingRank {
			// No downgrade: return existing session unchanged
			return existing
		}
		// Equal rank: same PID means same session (dedup), different PID means separate window
		if existing.PID == pid || pid == 0 || existing.PID == 0 {
			return existing
		}
		// Different PIDs at same rank = separate Claude Code windows (D-01), continue to create new
	}

	// No matching session found (or only same-rank different-PID sessions) -- create new
	now := time.Now()
	s := &Session{
		SessionID:      req.SessionID,
		ProjectPath:    req.Cwd,
		ProjectName:    projectName,
		PID:            pid,
		Source:         source,
		Details:        "Starting session...",
		SmallImageKey:  "starting",
		SmallText:      "Starting up...",
		Status:         StatusActive,
		ActivityCounts: EmptyActivityCounts(),
		StartedAt:      now,
		LastActivityAt: now,
	}

	r.sessions[req.SessionID] = s
	r.notifyChange()
	return s
}

// foundExisting finds an existing session for the same project that could be
// an upgrade candidate. Returns nil if no match. Matches by ProjectName.
// Used by the dedup logic in StartSessionWithSource.
// Must be called with mu held (read or write lock).
func (r *SessionRegistry) foundExisting(projectName string) *Session {
	for _, existing := range r.sessions {
		if existing.ProjectName == projectName {
			return existing
		}
	}
	return nil
}

// upgradeSession replaces a lower-ranked session with a higher-ranked one,
// preserving StartedAt, ActivityCounts, Model, TotalTokens, TotalCostUSD (per D-04).
// Returns the upgraded session. Caller must hold write lock.
// Uses copy-before-modify pattern for immutability.
func (r *SessionRegistry) upgradeSession(existing *Session, newID string, newPID int, newSource SessionSource) *Session {
	upgraded := *existing // immutable copy pattern
	upgraded.SessionID = newID
	upgraded.PID = newPID
	upgraded.Source = newSource
	// StartedAt, ActivityCounts, Model, TotalTokens, TotalCostUSD preserved (D-04, D-13)

	// Re-key the map: remove old ID, insert under new ID
	delete(r.sessions, existing.SessionID)
	r.sessions[newID] = &upgraded
	r.notifyChange()
	return &upgraded
}

// UpdateActivity updates tool activity for an existing session. Creates a copy
// of the session before modifying (immutable update pattern). Returns the
// updated session or nil if not found.
func (r *SessionRegistry) UpdateActivity(sessionID string, req ActivityRequest) *Session {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, ok := r.sessions[sessionID]
	if !ok {
		return nil
	}

	// Immutable update: copy the session struct
	updated := *session

	if req.SmallImageKey != "" {
		updated.SmallImageKey = req.SmallImageKey
	}
	if req.SmallText != "" {
		updated.SmallText = req.SmallText
	}
	if req.Details != "" {
		updated.Details = req.Details
	}
	if req.LastFile != "" {
		updated.LastFile = req.LastFile
	}
	if req.LastFilePath != "" {
		updated.LastFilePath = req.LastFilePath
	}
	if req.LastCommand != "" {
		updated.LastCommand = req.LastCommand
	}
	if req.LastQuery != "" {
		updated.LastQuery = req.LastQuery
	}

	// Increment activity counter using CounterMap
	if counterField, ok := CounterMap[updated.SmallImageKey]; ok {
		incrementCounter(&updated.ActivityCounts, counterField)
	}

	updated.LastActivityAt = time.Now()
	updated.Status = StatusActive

	r.sessions[sessionID] = &updated
	r.notifyChange()
	return &updated
}

// incrementCounter increments the named field in ActivityCounts.
func incrementCounter(ac *ActivityCounts, field string) {
	switch field {
	case "edits":
		ac.Edits++
	case "commands":
		ac.Commands++
	case "searches":
		ac.Searches++
	case "reads":
		ac.Reads++
	case "thinks":
		ac.Thinks++
	}
}

// UpdateSessionData updates metadata fields (model, branch, tokens, cost) for
// an existing session. Uses the immutable copy pattern.
func (r *SessionRegistry) UpdateSessionData(sessionID string, model string, branch string, totalTokens int64, totalCostUSD float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, ok := r.sessions[sessionID]
	if !ok {
		return
	}

	updated := *session
	updated.Model = model
	updated.Branch = branch
	updated.TotalTokens = totalTokens
	updated.TotalCostUSD = totalCostUSD

	r.sessions[sessionID] = &updated
	r.notifyChange()
}

// EndSession removes a session from the registry.
func (r *SessionRegistry) EndSession(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.sessions[sessionID]; ok {
		delete(r.sessions, sessionID)
		r.notifyChange()
	}
}

// GetSession returns a copy of the session with the given ID, or nil if not found.
func (r *SessionRegistry) GetSession(sessionID string) *Session {
	r.mu.RLock()
	defer r.mu.RUnlock()

	session, ok := r.sessions[sessionID]
	if !ok {
		return nil
	}

	copy := *session
	return &copy
}

// GetAllSessions returns copies of all sessions in the registry.
func (r *SessionRegistry) GetAllSessions() []*Session {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Session, 0, len(r.sessions))
	for _, s := range r.sessions {
		copy := *s
		result = append(result, &copy)
	}
	return result
}

// SessionCount returns the number of sessions currently tracked.
func (r *SessionRegistry) SessionCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.sessions)
}

// GetMostRecentSession returns a copy of the most relevant session.
// Prefers active sessions over idle ones. Among sessions of the same status,
// returns the one with the most recent LastActivityAt.
// Returns nil if the registry is empty.
func (r *SessionRegistry) GetMostRecentSession() *Session {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.sessions) == 0 {
		return nil
	}

	var best *Session
	for _, current := range r.sessions {
		if best == nil {
			best = current
			continue
		}

		// Prefer active over idle
		if best.Status == StatusActive && current.Status == StatusIdle {
			continue
		}
		if best.Status == StatusIdle && current.Status == StatusActive {
			best = current
			continue
		}

		// Same status: prefer most recent activity
		if current.LastActivityAt.After(best.LastActivityAt) {
			best = current
		}
	}

	if best == nil {
		return nil
	}

	copy := *best
	return &copy
}

// TransitionToIdle marks a session as idle. Used by the stale detection loop
// when a session exceeds the idle timeout. Uses the immutable copy pattern.
func (r *SessionRegistry) TransitionToIdle(sessionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, ok := r.sessions[sessionID]
	if !ok {
		return
	}

	updated := *session
	updated.Status = StatusIdle
	updated.SmallImageKey = "idle"
	updated.SmallText = "Idle"

	r.sessions[sessionID] = &updated
	r.notifyChange()
}

// SetLastActivityForTest overwrites LastActivityAt for a session.
// Intended for testing stale detection with controlled timestamps.
func (r *SessionRegistry) SetLastActivityForTest(sessionID string, t time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()

	session, ok := r.sessions[sessionID]
	if !ok {
		return
	}

	updated := *session
	updated.LastActivityAt = t
	r.sessions[sessionID] = &updated
}
