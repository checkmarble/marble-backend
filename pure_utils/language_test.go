package pure_utils

import (
	"testing"

	"golang.org/x/text/language"
)

func TestBCP47ToLocalizedLanguageName(t *testing.T) {
	tests := []struct {
		name        string
		bcp47Code   string
		displayLang language.Tag
		expected    string
		expectError bool
	}{
		{
			name:        "English code displayed in English",
			bcp47Code:   "en",
			displayLang: language.English,
			expected:    "English",
			expectError: false,
		},
		{
			name:        "French code displayed in English",
			bcp47Code:   "fr",
			displayLang: language.English,
			expected:    "French",
			expectError: false,
		},
		{
			name:        "Spanish code displayed in English",
			bcp47Code:   "es",
			displayLang: language.English,
			expected:    "Spanish",
			expectError: false,
		},
		{
			name:        "English code displayed in French",
			bcp47Code:   "en",
			displayLang: language.French,
			expected:    "anglais",
			expectError: false,
		},
		{
			name:        "Regional variant - US English",
			bcp47Code:   "en-US",
			displayLang: language.English,
			expected:    "American English",
			expectError: false,
		},
		{
			name:        "Regional variant - Canadian French",
			bcp47Code:   "fr-CA",
			displayLang: language.English,
			expected:    "Canadian French",
			expectError: false,
		},
		{
			name:        "Empty code should return error",
			bcp47Code:   "",
			displayLang: language.English,
			expected:    "",
			expectError: true,
		},
		{
			name:        "Invalid BCP 47 code should return error",
			bcp47Code:   "invalid-lang-code",
			displayLang: language.English,
			expected:    "",
			expectError: true,
		},
		{
			name:        "Malformed code should return error",
			bcp47Code:   "xyz123",
			displayLang: language.English,
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := BCP47ToLocalizedLanguageName(tt.bcp47Code, tt.displayLang)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected %q, got %q", tt.expected, result)
				}
			}
		})
	}
}

func TestBCP47ToEnglish(t *testing.T) {
	tests := []struct {
		name        string
		bcp47Code   string
		expected    string
		expectError bool
	}{
		{
			name:        "English code",
			bcp47Code:   "en",
			expected:    "English",
			expectError: false,
		},
		{
			name:        "French code",
			bcp47Code:   "fr",
			expected:    "French",
			expectError: false,
		},
		{
			name:        "German code",
			bcp47Code:   "de",
			expected:    "German",
			expectError: false,
		},
		{
			name:        "Spanish code",
			bcp47Code:   "es",
			expected:    "Spanish",
			expectError: false,
		},
		{
			name:        "Italian code",
			bcp47Code:   "it",
			expected:    "Italian",
			expectError: false,
		},
		{
			name:        "Portuguese code",
			bcp47Code:   "pt",
			expected:    "Portuguese",
			expectError: false,
		},
		{
			name:        "US English variant",
			bcp47Code:   "en-US",
			expected:    "American English",
			expectError: false,
		},
		{
			name:        "UK English variant",
			bcp47Code:   "en-GB",
			expected:    "British English",
			expectError: false,
		},
		{
			name:        "Brazilian Portuguese",
			bcp47Code:   "pt-BR",
			expected:    "Brazilian Portuguese",
			expectError: false,
		},
		{
			name:        "Empty code should return error",
			bcp47Code:   "",
			expected:    "",
			expectError: true,
		},
		{
			name:        "Invalid code should return error",
			bcp47Code:   "invalid",
			expected:    "",
			expectError: true,
		},
		{
			name:        "Nonsense code should return error",
			bcp47Code:   "abc123xyz",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := BCP47ToEnglish(tt.bcp47Code)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected %q, got %q", tt.expected, result)
				}
			}
		})
	}
}
