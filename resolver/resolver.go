// Package resolver converts session state into Discord Activity fields.
// It uses presets for message pools and StablePick for anti-flicker rotation.
package resolver

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tsanva/cc-discord-presence/analytics"
	"github.com/tsanva/cc-discord-presence/config"
	"github.com/tsanva/cc-discord-presence/discord"
	"github.com/tsanva/cc-discord-presence/preset"
	"github.com/tsanva/cc-discord-presence/session"
)

const (
	// maxFieldLength is the Discord API limit for Details/State fields.
	maxFieldLength = 128
	// largeImageKey is the fixed large image asset key per D-10.
	largeImageKey = "dsr-code"
)

// ResolvePresence converts current session state into a Discord Activity.
// Returns nil when there are no sessions (clears presence).
// Per D-08 to D-15 Layout D. The detail parameter controls data visibility.
// Per D-14: uses unique project count (not raw session count) for tier selection.
// Two windows on the same project = single-session display.
// Two windows on different projects = multi-session display.
func ResolvePresence(sessions []*session.Session, p *preset.MessagePreset, detail config.DisplayDetail, now time.Time) *discord.Activity {
	if len(sessions) == 0 {
		return nil
	}

	// Count unique projects for tier selection (D-14)
	uniqueProjects := make(map[string]struct{})
	for _, s := range sessions {
		uniqueProjects[s.ProjectName] = struct{}{}
	}

	if len(uniqueProjects) == 1 {
		// Single project: use most recent session for display (D-15)
		best := getMostRecentSession(sessions)
		return resolveSingle(best, p, detail, now)
	}
	return resolveMulti(sessions, p, detail, now)
}

// buildSinglePlaceholderValues creates the placeholder map for a single session,
// applying displayDetail level to control how much data is exposed.
func buildSinglePlaceholderValues(s *session.Session, detail config.DisplayDetail, duration time.Duration) map[string]string {
	values := map[string]string{
		"{activity}": s.SmallText,
		"{sessions}": "1",
		"{filepath}": s.LastFilePath,
		"{duration}": formatDuration(duration),
	}

	switch detail {
	case config.DetailMinimal:
		values["{project}"] = s.ProjectName
		values["{branch}"] = s.Branch
		values["{file}"] = s.ProjectName
		values["{command}"] = "..."
		values["{query}"] = "*"
		values["{model}"] = s.Model
		values["{tokens}"] = formatTokens(s.TotalTokens)
		values["{cost}"] = formatCost(s.TotalCostUSD)
	case config.DetailStandard:
		values["{project}"] = s.ProjectName
		values["{branch}"] = s.Branch
		values["{file}"] = filepath.Base(s.LastFilePath)
		if values["{file}"] == "" || values["{file}"] == "." {
			values["{file}"] = s.ProjectName
		}
		values["{command}"] = truncate(s.LastCommand, 20)
		if values["{command}"] == "" {
			values["{command}"] = "..."
		}
		values["{query}"] = s.LastQuery
		if values["{query}"] == "" {
			values["{query}"] = "*"
		}
		values["{model}"] = s.Model
		values["{tokens}"] = formatTokens(s.TotalTokens)
		values["{cost}"] = formatCost(s.TotalCostUSD)
	case config.DetailVerbose:
		values["{project}"] = s.ProjectName
		values["{branch}"] = s.Branch
		values["{file}"] = s.LastFilePath
		if values["{file}"] == "" {
			values["{file}"] = s.ProjectName
		}
		values["{command}"] = s.LastCommand
		if values["{command}"] == "" {
			values["{command}"] = "..."
		}
		values["{query}"] = s.LastQuery
		if values["{query}"] == "" {
			values["{query}"] = "*"
		}
		values["{model}"] = s.Model
		values["{tokens}"] = formatTokens(s.TotalTokens)
		values["{cost}"] = formatCost(s.TotalCostUSD)
	case config.DetailPrivate:
		values["{project}"] = "Project"
		values["{branch}"] = ""
		values["{file}"] = "file"
		values["{command}"] = "..."
		values["{query}"] = ""
		values["{model}"] = ""
		values["{tokens}"] = ""
		values["{cost}"] = ""
		values["{filepath}"] = ""
	}

	// Analytics placeholders (D-03 format, D-04 empty defaults).
	// Format functions handle displayDetail internally, so these are outside the switch.
	// Placeholders: {tokens_detail}, {top_tools}, {subagents}, {context_pct},
	//   {compactions}, {cost_detail}
	detailStr := string(detail)
	values["{tokens_detail}"] = analytics.FormatTokens(
		analytics.AggregateTokens(decodeTokenMap(s.TokenBreakdownRaw)), detailStr)
	values["{top_tools}"] = analytics.FormatTools(
		toolCounterFromMap(s.ToolCounts), detailStr)
	values["{subagents}"] = analytics.FormatSubagents(
		subagentTreeFromRaw(s.SubagentTreeRaw), detailStr)
	values["{context_pct}"] = analytics.FormatContextPct(s.ContextUsagePct, detailStr)
	values["{compactions}"] = formatCompactions(s.CompactionCount, detail)
	values["{cost_detail}"] = analytics.FormatCostDetail(
		decodeSessionCostBreakdown(s), detailStr)

	return values
}

