package integration

import (
	"context"
	"net/http"
	"testing"

	"github.com/gavv/httpexpect/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

func TestListScenarioIterationsMetadataAcrossAnOrganization(t *testing.T) {
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
	insertScenario := func(t *testing.T, orgId uuid.UUID) uuid.UUID {
		t.Helper()
		scenarioId := uuid.New()
		_, err := pgPool.Exec(bg, `
			INSERT INTO scenarios (id, org_id, name, description, trigger_object_type)
			VALUES ($1, $2, 'scenario', 'scenario', 'transactions')`,
			scenarioId, orgId)
		require.NoError(t, err)
		return scenarioId
	}
	insertIteration := func(t *testing.T, orgId, scenarioId uuid.UUID) string {
		t.Helper()
		iterationId := uuid.New()
		_, err := pgPool.Exec(bg, "INSERT INTO scenario_iterations (id, org_id, scenario_id) VALUES ($1, $2, $3)",
			iterationId, orgId, scenarioId)
		require.NoError(t, err)
		return iterationId.String()
	}
	ids := func(iterations []models.ScenarioIterationMetadata) []string {
		out := make([]string, len(iterations))
		for i, it := range iterations {
			out[i] = it.Id
		}
		return out
	}

	orgId, otherOrgId := insertOrg(t), insertOrg(t)
	first, second := insertScenario(t, orgId), insertScenario(t, orgId)
	firstIteration := insertIteration(t, orgId, first)
	secondIteration := insertIteration(t, orgId, second)
	otherOrgScenario := insertScenario(t, otherOrgId)
	otherOrgIteration := insertIteration(t, otherOrgId, otherOrgScenario)

	creds := models.Credentials{
		ActorIdentity:  models.Identity{UserId: "iterations-metadata-test-user"},
		OrganizationId: orgId,
		Roles:          []models.Role{models.ADMIN},
	}
	ctx := context.WithValue(bg, utils.ContextKeyCredentials, creds)
	uc := generateUsecaseWithCreds(testUsecases, creds)
	usecase := uc.NewScenarioIterationUsecase()

	t.Run("without a scenario, returns the whole organization and nothing else", func(t *testing.T) {
		iterations, err := usecase.ListScenarioIterationsMetadata(ctx, orgId, models.GetScenarioIterationFilters{})

		require.NoError(t, err)
		assert.ElementsMatch(t, []string{firstIteration, secondIteration}, ids(iterations))
		assert.NotContains(t, ids(iterations), otherOrgIteration)
	})

	t.Run("with a scenario, returns only that scenario", func(t *testing.T) {
		iterations, err := usecase.ListScenarioIterationsMetadata(ctx, orgId,
			models.GetScenarioIterationFilters{ScenarioId: first})

		require.NoError(t, err)
		assert.Equal(t, []string{firstIteration}, ids(iterations))
	})

	t.Run("another organization's scenario returns nothing", func(t *testing.T) {
		otherScenario := insertScenario(t, otherOrgId)
		insertIteration(t, otherOrgId, otherScenario)

		iterations, err := usecase.ListScenarioIterationsMetadata(ctx, orgId,
			models.GetScenarioIterationFilters{ScenarioId: otherScenario})

		require.NoError(t, err)
		assert.Empty(t, iterations)
	})

	t.Run("HTTP requests preserve optional filtering and reject invalid IDs", func(t *testing.T) {
		email := orgId.String() + "@metadata.test"
		adminUsecase := generateUsecaseWithCredForMarbleAdmin(testUsecases)
		userUsecase := adminUsecase.NewUserUseCase()
		_, err := userUsecase.AddUser(bg, models.CreateUser{
			Email:          email,
			OrganizationId: orgId,
			Role:           models.ADMIN,
		})
		require.NoError(t, err)

		e := httpexpect.Default(t, testServer.URL)
		token := e.POST("/token").
			WithHeader("Authorization", "Bearer "+firebaseDummyToken(email)).
			Expect().Status(http.StatusOK).
			JSON().Object().Value("access_token").String().NotEmpty().Raw()

		tests := []struct {
			name       string
			scenarioId string
			status     int
			iterations []string
		}{
			{name: "omitted", status: http.StatusOK, iterations: []string{firstIteration, secondIteration}},
			{name: "one scenario", scenarioId: first.String(), status: http.StatusOK, iterations: []string{firstIteration}},
			{name: "another organization", scenarioId: otherOrgScenario.String(), status: http.StatusOK, iterations: []string{}},
			{name: "malformed", scenarioId: "not-a-uuid", status: http.StatusBadRequest},
			{name: "explicit nil UUID", scenarioId: uuid.Nil.String(), status: http.StatusBadRequest},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				e := httpexpect.Default(t, testServer.URL)
				request := e.GET("/scenario-iterations/metadata").
					WithHeader("Authorization", "Bearer "+token)
				if tt.scenarioId != "" {
					request.WithQuery("scenario_id", tt.scenarioId)
				}
				response := request.Expect().Status(tt.status)
				if tt.status == http.StatusOK {
					response.JSON().Array().Length().IsEqual(len(tt.iterations))
					assert.ElementsMatch(t, tt.iterations, response.JSON().Path("$[*].id").Array().Raw())
				}
			})
		}
	})
}
