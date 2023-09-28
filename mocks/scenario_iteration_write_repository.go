package mocks

import (
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type ScenarioIterationWriteRepository struct {
	mock.Mock
}

func (s *ScenarioIterationWriteRepository) CreateScenarioIterationAndRules(tx repositories.Transaction, organizationId string, scenarioIteration models.CreateScenarioIterationInput) (models.ScenarioIteration, error) {
	args := s.Called(tx, organizationId, scenarioIteration)
	return args.Get(0).(models.ScenarioIteration), args.Error(1)
}

func (s *ScenarioIterationWriteRepository) UpdateScenarioIteration(tx repositories.Transaction, scenarioIteration models.UpdateScenarioIterationInput) (models.ScenarioIteration, error) {
	args := s.Called(tx, scenarioIteration)
	return args.Get(0).(models.ScenarioIteration), args.Error(1)
}

func (s *ScenarioIterationWriteRepository) UpdateScenarioIterationVersion(tx repositories.Transaction, scenarioIterationId string, newVersion int) error {
	args := s.Called(tx, scenarioIterationId, newVersion)
	return args.Error(0)
}

func (s *ScenarioIterationWriteRepository) DeleteScenarioIteration(tx repositories.Transaction, scenarioIterationId string) error {
	args := s.Called(tx, scenarioIterationId)
	return args.Error(0)
}