// resolveSingle builds a Layout D activity for a single active session.
func resolveSingle(s *session.Session, p *preset.MessagePreset, detail config.DisplayDetail, now time.Time) *discord.Activity {
	seed := HashString(s.SessionID)
	duration := now.Sub(s.StartedAt)
	values := buildSinglePlaceholderValues(s, detail, duration)

	// Pick detail message from preset pool for this activity icon
	pool := p.SingleSessionDetails[s.SmallImageKey]
	if len(pool) == 0 {
		pool = p.SingleSessionDetailsFallback
	}
	details := StablePick(pool, seed, now)
	details = replacePlaceholders(details, values)

	// Pick state message from preset pool (use seed+1 for independent rotation)
	state := StablePick(p.SingleSessionState, seed+1, now)
	state = replacePlaceholders(state, values)

	startTime := s.StartedAt
	largeText := s.ProjectName + " (" + s.Branch + ")"
	if detail == config.DetailPrivate {
		largeText = "Project"
	}

	return &discord.Activity{
		Details:    truncate(details, maxFieldLength),
		State:      truncate(state, maxFieldLength),
		LargeImage: largeImageKey,
		LargeText:  largeText,
		SmallImage: s.SmallImageKey,
		SmallText:  s.SmallText,
		StartTime:  &startTime,
		Buttons:    convertButtons(p.Buttons),
	}
}

