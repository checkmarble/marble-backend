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
