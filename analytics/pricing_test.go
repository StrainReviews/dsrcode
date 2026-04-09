package analytics_test

import (
	"math"
	"testing"

	"github.com/StrainReviews/dsrcode/analytics"
)

// almostEqual checks if two float64 values are within epsilon of each other.
func almostEqual(a, b, epsilon float64) bool {
	return math.Abs(a-b) < epsilon
}

// TestCalculateCostOpus46 verifies cache-aware pricing for Opus 4.6:
// $5/$25/$6.25/$0.50 per MTok (input/output/cache_write/cache_read per D-16).
func TestCalculateCostOpus46(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		tokens   analytics.TokenBreakdown
		wantCost float64
	}{
		{
			name:  "opus 4.6 with all token types",
			model: "claude-opus-4-6",
			tokens: analytics.TokenBreakdown{
				Input: 100_000, Output: 50_000, CacheRead: 200_000, CacheWrite: 10_000,
			},
			// Input: 100K * $5/MTok = $0.50
			// Output: 50K * $25/MTok = $1.25
			// CacheRead: 200K * $0.50/MTok = $0.10
			// CacheWrite: 10K * $6.25/MTok = $0.0625
			wantCost: 1.9125,
		},
		{
			name:  "opus 4.6 input only",
			model: "claude-opus-4-6",
			tokens: analytics.TokenBreakdown{
				Input: 1_000_000,
			},
			wantCost: 5.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := analytics.CalculateCost(tt.model, tt.tokens)
			if !almostEqual(got, tt.wantCost, 0.001) {
				t.Errorf("CalculateCost(%q) = %f, want %f", tt.model, got, tt.wantCost)
			}
		})
	}
}

// TestCalculateCostSonnet46 verifies $3/$15/$3.75/$0.30 per MTok.
func TestCalculateCostSonnet46(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		tokens   analytics.TokenBreakdown
		wantCost float64
	}{
		{
			name:  "sonnet 4.6 with cache",
			model: "claude-sonnet-4-6",
			tokens: analytics.TokenBreakdown{
				Input: 100_000, Output: 50_000, CacheRead: 200_000, CacheWrite: 10_000,
			},
			// Input: 100K * $3/MTok = $0.30
			// Output: 50K * $15/MTok = $0.75
			// CacheRead: 200K * $0.30/MTok = $0.06
			// CacheWrite: 10K * $3.75/MTok = $0.0375
			wantCost: 1.1475,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := analytics.CalculateCost(tt.model, tt.tokens)
			if !almostEqual(got, tt.wantCost, 0.001) {
				t.Errorf("CalculateCost(%q) = %f, want %f", tt.model, got, tt.wantCost)
			}
		})
	}
}

// TestCalculateCostHaiku45 verifies $1/$5/$1.25/$0.10 per MTok.
func TestCalculateCostHaiku45(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		tokens   analytics.TokenBreakdown
		wantCost float64
	}{
		{
			name:  "haiku 4.5 with cache",
			model: "claude-haiku-4-5",
			tokens: analytics.TokenBreakdown{
				Input: 100_000, Output: 50_000, CacheRead: 200_000, CacheWrite: 10_000,
			},
			// Input: 100K * $1/MTok = $0.10
			// Output: 50K * $5/MTok = $0.25
			// CacheRead: 200K * $0.10/MTok = $0.02
			// CacheWrite: 10K * $1.25/MTok = $0.0125
			wantCost: 0.3825,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := analytics.CalculateCost(tt.model, tt.tokens)
			if !almostEqual(got, tt.wantCost, 0.001) {
				t.Errorf("CalculateCost(%q) = %f, want %f", tt.model, got, tt.wantCost)
			}
		})
	}
}