// buildMultiPlaceholderValues creates the placeholder map for multiple sessions,
// aggregating projects, models, costs, and tokens across all sessions.
func buildMultiPlaceholderValues(sessions []*session.Session, detail config.DisplayDetail, duration time.Duration) map[string]string {
	n := len(sessions)

	// Aggregate totals
	var totalTokens int64
	var totalCost float64
	projectCounts := make(map[string]int)
	modelSet := make(map[string]bool)
	for _, s := range sessions {
		totalTokens += s.TotalTokens
		totalCost += s.TotalCostUSD
		projectCounts[s.ProjectName]++
		if s.Model != "" {
			modelSet[s.Model] = true
		}
	}

	// Build {projects}: "2x SRS, ApiServer" format per D-26
	// Use sorted keys for deterministic output
	sortedProjects := make([]string, 0, len(projectCounts))
	for name := range projectCounts {
		sortedProjects = append(sortedProjects, name)
	}
	sort.Strings(sortedProjects)

	projectParts := make([]string, 0, len(sortedProjects))
	for _, name := range sortedProjects {
		count := projectCounts[name]
		if count > 1 {
			projectParts = append(projectParts, fmt.Sprintf("%dx %s", count, name))
		} else {
			projectParts = append(projectParts, name)
		}
	}
	projects := strings.Join(projectParts, ", ")

	// Build {models}: unique model names (sorted for determinism)
	sortedModels := make([]string, 0, len(modelSet))
	for m := range modelSet {
		sortedModels = append(sortedModels, m)
	}
	sort.Strings(sortedModels)
	models := strings.Join(sortedModels, ", ")

	// Aggregate activity counts
	var totalCounts session.ActivityCounts
	for _, s := range sessions {
		totalCounts.Edits += s.ActivityCounts.Edits
		totalCounts.Commands += s.ActivityCounts.Commands
		totalCounts.Searches += s.ActivityCounts.Searches
		totalCounts.Reads += s.ActivityCounts.Reads
		totalCounts.Thinks += s.ActivityCounts.Thinks
	}
	statsLine := FormatStatsLine(totalCounts, duration)
	mode := DetectDominantMode(totalCounts)

	values := map[string]string{
		"{n}":        strconv.Itoa(n),
		"{sessions}": strconv.Itoa(n),
		"{stats}":    statsLine,
		"{mode}":     mode,
		"{duration}": formatDuration(duration),
	}

	switch detail {
	case config.DetailPrivate:
		values["{projects}"] = "Projects"
		values["{models}"] = ""
		values["{totalCost}"] = ""
		values["{totalTokens}"] = ""
	default:
		values["{projects}"] = projects
		values["{models}"] = models
		values["{totalCost}"] = formatCost(totalCost)
		values["{totalTokens}"] = formatTokens(totalTokens)
	}

	// Multi-session analytics aggregation per D-22.
	// {totalTokensDetail} and {cost_detail}: aggregate across ALL sessions.
	// Other analytics ({top_tools}, {subagents}, {context_pct}, {compactions}):
	// from the most active session only.
	detailStr := string(detail)
	allTokens := make(map[string]analytics.TokenBreakdown)
	allBaselines := make(map[string]analytics.TokenBreakdown)
	for _, s := range sessions {
		for model, bd := range decodeTokenMap(s.TokenBreakdownRaw) {
			existing := allTokens[model]
			allTokens[model] = analytics.TokenBreakdown{
				Input:      existing.Input + bd.Input,
				Output:     existing.Output + bd.Output,
				CacheRead:  existing.CacheRead + bd.CacheRead,
				CacheWrite: existing.CacheWrite + bd.CacheWrite,
			}
		}
		for model, bd := range decodeTokenMap(s.TokenBaselinesRaw) {
			existing := allBaselines[model]
			allBaselines[model] = analytics.TokenBreakdown{
				Input:      existing.Input + bd.Input,
				Output:     existing.Output + bd.Output,
				CacheRead:  existing.CacheRead + bd.CacheRead,
				CacheWrite: existing.CacheWrite + bd.CacheWrite,
			}
		}
	}
	values["{totalTokensDetail}"] = analytics.FormatTokens(
		analytics.AggregateTokens(allTokens), detailStr)

	// Recalculate {totalCost} using cache-aware pricing per D-17
	if detail != config.DetailPrivate {
		cacheAwareCost := analytics.CalculateSessionCost(allTokens, allBaselines)
		if cacheAwareCost > 0 {
			values["{totalCost}"] = formatCost(cacheAwareCost)
		}
	}

	// Aggregated cost breakdown for {cost_detail}
	aggCB := analytics.CalculateCostBreakdown(allTokens, allBaselines)
	values["{cost_detail}"] = analytics.FormatCostDetail(aggCB, detailStr)

	// Other analytics from most active session per D-22
	best := getMostRecentSession(sessions)
	if best != nil {
		values["{top_tools}"] = analytics.FormatTools(
			toolCounterFromMap(best.ToolCounts), detailStr)
		values["{subagents}"] = analytics.FormatSubagents(
			subagentTreeFromRaw(best.SubagentTreeRaw), detailStr)
		values["{context_pct}"] = analytics.FormatContextPct(best.ContextUsagePct, detailStr)
		values["{compactions}"] = formatCompactions(best.CompactionCount, detail)
	} else {
		values["{top_tools}"] = ""
		values["{subagents}"] = ""
		values["{context_pct}"] = ""
		values["{compactions}"] = ""
	}

	return values
}

