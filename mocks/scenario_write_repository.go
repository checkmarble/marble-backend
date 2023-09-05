package mocks

import (
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type ScenarioWriteRepository struct {
	mock.Mock
}

func (s *ScenarioWriteRepository) CreateScenario(tx repositories.Transaction, scenario models.CreateScenarioInput, newScenarioId string) error {
	args := s.Called(tx, scenario, newScenarioId)
	return args.Error(0)
}

func (s *ScenarioWriteRepository) UpdateScenario(tx repositories.Transaction, scenario models.UpdateScenarioInput) error {
	args := s.Called(tx, scenario)
	return args.Error(0)
}

func (s *ScenarioWriteRepository) UpdateScenarioLiveIterationId(tx repositories.Transaction, scenarioId string, scenarioIterationId *string) error {
	args := s.Called(tx, scenarioId, scenarioIterationId)
	return args.Error(0)
}
