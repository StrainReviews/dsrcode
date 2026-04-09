package resolver

import (
	"fmt"
	"strings"
	"time"

	"github.com/StrainReviews/dsrcode/session"
)

// FormatStatsLine produces "23 edits . 8 cmds . 1h 15m deep" per D-25.
// Only non-zero counts are included. Uses singular for count=1.
func FormatStatsLine(counts session.ActivityCounts, duration time.Duration) string {
	parts := make([]string, 0, 6)

	if counts.Edits > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", counts.Edits, pluralize("edit", counts.Edits)))
	}
	if counts.Commands > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", counts.Commands, pluralize("cmd", counts.Commands)))
	}
	if counts.Searches > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", counts.Searches, pluralize("search", counts.Searches)))
	}
	if counts.Reads > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", counts.Reads, pluralize("read", counts.Reads)))
	}
	if counts.Thinks > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", counts.Thinks, pluralize("think", counts.Thinks)))
	}

	if duration >= time.Minute {
		hours := int(duration.Hours())
		mins := int(duration.Minutes()) % 60
		if hours > 0 {
			if mins > 0 {
				parts = append(parts, fmt.Sprintf("%dh %dm deep", hours, mins))
			} else {
				parts = append(parts, fmt.Sprintf("%dh deep", hours))
			}
		} else {
			parts = append(parts, fmt.Sprintf("%dm deep", mins))
		}
	}

	return strings.Join(parts, " \u00b7 ") // Unicode middle dot
}

func pluralize(word string, count int) string {
	if count == 1 {
		return word
	}
	if word == "search" {
		return "searches"
	}
	return word + "s"
}

// DetectDominantMode returns the activity icon that accounts for >50% of total activity.
// If no single activity is dominant, returns "multi-session" per D-25.
// If all counts are zero, returns "idle".
func DetectDominantMode(counts session.ActivityCounts) string {
	total := counts.Edits + counts.Commands + counts.Searches + counts.Reads + counts.Thinks
	if total == 0 {
		return "idle"
	}

	type entry struct {
		icon  string
		count int
	}
	activities := []entry{
		{"coding", counts.Edits},
		{"terminal", counts.Commands},
		{"searching", counts.Searches},
		{"reading", counts.Reads},
		{"thinking", counts.Thinks},
	}

	for _, a := range activities {
		if float64(a.count)/float64(total) > 0.5 {
			return a.icon
		}
	}
	return "multi-session"
}
