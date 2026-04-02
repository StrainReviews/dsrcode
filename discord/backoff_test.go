package discord_test

import (
	"testing"
)

// TestBackoffSequence verifies that NewBackoff(1s, 30s).Next() returns the
// expected exponential sequence: 1s, 2s, 4s, 8s, 16s, 30s, 30s (capped at max).
func TestBackoffSequence(t *testing.T) {
	t.Skip("not implemented yet")
}

// TestBackoffReset verifies that after 3 Next() calls, calling Reset() makes
// the next Next() return the initial duration (1s) again.
func TestBackoffReset(t *testing.T) {
	t.Skip("not implemented yet")
}

// TestBackoffMax verifies that the backoff duration never exceeds the
// configured maximum, regardless of how many Next() calls are made.
func TestBackoffMax(t *testing.T) {
	t.Skip("not implemented yet")
}
