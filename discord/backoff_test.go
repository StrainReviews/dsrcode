package discord_test

import (
	"testing"
	"time"

	"github.com/tsanva/cc-discord-presence/discord"
)

// TestBackoffSequence verifies that NewBackoff(1s, 30s).Next() returns the
// expected exponential sequence: 1s, 2s, 4s, 8s, 16s, 30s, 30s (capped at max).
func TestBackoffSequence(t *testing.T) {
	b := discord.NewBackoff(1*time.Second, 30*time.Second)

	expected := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		30 * time.Second,
		30 * time.Second,
	}

	for i, want := range expected {
		got := b.Next()
		if got != want {
			t.Errorf("Next() call %d = %v, want %v", i+1, got, want)
		}
	}
}

// TestBackoffReset verifies that after 3 Next() calls, calling Reset() makes
// the next Next() return the initial duration (1s) again.
func TestBackoffReset(t *testing.T) {
	b := discord.NewBackoff(1*time.Second, 30*time.Second)

	// Advance 3 times: 1s, 2s, 4s
	b.Next()
	b.Next()
	b.Next()

	b.Reset()

	got := b.Next()
	if got != 1*time.Second {
		t.Errorf("After Reset(), Next() = %v, want %v", got, 1*time.Second)
	}
}

// TestBackoffMax verifies that the backoff duration never exceeds the
// configured maximum, regardless of how many Next() calls are made.
func TestBackoffMax(t *testing.T) {
	b := discord.NewBackoff(1*time.Second, 30*time.Second)

	for i := 0; i < 10; i++ {
		got := b.Next()
		if got > 30*time.Second {
			t.Errorf("Next() call %d = %v, exceeds max %v", i+1, got, 30*time.Second)
		}
	}
}