// resolveMulti builds a Layout D activity for multiple concurrent sessions.
// Per D-27: tier-based messages for 2, 3, 4, and 5+ sessions.
// Per D-14: uses unique project count for tier key and {n} placeholder.
func resolveMulti(sessions []*session.Session, p *preset.MessagePreset, detail config.DisplayDetail, now time.Time) *discord.Activity {
	// Compute unique projects for tier selection (D-14)
	projectSet := make(map[string]bool)
	for _, s := range sessions {
		projectSet[s.ProjectName] = true
	}
	uniqueCount := len(projectSet)

	// Find earliest session for seed and start time
	earliest := sessions[0]
	for _, s := range sessions[1:] {
		if s.StartedAt.Before(earliest.StartedAt) {
			earliest = s
		}
	}
	seed := HashString(earliest.SessionID)

	// Tier key uses unique project count per D-14
	var pool []string
	tierKey := strconv.Itoa(uniqueCount)
	if uniqueCount <= 4 {
		pool = p.MultiSessionMessages[tierKey]
	}
	if len(pool) == 0 {
		pool = p.MultiSessionOverflow
	}

	duration := now.Sub(earliest.StartedAt)
	values := buildMultiPlaceholderValues(sessions, detail, duration)
	// Override {n} with unique project count for "across {n} projects" messages (D-14)
	values["{n}"] = strconv.Itoa(uniqueCount)
	// {sessions} stays as raw session count (already set by buildMultiPlaceholderValues)

	details := StablePick(pool, seed, now)
	details = replacePlaceholders(details, values)

	// Multi-session state: use aggregated stats line as state
	state := values["{stats}"]
	if state == "" {
		state = "Just getting started"
	}

	// Find most recent active session for icon
	mostRecent := getMostRecentSession(sessions)
	smallImageKey := mostRecent.SmallImageKey
	smallText := StablePick(p.MultiSessionTooltips, seed+1, now)
	if smallText == "" {
		smallText = mostRecent.SmallText
	}

	largeText := fmt.Sprintf("%d projects active", uniqueCount)
	if detail == config.DetailPrivate {
		largeText = "Projects"
	}

	startTime := earliest.StartedAt
	return &discord.Activity{
		Details:    truncate(details, maxFieldLength),
		State:      truncate(state, maxFieldLength),
		LargeImage: largeImageKey,
		LargeText:  largeText,
		SmallImage: smallImageKey,
		SmallText:  smallText,
		StartTime:  &startTime,
		Buttons:    convertButtons(p.Buttons),
	}
}

// getMostRecentSession returns the session with the most recent activity,
// preferring active sessions over idle ones.
func getMostRecentSession(sessions []*session.Session) *session.Session {
	if len(sessions) == 0 {
		return nil
	}
	best := sessions[0]
	for _, s := range sessions[1:] {
		if best.Status == session.StatusIdle && s.Status == session.StatusActive {
			best = s
			continue
		}
		if best.Status == session.StatusActive && s.Status == session.StatusIdle {
			continue
		}
		if s.LastActivityAt.After(best.LastActivityAt) {
			best = s
		}
	}
	return best
}

// convertButtons converts preset buttons to discord buttons.
func convertButtons(presetButtons []preset.Button) []discord.Button {
	if len(presetButtons) == 0 {
		return nil
	}
	buttons := make([]discord.Button, 0, len(presetButtons))
	for _, b := range presetButtons {
		buttons = append(buttons, discord.Button{
			Label: b.Label,
			URL:   b.URL,
		})
	}
	return buttons
}

// formatTokens formats a token count for display.
// 1500000 -> "1.5M", 150000 -> "150K", 1500 -> "1.5K", 500 -> "500"
func formatTokens(n int64) string {
	switch {
	case n >= 1_000_000:
		val := float64(n) / 1_000_000
		if val == float64(int64(val)) {
			return fmt.Sprintf("%dM", int64(val))
		}
		return fmt.Sprintf("%.1fM", val)
	case n >= 1_000:
		val := float64(n) / 1_000
		if val == float64(int64(val)) {
			return fmt.Sprintf("%dK", int64(val))
		}
		return fmt.Sprintf("%.1fK", val)
	default:
		return strconv.FormatInt(n, 10)
	}
}

