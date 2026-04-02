package resolver

import "time"

// MessageRotationInterval is the time window for stable message selection per D-23.
// Messages stay the same within a 5-minute window to prevent flickering.
const MessageRotationInterval = 5 * time.Minute

// StablePick selects a deterministic item from a pool using Knuth multiplicative hash.
// Same pool+seed+timeWindow always returns the same index (anti-flicker per D-23).
// seed should be derived from session-specific data (e.g., hash of sessionID).
func StablePick(pool []string, seed int64, now time.Time) string {
	if len(pool) == 0 {
		return ""
	}
	bucket := now.Unix() / int64(MessageRotationInterval.Seconds())
	// Knuth multiplicative hash constant: 2654435761
	combined := uint32(bucket*2654435761 + seed*2654435761)
	index := combined % uint32(len(pool))
	return pool[index]
}

// HashString returns a stable int64 hash of a string for use as StablePick seed.
func HashString(s string) int64 {
	var h int64
	for _, c := range s {
		h = h*31 + int64(c)
	}
	return h
}
