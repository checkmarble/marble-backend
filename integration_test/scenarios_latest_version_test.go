package integration

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

func TestListScenariosReturnsTheLatestVersion(t *testing.T) {
	bg := context.Background()

	insertOrg := func(t *testing.T) uuid.UUID {
		t.Helper()
		tenantId, orgId := uuid.New(), uuid.New()
		_, err := pgPool.Exec(bg, "INSERT INTO tenants (id, name) VALUES ($1, $2)", tenantId, tenantId.String())
		require.NoError(t, err)
		_, err = pgPool.Exec(bg, "INSERT INTO organizations (id, name, tenant_id) VALUES ($1, $2, $3)",
			orgId, orgId.String(), tenantId)
		require.NoError(t, err)
		return orgId
	}
	insertScenario := func(t *testing.T, orgId uuid.UUID) string {
		t.Helper()
		scenarioId := uuid.New()
		_, err := pgPool.Exec(bg, `
			INSERT INTO scenarios (id, org_id, name, description, trigger_object_type)
			VALUES ($1, $2, 'scenario', 'scenario', 'transactions')`,
			scenarioId, orgId)
		require.NoError(t, err)
		return scenarioId.String()
	}
	insertIteration := func(t *testing.T, orgId uuid.UUID, scenarioId string, version *int) {
		t.Helper()
		_, err := pgPool.Exec(bg,
			"INSERT INTO scenario_iterations (id, org_id, scenario_id, version) VALUES ($1, $2, $3, $4)",
			uuid.New(), orgId, scenarioId, version)
		require.NoError(t, err)
	}

	orgId, otherOrgId := insertOrg(t), insertOrg(t)

	versioned := insertScenario(t, orgId)
	insertIteration(t, orgId, versioned, utils.Ptr(1))
	insertIteration(t, orgId, versioned, utils.Ptr(3))
	insertIteration(t, orgId, versioned, utils.Ptr(2))
	insertIteration(t, orgId, versioned, nil)

	draftOnly := insertScenario(t, orgId)
	insertIteration(t, orgId, draftOnly, nil)

	withoutIterations := insertScenario(t, orgId)

	otherOrgScenario := insertScenario(t, otherOrgId)
	insertIteration(t, otherOrgId, otherOrgScenario, utils.Ptr(9))

	creds := models.Credentials{
		ActorIdentity:  models.Identity{UserId: "scenarios-latest-version-test-user"},
		OrganizationId: orgId,
		RoleBindings:   models.NativeRoleBindings([]models.Role{models.ADMIN}),
	}
	ctx := context.WithValue(bg, utils.ContextKeyCredentials, creds)
	usecasesWithCreds := generateUsecaseWithCreds(testUsecases, creds)
	usecase := usecasesWithCreds.NewScenarioUsecase()

	scenarios, err := usecase.ListScenarios(ctx, orgId)
	require.NoError(t, err)

	latestVersions := make(map[string]*int, len(scenarios))
	for _, scenario := range scenarios {
		latestVersions[scenario.Id] = scenario.LatestVersion
	}

	assert.Equal(t, map[string]*int{
		versioned:         utils.Ptr(3),
		draftOnly:         nil,
		withoutIterations: nil,
	}, latestVersions)
}
