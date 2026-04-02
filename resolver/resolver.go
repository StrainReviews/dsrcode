// Package resolver converts session state into Discord Activity fields.
// It uses presets for message pools and StablePick for anti-flicker rotation.
package resolver

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/tsanva/cc-discord-presence/discord"
	"github.com/tsanva/cc-discord-presence/preset"
	"github.com/tsanva/cc-discord-presence/session"
)

const (
	// maxFieldLength is the Discord API limit for Details/State fields.
	maxFieldLength = 128
	// largeImageKey is the fixed large image asset key per D-10.
	largeImageKey = "dsr-code"
)

// ResolvePresence converts current session state into a Discord Activity.
// Returns nil when there are no sessions (clears presence).
// Per D-08 to D-15 Layout D.
func ResolvePresence(sessions []*session.Session, p *preset.MessagePreset, now time.Time) *discord.Activity {
	if len(sessions) == 0 {
		return nil
	}

	if len(sessions) == 1 {
		return resolveSingle(sessions[0], p, now)
	}
	return resolveMulti(sessions, p, now)
}

// resolveSingle builds a Layout D activity for a single active session.
func resolveSingle(s *session.Session, p *preset.MessagePreset, now time.Time) *discord.Activity {
	seed := HashString(s.SessionID)

	// Pick detail message from preset pool for this activity icon
	pool := p.SingleSessionDetails[s.SmallImageKey]
	if len(pool) == 0 {
		pool = p.SingleSessionDetailsFallback
	}
	details := StablePick(pool, seed, now)

	// Replace placeholders in details
	details = replacePlaceholders(details, map[string]string{
		"{project}": s.ProjectName,
		"{branch}":  s.Branch,
	})

	// Pick state message from preset pool (use seed+1 for independent rotation)
	state := StablePick(p.SingleSessionState, seed+1, now)

	// Replace placeholders in state
	duration := now.Sub(s.StartedAt)
	state = replacePlaceholders(state, map[string]string{
		"{model}":    s.Model,
		"{tokens}":   formatTokens(s.TotalTokens),
		"{cost}":     formatCost(s.TotalCostUSD),
		"{duration}": formatDuration(duration),
	})

	startTime := s.StartedAt
	activity := &discord.Activity{
		Details:    truncate(details, maxFieldLength),
		State:      truncate(state, maxFieldLength),
		LargeImage: largeImageKey,
		LargeText:  s.ProjectName + " (" + s.Branch + ")",
		SmallImage: s.SmallImageKey,
		SmallText:  s.SmallText,
		StartTime:  &startTime,
		Buttons:    convertButtons(p.Buttons),
	}

	return activity
}

