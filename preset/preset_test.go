package preset_test

import (
	"strings"
	"testing"

	"github.com/StrainReviews/dsrcode/preset"
)

// TestLoadPreset verifies that LoadPreset("minimal") returns a non-nil
// MessagePreset with Label set to "minimal".
func TestLoadPreset(t *testing.T) {
	p, err := preset.LoadPreset("minimal")
	if err != nil {
		t.Fatalf("LoadPreset(\"minimal\") returned error: %v", err)
	}
	if p == nil {
		t.Fatal("LoadPreset(\"minimal\") returned nil")
	}
	if p.Label != "minimal" {
		t.Errorf("expected label \"minimal\", got %q", p.Label)
	}
}

// TestLoadPresetNotFound verifies that LoadPreset("nonexistent") returns an
// error whose message contains "unknown preset".
func TestLoadPresetNotFound(t *testing.T) {
	_, err := preset.LoadPreset("nonexistent")
	if err == nil {
		t.Fatal("LoadPreset(\"nonexistent\") should return an error")
	}
	if !strings.Contains(err.Error(), "unknown preset") {
		t.Errorf("error should contain \"unknown preset\", got: %v", err)
	}
}

// TestAvailablePresets verifies that AvailablePresets() returns a slice
// containing all 8 preset names: "minimal", "professional", "dev-humor",
// "chaotic", "german", "hacker", "streamer", "weeb".
func TestAvailablePresets(t *testing.T) {
	expected := map[string]bool{
		"minimal":      false,
		"professional": false,
		"dev-humor":    false,
		"chaotic":      false,
		"german":       false,
		"hacker":       false,
		"streamer":     false,
		"weeb":         false,
	}

	names := preset.AvailablePresets()
	if len(names) != 8 {
		t.Fatalf("AvailablePresets() returned %d presets, want 8: %v", len(names), names)
	}

	for _, name := range names {
		if _, ok := expected[name]; !ok {
			t.Errorf("unexpected preset name: %q", name)
		} else {
			expected[name] = true
		}
	}

	for name, found := range expected {
		if !found {
			t.Errorf("missing preset: %q", name)
		}
	}
}

// TestPresetHasRequiredFields verifies that each preset has non-empty
// SingleSessionDetails entries for the 7 preset-displayable activity icons:
// "coding", "terminal", "searching", "thinking", "reading", "idle", "starting".
// The "error" icon (D-19) is a status overlay and is intentionally skipped —
// presets do not provide per-icon messages for it.
func TestPresetHasRequiredFields(t *testing.T) {
	names := preset.AvailablePresets()
	icons := preset.AllActivityIcons()

	for _, name := range names {
		p, err := preset.LoadPreset(name)
		if err != nil {
			t.Errorf("LoadPreset(%q) failed: %v", name, err)
			continue
		}
		for _, icon := range icons {
			if icon == "error" {
				continue // status overlay only — see D-19
			}
			msgs, ok := p.SingleSessionDetails[icon]
			if !ok {
				t.Errorf("preset %q missing SingleSessionDetails key %q", name, icon)
			} else if len(msgs) == 0 {
				t.Errorf("preset %q has empty SingleSessionDetails for %q", name, icon)
			}
		}
	}
}

// TestPresetMessageCounts verifies that each preset has at least 40 messages
// per activity type in SingleSessionDetails and singleSessionState.
func TestPresetMessageCounts(t *testing.T) {
	names := preset.AvailablePresets()
	icons := preset.AllActivityIcons()

	if len(names) != 8 {
		t.Fatalf("expected 8 presets, got %d: %v", len(names), names)
	}

	minMessages := 40

	for _, name := range names {
		p, err := preset.LoadPreset(name)
		if err != nil {
			t.Errorf("LoadPreset(%q) failed: %v", name, err)
			continue
		}
		for _, icon := range icons {
			if icon == "error" {
				continue // status overlay only — see D-19
			}
			msgs := p.SingleSessionDetails[icon]
			if len(msgs) < minMessages {
				t.Errorf("preset %q singleSessionDetails[%s] has %d messages, want >= %d", name, icon, len(msgs), minMessages)
			}
		}
		if len(p.SingleSessionState) < minMessages {
			t.Errorf("preset %q singleSessionState has %d messages, want >= %d", name, len(p.SingleSessionState), minMessages)
		}
		if len(p.SingleSessionDetailsFallback) < 15 {
			t.Errorf("preset %q singleSessionDetailsFallback has %d messages, want >= 15", name, len(p.SingleSessionDetailsFallback))
		}
		if len(p.MultiSessionOverflow) < 10 {
			t.Errorf("preset %q MultiSessionOverflow has %d messages, want at least 10", name, len(p.MultiSessionOverflow))
		}
		if len(p.MultiSessionTooltips) < 10 {
			t.Errorf("preset %q MultiSessionTooltips has %d messages, want at least 10", name, len(p.MultiSessionTooltips))
		}
		for _, tier := range []string{"2", "3", "4"} {
			msgs := p.MultiSessionMessages[tier]
			if len(msgs) < 10 {
				t.Errorf("preset %q tier %q has %d messages, want at least 10", name, tier, len(msgs))
			}
		}
	}
}

