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

func (s *ScenarioPublisherRepository) UpdateScenarioLiveIterationId(ctx context.Context, tx repositories.Transaction_deprec, scenarioId string, scenarioIterationId *string) error {
	args := s.Called(tx, scenarioId, scenarioIterationId)
	return args.Error(0)
}

func (s *ScenarioPublisherRepository) ListScenarioIterations(ctx context.Context, tx repositories.Transaction_deprec, organizationId string, filters models.GetScenarioIterationFilters) ([]models.ScenarioIteration, error) {
	args := s.Called(tx, organizationId, filters)
	return args.Get(0).([]models.ScenarioIteration), args.Error(1)
}

func (s *ScenarioPublisherRepository) UpdateScenarioIterationVersion(ctx context.Context, tx repositories.Transaction_deprec, scenarioIterationId string, newVersion int) error {
	args := s.Called(tx, scenarioIterationId, newVersion)
	return args.Error(0)
}
