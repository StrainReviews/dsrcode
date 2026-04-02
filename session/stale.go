package session

import (
	"context"
	"log/slog"
	"time"
)

// CheckStaleSessions runs a periodic check on all sessions per D-29 to D-31.
//   - Sessions whose PID is no longer alive: remove entirely
//   - Sessions with LastActivityAt older than removeTimeout: remove entirely
//   - Sessions with LastActivityAt older than idleTimeout: transition to StatusIdle
//
// Runs in a goroutine, respects context for clean shutdown.
func CheckStaleSessions(ctx context.Context, registry *SessionRegistry, idleTimeout, removeTimeout, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			CheckOnce(registry, idleTimeout, removeTimeout)
		}
	}
}

// CheckOnce performs a single pass over all sessions, transitioning idle and
// removing stale sessions. Exported so external test packages can call it directly.
func CheckOnce(registry *SessionRegistry, idleTimeout, removeTimeout time.Duration) {
	now := time.Now()
	sessions := registry.GetAllSessions()

	for _, s := range sessions {
		elapsed := now.Sub(s.LastActivityAt)

		// PID liveness check
		if s.PID > 0 && !IsPidAlive(s.PID) {
			slog.Info("removing stale session (PID dead)", "sessionId", s.SessionID, "pid", s.PID)
			registry.EndSession(s.SessionID)
			continue
		}

		// Remove timeout (per D-29: 30min default)
		if elapsed > removeTimeout {
			slog.Info("removing stale session (timeout)", "sessionId", s.SessionID, "elapsed", elapsed)
			registry.EndSession(s.SessionID)
			continue
		}

		// Idle timeout (per D-29: 10min default)
		if elapsed > idleTimeout && s.Status == StatusActive {
			slog.Debug("marking session idle", "sessionId", s.SessionID, "elapsed", elapsed)
			registry.TransitionToIdle(s.SessionID)
		}
	}
}
