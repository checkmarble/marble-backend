package api

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
)

func TestParseOptionalScenarioId(t *testing.T) {
	scenarioId := uuid.New()

	tests := []struct {
		name     string
		raw      string
		expected uuid.UUID
	}{
		{name: "absent means every scenario", raw: "", expected: uuid.Nil},
		{name: "a scenario", raw: scenarioId.String(), expected: scenarioId},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := parseOptionalScenarioId(tt.raw)

			require.NoError(t, err)
			assert.Equal(t, tt.expected, parsed)
		})
	}
}

func TestParseOptionalScenarioId_ReportsSomethingThatIsNotAnIdAsBadInput(t *testing.T) {
	_, err := parseOptionalScenarioId("not-a-uuid")

	assert.ErrorIs(t, err, models.BadParameterError)
}
