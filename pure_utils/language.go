package pure_utils

import (
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

// BCP47ToHumanReadable converts a BCP 47 language code to human-readable format
func BCP47ToHumanReadable(bcp47Code string, displayLang language.Tag) string {
	if bcp47Code == "" {
		return ""
	}

	// Parse the BCP 47 language tag
	tag, err := language.Parse(bcp47Code)
	if err != nil {
		// If parsing fails, return the original code
		return bcp47Code
	}

	// Create a display formatter for the specified display language
	// For example, if displayLang is English, all names will be in English
	formatter := display.Tags(displayLang)

	return formatter.Name(tag)
}

// BCP47ToEnglish is a convenience function that displays language names in English
func BCP47ToEnglish(bcp47Code string) string {
	return BCP47ToHumanReadable(bcp47Code, language.English)
}
