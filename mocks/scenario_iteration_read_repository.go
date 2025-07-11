package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type ScenarioIterationReadRepository struct {
	mock.Mock
}

func (s *ScenarioIterationReadRepository) GetScenarioIteration(ctx context.Context, exec repositories.Executor,
	scenarioIterationId string,
) (models.ScenarioIteration, error) {
	args := s.Called(exec, scenarioIterationId)
	return args.Get(0).(models.ScenarioIteration), args.Error(1)
}

func (s *ScenarioIterationReadRepository) ListScenarioIterations(ctx context.Context, exec repositories.Executor,
	organizationId string, filters models.GetScenarioIterationFilters,
) ([]models.ScenarioIteration, error) {
	args := s.Called(exec, organizationId, filters)
	return args.Get(0).([]models.ScenarioIteration), args.Error(1)
}
