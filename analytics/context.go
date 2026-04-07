package analytics

import (
	"fmt"
	"math"
	"strconv"
)

// ParseContextPct parses a string percentage value into *float64.
// Returns nil for empty, "null", or unparseable values per D-04.
func ParseContextPct(value string) *float64 {
	if value == "" || value == "null" {
		return nil
	}
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return nil
	}
	return &f
}

// FormatContextPct formats a context usage percentage for display.
//
// Levels per D-03:
//
//	minimal:  "78%"
//	standard: "78% context"
//	verbose:  "78% context" (use FormatContextPctVerbose for token details)
//	private:  ""
//
// Nil input returns "" per D-04.
func FormatContextPct(pct *float64, displayDetail string) string {
	if pct == nil {
		return ""
	}

	switch displayDetail {
	case "private":
		return ""
	case "verbose":
		return fmt.Sprintf("%d%% context", int(math.Floor(*pct)))
	case "standard":
		return fmt.Sprintf("%d%% context", int(math.Floor(*pct)))
	default: // minimal
		return fmt.Sprintf("%d%%", int(math.Floor(*pct)))
	}
}

// FormatContextPctVerbose formats context usage with token details:
// "78% (156K/200K)" showing used and total tokens.
// Returns "" if pct is nil.
func FormatContextPctVerbose(pct *float64, usedTokens, totalTokens int64) string {
	if pct == nil {
		return ""
	}
	return fmt.Sprintf("%d%% (%s/%s)", int(math.Floor(*pct)), formatCount(usedTokens), formatCount(totalTokens))
}
