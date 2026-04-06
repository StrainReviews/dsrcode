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

// uniqueProjectPreset returns a minimal preset that distinguishes single-session
// from multi-session via predictable placeholder patterns.
func uniqueProjectPreset() *preset.MessagePreset {
	return &preset.MessagePreset{
		Label:       "test-unique",
		Description: "test unique project tier selection",
		SingleSessionDetails: map[string][]string{
			"coding": {"Editing {project} ({branch})"},
		},
		SingleSessionDetailsFallback: []string{"Working on {project}"},
		SingleSessionState:           []string{"{model} | {tokens} tokens"},
		MultiSessionMessages: map[string][]string{
			"2": {"{n} Projekte aktiv"},
			"3": {"{n} Projekte aktiv"},
		},
		MultiSessionOverflow: []string{"{n} sessions blazing"},
		MultiSessionTooltips: []string{"Multi-session mode"},
	}
}

// TestResolverUniqueProjectTier contains 2 table-driven subtests verifying that
// the resolver uses unique project count (not raw session count) for tier
// selection per D-14.
//
// These tests compile against the existing ResolvePresence API but FAIL because
// the current implementation uses len(sessions) rather than unique project count
// for the single/multi-session tier decision.
func TestResolverUniqueProjectTier(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "Two_sessions_same_project_uses_single",
			run: func(t *testing.T) {
				// Two sessions with the same ProjectName but different
				// SessionIDs and PIDs (two terminal windows on same project).
				// Per D-14: unique project count = 1, so the resolver should
				// use single-session message pool. Details should contain
				// project-specific placeholders, not multi-session "{n} Projekte".
				now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)

				sessions := []*session.Session{
					{
						SessionID:      "window-1",
						ProjectName:    "StrainReviewsScanner",
						ProjectPath:    "/home/user/SRS",
						Branch:         "main",
						Model:          "Opus 4.6",
						PID:            100,
						SmallImageKey:  "coding",
						SmallText:      "Editing a file",
						TotalTokens:    500000,
						TotalCostUSD:   0.05,
						ActivityCounts: session.ActivityCounts{Edits: 10},
						Status:         session.StatusActive,
						StartedAt:      now.Add(-30 * time.Minute),
						LastActivityAt: now.Add(-1 * time.Minute),
					},
					{
						SessionID:      "window-2",
						ProjectName:    "StrainReviewsScanner",
						ProjectPath:    "/home/user/SRS",
						Branch:         "main",
						Model:          "Opus 4.6",
						PID:            200,
						SmallImageKey:  "coding",
						SmallText:      "Editing a file",
						TotalTokens:    300000,
						TotalCostUSD:   0.03,
						ActivityCounts: session.ActivityCounts{Edits: 5},
						Status:         session.StatusActive,
						StartedAt:      now.Add(-20 * time.Minute),
						LastActivityAt: now.Add(-2 * time.Minute),
					},
				}

				p := uniqueProjectPreset()
				activity := resolver.ResolvePresence(sessions, p, config.DetailMinimal, now)

				if activity == nil {
					t.Fatal("ResolvePresence returned nil")
				}

				// Single-session tier: Details should contain project name
				// from the single-session message pool, NOT "{n} Projekte"
				if strings.Contains(activity.Details, "Projekte") {
					t.Errorf("Details = %q, should NOT contain 'Projekte' for same-project sessions", activity.Details)
				}
				if !strings.Contains(activity.Details, "StrainReviewsScanner") {
					t.Errorf("Details = %q, should contain project name (single-session tier)", activity.Details)
				}
			},
		},
		{
			name: "Two_sessions_different_projects_uses_multi",
			run: func(t *testing.T) {
				// Two sessions with different ProjectNames. Per D-14: unique
				// project count = 2, so the resolver should use multi-session
				// message pool. Details should contain "{n}" or project list.
				now := time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC)

				sessions := []*session.Session{
					{
						SessionID:      "sess-srs",
						ProjectName:    "StrainReviewsScanner",
						ProjectPath:    "/home/user/SRS",
						Branch:         "main",
						Model:          "Opus 4.6",
						PID:            100,
						SmallImageKey:  "coding",
						SmallText:      "Editing a file",
						TotalTokens:    500000,
						TotalCostUSD:   0.05,
						ActivityCounts: session.ActivityCounts{Edits: 10},
						Status:         session.StatusActive,
						StartedAt:      now.Add(-30 * time.Minute),
						LastActivityAt: now.Add(-1 * time.Minute),
					},
					{
						SessionID:      "sess-ccdp",
						ProjectName:    "cc-discord-presence",
						ProjectPath:    "/home/user/ccdp",
						Branch:         "feature",
						Model:          "Sonnet 4.6",
						PID:            200,
						SmallImageKey:  "terminal",
						SmallText:      "Running commands",
						TotalTokens:    200000,
						TotalCostUSD:   0.02,
						ActivityCounts: session.ActivityCounts{Commands: 5},
						Status:         session.StatusActive,
						StartedAt:      now.Add(-20 * time.Minute),
						LastActivityAt: now.Add(-2 * time.Minute),
					},
				}

				p := uniqueProjectPreset()
				activity := resolver.ResolvePresence(sessions, p, config.DetailMinimal, now)

				if activity == nil {
					t.Fatal("ResolvePresence returned nil")
				}

				// Multi-session tier: Details should contain "2" (from {n})
				// or "Projekte" (from the multi-session message pool)
				if !strings.Contains(activity.Details, "2") && !strings.Contains(activity.Details, "Projekte") {
					t.Errorf("Details = %q, should contain '2' or 'Projekte' for different-project sessions", activity.Details)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.run)
	}
}
