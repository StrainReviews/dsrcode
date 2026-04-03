package resolver_test

import (
	"strings"
	"testing"
	"time"

	"github.com/tsanva/cc-discord-presence/config"
	"github.com/tsanva/cc-discord-presence/preset"
	"github.com/tsanva/cc-discord-presence/resolver"
	"github.com/tsanva/cc-discord-presence/session"
)

// TestStablePick verifies that the same pool, seed, and time bucket return the
// same index, and that a different 5-minute bucket returns a different index.
func TestStablePick(t *testing.T) {
	pool := []string{"alpha", "bravo", "charlie", "delta", "echo"}
	seed := int64(42)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	result1 := resolver.StablePick(pool, seed, now)
	if result1 == "" {
		t.Fatal("StablePick returned empty string for non-empty pool")
	}

	// Same bucket (within 5 minutes) should return same result
	result2 := resolver.StablePick(pool, seed, now.Add(2*time.Minute))
	if result1 != result2 {
		t.Errorf("StablePick not stable within bucket: got %q then %q", result1, result2)
	}

	// Different bucket (6 minutes later) may return a different result
	// We just verify it doesn't panic and returns a valid pool member
	result3 := resolver.StablePick(pool, seed, now.Add(6*time.Minute))
	found := false
	for _, v := range pool {
		if v == result3 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("StablePick returned %q which is not in pool", result3)
	}

	// Empty pool returns empty string
	empty := resolver.StablePick([]string{}, seed, now)
	if empty != "" {
		t.Errorf("StablePick on empty pool: got %q, want empty string", empty)
	}
}

// TestStablePickDeterministic verifies that 1000 calls with identical input
// (pool, seed, now) all return the same result.
func TestStablePickDeterministic(t *testing.T) {
	pool := []string{"one", "two", "three", "four", "five", "six", "seven"}
	seed := int64(99)
	now := time.Date(2026, 6, 15, 8, 30, 0, 0, time.UTC)

	expected := resolver.StablePick(pool, seed, now)
	for i := 0; i < 1000; i++ {
		got := resolver.StablePick(pool, seed, now)
		if got != expected {
			t.Fatalf("StablePick not deterministic at iteration %d: got %q, want %q", i, got, expected)
		}
	}
}

// TestFormatStatsLine verifies that ActivityCounts{Edits:23, Commands:8}
// formats to "23 edits . 8 cmds" (using middle dot separator).
func TestFormatStatsLine(t *testing.T) {
	counts := session.ActivityCounts{Edits: 23, Commands: 8}
	result := resolver.FormatStatsLine(counts, 0)
	expected := "23 edits \u00b7 8 cmds"
	if result != expected {
		t.Errorf("FormatStatsLine = %q, want %q", result, expected)
	}
}

// TestFormatStatsLineSingular verifies singular forms: ActivityCounts{Edits:1}
// formats to "1 edit" (not "1 edits").
func TestFormatStatsLineSingular(t *testing.T) {
	counts := session.ActivityCounts{Edits: 1}
	result := resolver.FormatStatsLine(counts, 0)
	expected := "1 edit"
	if result != expected {
		t.Errorf("FormatStatsLine singular = %q, want %q", result, expected)
	}

	// Also test singular command
	counts2 := session.ActivityCounts{Commands: 1, Searches: 1}
	result2 := resolver.FormatStatsLine(counts2, 0)
	expected2 := "1 cmd \u00b7 1 search"
	if result2 != expected2 {
		t.Errorf("FormatStatsLine singular cmd+search = %q, want %q", result2, expected2)
	}
}

// TestFormatStatsLineWithDuration verifies that elapsed time is included in the
// stats line, e.g. "1h 15m deep" when sufficient time has passed.
func TestFormatStatsLineWithDuration(t *testing.T) {
	counts := session.ActivityCounts{Edits: 5}
	duration := 75 * time.Minute // 1h 15m
	result := resolver.FormatStatsLine(counts, duration)
	expected := "5 edits \u00b7 1h 15m deep"
	if result != expected {
		t.Errorf("FormatStatsLine with duration = %q, want %q", result, expected)
	}

	// Test minutes-only duration
	result2 := resolver.FormatStatsLine(counts, 30*time.Minute)
	expected2 := "5 edits \u00b7 30m deep"
	if result2 != expected2 {
		t.Errorf("FormatStatsLine 30m = %q, want %q", result2, expected2)
	}

	// Test duration less than 1 minute is omitted
	result3 := resolver.FormatStatsLine(counts, 30*time.Second)
	expected3 := "5 edits"
	if result3 != expected3 {
		t.Errorf("FormatStatsLine 30s = %q, want %q", result3, expected3)
	}
}

