package preset

// MessagePreset defines all rotating messages for a single display style.
// Per D-22: each preset covers single-session details (per activity icon),
// single-session state, multi-session tier messages, and optional buttons.
type MessagePreset struct {
	Label       string `json:"label"`
	Description string `json:"description"`

	// SingleSessionDetails: map of activity icon key -> message pool.
	// Keys: "coding", "terminal", "searching", "thinking", "reading", "idle", "starting"
	// Values: pool of rotating detail strings.
	// Placeholders: {project}, {branch}, {file}, {filepath}, {command}, {query}, {activity}
	SingleSessionDetails map[string][]string `json:"singleSessionDetails"`

	// SingleSessionDetailsFallback: used when activity icon has no specific messages.
	SingleSessionDetailsFallback []string `json:"singleSessionDetailsFallback"`

	// SingleSessionState: rotating state line messages.
	// Placeholders: {model}, {tokens}, {cost}, {duration}, {sessions}
	SingleSessionState []string `json:"singleSessionState"`

	// MultiSessionMessages: tier-based messages for 2, 3, 4 concurrent sessions.
	// Keys: "2", "3", "4"
	// Placeholders: {n}, {projects}, {models}, {totalCost}, {totalTokens}, {stats}, {mode}, {sessions}, {duration}
	MultiSessionMessages map[string][]string `json:"multiSessionMessages"`

	// MultiSessionOverflow: messages for 5+ sessions.
	// Placeholders: {n}, {projects}, {models}, {totalCost}, {totalTokens}, {stats}, {mode}, {sessions}, {duration}
	MultiSessionOverflow []string `json:"multiSessionOverflow"`

	// MultiSessionTooltips: tooltip text for multi-session large image.
	MultiSessionTooltips []string `json:"multiSessionTooltips"`

	// Buttons: preset-specific clickable links (max 2 per Discord limit).
	Buttons []Button `json:"buttons,omitempty"`
}

// Button defines a clickable link shown on the Discord presence.
// Per D-32: max 2 buttons, label 1-32 chars, only visible to other users.
type Button struct {
	Label string `json:"label"`
	URL   string `json:"url"`
}

// BilingualMessagePreset wraps MessagePreset with per-language message pools.
// Per D-29: top-level label + description with language sub-objects for messages.
// The storage format for bilingual preset JSON files. The resolver consumes
// a plain MessagePreset (one language selected at load time).
type BilingualMessagePreset struct {
	Label       string                    `json:"label"`
	Description map[string]string         `json:"description"` // {"en": "...", "de": "..."}
	Messages    map[string]*MessagePreset `json:"messages"`    // {"en": {...}, "de": {...}}
	Buttons     []Button                  `json:"buttons,omitempty"`
}

// AllActivityIcons returns the 8 activity icon keys: 7 preset-displayable
// activities (coding, terminal, searching, thinking, reading, idle, starting
// per D-07) plus "error" — a status overlay icon used by StopFailure (D-19).
//
// "error" has no per-preset SingleSessionDetails messages: when StopFailure
// fires, the SmallImage swaps to "error" temporarily while State and Details
// continue to show the previous activity's content. Tests that iterate
// activity icons against preset message pools must therefore skip "error".
func AllActivityIcons() []string {
	return []string{"coding", "terminal", "searching", "thinking", "reading", "idle", "starting", "error"}
}
