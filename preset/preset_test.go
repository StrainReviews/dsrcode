package preset_test

import (
	"strings"
	"testing"

	"github.com/tsanva/cc-discord-presence/preset"
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
// SingleSessionDetails entries for all 7 activity icons: "coding", "terminal",
// "searching", "thinking", "reading", "idle", "starting".
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
			msgs, ok := p.SingleSessionDetails[icon]
			if !ok {
				t.Errorf("preset %q missing SingleSessionDetails key %q", name, icon)
			} else if len(msgs) == 0 {
				t.Errorf("preset %q has empty SingleSessionDetails for %q", name, icon)
			}
		}
	}
}

// TestPresetMessageCounts verifies that each preset has at least 10 messages
// per activity type in SingleSessionDetails.
func TestPresetMessageCounts(t *testing.T) {
	names := preset.AvailablePresets()
	icons := preset.AllActivityIcons()

	for _, name := range names {
		p, err := preset.LoadPreset(name)
		if err != nil {
			t.Errorf("LoadPreset(%q) failed: %v", name, err)
			continue
		}
		for _, icon := range icons {
			msgs := p.SingleSessionDetails[icon]
			if len(msgs) < 10 {
				t.Errorf("preset %q icon %q has %d messages, want at least 10", name, icon, len(msgs))
			}
		}
		if len(p.SingleSessionState) < 10 {
			t.Errorf("preset %q SingleSessionState has %d messages, want at least 10", name, len(p.SingleSessionState))
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
