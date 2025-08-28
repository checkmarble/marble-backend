package pure_utils

import (
	"errors"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

// BCP47ToLocalizedLanguageName converts a BCP 47 language code to human-readable format
func BCP47ToLocalizedLanguageName(bcp47Code string, displayLang language.Tag) (string, error) {
	if bcp47Code == "" {
		return "", errors.New("bcp47Code is empty")
	}

	// Parse the BCP 47 language tag
	tag, err := language.Parse(bcp47Code)
	if err != nil {
		// If parsing fails, return the original code
		return "", err
	}

	// Create a display formatter for the specified display language
	// For example, if displayLang is English, all names will be in English
	formatter := display.Tags(displayLang)

	return formatter.Name(tag), nil
}

// BCP47ToEnglish is a convenience function that displays language names in English
func BCP47ToEnglish(bcp47Code string) (string, error) {
	return BCP47ToLocalizedLanguageName(bcp47Code, language.English)
}
