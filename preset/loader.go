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
func LoadPreset(name string) (*MessagePreset, error) {
	data, err := presetFS.ReadFile(fmt.Sprintf("presets/%s.json", name))
	if err != nil {
		return nil, fmt.Errorf("unknown preset %q: %w", name, err)
	}
	var preset MessagePreset
	if err := json.Unmarshal(data, &preset); err != nil {
		return nil, fmt.Errorf("invalid preset %q: %w", name, err)
	}
	return &preset, nil
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
