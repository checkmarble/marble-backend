package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type ScenarioIterationReadWriterRepository struct {
	ScenarioIterationReadRepository
	ScenarioIterationWriteRepository
}

type ScenarioIterationReadRepository struct {
	mock.Mock
}

func (s *ScenarioIterationReadRepository) ListScenarioIterationsMetadata(
	ctx context.Context,
	exec repositories.Executor,
	organizationId uuid.UUID,
	filters models.GetScenarioIterationFilters,
) ([]models.ScenarioIterationMetadata, error) {
	args := s.Called(ctx, exec, organizationId, filters)

	return args.Get(0).([]models.ScenarioIterationMetadata), args.Error(1)
}

func (s *ScenarioIterationReadRepository) GetScenarioIteration(ctx context.Context, exec repositories.Executor,
	scenarioIterationId string,
	useCache bool,
) (models.ScenarioIteration, error) {
	args := s.Called(ctx, exec, scenarioIterationId, useCache)
	return args.Get(0).(models.ScenarioIteration), args.Error(1)
}

func (s *ScenarioIterationReadRepository) ListScenarioIterations(ctx context.Context, exec repositories.Executor,
	organizationId uuid.UUID, filters models.GetScenarioIterationFilters,
) ([]models.ScenarioIteration, error) {
	args := s.Called(exec, organizationId, filters)
	return args.Get(0).([]models.ScenarioIteration), args.Error(1)
}
