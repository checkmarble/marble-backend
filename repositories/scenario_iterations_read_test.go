package repositories

import (
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestScenarioIterationsMetadataQuery_NarrowsToTheGivenScenario(t *testing.T) {
	orgId, scenarioId := uuid.New(), uuid.New()

	sql, args, err := scenarioIterationsMetadataQuery(orgId,
		models.GetScenarioIterationFilters{ScenarioId: scenarioId}).ToSql()

	require.NoError(t, err)
	require.Contains(t, sql, "org_id = $1")
	require.Contains(t, sql, "scenario_id = $2")
	require.Equal(t, []any{orgId.String(), scenarioId.String()}, args)
}

func TestScenarioIterationsMetadataQuery_WithoutScenarioStaysWithinTheOrganization(t *testing.T) {
	orgId := uuid.New()

	sql, args, err := scenarioIterationsMetadataQuery(orgId, models.GetScenarioIterationFilters{}).ToSql()

	require.NoError(t, err)
	require.Contains(t, sql, "org_id = $1")
	require.NotContains(t, sql, "scenario_id =")
	require.Equal(t, []any{orgId.String()}, args)
}
