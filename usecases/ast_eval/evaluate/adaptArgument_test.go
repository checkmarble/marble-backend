package evaluate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdaptArgumentToListOfStrings_list_of_strings(t *testing.T) {
	strings, err := adaptArgumentToListOfStrings([]string{"aa"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"aa"}, strings)
}

func TestAdaptArgumentToListOfStrings_list_of_any(t *testing.T) {
	strings, err := adaptArgumentToListOfStrings([]any{"aa"})
	assert.NoError(t, err)
	assert.Equal(t, []string{"aa"}, strings)
}

func TestAdaptArgumentToListOfStrings_list_of_int_fail(t *testing.T) {
	_, err := adaptArgumentToListOfStrings([]int{44})
	assert.Error(t, err)
}

func TestAdaptArgumentToListOfStrings_list_of_any_fail(t *testing.T) {
	_, err := adaptArgumentToListOfStrings([]any{"33", 43})
	assert.Error(t, err)
}

func TestAdaptArgumentToListOfThings_list_of_same_type(t *testing.T) {
	type Thing struct {
		name string
	}
	things := []Thing{{name: "Wednesday"}, {name: "Pugsley"}}

	list, err := adaptArgumentToListOfThings[Thing](things)
	assert.NoError(t, err)
	assert.Equal(t, things, list)
}

func TestAdaptArgumentToListOfThings_list_of_different_types(t *testing.T) {
	type Thing struct {
		name string
	}
	things := []any{Thing{name: "Wednesday"}, "Addams"}

	_, err := adaptArgumentToListOfThings[Thing](things)
	assert.Error(t, err)
}

func TestAdaptArgumentToListOfRanges(t *testing.T) {
	tests := []struct {
		name     string
		argument any
		expected [][2]int
		wantErr  bool
	}{
		{
			name:     "typed ranges",
			argument: [][2]int{{1, 3}, {5, 8}},
			expected: [][2]int{{1, 3}, {5, 8}},
		},
		{
			name:     "JSON-shaped ranges",
			argument: []any{[]any{float64(1), float64(3)}, []any{5, 8}},
			expected: [][2]int{{1, 3}, {5, 8}},
		},
		{
			name:     "reversed typed range",
			argument: [][2]int{{3, 1}},
			wantErr:  true,
		},
		{
			name:     "reversed JSON-shaped range",
			argument: []any{[]any{3, 1}},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := adaptArgumentToListOfRanges(tt.argument)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