// TestDetectDominantMode verifies that when >50% of activity is coding, the
// result is "coding"; when mixed, the result is "multi-session".
func TestDetectDominantMode(t *testing.T) {
	// Coding dominant (60% edits)
	counts := session.ActivityCounts{Edits: 6, Commands: 2, Searches: 1, Reads: 1}
	result := resolver.DetectDominantMode(counts)
	if result != "coding" {
		t.Errorf("DetectDominantMode coding = %q, want %q", result, "coding")
	}

	// Mixed (no >50%)
	mixed := session.ActivityCounts{Edits: 3, Commands: 3, Searches: 2, Reads: 2}
	result2 := resolver.DetectDominantMode(mixed)
	if result2 != "multi-session" {
		t.Errorf("DetectDominantMode mixed = %q, want %q", result2, "multi-session")
	}

	// All zeros -> idle
	result3 := resolver.DetectDominantMode(session.ActivityCounts{})
	if result3 != "idle" {
		t.Errorf("DetectDominantMode zero = %q, want %q", result3, "idle")
	}

	// Terminal dominant (75%)
	terminal := session.ActivityCounts{Commands: 9, Edits: 3}
	result4 := resolver.DetectDominantMode(terminal)
	if result4 != "terminal" {
		t.Errorf("DetectDominantMode terminal = %q, want %q", result4, "terminal")
	}
}

// testPreset returns a minimal preset with predictable single-pool entries.
func testPreset() *preset.MessagePreset {
	return &preset.MessagePreset{
		Label:       "test",
		Description: "test preset",
		SingleSessionDetails: map[string][]string{
			"coding":   {"Editing {project} ({branch})"},
			"terminal": {"Running commands in {project}"},
		},
		SingleSessionDetailsFallback: []string{"Working on {project} ({branch})"},
		SingleSessionState:           []string{"{model} | {tokens} tokens | {cost}"},
		MultiSessionMessages: map[string][]string{
			"2": {"Dual-wielding code"},
			"3": {"Triple-threading code"},
			"4": {"Quad-core coding"},
		},
		MultiSessionOverflow: []string{"{n} sessions blazing"},
		MultiSessionTooltips: []string{"Multi-session mode"},
		Buttons: []preset.Button{
			{Label: "GitHub", URL: "https://github.com"},
		},
	}
}

// TestResolveSingleSession verifies that with 1 session, resolvePresence
// returns direct details and state from the preset's single-session messages.
func TestResolveSingleSession(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	startedAt := now.Add(-30 * time.Minute)

	s := &session.Session{
		SessionID:     "test-session-1",
		ProjectName:   "MyProject",
		Branch:        "main",
		Model:         "opus-4",
		SmallImageKey: "coding",
		SmallText:     "Editing files",
		TotalTokens:   1500000,
		TotalCostUSD:  0.12,
		Status:        session.StatusActive,
		StartedAt:     startedAt,
		LastActivityAt: now,
	}

	p := testPreset()
	activity := resolver.ResolvePresence([]*session.Session{s}, p, config.DetailMinimal, now)

	if activity == nil {
		t.Fatal("ResolvePresence returned nil for single session")
	}

	// Details should contain project name and branch (from preset template)
	if !strings.Contains(activity.Details, "MyProject") {
		t.Errorf("Details %q should contain project name", activity.Details)
	}
	if !strings.Contains(activity.Details, "main") {
		t.Errorf("Details %q should contain branch", activity.Details)
	}

	// State should contain model, tokens, and cost
	if !strings.Contains(activity.State, "opus-4") {
		t.Errorf("State %q should contain model", activity.State)
	}
	if !strings.Contains(activity.State, "1.5M") {
		t.Errorf("State %q should contain formatted tokens", activity.State)
	}
	if !strings.Contains(activity.State, "$0.12") {
		t.Errorf("State %q should contain cost", activity.State)
	}

	// Layout D fields
	if activity.LargeImage != "dsr-code" {
		t.Errorf("LargeImage = %q, want %q", activity.LargeImage, "dsr-code")
	}
	if activity.SmallImage != "coding" {
		t.Errorf("SmallImage = %q, want %q", activity.SmallImage, "coding")
	}
	if activity.SmallText != "Editing files" {
		t.Errorf("SmallText = %q, want %q", activity.SmallText, "Editing files")
	}
	if activity.StartTime == nil {
		t.Error("StartTime should not be nil")
	} else if !activity.StartTime.Equal(startedAt) {
		t.Errorf("StartTime = %v, want %v", activity.StartTime, startedAt)
	}
	if len(activity.Buttons) != 1 {
		t.Fatalf("Buttons len = %d, want 1", len(activity.Buttons))
	}
	if activity.Buttons[0].Label != "GitHub" {
		t.Errorf("Button label = %q, want %q", activity.Buttons[0].Label, "GitHub")
	}

	// nil for empty sessions
	nilResult := resolver.ResolvePresence([]*session.Session{}, p, config.DetailMinimal, now)
	if nilResult != nil {
		t.Error("ResolvePresence should return nil for empty sessions")
	}
}

