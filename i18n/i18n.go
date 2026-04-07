// Package i18n provides bilingual localization for code strings (status output,
// error messages, labels) using go-i18n. Locale files are embedded via go:embed
// for single-binary distribution. Per D-30 and D-35, this handles functional
// code strings only -- creative preset messages use the preset JSON format.
package i18n

import (
	"embed"
	"encoding/json"
	"log/slog"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed active.*.json
var localeFS embed.FS

var bundle *i18n.Bundle

func init() {
	bundle = i18n.NewBundle(language.English)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)

	// Load embedded locale files
	if data, err := localeFS.ReadFile("active.en.json"); err == nil {
		if _, err := bundle.ParseMessageFileBytes(data, "active.en.json"); err != nil {
			slog.Warn("i18n: failed to parse active.en.json", "error", err)
		}
	}
	if data, err := localeFS.ReadFile("active.de.json"); err == nil {
		if _, err := bundle.ParseMessageFileBytes(data, "active.de.json"); err != nil {
			slog.Warn("i18n: failed to parse active.de.json", "error", err)
		}
	}
}

// NewLocalizer creates a localizer for the given language tag.
// Falls back to English for unknown tags.
func NewLocalizer(lang string) *i18n.Localizer {
	return i18n.NewLocalizer(bundle, lang)
}

// T is a convenience function that localizes a message by ID.
// Falls back to the English default if the translation is missing.
func T(loc *i18n.Localizer, id string) string {
	msg, err := loc.Localize(&i18n.LocalizeConfig{
		MessageID: id,
	})
	if err != nil {
		slog.Debug("i18n: missing translation", "id", id, "error", err)
		return id // Return the ID itself as fallback
	}
	return msg
}

// TWithData localizes a message with template data.
func TWithData(loc *i18n.Localizer, id string, data map[string]interface{}) string {
	msg, err := loc.Localize(&i18n.LocalizeConfig{
		MessageID:    id,
		TemplateData: data,
	})
	if err != nil {
		slog.Debug("i18n: missing translation", "id", id, "error", err)
		return id
	}
	return msg
}
