package mocks

import (
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type ScenarioPublisherRepository struct {
	mock.Mock
}

func (s *ScenarioPublisherRepository) UpdateScenarioLiveIterationId(tx repositories.Transaction, scenarioId string, scenarioIterationId *string) error {
	args := s.Called(tx, scenarioId, scenarioIterationId)
	return args.Error(0)
}

func (s *ScenarioPublisherRepository) ListScenarioIterations(tx repositories.Transaction, organizationId string, filters models.GetScenarioIterationFilters) ([]models.ScenarioIteration, error) {
	args := s.Called(tx, organizationId, filters)
	return args.Get(0).([]models.ScenarioIteration), args.Error(1)
}

func (s *ScenarioPublisherRepository) UpdateScenarioIterationVersion(tx repositories.Transaction, scenarioIterationId string, newVersion int) error {
	args := s.Called(tx, scenarioIterationId, newVersion)
	return args.Error(0)
}