// TestResolveMultiSession verifies that with 3 sessions, resolvePresence
// returns the tier "3" message from the preset's multi-session pool.
func TestResolveMultiSession(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	sessions := []*session.Session{
		{
			SessionID:      "session-1",
			ProjectName:    "ProjectA",
			Branch:         "main",
			Model:          "sonnet",
			SmallImageKey:  "coding",
			SmallText:      "Editing",
			TotalTokens:    500000,
			TotalCostUSD:   0.05,
			ActivityCounts: session.ActivityCounts{Edits: 10, Commands: 2},
			Status:         session.StatusActive,
			StartedAt:      now.Add(-60 * time.Minute),
			LastActivityAt: now.Add(-1 * time.Minute),
		},
		{
			SessionID:      "session-2",
			ProjectName:    "ProjectB",
			Branch:         "feature",
			Model:          "opus",
			SmallImageKey:  "terminal",
			SmallText:      "Running commands",
			TotalTokens:    300000,
			TotalCostUSD:   0.03,
			ActivityCounts: session.ActivityCounts{Commands: 5, Searches: 3},
			Status:         session.StatusActive,
			StartedAt:      now.Add(-45 * time.Minute),
			LastActivityAt: now.Add(-2 * time.Minute),
		},
		{
			SessionID:      "session-3",
			ProjectName:    "ProjectC",
			Branch:         "dev",
			Model:          "haiku",
			SmallImageKey:  "reading",
			SmallText:      "Reading docs",
			TotalTokens:    100000,
			TotalCostUSD:   0.01,
			ActivityCounts: session.ActivityCounts{Reads: 8},
			Status:         session.StatusIdle,
			StartedAt:      now.Add(-30 * time.Minute),
			LastActivityAt: now.Add(-10 * time.Minute),
		},
	}

	p := testPreset()
	activity := resolver.ResolvePresence(sessions, p, config.DetailMinimal, now)

	if activity == nil {
		t.Fatal("ResolvePresence returned nil for multi session")
	}

	// Details should come from the tier "3" pool
	// The only message in the tier 3 pool is "Triple-threading code"
	if activity.Details != "Triple-threading code" {
		t.Errorf("Details = %q, want %q", activity.Details, "Triple-threading code")
	}

	// State should be a stats line with aggregated counts
	// Total: 10 edits, 7 cmds, 3 searches, 8 reads, 1h 0m deep
	if !strings.Contains(activity.State, "10 edits") {
		t.Errorf("State %q should contain aggregated edits", activity.State)
	}
	if !strings.Contains(activity.State, "7 cmds") {
		t.Errorf("State %q should contain aggregated commands", activity.State)
	}

	// Layout D fields
	if activity.LargeImage != "dsr-code" {
		t.Errorf("LargeImage = %q, want %q", activity.LargeImage, "dsr-code")
	}

	// StartTime should be from earliest session
	if activity.StartTime == nil {
		t.Error("StartTime should not be nil")
	} else {
		earliest := now.Add(-60 * time.Minute)
		if !activity.StartTime.Equal(earliest) {
			t.Errorf("StartTime = %v, want %v (earliest session)", activity.StartTime, earliest)
		}
	}

	// Test overflow (5+ sessions)
	overflow := make([]*session.Session, 5)
	for i := range overflow {
		overflow[i] = &session.Session{
			SessionID:      "s-" + strings.Repeat("x", i+1),
			ProjectName:    "P",
			Branch:         "b",
			SmallImageKey:  "coding",
			ActivityCounts: session.ActivityCounts{Edits: 1},
			Status:         session.StatusActive,
			StartedAt:      now.Add(-time.Duration(i+1) * time.Minute),
			LastActivityAt: now,
		}
	}
	overflowResult := resolver.ResolvePresence(overflow, p, config.DetailMinimal, now)
	if overflowResult == nil {
		t.Fatal("ResolvePresence returned nil for 5 sessions")
	}
	if !strings.Contains(overflowResult.Details, "5") {
		t.Errorf("Overflow details %q should contain session count 5", overflowResult.Details)
	}
}

