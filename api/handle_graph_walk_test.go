package api

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
)

func TestParseGraphRelationGroups(t *testing.T) {
	first := uuid.New()
	second := uuid.New()

	tests := []struct {
		name     string
		raw      string
		expected []uuid.UUID
	}{
		{name: "absent means every group", raw: "", expected: nil},
		{name: "one group", raw: first.String(), expected: []uuid.UUID{first}},
		{
			name:     "several groups",
			raw:      first.String() + "," + second.String(),
			expected: []uuid.UUID{first, second},
		},
		{
			// A list is easier to build with a trailing comma or a space after each one, and
			// neither says anything different from the list without them.
			name:     "padding and empty segments are not an error",
			raw:      " " + first.String() + ", " + second.String() + ",",
			expected: []uuid.UUID{first, second},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			groupIds, err := parseGraphRelationGroups(tt.raw)

			require.NoError(t, err)
			assert.Equal(t, tt.expected, groupIds)
		})
	}
}

func TestParseGraphRelationGroups_ReportsSomethingThatIsNotAGroupIdAsBadInput(t *testing.T) {
	// Bad input from a query string is the caller's mistake, not ours: it must not read as an
	// internal failure, which is what would be reported and alerted on otherwise.
	_, err := parseGraphRelationGroups("same_iban")

	assert.ErrorIs(t, err, models.BadParameterError)
}
