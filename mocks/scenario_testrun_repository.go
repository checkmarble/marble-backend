package mocks

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/stretchr/testify/mock"
)

type ScenatioTestrunRepository struct {
	mock.Mock
}

func (s *ScenatioTestrunRepository) CreateTestRun(ctx context.Context, tx repositories.Transaction, testrunID string,
	input models.ScenarioTestRunInput,
) error {
	args := s.Called(ctx, tx, testrunID, input)
	return args.Error(0)
}

func (s *ScenatioTestrunRepository) GetByScenarioIterationID(ctx context.Context,
	exec repositories.Executor, scenarioID string,
) (models.ScenarioTestRun, error) {
	args := s.Called(ctx, exec, scenarioID)
	return args.Get(0).(models.ScenarioTestRun), args.Error(1)
}

func (s *ScenatioTestrunRepository) GetByID(ctx context.Context, exec repositories.Executor, testrunID string) (models.ScenarioTestRun, error) {
	args := s.Called(ctx, exec, testrunID)
	return args.Get(0).(models.ScenarioTestRun), args.Error(1)
}
