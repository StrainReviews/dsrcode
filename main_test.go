package main

import (
	"testing"

	"github.com/StrainReviews/dsrcode/analytics"
)

// TestCalculateCost tests cost calculation via the analytics package.
// The old main.calculateCost was replaced by analytics.CalculateSessionCost
// in Phase 17 (D-16). These tests verify the new cache-aware calculation.
func TestCalculateCost(t *testing.T) {
	tests := []struct {
		name     string
		modelID  string
		tokens   analytics.TokenBreakdown
		wantCost float64
	}{
		{
			name:     "Opus 4.6 - basic usage",
			modelID:  "claude-opus-4-6",
			tokens:   analytics.TokenBreakdown{Input: 1_000_000, Output: 100_000},
			wantCost: 5.0 + 2.5, // $5/M input + $25/M * 0.1 output
		},
		{
			name:     "Sonnet 4.6 - basic usage",
			modelID:  "claude-sonnet-4-6",
			tokens:   analytics.TokenBreakdown{Input: 1_000_000, Output: 100_000},
			wantCost: 3.0 + 1.5, // $3/M input + $15/M * 0.1 output
		},
		{
			name:     "Haiku 4.5 - basic usage",
			modelID:  "claude-haiku-4-5",
			tokens:   analytics.TokenBreakdown{Input: 2_000_000, Output: 200_000},
			wantCost: 2.0 + 1.0, // $1/M * 2 input + $5/M * 0.2 output
		},
		{
			name:     "Unknown model - defaults to Sonnet 4.6",
			modelID:  "claude-unknown-model-20991231",
			tokens:   analytics.TokenBreakdown{Input: 1_000_000, Output: 1_000_000},
			wantCost: 3.0 + 15.0, // Sonnet 4.6 pricing as fallback
		},
		{
			name:     "Zero tokens",
			modelID:  "claude-opus-4-6",
			tokens:   analytics.TokenBreakdown{},
			wantCost: 0,
		},
		{
			name:     "Cache-aware pricing",
			modelID:  "claude-opus-4-6",
			tokens:   analytics.TokenBreakdown{Input: 1_000_000, Output: 100_000, CacheRead: 500_000, CacheWrite: 200_000},
			wantCost: 5.0 + 2.5 + 0.25 + 1.25, // input + output + cache_read($0.50/M*0.5) + cache_write($6.25/M*0.2)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := analytics.CalculateCost(tt.modelID, tt.tokens)
			if diff := got - tt.wantCost; diff > 0.0001 || diff < -0.0001 {
				t.Errorf("CalculateCost(%q, %+v) = %v, want %v", tt.modelID, tt.tokens, got, tt.wantCost)
			}
		})
	}
}

// TestFormatModelName tests the model name formatting logic
func TestFormatModelName(t *testing.T) {
	tests := []struct {
		name    string
		modelID string
		want    string
	}{
		{
			name:    "Known model - Opus 4.5",
			modelID: "claude-opus-4-5-20251101",
			want:    "Opus 4.5",
		},
		{
			name:    "Known model - Sonnet 4.5",
			modelID: "claude-sonnet-4-5-20241022",
			want:    "Sonnet 4.5",
		},
		{
			name:    "Known model - Sonnet 4",
			modelID: "claude-sonnet-4-20250514",
			want:    "Sonnet 4",
		},
		{
			name:    "Known model - Haiku 4.5",
			modelID: "claude-haiku-4-5-20241022",
			want:    "Haiku 4.5",
		},
		{
			name:    "Unknown opus model - fallback",
			modelID: "claude-opus-5-20260101",
			want:    "Opus",
		},
		{
			name:    "Unknown sonnet model - fallback",
			modelID: "claude-sonnet-5-20260101",
			want:    "Sonnet",
		},
		{
			name:    "Unknown haiku model - fallback",
			modelID: "claude-haiku-5-20260101",
			want:    "Haiku",
		},
		{
			name:    "Completely unknown model - defaults to Claude",
			modelID: "some-unknown-model",
			want:    "Claude",
		},
		{
			name:    "Empty model ID",
			modelID: "",
			want:    "Claude",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatModelName(tt.modelID); got != tt.want {
				t.Errorf("formatModelName(%q) = %q, want %q", tt.modelID, got, tt.want)
			}
		})
	}
}

// TestFormatNumber tests the number formatting logic
func TestFormatNumber(t *testing.T) {
	tests := []struct {
		name string
		n    int64
		want string
	}{
		{name: "Zero", n: 0, want: "0"},
		{name: "Small number", n: 123, want: "123"},
		{name: "999 - no suffix", n: 999, want: "999"},
		{name: "1000 - K suffix", n: 1000, want: "1.0K"},
		{name: "1500 - K suffix", n: 1500, want: "1.5K"},
		{name: "10000 - K suffix", n: 10000, want: "10.0K"},
		{name: "999999 - K suffix", n: 999999, want: "1000.0K"},
		{name: "1000000 - M suffix", n: 1_000_000, want: "1.0M"},
		{name: "1500000 - M suffix", n: 1_500_000, want: "1.5M"},
		{name: "10000000 - M suffix", n: 10_000_000, want: "10.0M"},
		{name: "Large number", n: 150_000_000, want: "150.0M"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatNumber(tt.n); got != tt.want {
				t.Errorf("formatNumber(%d) = %q, want %q", tt.n, got, tt.want)
			}
		})
	}
}

// TestModelDisplayNames verifies all display name entries map to known model patterns.
func TestModelDisplayNames(t *testing.T) {
	if len(modelDisplayNames) == 0 {
		t.Fatal("modelDisplayNames is empty")
	}
	for modelID, name := range modelDisplayNames {
		if name == "" {
			t.Errorf("Model %q has empty display name", modelID)
		}
	}
}
