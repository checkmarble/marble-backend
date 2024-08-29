package mocks

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/stretchr/testify/mock"
)

type ScheduledExecutionUsecaseRepository struct {
	mock.Mock
}

func (s *ScheduledExecutionUsecaseRepository) GetScheduledExecution(ctx context.Context,
	exec repositories.Executor, id string,
) (models.ScheduledExecution, error) {
	args := s.Called(exec, id)
	return args.Get(0).(models.ScheduledExecution), args.Error(1)
}

func (s *ScheduledExecutionUsecaseRepository) ListScheduledExecutions(ctx context.Context,
	exec repositories.Executor, filters models.ListScheduledExecutionsFilters,
) ([]models.ScheduledExecution, error) {
	args := s.Called(exec, filters)
	return args.Get(0).([]models.ScheduledExecution), args.Error(1)
}

func (s *ScheduledExecutionUsecaseRepository) CreateScheduledExecution(ctx context.Context,
	exec repositories.Executor, input models.CreateScheduledExecutionInput, newScheduledExecutionId string,
) error {
	args := s.Called(exec, input, newScheduledExecutionId)
	return args.Error(0)
}

func (s *ScheduledExecutionUsecaseRepository) UpdateScheduledExecutionStatus(
	ctx context.Context,
	exec repositories.Executor,
	updateScheduledEx models.UpdateScheduledExecutionStatusInput,
) (executed bool, err error) {
	args := s.Called(ctx, exec, updateScheduledEx)
	return true, args.Error(0)
}

func (s *ScheduledExecutionUsecaseRepository) GetScenarioById(ctx context.Context,
	exec repositories.Executor, scenarioId string,
) (models.Scenario, error) {
	args := s.Called(exec, scenarioId)
	return args.Get(0).(models.Scenario), args.Error(1)
}

func (s *ScheduledExecutionUsecaseRepository) GetScenarioIteration(ctx context.Context,
	exec repositories.Executor, scenarioIterationId string,
) (models.ScenarioIteration, error) {
	args := s.Called(exec, scenarioIterationId)
	return args.Get(0).(models.ScenarioIteration), args.Error(1)
}