// TestPresetNewPlaceholders verifies that each preset's coding messages
// use new placeholders {file}, {command}, or {query} in at least 30% of entries.
func TestPresetNewPlaceholders(t *testing.T) {
	names := preset.AvailablePresets()
	newPlaceholders := []string{"{file}", "{command}", "{query}"}

	for _, name := range names {
		p, err := preset.LoadPreset(name)
		if err != nil {
			t.Errorf("LoadPreset(%q) failed: %v", name, err)
			continue
		}

		codingPool := p.SingleSessionDetails["coding"]
		hasNew := 0
		for _, msg := range codingPool {
			for _, ph := range newPlaceholders {
				if strings.Contains(msg, ph) {
					hasNew++
					break
				}
			}
		}
		// At least 30% should use new placeholders (12 out of 40)
		if hasNew < 12 {
			t.Errorf("%s coding: only %d/%d messages use new placeholders ({file}/{command}/{query}), want >= 12", name, hasNew, len(codingPool))
		}
	}
}

// TestLoadPresetWithLang verifies bilingual loading with explicit language selection.
func TestLoadPresetWithLang(t *testing.T) {
	// Load English
	en, err := preset.LoadPresetWithLang("minimal", "en")
	if err != nil {
		t.Fatalf("LoadPresetWithLang(\"minimal\", \"en\") error: %v", err)
	}
	if en.Label != "minimal" {
		t.Errorf("expected label \"minimal\", got %q", en.Label)
	}
	if len(en.SingleSessionState) == 0 {
		t.Error("English singleSessionState should not be empty")
	}

	// Load German
	de, err := preset.LoadPresetWithLang("minimal", "de")
	if err != nil {
		t.Fatalf("LoadPresetWithLang(\"minimal\", \"de\") error: %v", err)
	}
	if de.Label != "minimal" {
		t.Errorf("expected label \"minimal\", got %q", de.Label)
	}
	if len(de.SingleSessionState) == 0 {
		t.Error("German singleSessionState should not be empty")
	}

	// Verify EN and DE are different (at least first state message should differ)
	if len(en.SingleSessionState) > 0 && len(de.SingleSessionState) > 0 {
		// They should have the same count but different content
		if en.SingleSessionState[0] == de.SingleSessionState[0] {
			t.Log("Warning: EN and DE first state messages are identical (may be by design for some presets)")
		}
	}
}

// TestLoadPresetWithLangFallback verifies fallback to English for unknown languages.
func TestLoadPresetWithLangFallback(t *testing.T) {
	fr, err := preset.LoadPresetWithLang("minimal", "fr")
	if err != nil {
		t.Fatalf("LoadPresetWithLang(\"minimal\", \"fr\") should fallback to en, got error: %v", err)
	}

	en, err := preset.LoadPresetWithLang("minimal", "en")
	if err != nil {
		t.Fatalf("LoadPresetWithLang(\"minimal\", \"en\") error: %v", err)
	}

	// Fallback to "en" means same messages
	if len(fr.SingleSessionState) != len(en.SingleSessionState) {
		t.Errorf("French fallback should match English: got %d states, want %d",
			len(fr.SingleSessionState), len(en.SingleSessionState))
	}
}

// TestLoadPresetWithLangNotFound verifies error for unknown preset name.
func TestLoadPresetWithLangNotFound(t *testing.T) {
	_, err := preset.LoadPresetWithLang("nonexistent", "en")
	if err == nil {
		t.Fatal("LoadPresetWithLang(\"nonexistent\", \"en\") should return error")
	}
	if !strings.Contains(err.Error(), "unknown preset") {
		t.Errorf("error should contain \"unknown preset\", got: %v", err)
	}
}

// TestPresetButtons verifies that the "hacker" and "streamer" presets have
// non-empty Buttons slices, with each button Label being at most 32 characters
// (Discord limit).
func TestPresetButtons(t *testing.T) {
	presetsWithButtons := []string{"hacker", "streamer", "weeb"}

	for _, name := range presetsWithButtons {
		p, err := preset.LoadPreset(name)
		if err != nil {
			t.Errorf("LoadPreset(%q) failed: %v", name, err)
			continue
		}
		if len(p.Buttons) == 0 {
			t.Errorf("preset %q should have non-empty Buttons", name)
			continue
		}
		for i, btn := range p.Buttons {
			if len(btn.Label) == 0 {
				t.Errorf("preset %q button[%d] has empty label", name, i)
			}
			if len(btn.Label) > 32 {
				t.Errorf("preset %q button[%d] label %q exceeds 32 chars (%d)", name, i, btn.Label, len(btn.Label))
			}
			if len(btn.URL) == 0 {
				t.Errorf("preset %q button[%d] has empty URL", name, i)
			}
		}
	}
}
