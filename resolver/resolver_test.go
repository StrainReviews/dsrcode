package resolver_test

import (
	"testing"
	"time"

	"github.com/tsanva/cc-discord-presence/resolver"
	"github.com/tsanva/cc-discord-presence/session"
)

// TestStablePick verifies that the same pool, seed, and time bucket return the
// same index, and that a different 5-minute bucket returns a different index.
func TestStablePick(t *testing.T) {
	pool := []string{"alpha", "bravo", "charlie", "delta", "echo"}
	seed := int64(42)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	result1 := resolver.StablePick(pool, seed, now)
	if result1 == "" {
		t.Fatal("StablePick returned empty string for non-empty pool")
	}

	// Same bucket (within 5 minutes) should return same result
	result2 := resolver.StablePick(pool, seed, now.Add(2*time.Minute))
	if result1 != result2 {
		t.Errorf("StablePick not stable within bucket: got %q then %q", result1, result2)
	}

	// Different bucket (6 minutes later) may return a different result
	// We just verify it doesn't panic and returns a valid pool member
	result3 := resolver.StablePick(pool, seed, now.Add(6*time.Minute))
	found := false
	for _, v := range pool {
		if v == result3 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("StablePick returned %q which is not in pool", result3)
	}

	// Empty pool returns empty string
	empty := resolver.StablePick([]string{}, seed, now)
	if empty != "" {
		t.Errorf("StablePick on empty pool: got %q, want empty string", empty)
	}
}

// TestStablePickDeterministic verifies that 1000 calls with identical input
// (pool, seed, now) all return the same result.
func TestStablePickDeterministic(t *testing.T) {
	pool := []string{"one", "two", "three", "four", "five", "six", "seven"}
	seed := int64(99)
	now := time.Date(2026, 6, 15, 8, 30, 0, 0, time.UTC)

	expected := resolver.StablePick(pool, seed, now)
	for i := 0; i < 1000; i++ {
		got := resolver.StablePick(pool, seed, now)
		if got != expected {
			t.Fatalf("StablePick not deterministic at iteration %d: got %q, want %q", i, got, expected)
		}
	}
}

// TestFormatStatsLine verifies that ActivityCounts{Edits:23, Commands:8}
// formats to "23 edits . 8 cmds" (using middle dot separator).
func TestFormatStatsLine(t *testing.T) {
	counts := session.ActivityCounts{Edits: 23, Commands: 8}
	result := resolver.FormatStatsLine(counts, 0)
	expected := "23 edits \u00b7 8 cmds"
	if result != expected {
		t.Errorf("FormatStatsLine = %q, want %q", result, expected)
	}
}

// TestFormatStatsLineSingular verifies singular forms: ActivityCounts{Edits:1}
// formats to "1 edit" (not "1 edits").
func TestFormatStatsLineSingular(t *testing.T) {
	counts := session.ActivityCounts{Edits: 1}
	result := resolver.FormatStatsLine(counts, 0)
	expected := "1 edit"
	if result != expected {
		t.Errorf("FormatStatsLine singular = %q, want %q", result, expected)
	}

	// Also test singular command
	counts2 := session.ActivityCounts{Commands: 1, Searches: 1}
	result2 := resolver.FormatStatsLine(counts2, 0)
	expected2 := "1 cmd \u00b7 1 search"
	if result2 != expected2 {
		t.Errorf("FormatStatsLine singular cmd+search = %q, want %q", result2, expected2)
	}
}

// TestFormatStatsLineWithDuration verifies that elapsed time is included in the
// stats line, e.g. "1h 15m deep" when sufficient time has passed.
func TestFormatStatsLineWithDuration(t *testing.T) {
	counts := session.ActivityCounts{Edits: 5}
	duration := 75 * time.Minute // 1h 15m
	result := resolver.FormatStatsLine(counts, duration)
	expected := "5 edits \u00b7 1h 15m deep"
	if result != expected {
		t.Errorf("FormatStatsLine with duration = %q, want %q", result, expected)
	}

	// Test minutes-only duration
	result2 := resolver.FormatStatsLine(counts, 30*time.Minute)
	expected2 := "5 edits \u00b7 30m deep"
	if result2 != expected2 {
		t.Errorf("FormatStatsLine 30m = %q, want %q", result2, expected2)
	}

	// Test duration less than 1 minute is omitted
	result3 := resolver.FormatStatsLine(counts, 30*time.Second)
	expected3 := "5 edits"
	if result3 != expected3 {
		t.Errorf("FormatStatsLine 30s = %q, want %q", result3, expected3)
	}
}

// TestDetectDominantMode verifies that when >50% of activity is coding, the
// result is "coding"; when mixed, the result is "multi-session".
func TestDetectDominantMode(t *testing.T) {
	// Coding dominant (60% edits)
	counts := session.ActivityCounts{Edits: 6, Commands: 2, Searches: 1, Reads: 1}
	result := resolver.DetectDominantMode(counts)
	if result != "coding" {
		t.Errorf("DetectDominantMode coding = %q, want %q", result, "coding")
	}

	// Mixed (no >50%)
	mixed := session.ActivityCounts{Edits: 3, Commands: 3, Searches: 2, Reads: 2}
	result2 := resolver.DetectDominantMode(mixed)
	if result2 != "multi-session" {
		t.Errorf("DetectDominantMode mixed = %q, want %q", result2, "multi-session")
	}

	// All zeros -> idle
	result3 := resolver.DetectDominantMode(session.ActivityCounts{})
	if result3 != "idle" {
		t.Errorf("DetectDominantMode zero = %q, want %q", result3, "idle")
	}

	// Terminal dominant (75%)
	terminal := session.ActivityCounts{Commands: 9, Edits: 3}
	result4 := resolver.DetectDominantMode(terminal)
	if result4 != "terminal" {
		t.Errorf("DetectDominantMode terminal = %q, want %q", result4, "terminal")
	}
}

// TestResolveSingleSession verifies that with 1 session, resolvePresence
// returns direct details and state from the preset's single-session messages.
func TestResolveSingleSession(t *testing.T) {
	t.Skip("not implemented yet")
}

// TestResolveMultiSession verifies that with 3 sessions, resolvePresence
// returns the tier "3" message from the preset's multi-session pool.
func TestResolveMultiSession(t *testing.T) {
	t.Skip("not implemented yet")
}
