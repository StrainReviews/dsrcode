package analytics_test

import (
	"testing"

	"github.com/StrainReviews/dsrcode/analytics"
)

// float64Ptr is a helper to create *float64 from a literal.
func float64Ptr(f float64) *float64 {
	return &f
}

// TestParseContextPct verifies parsing "78.5" -> *float64 = 78.5.
func TestParseContextPct(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  *float64
	}{
		{
			name:  "normal percentage",
			input: "78.5",
			want:  float64Ptr(78.5),
		},
		{
			name:  "zero",
			input: "0",
			want:  float64Ptr(0),
		},
		{
			name:  "full context",
			input: "100",
			want:  float64Ptr(100),
		},
		{
			name:  "high precision",
			input: "45.678",
			want:  float64Ptr(45.678),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := analytics.ParseContextPct(tt.input)
			if got == nil {
				t.Fatal("ParseContextPct returned nil")
			}
			if *got != *tt.want {
				t.Errorf("ParseContextPct(%q) = %f, want %f", tt.input, *got, *tt.want)
			}
		})
	}
}

// TestParseContextPctNull verifies null/empty value -> nil (D-04: empty placeholder).
func TestParseContextPctNull(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{name: "empty string", input: ""},
		{name: "null string", input: "null"},
		{name: "invalid string", input: "not-a-number"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := analytics.ParseContextPct(tt.input)
			if got != nil {
				t.Errorf("ParseContextPct(%q) = %f, want nil", tt.input, *got)
			}
		})
	}
}

// TestFormatContextPctMinimal verifies minimal display: 78.5 -> "78%".
func TestFormatContextPctMinimal(t *testing.T) {
	tests := []struct {
		name          string
		pct           *float64
		displayDetail string
		want          string
	}{
		{
			name:          "normal percentage",
			pct:           float64Ptr(78.5),
			displayDetail: "minimal",
			want:          "78%",
		},
		{
			name:          "round number",
			pct:           float64Ptr(50.0),
			displayDetail: "minimal",
			want:          "50%",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := analytics.FormatContextPct(tt.pct, tt.displayDetail)
			if got != tt.want {
				t.Errorf("FormatContextPct(%q) = %q, want %q", tt.displayDetail, got, tt.want)
			}
		})
	}
}

// TestFormatContextPctStandard verifies standard display: 78.5 -> "78% context".
func TestFormatContextPctStandard(t *testing.T) {
	got := analytics.FormatContextPct(float64Ptr(78.5), "standard")
	want := "78% context"
	if got != want {
		t.Errorf("FormatContextPct(standard) = %q, want %q", got, want)
	}
}

// TestFormatContextPctVerbose verifies verbose display: 78.5 -> "78% (156K/200K)"
// (needs total tokens context for the calculation).
func TestFormatContextPctVerbose(t *testing.T) {
	// Verbose format with token context
	got := analytics.FormatContextPctVerbose(float64Ptr(78.5), 156_000, 200_000)
	want := "78% (156K/200K)"
	if got != want {
		t.Errorf("FormatContextPctVerbose() = %q, want %q", got, want)
	}
}

// TestFormatContextPctNil verifies nil -> "" (D-04).
func TestFormatContextPctNil(t *testing.T) {
	tests := []struct {
		name          string
		displayDetail string
	}{
		{name: "minimal", displayDetail: "minimal"},
		{name: "standard", displayDetail: "standard"},
		{name: "verbose", displayDetail: "verbose"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := analytics.FormatContextPct(nil, tt.displayDetail)
			if got != "" {
				t.Errorf("FormatContextPct(nil, %q) = %q, want empty string", tt.displayDetail, got)
			}
		})
	}
}
