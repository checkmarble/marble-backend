package mocks

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/stretchr/testify/mock"
)

type ScheduledExecutionRepository struct {
	mock.Mock
}

func (s *ScheduledExecutionRepository) GetScheduledExecution(tx repositories.Transaction, id string) (models.ScheduledExecution, error) {
	args := s.Called(tx, id)
	return args.Get(0).(models.ScheduledExecution), args.Error(1)
}

func (s *ScheduledExecutionRepository) ListScheduledExecutionsOfScenario(tx repositories.Transaction, scenarioId string) ([]models.ScheduledExecution, error) {
	args := s.Called(tx, scenarioId)
	return args.Get(0).([]models.ScheduledExecution), args.Error(1)
}

func (s *ScheduledExecutionRepository) ListScheduledExecutionsOfOrganization(tx repositories.Transaction, organizationId string) ([]models.ScheduledExecution, error) {
	args := s.Called(tx, organizationId)
	return args.Get(0).([]models.ScheduledExecution), args.Error(1)
}

func (s *ScheduledExecutionRepository) CreateScheduledExecution(tx repositories.Transaction, input models.CreateScheduledExecutionInput, newScheduledExecutionId string) error {
	args := s.Called(tx, input, newScheduledExecutionId)
	return args.Error(0)
}

func (s *ScheduledExecutionRepository) UpdateScheduledExecution(tx repositories.Transaction, updateScheduledEx models.UpdateScheduledExecutionInput) error {
	args := s.Called(tx, updateScheduledEx)
	return args.Error(0)
}
