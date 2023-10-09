package mocks

import (
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type ScheduledExecutionRepository struct {
	mock.Mock
}

func (s *ScheduledExecutionRepository) GetScheduledExecution(tx repositories.Transaction, id string) (models.ScheduledExecution, error) {
	args := s.Called(tx, id)
	return args.Get(0).(models.ScheduledExecution), args.Error(1)
}

func (s *ScheduledExecutionRepository) ListScheduledExecutions(tx repositories.Transaction, filters models.ListScheduledExecutionsFilters) ([]models.ScheduledExecution, error) {
	args := s.Called(tx, filters)
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
