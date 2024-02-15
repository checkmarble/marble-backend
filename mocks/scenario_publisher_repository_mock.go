package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type ScenarioPublisherRepository struct {
	mock.Mock
}

func (s *ScenarioPublisherRepository) UpdateScenarioLiveIterationId(ctx context.Context,
	exec repositories.Executor, scenarioId string, scenarioIterationId *string,
) error {
	args := s.Called(exec, scenarioId, scenarioIterationId)
	return args.Error(0)
}

func (s *ScenarioPublisherRepository) ListScenarioIterations(ctx context.Context,
	exec repositories.Executor, organizationId string, filters models.GetScenarioIterationFilters,
) ([]models.ScenarioIteration, error) {
	args := s.Called(exec, organizationId, filters)
	return args.Get(0).([]models.ScenarioIteration), args.Error(1)
}

func (s *ScenarioPublisherRepository) UpdateScenarioIterationVersion(ctx context.Context,
	exec repositories.Executor, scenarioIterationId string, newVersion int,
) error {
	args := s.Called(exec, scenarioIterationId, newVersion)
	return args.Error(0)
}