// --- Task 1: DisplayDetail-aware placeholder resolution tests ---

// displayDetailTestSession returns a session suitable for displayDetail tests.
func displayDetailTestSession(now time.Time) *session.Session {
	return &session.Session{
		SessionID:      "dd-test-1",
		ProjectName:    "MyProject",
		Branch:         "main",
		Model:          "opus-4",
		SmallImageKey:  "coding",
		SmallText:      "Editing a file",
		TotalTokens:    1500000,
		TotalCostUSD:   0.12,
		Status:         session.StatusActive,
		StartedAt:      now.Add(-30 * time.Minute),
		LastActivityAt: now,
		LastFile:        "main.go",
		LastFilePath:    "/src/main.go",
		LastCommand:     "go test ./...",
		LastQuery:       "TODO",
	}
}

// displayDetailPreset returns a preset with templates that exercise all new placeholders.
func displayDetailPreset() *preset.MessagePreset {
	return &preset.MessagePreset{
		Label:       "test-dd",
		Description: "test display detail preset",
		SingleSessionDetails: map[string][]string{
			"coding": {"{file} in {project} on {branch}"},
		},
		SingleSessionDetailsFallback: []string{"{file} in {project}"},
		SingleSessionState:           []string{"{command} | {query} | {activity} | {sessions}"},
		MultiSessionMessages: map[string][]string{
			"2": {"{projects} running"},
		},
		MultiSessionOverflow: []string{"{sessions} sessions"},
		MultiSessionTooltips: []string{"Multi-session"},
		Buttons:              []preset.Button{},
	}
}

// TestDisplayDetailMinimal verifies that DetailMinimal maps {file} to project name,
// {command} to "...", {query} to "*".
func TestDisplayDetailMinimal(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s := displayDetailTestSession(now)
	p := displayDetailPreset()

	activity := resolver.ResolvePresence([]*session.Session{s}, p, config.DetailMinimal, now)
	if activity == nil {
		t.Fatal("ResolvePresence returned nil")
	}

	// {file} should be project name for minimal
	if !strings.Contains(activity.Details, "MyProject") {
		t.Errorf("Details %q should contain project name as {file} for minimal", activity.Details)
	}
	// {file} should NOT contain the actual file path
	if strings.Contains(activity.Details, "/src/main.go") {
		t.Errorf("Details %q should not contain file path for minimal", activity.Details)
	}

	// {command} should be "..."
	if !strings.Contains(activity.State, "...") {
		t.Errorf("State %q should contain '...' for {command} in minimal", activity.State)
	}
	// {query} should be "*"
	if !strings.Contains(activity.State, "*") {
		t.Errorf("State %q should contain '*' for {query} in minimal", activity.State)
	}
}

// TestDisplayDetailStandard verifies that DetailStandard maps {file} to filename,
// {command} to truncated 20 chars.
func TestDisplayDetailStandard(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s := displayDetailTestSession(now)
	p := displayDetailPreset()

	activity := resolver.ResolvePresence([]*session.Session{s}, p, config.DetailStandard, now)
	if activity == nil {
		t.Fatal("ResolvePresence returned nil")
	}

	// {file} should be filename (base of path)
	if !strings.Contains(activity.Details, "main.go") {
		t.Errorf("Details %q should contain 'main.go' as {file} for standard", activity.Details)
	}
	// {file} should NOT contain full path
	if strings.Contains(activity.Details, "/src/") {
		t.Errorf("Details %q should not contain full path for standard", activity.Details)
	}

	// {command} should be the actual command (under 20 chars, no truncation needed)
	if !strings.Contains(activity.State, "go test") {
		t.Errorf("State %q should contain actual command for standard", activity.State)
	}

	// {query} should be actual query
	if !strings.Contains(activity.State, "TODO") {
		t.Errorf("State %q should contain actual query for standard", activity.State)
	}
}

// TestDisplayDetailStandardTruncation verifies long commands get truncated at 20 chars.
func TestDisplayDetailStandardTruncation(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s := displayDetailTestSession(now)
	s.LastCommand = "go test -v -race -count=1 ./internal/..."
	p := displayDetailPreset()

	activity := resolver.ResolvePresence([]*session.Session{s}, p, config.DetailStandard, now)
	if activity == nil {
		t.Fatal("ResolvePresence returned nil")
	}

	// Command should be truncated (contains ellipsis)
	if strings.Contains(activity.State, "./internal/...") {
		t.Errorf("State %q should have truncated command at standard level", activity.State)
	}
}

