package pure_utils

import (
	"testing"
)

func TestCountryToAlpha2(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Exact matches - Alpha-2 codes
		{
			name:     "Alpha-2 code US",
			input:    "US",
			expected: "US",
		},
		{
			name:     "Alpha-2 code lowercase",
			input:    "us",
			expected: "US",
		},
		{
			name:     "Alpha-2 code FR",
			input:    "FR",
			expected: "FR",
		},
		{
			name:     "Alpha-2 code with spaces",
			input:    "  DE  ",
			expected: "DE",
		},

		// Exact matches - Alpha-3 codes
		{
			name:     "Alpha-3 code USA",
			input:    "USA",
			expected: "US",
		},
		{
			name:     "Alpha-3 code FRA",
			input:    "FRA",
			expected: "FR",
		},
		{
			name:     "Alpha-3 code GBR",
			input:    "GBR",
			expected: "GB",
		},

		// Exact matches - Full names
		{
			name:     "Full name United States",
			input:    "United States",
			expected: "US",
		},
		{
			name:     "Full name France",
			input:    "France",
			expected: "FR",
		},
		{
			name:     "Full name Germany",
			input:    "Germany",
			expected: "DE",
		},
		{
			name:     "Full name United Kingdom",
			input:    "United Kingdom",
			expected: "GB",
		},

		// Fuzzy matches - Typos
		{
			name:     "Typo Frnace -> France",
			input:    "Frnace",
			expected: "FR",
		},
		{
			name:     "Typo Germeny -> Germany",
			input:    "Germeny",
			expected: "DE",
		},
		{
			name:     "Typo Brasil -> Brazil",
			input:    "Brasil",
			expected: "BR",
		},
		{
			name:     "Typo Austalia -> Australia",
			input:    "Austalia",
			expected: "AU",
		},
		{
			name:     "Typo Japon -> Japan",
			input:    "Japon",
			expected: "JP",
		},

		// ISO 3166-2 (Subdivisions)
		{
			name:     "ISO 3166-2 CH-AI (Switzerland)",
			input:    "CH-AI",
			expected: "CH",
		},
		{
			name:     "ISO 3166-2 US-NY (USA)",
			input:    "US-NY",
			expected: "US",
		},
		{
			name:     "ISO 3166-2 FR-75 (France)",
			input:    "FR-75",
			expected: "FR",
		},

		// Country names with hyphens
		{
			name:     "Guinea-Bissau exact",
			input:    "Guinea-Bissau",
			expected: "GW",
		},
		{
			name:     "Guinea-Bissau typo",
			input:    "Guine-Bissau",
			expected: "GW",
		},
		{
			name:     "Timor-Leste typo",
			input:    "Timor-Lest",
			expected: "TL",
		},

		// Edge cases
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Whitespace only",
			input:    "   ",
			expected: "",
		},
		{
			name:     "Unrecognizable input",
			input:    "xyzabc123",
			expected: "xyzabc123",
		},
		{
			name:     "Too short to match",
			input:    "X",
			expected: "X",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CountryToAlpha2(tt.input)
			if result != tt.expected {
				t.Errorf("CountryToAlpha2(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
