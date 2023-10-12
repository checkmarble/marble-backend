package mocks

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/stretchr/testify/mock"
)

type ScheduledExecutionUsecaseRepository struct {
	mock.Mock
}

func (s *ScheduledExecutionUsecaseRepository) GetScheduledExecution(tx repositories.Transaction, id string) (models.ScheduledExecution, error) {
	args := s.Called(tx, id)
	return args.Get(0).(models.ScheduledExecution), args.Error(1)
}

func (s *ScheduledExecutionUsecaseRepository) ListScheduledExecutions(tx repositories.Transaction, filters models.ListScheduledExecutionsFilters) ([]models.ScheduledExecution, error) {
	args := s.Called(tx, filters)
	return args.Get(0).([]models.ScheduledExecution), args.Error(1)
}

func (s *ScheduledExecutionUsecaseRepository) CreateScheduledExecution(tx repositories.Transaction, input models.CreateScheduledExecutionInput, newScheduledExecutionId string) error {
	args := s.Called(tx, input, newScheduledExecutionId)
	return args.Error(0)
}

func (s *ScheduledExecutionUsecaseRepository) UpdateScheduledExecution(tx repositories.Transaction, updateScheduledEx models.UpdateScheduledExecutionInput) error {
	args := s.Called(tx, updateScheduledEx)
	return args.Error(0)
}

func (s *ScheduledExecutionUsecaseRepository) GetScenarioById(tx repositories.Transaction, scenarioId string) (models.Scenario, error) {
	args := s.Called(tx, scenarioId)
	return args.Get(0).(models.Scenario), args.Error(1)
}

func (s *ScheduledExecutionUsecaseRepository) GetScenarioIteration(tx repositories.Transaction, scenarioIterationId string) (models.ScenarioIteration, error) {
	args := s.Called(tx, scenarioIterationId)
	return args.Get(0).(models.ScenarioIteration), args.Error(1)
}