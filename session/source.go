// Package session provides session source classification for dedup logic.
package session

import (
	"encoding/json"
	"strings"
)

// SessionSource indicates how a session was created (JSONL fallback, HTTP hook, or real Claude Code UUID).
// Higher-ranked sources replace lower-ranked ones for the same project (upgrade chain).
type SessionSource int

const (
	SourceJSONL  SessionSource = iota // 0 -- JSONL fallback (synthetic "jsonl-" prefix, PID=0)
	SourceHTTP                        // 1 -- HTTP hook with synthetic ID ("http-" prefix)
	SourceClaude                      // 2 -- Real Claude Code session (UUID)
)

// sourceNames maps enum values to their string representation.
var sourceNames = [...]string{
	SourceJSONL:  "jsonl",
	SourceHTTP:   "http",
	SourceClaude: "claude",
}

// String returns the human-readable name of the session source.
func (s SessionSource) String() string {
	if int(s) < len(sourceNames) {
		return sourceNames[s]
	}
	return "unknown"
}

// MarshalJSON serializes SessionSource as a JSON string (per D-24).
// Produces "claude" not 2 in JSON output.
func (s SessionSource) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// sourceFromID infers the SessionSource from a session ID string.
// IDs with "jsonl-" prefix are JSONL, "http-" prefix are HTTP, everything else is Claude UUID.
func sourceFromID(id string) SessionSource {
	if strings.HasPrefix(id, "jsonl-") {
		return SourceJSONL
	}
	if strings.HasPrefix(id, "http-") {
		return SourceHTTP
	}
	return SourceClaude
}

// sourceRank returns the priority rank for upgrade comparison.
// Higher rank = better quality session source.
func sourceRank(s SessionSource) int {
	return int(s) // iota order matches rank: JSONL=0 < HTTP=1 < Claude=2
}
