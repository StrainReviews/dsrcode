package discord

import "time"

// Backoff implements exponential backoff with a configurable min and max duration.
// Per D-42: default 1s min, 30s max, doubles each call to Next().
type Backoff struct {
	min     time.Duration
	max     time.Duration
	current time.Duration
}

// NewBackoff creates a new Backoff starting at min, capping at max.
func NewBackoff(min, max time.Duration) *Backoff {
	return &Backoff{
		min:     min,
		max:     max,
		current: min,
	}
}

// Next returns the current backoff duration and advances to the next value.
// The sequence doubles: 1s, 2s, 4s, 8s, 16s, 30s, 30s, 30s...
func (b *Backoff) Next() time.Duration {
	d := b.current
	b.current *= 2
	if b.current > b.max {
		b.current = b.max
	}
	return d
}

// Reset resets the backoff to its initial min value.
// Call after a successful connection per D-44.
func (b *Backoff) Reset() {
	b.current = b.min
}

// DefaultBackoff returns a Backoff configured per D-42 defaults (1s min, 30s max).
func DefaultBackoff() *Backoff {
	return NewBackoff(1*time.Second, 30*time.Second)
}
