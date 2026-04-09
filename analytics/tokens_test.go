package analytics_test

import (
	"testing"

	"github.com/StrainReviews/dsrcode/analytics"
)

// TestTokenBreakdownTotal verifies Total() returns sum of Input+Output+CacheRead+CacheWrite.
func TestTokenBreakdownTotal(t *testing.T) {
	tests := []struct {
		name  string
		input analytics.TokenBreakdown
		want  int64
	}{
		{
			name: "all fields populated",
			input: analytics.TokenBreakdown{
				Input:      250_000,
				Output:     80_000,
				CacheRead:  156_000,
				CacheWrite: 14_000,
			},
			want: 500_000,
		},
		{
			name:  "zero values",
			input: analytics.TokenBreakdown{},
			want:  0,
		},
		{
			name: "only input and output",
			input: analytics.TokenBreakdown{
				Input:  100_000,
				Output: 50_000,
			},
			want: 150_000,
		},
		{
			name: "only cache fields",
			input: analytics.TokenBreakdown{
				CacheRead:  200_000,
				CacheWrite: 30_000,
			},
			want: 230_000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.input.Total()
			if got != tt.want {
				t.Errorf("Total() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestAggregateTokens verifies that multiple per-model TokenBreakdowns aggregate into a single total.
func TestAggregateTokens(t *testing.T) {
	tests := []struct {
		name   string
		models map[string]analytics.TokenBreakdown
		want   analytics.TokenBreakdown
	}{
		{
			name: "two models",
			models: map[string]analytics.TokenBreakdown{
				"claude-opus-4-6": {
					Input: 200_000, Output: 60_000, CacheRead: 100_000, CacheWrite: 10_000,
				},
				"claude-haiku-4-5": {
					Input: 50_000, Output: 20_000, CacheRead: 56_000, CacheWrite: 4_000,
				},
			},
			want: analytics.TokenBreakdown{
				Input: 250_000, Output: 80_000, CacheRead: 156_000, CacheWrite: 14_000,
			},
		},
		{
			name:   "empty map",
			models: map[string]analytics.TokenBreakdown{},
			want:   analytics.TokenBreakdown{},
		},
		{
			name: "single model",
			models: map[string]analytics.TokenBreakdown{
				"claude-sonnet-4-6": {Input: 100_000, Output: 40_000},
			},
			want: analytics.TokenBreakdown{Input: 100_000, Output: 40_000},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := analytics.AggregateTokens(tt.models)
			if got != tt.want {
				t.Errorf("AggregateTokens() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestFormatTokensMinimal verifies minimal display: 250000 -> "250K".
func TestFormatTokensMinimal(t *testing.T) {
	tests := []struct {
		name          string
		input         analytics.TokenBreakdown
		displayDetail string
		want          string
	}{
		{
			name: "250K total",
			input: analytics.TokenBreakdown{
				Input: 200_000, Output: 50_000,
			},
			displayDetail: "minimal",
			want:          "250K",
		},
		{
			name: "1.2M total",
			input: analytics.TokenBreakdown{
				Input: 800_000, Output: 400_000,
			},
			displayDetail: "minimal",
			want:          "1.2M",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := analytics.FormatTokens(tt.input, tt.displayDetail)
			if got != tt.want {
				t.Errorf("FormatTokens(%q) = %q, want %q", tt.displayDetail, got, tt.want)
			}
		})
	}
}

// TestFormatTokensStandard verifies standard display with unicode arrows:
// input=250000 output=80000 -> "250K\u2191 80K\u2193"
func TestFormatTokensStandard(t *testing.T) {
	tests := []struct {
		name          string
		input         analytics.TokenBreakdown
		displayDetail string
		want          string
	}{
		{
			name: "standard with arrows",
			input: analytics.TokenBreakdown{
				Input: 250_000, Output: 80_000,
			},
			displayDetail: "standard",
			want:          "250K\u2191 80K\u2193",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := analytics.FormatTokens(tt.input, tt.displayDetail)
			if got != tt.want {
				t.Errorf("FormatTokens(%q) = %q, want %q", tt.displayDetail, got, tt.want)
			}
		})
	}
}

// TestFormatTokensVerbose verifies verbose display adds cache:
// "250K\u2191 80K\u2193 156Kc"
func TestFormatTokensVerbose(t *testing.T) {
	tests := []struct {
		name          string
		input         analytics.TokenBreakdown
		displayDetail string
		want          string
	}{
		{
			name: "verbose with cache",
			input: analytics.TokenBreakdown{
				Input: 250_000, Output: 80_000, CacheRead: 156_000,
			},
			displayDetail: "verbose",
			want:          "250K\u2191 80K\u2193 156Kc",
		},
		{
			name: "verbose no cache",
			input: analytics.TokenBreakdown{
				Input: 250_000, Output: 80_000,
			},
			displayDetail: "verbose",
			want:          "250K\u2191 80K\u2193",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := analytics.FormatTokens(tt.input, tt.displayDetail)
			if got != tt.want {
				t.Errorf("FormatTokens(%q) = %q, want %q", tt.displayDetail, got, tt.want)
			}
		})
	}
}

// TestFormatTokensZero verifies zero tokens return empty string per D-04.
func TestFormatTokensZero(t *testing.T) {
	got := analytics.FormatTokens(analytics.TokenBreakdown{}, "minimal")
	if got != "" {
		t.Errorf("FormatTokens(zero) = %q, want empty string", got)
	}
}