// resolveMulti builds a Layout D activity for multiple concurrent sessions.
// Per D-27: tier-based messages for 2, 3, 4, and 5+ sessions.
func resolveMulti(sessions []*session.Session, p *preset.MessagePreset, now time.Time) *discord.Activity {
	n := len(sessions)

	// Find earliest session for seed and start time
	earliest := sessions[0]
	for _, s := range sessions[1:] {
		if s.StartedAt.Before(earliest.StartedAt) {
			earliest = s
		}
	}
	seed := HashString(earliest.SessionID)

	// Pick message from tier-appropriate pool
	var pool []string
	tierKey := strconv.Itoa(n)
	if n <= 4 {
		pool = p.MultiSessionMessages[tierKey]
	}
	if len(pool) == 0 {
		pool = p.MultiSessionOverflow
	}

	details := StablePick(pool, seed, now)

	// Aggregate activity counts across all sessions
	var totalCounts session.ActivityCounts
	for _, s := range sessions {
		totalCounts.Edits += s.ActivityCounts.Edits
		totalCounts.Commands += s.ActivityCounts.Commands
		totalCounts.Searches += s.ActivityCounts.Searches
		totalCounts.Reads += s.ActivityCounts.Reads
		totalCounts.Thinks += s.ActivityCounts.Thinks
	}

	duration := now.Sub(earliest.StartedAt)
	statsLine := FormatStatsLine(totalCounts, duration)
	mode := DetectDominantMode(totalCounts)

	// Replace placeholders
	details = replacePlaceholders(details, map[string]string{
		"{n}":     strconv.Itoa(n),
		"{stats}": statsLine,
		"{mode}":  mode,
	})

	// Pick state: use stats line as the state
	state := statsLine
	if state == "" {
		state = "Just getting started"
	}

	// Find most recent active session for icon
	mostRecent := getMostRecentSession(sessions)
	smallImageKey := mostRecent.SmallImageKey
	smallText := StablePick(p.MultiSessionTooltips, seed+1, now)
	if smallText == "" {
		smallText = mostRecent.SmallText
	}

	startTime := earliest.StartedAt
	return &discord.Activity{
		Details:    truncate(details, maxFieldLength),
		State:      truncate(state, maxFieldLength),
		LargeImage: largeImageKey,
		LargeText:  fmt.Sprintf("%d sessions active", n),
		SmallImage: smallImageKey,
		SmallText:  smallText,
		StartTime:  &startTime,
		Buttons:    convertButtons(p.Buttons),
	}
}

// getMostRecentSession returns the session with the most recent activity,
// preferring active sessions over idle ones.
func getMostRecentSession(sessions []*session.Session) *session.Session {
	if len(sessions) == 0 {
		return nil
	}
	best := sessions[0]
	for _, s := range sessions[1:] {
		if best.Status == session.StatusIdle && s.Status == session.StatusActive {
			best = s
			continue
		}
		if best.Status == session.StatusActive && s.Status == session.StatusIdle {
			continue
		}
		if s.LastActivityAt.After(best.LastActivityAt) {
			best = s
		}
	}
	return best
}

// convertButtons converts preset buttons to discord buttons.
func convertButtons(presetButtons []preset.Button) []discord.Button {
	if len(presetButtons) == 0 {
		return nil
	}
	buttons := make([]discord.Button, 0, len(presetButtons))
	for _, b := range presetButtons {
		buttons = append(buttons, discord.Button{
			Label: b.Label,
			URL:   b.URL,
		})
	}
	return buttons
}

// formatTokens formats a token count for display.
// 1500000 -> "1.5M", 150000 -> "150K", 1500 -> "1.5K", 500 -> "500"
func formatTokens(n int64) string {
	switch {
	case n >= 1_000_000:
		val := float64(n) / 1_000_000
		if val == float64(int64(val)) {
			return fmt.Sprintf("%dM", int64(val))
		}
		return fmt.Sprintf("%.1fM", val)
	case n >= 1_000:
		val := float64(n) / 1_000
		if val == float64(int64(val)) {
			return fmt.Sprintf("%dK", int64(val))
		}
		return fmt.Sprintf("%.1fK", val)
	default:
		return strconv.FormatInt(n, 10)
	}
}

// formatCost formats a USD cost with dollar sign. 0.12 -> "$0.12", 1.5 -> "$1.50"
func formatCost(c float64) string {
	return fmt.Sprintf("$%.2f", c)
}

// formatDuration formats a duration for display. 75m -> "1h 15m", 5m -> "5m"
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if hours > 0 {
		if mins > 0 {
			return fmt.Sprintf("%dh %dm", hours, mins)
		}
		return fmt.Sprintf("%dh", hours)
	}
	if mins > 0 {
		return fmt.Sprintf("%dm", mins)
	}
	return "<1m"
}

// truncate shortens s to max characters, appending ellipsis if truncated.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "\u2026"
}

// replacePlaceholders replaces all {key} placeholders in a template string.
func replacePlaceholders(template string, values map[string]string) string {
	result := template
	for k, v := range values {
		result = strings.ReplaceAll(result, k, v)
	}
	return result
}