// formatCost formats a USD cost with dollar sign. 0.12 -> "$0.12", 1.5 -> "$1.50"
func formatCost(c float64) string {
	return fmt.Sprintf("$%.2f", c)
}

// formatDuration formats a duration for display. 75m -> "1h 15m", 5m -> "5m"
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if hours > 0 {
		if mins > 0 {
			return fmt.Sprintf("%dh %dm", hours, mins)
		}
		return fmt.Sprintf("%dh", hours)
	}
	if mins > 0 {
		return fmt.Sprintf("%dm", mins)
	}
	return "<1m"
}

// truncate shortens s to max characters, appending ellipsis if truncated.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "\u2026"
}

// replacePlaceholders replaces all {key} placeholders in a template string.
// D-04: trims double spaces and trailing dots from resolved strings to handle
// empty placeholders leaving gaps like "Using  on " or "Stats: . ".
func replacePlaceholders(template string, values map[string]string) string {
	result := template
	for k, v := range values {
		result = strings.ReplaceAll(result, k, v)
	}
	// Collapse multiple spaces into single space
	result = strings.Join(strings.Fields(result), " ")
	// Trim trailing dots and spaces
	result = strings.TrimRight(result, ". ")
	return result
}

// formatCompactions formats compaction count for display based on displayDetail.
//
// Levels per D-03:
//
//	minimal:  "2"
//	standard: "2 compactions"
//	verbose:  "2x compact"
//	private:  ""
//
// Zero count returns "" per D-04.
func formatCompactions(count int, detail config.DisplayDetail) string {
	if count == 0 || detail == config.DetailPrivate {
		return ""
	}
	switch detail {
	case config.DetailMinimal:
		return strconv.Itoa(count)
	case config.DetailStandard:
		return fmt.Sprintf("%d %s", count, compactionPlural(count))
	case config.DetailVerbose:
		return fmt.Sprintf("%dx compact", count)
	}
	return ""
}

// compactionPlural returns "compaction" for 1, "compactions" otherwise.
func compactionPlural(n int) string {
	if n == 1 {
		return "compaction"
	}
	return "compactions"
}

// decodeTokenMap unmarshals a json.RawMessage into map[string]analytics.TokenBreakdown.
// Returns nil for nil or empty input per D-04.
func decodeTokenMap(raw json.RawMessage) map[string]analytics.TokenBreakdown {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]analytics.TokenBreakdown
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	return m
}

// toolCounterFromMap creates an analytics.ToolCounter from a map[string]int.
// Returns an empty counter for nil input.
func toolCounterFromMap(counts map[string]int) *analytics.ToolCounter {
	tc := analytics.NewToolCounter()
	for name, count := range counts {
		for i := 0; i < count; i++ {
			tc.Record(name)
		}
	}
	return tc
}

// subagentTreeFromRaw unmarshals a json.RawMessage into a SubagentTree.
// Returns an empty tree for nil or empty input per D-04.
func subagentTreeFromRaw(raw json.RawMessage) *analytics.SubagentTree {
	tree := analytics.NewSubagentTree()
	if len(raw) == 0 {
		return tree
	}
	var entries []analytics.SubagentEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return tree
	}
	for _, e := range entries {
		tree.Spawn(e)
	}
	return tree
}

// decodeSessionCostBreakdown computes a CostBreakdown from a session's token data.
// Uses cache-aware pricing with baselines per D-17.
func decodeSessionCostBreakdown(s *session.Session) analytics.CostBreakdown {
	tokens := decodeTokenMap(s.TokenBreakdownRaw)
	baselines := decodeTokenMap(s.TokenBaselinesRaw)
	if len(tokens) == 0 {
		return analytics.CostBreakdown{}
	}
	return analytics.CalculateCostBreakdown(tokens, baselines)
}
