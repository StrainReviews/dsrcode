package preset

// MessagePreset defines all rotating messages for a single display style.
// Per D-22: each preset covers single-session details (per activity icon),
// single-session state, multi-session tier messages, and optional buttons.
type MessagePreset struct {
	Label       string `json:"label"`
	Description string `json:"description"`

	// SingleSessionDetails: map of activity icon key -> message pool.
	// Keys: "coding", "terminal", "searching", "thinking", "reading", "idle", "starting"
	// Values: pool of rotating detail strings (use {project}, {branch} placeholders).
	SingleSessionDetails map[string][]string `json:"singleSessionDetails"`

	// SingleSessionDetailsFallback: used when activity icon has no specific messages.
	SingleSessionDetailsFallback []string `json:"singleSessionDetailsFallback"`

	// SingleSessionState: rotating state line messages.
	// Placeholders: {model}, {tokens}, {cost}, {duration}.
	SingleSessionState []string `json:"singleSessionState"`

	// MultiSessionMessages: tier-based messages for 2, 3, 4 concurrent sessions.
	// Keys: "2", "3", "4"
	// Placeholders: {n} for session count.
	MultiSessionMessages map[string][]string `json:"multiSessionMessages"`

	// MultiSessionOverflow: messages for 5+ sessions.
	// Placeholder: {n} for count.
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

// AllActivityIcons returns the 7 activity icon keys per D-07.
func AllActivityIcons() []string {
	return []string{"coding", "terminal", "searching", "thinking", "reading", "idle", "starting"}
}
