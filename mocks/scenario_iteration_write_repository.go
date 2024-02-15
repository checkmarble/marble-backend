package mocks

import (
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type ScenarioIterationWriteRepository struct {
	mock.Mock
}

func (s *ScenarioIterationWriteRepository) CreateScenarioIterationAndRules(
	exec repositories.Executor, organizationId string, scenarioIteration models.CreateScenarioIterationInput,
) (models.ScenarioIteration, error) {
	args := s.Called(exec, organizationId, scenarioIteration)
	return args.Get(0).(models.ScenarioIteration), args.Error(1)
}

func (s *ScenarioIterationWriteRepository) UpdateScenarioIteration(exec repositories.Executor,
	scenarioIteration models.UpdateScenarioIterationInput,
) (models.ScenarioIteration, error) {
	args := s.Called(exec, scenarioIteration)
	return args.Get(0).(models.ScenarioIteration), args.Error(1)
}

func (s *ScenarioIterationWriteRepository) UpdateScenarioIterationVersion(
	exec repositories.Executor, scenarioIterationId string, newVersion int,
) error {
	args := s.Called(exec, scenarioIterationId, newVersion)
	return args.Error(0)
}

func (s *ScenarioIterationWriteRepository) DeleteScenarioIteration(exec repositories.Executor, scenarioIterationId string) error {
	args := s.Called(exec, scenarioIterationId)
	return args.Error(0)
}
