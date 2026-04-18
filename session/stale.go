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

		// PID liveness check — skip for sessions whose PID is not authoritative.
		//
		// The PID recorded for a session may not be the actual Claude Code process:
		//   - SourceHTTP synthetic IDs ("http-*") always use a wrapper-process PID
		//     from start.sh/start.ps1 — the wrapper exits within seconds.
		//   - SourceClaude UUID IDs also arrive via the HTTP hook path in production
		//     (Claude Code does not spawn dsrcode directly), carrying the same
		//     short-lived wrapper PID via X-Claude-PID header or os.Getppid().
		//
		// On Windows, orphan processes are not reparented to PID 1 — once the
		// wrapper exits, IsPidAlive(wrapperPID) is permanently false even though
		// Claude Code itself is active. On Unix the wrapper parent chain has the
		// same weakness (start.sh exits before Claude does).
		//
		// The removeTimeout backstop (30min default) still reaches truly stale
		// sessions that have stopped sending hooks entirely.
		if s.PID > 0 && s.Source != SourceHTTP && s.Source != SourceClaude && !IsPidAlive(s.PID) {
			if elapsed > 2*time.Minute {
				slog.Info("removing stale session (PID dead, no recent activity)", "sessionId", s.SessionID, "pid", s.PID, "elapsed", elapsed)
				registry.EndSession(s.SessionID)
				continue
			}
			slog.Debug("PID dead but session has recent activity, skipping removal", "sessionId", s.SessionID, "pid", s.PID, "elapsed", elapsed)
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