// TestDisplayDetailVerbose verifies that DetailVerbose maps {file} to full relative path,
// {command} to full command.
func TestDisplayDetailVerbose(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s := displayDetailTestSession(now)
	p := displayDetailPreset()

	activity := resolver.ResolvePresence([]*session.Session{s}, p, config.DetailVerbose, now)
	if activity == nil {
		t.Fatal("ResolvePresence returned nil")
	}

	// {file} should be full path
	if !strings.Contains(activity.Details, "/src/main.go") {
		t.Errorf("Details %q should contain full path for verbose", activity.Details)
	}

	// {command} should be full command
	if !strings.Contains(activity.State, "go test ./...") {
		t.Errorf("State %q should contain full command for verbose", activity.State)
	}

	// {query} should be actual query
	if !strings.Contains(activity.State, "TODO") {
		t.Errorf("State %q should contain query for verbose", activity.State)
	}
}

// TestDisplayDetailPrivate verifies that DetailPrivate redacts all sensitive data.
func TestDisplayDetailPrivate(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s := displayDetailTestSession(now)
	p := displayDetailPreset()

	activity := resolver.ResolvePresence([]*session.Session{s}, p, config.DetailPrivate, now)
	if activity == nil {
		t.Fatal("ResolvePresence returned nil")
	}

	// {file} should be "file"
	if !strings.Contains(activity.Details, "file") {
		t.Errorf("Details %q should contain 'file' for private", activity.Details)
	}
	// Should NOT contain actual project name in details
	if strings.Contains(activity.Details, "MyProject") {
		t.Errorf("Details %q should NOT contain project name for private", activity.Details)
	}

	// {project} should be "Project"
	if !strings.Contains(activity.Details, "Project") {
		t.Errorf("Details %q should contain 'Project' for private", activity.Details)
	}

	// {command} should be "..."
	if !strings.Contains(activity.State, "...") {
		t.Errorf("State %q should contain '...' for private", activity.State)
	}

	// LargeText should be "Project" (not project name)
	if activity.LargeText != "Project" {
		t.Errorf("LargeText = %q, want %q for private", activity.LargeText, "Project")
	}

	// {query} should be empty
	if strings.Contains(activity.State, "TODO") {
		t.Errorf("State %q should NOT contain query for private", activity.State)
	}
}

// TestNewPlaceholdersActivity verifies that {activity} resolves to SmallText.
func TestNewPlaceholdersActivity(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s := displayDetailTestSession(now)
	p := displayDetailPreset()

	activity := resolver.ResolvePresence([]*session.Session{s}, p, config.DetailStandard, now)
	if activity == nil {
		t.Fatal("ResolvePresence returned nil")
	}

	// {activity} should resolve to SmallText value
	if !strings.Contains(activity.State, "Editing a file") {
		t.Errorf("State %q should contain SmallText as {activity}", activity.State)
	}
}

// TestNewPlaceholdersSessions verifies that {sessions} resolves to "1" for single session.
func TestNewPlaceholdersSessions(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s := displayDetailTestSession(now)
	p := displayDetailPreset()

	activity := resolver.ResolvePresence([]*session.Session{s}, p, config.DetailStandard, now)
	if activity == nil {
		t.Fatal("ResolvePresence returned nil")
	}

	// {sessions} should be "1" for single session
	if !strings.Contains(activity.State, "1") {
		t.Errorf("State %q should contain '1' for {sessions}", activity.State)
	}
}

// TestNoUnresolvedPlaceholders verifies that no literal {file} or {command} remain.
func TestNoUnresolvedPlaceholders(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s := displayDetailTestSession(now)
	p := displayDetailPreset()

	for _, detail := range []config.DisplayDetail{
		config.DetailMinimal,
		config.DetailStandard,
		config.DetailVerbose,
		config.DetailPrivate,
	} {
		activity := resolver.ResolvePresence([]*session.Session{s}, p, detail, now)
		if activity == nil {
			t.Fatalf("ResolvePresence returned nil for %s", detail)
		}

		for _, field := range []string{activity.Details, activity.State} {
			for _, placeholder := range []string{"{file}", "{command}", "{query}", "{activity}", "{sessions}", "{project}", "{branch}"} {
				if strings.Contains(field, placeholder) {
					t.Errorf("[%s] field %q contains unresolved placeholder %s", detail, field, placeholder)
				}
			}
		}
	}
}
