package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTruncateIdentifier(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "short identifier", input: "test", expected: "test"},
		{
			name:     "identifier with limit length",
			input:    "this_is_exactly_sixty_three_characters_long_name_table_aaaaa",
			expected: "this_is_exactly_sixty_three_characters_long_name_table_aaaaa",
		},
		{
			name:     "identifier longer than limit length",
			input:    "very_long_table_name_that_exceeds_the_postgresql_maximum_identifier_length_other",
			expected: "very_long_table_name_that_exceeds_the_postgresql_maximum_2492ea",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := TruncateIdentifier(test.input)
			assert.Equal(t, test.expected, result)
		})
	}
}
