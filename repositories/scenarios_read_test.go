package repositories

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestScenariosWithLatestVersionQuery_JoinsIterationsOnceWithinTheOrganization(t *testing.T) {
	orgId := uuid.New()

	sql, args, err := scenariosWithLatestVersionQuery(orgId).ToSql()

	require.NoError(t, err)
	require.Contains(t, sql, "max(si.version) AS latest_version")
	require.Contains(t, sql, "LEFT JOIN scenario_iterations AS si ON si.scenario_id = s.id AND si.org_id = s.org_id")
	require.Contains(t, sql, "WHERE s.org_id = $1")
	require.Contains(t, sql, "GROUP BY s.id")
	require.Equal(t, []any{orgId.String()}, args)
}
