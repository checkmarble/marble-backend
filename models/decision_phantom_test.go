package models

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdaptScenarExecToPhantomDecisionCopiesScreeningWithNewID(t *testing.T) {
	for _, tt := range []struct {
		name    string
		matches int
	}{
		{name: "without matches", matches: 0},
		{name: "with matches", matches: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			original := ScreeningWithMatches{Screening: Screening{
				Id:              uuid.NewString(),
				DecisionId:      uuid.NewString(),
				NumberOfMatches: tt.matches,
			}}
			execution := ScenarioExecution{
				OrganizationId:      uuid.New(),
				ScreeningExecutions: []ScreeningWithMatches{original},
			}

			phantom := AdaptScenarExecToPhantomDecision(execution)

			require.Len(t, phantom.ScreeningExecutions, 1)
			copied := phantom.ScreeningExecutions[0]
			assert.NotEmpty(t, copied.Id)
			assert.NotEqual(t, original.Id, copied.Id)
			assert.Equal(t, phantom.PhantomDecisionId, copied.DecisionId)
			assert.Equal(t, execution.OrganizationId, copied.OrgId)
			assert.Equal(t, tt.matches, copied.NumberOfMatches)
			assert.Equal(t, original, execution.ScreeningExecutions[0])
		})
	}
}
