package mocks

import (
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type ScenarioIterationReadRepository struct {
	mock.Mock
}

func (s *ScenarioIterationReadRepository) GetScenarioIteration(tx repositories.Transaction, scenarioIterationId string) (models.ScenarioIteration, error) {
	args := s.Called(tx, scenarioIterationId)
	return args.Get(0).(models.ScenarioIteration), args.Error(1)
}

func (s *ScenarioIterationReadRepository) ListScenarioIterations(tx repositories.Transaction, organizationId string, filters models.GetScenarioIterationFilters) ([]models.ScenarioIteration, error) {
	args := s.Called(tx, organizationId, filters)
	return args.Get(0).([]models.ScenarioIteration), args.Error(1)
}
