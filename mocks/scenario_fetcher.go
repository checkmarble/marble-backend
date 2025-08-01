package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type ScenarioFetcher struct {
	mock.Mock
}

func (m *ScenarioFetcher) FetchScenarioAndIteration(
	ctx context.Context,
	exec repositories.Executor,
	scenarioIterationId string,
) (models.ScenarioAndIteration, error) {
	args := m.Called(ctx, exec, scenarioIterationId)
	return args.Get(0).(models.ScenarioAndIteration), args.Error(1)
}

func (m *ScenarioFetcher) ListLiveIterationsAndNeighbors(ctx context.Context,
	exec repositories.Executor, orgId string,
) ([]models.ScenarioIteration, error) {
	args := m.Called(ctx, exec, orgId)
	return []models.ScenarioIteration{}, args.Error(1)
}
