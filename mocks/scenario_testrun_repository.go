package mocks

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/stretchr/testify/mock"
)

type ScenarioTestrunRepository struct {
	mock.Mock
}

func (s *ScenarioTestrunRepository) CreateTestRun(ctx context.Context, tx repositories.Transaction, testrunID string,
	input models.ScenarioTestRunInput,
) error {
	args := s.Called(ctx, tx, testrunID, input)
	return args.Error(0)
}

func (s *ScenarioTestrunRepository) UpdateTestRunStatus(ctx context.Context, exec repositories.Executor,
	scenarioIterationID string, status models.TestrunStatus,
) error {
	args := s.Called(ctx, exec, scenarioIterationID, status)
	return args.Error(0)
}

func (s *ScenarioTestrunRepository) UpdateTestRunStatusByLiveVersion(ctx context.Context, exec repositories.Executor,
	liveVersionID string, status models.TestrunStatus,
) error {
	args := s.Called(ctx, exec, liveVersionID, status)
	return args.Error(0)
}

func (s *ScenarioTestrunRepository) GetTestRunByLiveVersionID(ctx context.Context, exec repositories.Executor,
	liveVersionID string,
) (*models.ScenarioTestRun, error) {
	args := s.Called(ctx, exec, liveVersionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ScenarioTestRun), args.Error(1)
}

func (s *ScenarioTestrunRepository) GetActiveTestRunByScenarioIterationID(ctx context.Context,
	exec repositories.Executor, scenarioID string,
) (*models.ScenarioTestRun, error) {
	args := s.Called(ctx, exec, scenarioID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ScenarioTestRun), args.Error(1)
}

func (s *ScenarioTestrunRepository) GetTestRunByID(ctx context.Context, exec repositories.Executor, testrunID string) (models.ScenarioTestRun, error) {
	args := s.Called(ctx, exec, testrunID)
	return args.Get(0).(models.ScenarioTestRun), args.Error(1)
}

func (s *ScenarioTestrunRepository) ListTestRunsByScenarioID(ctx context.Context,
	exec repositories.Executor, scenarioID string,
) ([]models.ScenarioTestRun, error) {
	args := s.Called(ctx, exec, scenarioID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ScenarioTestRun), args.Error(1)
}
