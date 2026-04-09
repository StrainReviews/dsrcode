package analytics_test

import (
	"testing"

	"github.com/StrainReviews/dsrcode/analytics"
)

// TestDetectCompaction verifies that isCompactSummary=true in a JSONL entry
// is correctly detected as a compaction event.
func TestDetectCompaction(t *testing.T) {
	tests := []struct {
		name    string
		jsonl   string
		want    bool
	}{
		{
			name:  "compact summary present",
			jsonl: `{"type":"assistant","isCompactSummary":true,"message":{"model":"claude-opus-4-6"}}`,
			want:  true,
		},
		{
			name:  "compact summary false",
			jsonl: `{"type":"assistant","isCompactSummary":false,"message":{"model":"claude-opus-4-6"}}`,
			want:  false,
		},
		{
			name:  "no compact summary field",
			jsonl: `{"type":"assistant","message":{"model":"claude-opus-4-6"}}`,
			want:  false,
		},
		{
			name:  "non-assistant type",
			jsonl: `{"type":"human","isCompactSummary":true}`,
			want:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := analytics.DetectCompaction([]byte(tt.jsonl))
			if got != tt.want {
				t.Errorf("DetectCompaction() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestBaselineAccumulation verifies that when new < old, old values accumulate
// to baseline per D-19 (compaction resets JSONL token counts).
func TestBaselineAccumulation(t *testing.T) {
	tests := []struct {
		name         string
		stored       analytics.TokenBreakdown
		current      analytics.TokenBreakdown
		wantBaseline analytics.TokenBreakdown
	}{
		{
			name: "compaction detected - new < old means baseline accumulates",
			stored: analytics.TokenBreakdown{
				Input: 250_000, Output: 80_000, CacheRead: 156_000, CacheWrite: 14_000,
			},
			current: analytics.TokenBreakdown{
				Input: 50_000, Output: 10_000, CacheRead: 30_000, CacheWrite: 2_000,
			},
			wantBaseline: analytics.TokenBreakdown{
				Input: 250_000, Output: 80_000, CacheRead: 156_000, CacheWrite: 14_000,
			},
		},
		{
			name: "no compaction - new >= old means no baseline change",
			stored: analytics.TokenBreakdown{
				Input: 100_000, Output: 40_000,
			},
			current: analytics.TokenBreakdown{
				Input: 150_000, Output: 60_000,
			},
			wantBaseline: analytics.TokenBreakdown{},
		},
		{
			name:   "zero stored - first data, no baseline",
			stored: analytics.TokenBreakdown{},
			current: analytics.TokenBreakdown{
				Input: 100_000, Output: 40_000,
			},
			wantBaseline: analytics.TokenBreakdown{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := analytics.ComputeBaseline(tt.stored, tt.current)
			if got != tt.wantBaseline {
				t.Errorf("ComputeBaseline() = %+v, want %+v", got, tt.wantBaseline)
			}
		})
	}
}

// TestEffectiveTotal verifies that effective total = current + baseline.
func TestEffectiveTotal(t *testing.T) {
	tests := []struct {
		name     string
		current  analytics.TokenBreakdown
		baseline analytics.TokenBreakdown
		want     analytics.TokenBreakdown
	}{
		{
			name: "current plus baseline",
			current: analytics.TokenBreakdown{
				Input: 50_000, Output: 10_000, CacheRead: 30_000, CacheWrite: 2_000,
			},
			baseline: analytics.TokenBreakdown{
				Input: 250_000, Output: 80_000, CacheRead: 156_000, CacheWrite: 14_000,
			},
			want: analytics.TokenBreakdown{
				Input: 300_000, Output: 90_000, CacheRead: 186_000, CacheWrite: 16_000,
			},
		},
		{
			name: "no baseline",
			current: analytics.TokenBreakdown{
				Input: 100_000, Output: 40_000,
			},
			baseline: analytics.TokenBreakdown{},
			want: analytics.TokenBreakdown{
				Input: 100_000, Output: 40_000,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := analytics.EffectiveTotal(tt.current, tt.baseline)
			if got != tt.want {
				t.Errorf("EffectiveTotal() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestCompactionDedup verifies that the same UUID is not counted twice.
func TestCompactionDedup(t *testing.T) {
	tests := []struct {
		name  string
		uuids []string
		want  int
	}{
		{
			name:  "unique UUIDs",
			uuids: []string{"uuid-1", "uuid-2", "uuid-3"},
			want:  3,
		},
		{
			name:  "duplicate UUIDs",
			uuids: []string{"uuid-1", "uuid-2", "uuid-1", "uuid-3", "uuid-2"},
			want:  3,
		},
		{
			name:  "all same UUID",
			uuids: []string{"uuid-1", "uuid-1", "uuid-1"},
			want:  1,
		},
		{
			name:  "empty",
			uuids: []string{},
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := analytics.NewCompactionTracker()
			for _, uuid := range tt.uuids {
				tracker.Record(uuid)
			}
			got := tracker.Count()
			if got != tt.want {
				t.Errorf("CompactionTracker.Count() = %d, want %d", got, tt.want)
			}
		})
	}
}
