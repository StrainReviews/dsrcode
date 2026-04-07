package preset

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed presets/*.json
var presetFS embed.FS

// LoadPreset loads a MessagePreset by name from embedded JSON files.
// For bilingual (D-29) presets, returns the English ("en") messages.
// Use LoadPresetWithLang for explicit language selection.
func LoadPreset(name string) (*MessagePreset, error) {
	return LoadPresetWithLang(name, "en")
}

// AvailablePresets returns all preset names from the embedded filesystem.
func AvailablePresets() []string {
	entries, err := presetFS.ReadDir("presets")
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	return names
}

// MustLoadPreset loads a preset or panics. Use for startup where preset is required.
func MustLoadPreset(name string) *MessagePreset {
	p, err := LoadPreset(name)
	if err != nil {
		panic(err)
	}
	return p
}

// LoadPresetWithLang loads a preset and selects the specified language.
// Falls back to "en" if the requested language is not available.
// Supports both legacy (flat) and bilingual (D-29 wrapper) formats.
func LoadPresetWithLang(name, lang string) (*MessagePreset, error) {
	data, err := presetFS.ReadFile(fmt.Sprintf("presets/%s.json", name))
	if err != nil {
		return nil, fmt.Errorf("unknown preset %q: %w", name, err)
	}

	// Try bilingual format first (D-29)
	var bilingual BilingualMessagePreset
	if err := json.Unmarshal(data, &bilingual); err == nil && bilingual.Messages != nil {
		// Bilingual format: select language
		if msgs, ok := bilingual.Messages[lang]; ok {
			result := copyMessagePreset(msgs)
			result.Label = bilingual.Label
			if desc, ok := bilingual.Description[lang]; ok {
				result.Description = desc
			}
			if len(bilingual.Buttons) > 0 {
				result.Buttons = bilingual.Buttons
			}
			return result, nil
		}
		// Fallback to English
		if msgs, ok := bilingual.Messages["en"]; ok {
			result := copyMessagePreset(msgs)
			result.Label = bilingual.Label
			if desc, ok := bilingual.Description["en"]; ok {
				result.Description = desc
			}
			if len(bilingual.Buttons) > 0 {
				result.Buttons = bilingual.Buttons
			}
			return result, nil
		}
	}

	// Legacy format: flat MessagePreset (backward compat)
	var legacy MessagePreset
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("invalid preset %q: %w", name, err)
	}
	return &legacy, nil
}

// MustLoadPresetWithLang loads a preset with language or panics.
func MustLoadPresetWithLang(name, lang string) *MessagePreset {
	p, err := LoadPresetWithLang(name, lang)
	if err != nil {
		panic(err)
	}
	return p
}

// copyMessagePreset returns a shallow copy of a MessagePreset to avoid
// mutating the shared unmarshaled data when attaching label/description/buttons.
func copyMessagePreset(src *MessagePreset) *MessagePreset {
	cp := *src
	return &cp
}