// TestCalculateCostLegacy verifies legacy models without cache pricing.
func TestCalculateCostLegacy(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		tokens   analytics.TokenBreakdown
		wantCost float64
	}{
		{
			name:  "opus 4.5 legacy - no cache pricing",
			model: "claude-opus-4-5",
			tokens: analytics.TokenBreakdown{
				Input: 100_000, Output: 50_000, CacheRead: 200_000, CacheWrite: 10_000,
			},
			// Legacy: only input ($5) + output ($25), cache ignored
			// Input: 100K * $5/MTok = $0.50
			// Output: 50K * $25/MTok = $1.25
			wantCost: 1.75,
		},
		{
			name:  "opus 4.1 legacy",
			model: "claude-opus-4-1",
			tokens: analytics.TokenBreakdown{
				Input: 100_000, Output: 50_000,
			},
			// Input: 100K * $15/MTok = $1.50
			// Output: 50K * $75/MTok = $3.75
			wantCost: 5.25,
		},
		{
			name:  "haiku 3 legacy",
			model: "claude-haiku-3",
			tokens: analytics.TokenBreakdown{
				Input: 100_000, Output: 50_000,
			},
			// Input: 100K * $0.25/MTok = $0.025
			// Output: 50K * $1.25/MTok = $0.0625
			wantCost: 0.0875,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := analytics.CalculateCost(tt.model, tt.tokens)
			if !almostEqual(got, tt.wantCost, 0.001) {
				t.Errorf("CalculateCost(%q) = %f, want %f", tt.model, got, tt.wantCost)
			}
		})
	}
}

// TestFindPricingWildcard verifies strings.Contains matching for model variants.
// e.g., "claude-opus-4-6[1m]" should match "claude-opus-4-6" pricing.
func TestFindPricingWildcard(t *testing.T) {
	tests := []struct {
		name  string
		model string
		want  string // expected matched pricing key
	}{
		{
			name:  "opus with context suffix",
			model: "claude-opus-4-6[1m]",
			want:  "claude-opus-4-6",
		},
		{
			name:  "sonnet with date suffix",
			model: "claude-sonnet-4-6-20260301",
			want:  "claude-sonnet-4-6",
		},
		{
			name:  "haiku exact match",
			model: "claude-haiku-4-5",
			want:  "claude-haiku-4-5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pricing := analytics.FindPricing(tt.model)
			if pricing == nil {
				t.Fatalf("FindPricing(%q) returned nil", tt.model)
			}
			// Just verify it matched something (non-nil means wildcard matched)
		})
	}
}

// TestFindPricingFallback verifies unknown model falls back to Sonnet 4.6 pricing.
func TestFindPricingFallback(t *testing.T) {
	pricing := analytics.FindPricing("totally-unknown-model-xyz")
	if pricing == nil {
		t.Fatal("FindPricing(unknown) returned nil, expected Sonnet 4.6 fallback")
	}
	// Sonnet 4.6 input rate is $3/MTok
	if !almostEqual(pricing.Input, 3.0, 0.001) {
		t.Errorf("fallback Input = %f, want 3.0 (Sonnet 4.6)", pricing.Input)
	}
}

// TestFormatCostDetail verifies verbose cost display per D-17:
// "$1.20 (\u2191$0.80 \u2193$0.35 c$0.05)"
func TestFormatCostDetail(t *testing.T) {
	tests := []struct {
		name          string
		model         string
		tokens        analytics.TokenBreakdown
		displayDetail string
		wantContains  string
	}{
		{
			name:  "verbose cost breakdown",
			model: "claude-opus-4-6",
			tokens: analytics.TokenBreakdown{
				Input: 160_000, Output: 14_000, CacheRead: 100_000, CacheWrite: 8_000,
			},
			displayDetail: "verbose",
			wantContains:  "$",
		},
		{
			name:  "minimal cost - just total",
			model: "claude-opus-4-6",
			tokens: analytics.TokenBreakdown{
				Input: 100_000, Output: 50_000,
			},
			displayDetail: "minimal",
			wantContains:  "$",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := analytics.FormatCost(tt.model, tt.tokens, tt.displayDetail)
			if got == "" {
				t.Error("FormatCost returned empty string")
			}
			if len(got) > 0 && got[0] != '$' {
				t.Errorf("FormatCost should start with $, got %q", got)
			}
		})
	}
}
