// Package analytics provides domain types and formatting for session analytics:
// token breakdown, compaction detection, subagent tracking, tool statistics,
// context usage, pricing, and file persistence.
package analytics

import "fmt"

// TokenBreakdown holds the per-type token counts from a Claude Code session.
// Fields map to JSONL message.usage: input_tokens, output_tokens,
// cache_read_input_tokens, cache_creation_input_tokens.
type TokenBreakdown struct {
	Input      int64 `json:"input"`
	Output     int64 `json:"output"`
	CacheRead  int64 `json:"cacheRead"`
	CacheWrite int64 `json:"cacheWrite"`
}

// Total returns the sum of all four token types.
func (t TokenBreakdown) Total() int64 {
	return t.Input + t.Output + t.CacheRead + t.CacheWrite
}

// AggregateTokens merges per-model token breakdowns into a single total.
func AggregateTokens(perModel map[string]TokenBreakdown) TokenBreakdown {
	var total TokenBreakdown
	for _, tb := range perModel {
		total.Input += tb.Input
		total.Output += tb.Output
		total.CacheRead += tb.CacheRead
		total.CacheWrite += tb.CacheWrite
	}
	return total
}

// FormatTokens formats a TokenBreakdown for display based on displayDetail level.
//
// Levels per D-03:
//
//	minimal:  total count only ("250K")
//	standard: input+output with arrows ("250K↑ 80K↓")
//	verbose:  input+output+cache ("250K↑ 80K↓ 156Kc")
//	private:  empty string
//
// Zero total returns "" per D-04.
func FormatTokens(tokens TokenBreakdown, displayDetail string) string {
	if tokens.Total() == 0 {
		return ""
	}

	switch displayDetail {
	case "private":
		return ""
	case "verbose":
		cache := tokens.CacheRead + tokens.CacheWrite
		result := fmt.Sprintf("%s\u2191 %s\u2193", formatCount(tokens.Input), formatCount(tokens.Output))
		if cache > 0 {
			result += " " + formatCount(cache) + "c"
		}
		return result
	case "standard":
		return fmt.Sprintf("%s\u2191 %s\u2193", formatCount(tokens.Input), formatCount(tokens.Output))
	default: // minimal
		return formatCount(tokens.Total())
	}
}

// formatCount abbreviates a token count: 250000 -> "250K", 1200000 -> "1.2M".
func formatCount(n int64) string {
	if n >= 1_000_000 {
		m := float64(n) / 1_000_000
		if m == float64(int64(m)) {
			return fmt.Sprintf("%dM", int64(m))
		}
		return fmt.Sprintf("%.1fM", m)
	}
	if n >= 1_000 {
		k := float64(n) / 1_000
		if k == float64(int64(k)) {
			return fmt.Sprintf("%dK", int64(k))
		}
		return fmt.Sprintf("%.1fK", k)
	}
	return fmt.Sprintf("%d", n)
}
