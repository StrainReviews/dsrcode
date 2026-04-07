package analytics

import (
	"fmt"
	"strings"
)

// ModelPricing holds per-MTok rates for a Claude model.
// Rates are in USD per million tokens.
type ModelPricing struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheWrite float64 `json:"cacheWrite"`
	CacheRead  float64 `json:"cacheRead"`
}

// modelPrices maps model name prefixes to their pricing per D-16.
// 6 models with 4 rates each (input/output/cache_write/cache_read per MTok).
var modelPrices = map[string]ModelPricing{
	"claude-opus-4-6":   {Input: 5.0, Output: 25.0, CacheWrite: 6.25, CacheRead: 0.50},
	"claude-sonnet-4-6": {Input: 3.0, Output: 15.0, CacheWrite: 3.75, CacheRead: 0.30},
	"claude-haiku-4-5":  {Input: 1.0, Output: 5.0, CacheWrite: 1.25, CacheRead: 0.10},
	"claude-opus-4-5":   {Input: 5.0, Output: 25.0, CacheWrite: 0, CacheRead: 0},
	"claude-opus-4-1":   {Input: 15.0, Output: 75.0, CacheWrite: 0, CacheRead: 0},
	"claude-haiku-3":    {Input: 0.25, Output: 1.25, CacheWrite: 0, CacheRead: 0},
}

// FindPricing returns the pricing for a model ID using strings.Contains
// wildcard matching. Falls back to Sonnet 4.6 pricing for unknown models.
// Returns a pointer to allow nil checks in tests.
func FindPricing(modelID string) *ModelPricing {
	for key, pricing := range modelPrices {
		if strings.Contains(modelID, key) {
			p := pricing
			return &p
		}
	}
	// Fallback to Sonnet 4.6
	fallback := modelPrices["claude-sonnet-4-6"]
	return &fallback
}

// CalculateCost computes the total cost in USD for a single model's token usage.
// Uses cache-aware 4-rate pricing per D-16.
func CalculateCost(modelID string, tokens TokenBreakdown) float64 {
	pricing := FindPricing(modelID)
	return pricing.Input*float64(tokens.Input)/1e6 +
		pricing.Output*float64(tokens.Output)/1e6 +
		pricing.CacheWrite*float64(tokens.CacheWrite)/1e6 +
		pricing.CacheRead*float64(tokens.CacheRead)/1e6
}

// CalculateSessionCost computes the total session cost across all models,
// accounting for baselines from compaction recovery.
func CalculateSessionCost(perModel, baselines map[string]TokenBreakdown) float64 {
	total := 0.0
	for model, tokens := range perModel {
		effective := tokens
		if baseline, ok := baselines[model]; ok {
			effective = EffectiveTotal(tokens, baseline)
		}
		total += CalculateCost(model, effective)
	}
	return total
}

// CostBreakdown holds per-type cost details for verbose display per D-17.
type CostBreakdown struct {
	InputCost      float64 `json:"inputCost"`
	OutputCost     float64 `json:"outputCost"`
	CacheWriteCost float64 `json:"cacheWriteCost"`
	CacheReadCost  float64 `json:"cacheReadCost"`
	Total          float64 `json:"total"`
}

// CalculateCostBreakdown computes a detailed cost breakdown across all models.
func CalculateCostBreakdown(perModel, baselines map[string]TokenBreakdown) CostBreakdown {
	var cb CostBreakdown
	for model, tokens := range perModel {
		effective := tokens
		if baseline, ok := baselines[model]; ok {
			effective = EffectiveTotal(tokens, baseline)
		}
		pricing := FindPricing(model)
		cb.InputCost += pricing.Input * float64(effective.Input) / 1e6
		cb.OutputCost += pricing.Output * float64(effective.Output) / 1e6
		cb.CacheWriteCost += pricing.CacheWrite * float64(effective.CacheWrite) / 1e6
		cb.CacheReadCost += pricing.CacheRead * float64(effective.CacheRead) / 1e6
	}
	cb.Total = cb.InputCost + cb.OutputCost + cb.CacheWriteCost + cb.CacheReadCost
	return cb
}

// FormatCost formats the cost for a single model's token usage.
//
// Levels per D-17:
//
//	minimal/standard: "$1.20"
//	verbose:          "$1.20 (↑$0.80 ↓$0.35 c$0.05)"
//	private:          ""
func FormatCost(modelID string, tokens TokenBreakdown, displayDetail string) string {
	if displayDetail == "private" {
		return ""
	}
	pricing := FindPricing(modelID)
	inputCost := pricing.Input * float64(tokens.Input) / 1e6
	outputCost := pricing.Output * float64(tokens.Output) / 1e6
	cacheWriteCost := pricing.CacheWrite * float64(tokens.CacheWrite) / 1e6
	cacheReadCost := pricing.CacheRead * float64(tokens.CacheRead) / 1e6
	total := inputCost + outputCost + cacheWriteCost + cacheReadCost

	if displayDetail == "verbose" {
		cacheCost := cacheWriteCost + cacheReadCost
		return fmt.Sprintf("$%.2f (\u2191$%.2f \u2193$%.2f c$%.2f)", total, inputCost, outputCost, cacheCost)
	}
	return fmt.Sprintf("$%.2f", total)
}

// FormatCostDetail formats a CostBreakdown for display based on displayDetail.
//
// Levels per D-17:
//
//	minimal/standard: "$1.20"
//	verbose:          "$1.20 (↑$0.80 ↓$0.35 c$0.05)"
//	private:          ""
func FormatCostDetail(cb CostBreakdown, displayDetail string) string {
	if displayDetail == "private" {
		return ""
	}
	if displayDetail == "verbose" {
		cacheCost := cb.CacheWriteCost + cb.CacheReadCost
		return fmt.Sprintf("$%.2f (\u2191$%.2f \u2193$%.2f c$%.2f)", cb.Total, cb.InputCost, cb.OutputCost, cacheCost)
	}
	return fmt.Sprintf("$%.2f", cb.Total)
}
