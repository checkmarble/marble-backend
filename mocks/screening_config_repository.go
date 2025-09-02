package mocks

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/stretchr/testify/mock"
)

type ScreeningConfigRepository struct {
	mock.Mock
}

func (r *ScreeningConfigRepository) ListScreeningConfigs(
	ctx context.Context,
	exec repositories.Executor,
	scenarioIterationId string,
	useCache bool,
) ([]models.ScreeningConfig, error) {
	args := r.Called(ctx, exec, scenarioIterationId, useCache)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ScreeningConfig), args.Error(1)
}

func (r *ScreeningConfigRepository) GetScreeningConfig(
	ctx context.Context,
	exec repositories.Executor,
	scenarioIterationId string,
	screeningId string,
) (models.ScreeningConfig, error) {
	args := r.Called(ctx, exec, scenarioIterationId, screeningId)
	if args.Get(0) == nil {
		return models.ScreeningConfig{}, args.Error(1)
	}
	return args.Get(0).(models.ScreeningConfig), args.Error(1)
}

func (r *ScreeningConfigRepository) CreateScreeningConfig(
	ctx context.Context,
	exec repositories.Executor,
	scenarioIterationId string,
	screeningConfig models.UpdateScreeningConfigInput,
) (models.ScreeningConfig, error) {
	args := r.Called(ctx, exec, scenarioIterationId, screeningConfig)
	return args.Get(0).(models.ScreeningConfig), args.Error(1)
}

func (r *ScreeningConfigRepository) UpdateScreeningConfig(
	ctx context.Context,
	exec repositories.Executor,
	scenarioIterationId string,
	screeningId string,
	screeningConfig models.UpdateScreeningConfigInput,
) (models.ScreeningConfig, error) {
	args := r.Called(ctx, exec, scenarioIterationId, screeningId, screeningConfig)
	return args.Get(0).(models.ScreeningConfig), args.Error(1)
}

func (r *ScreeningConfigRepository) DeleteScreeningConfig(
	ctx context.Context,
	exec repositories.Executor,
	scenarioIterationId, configId string,
) error {
	args := r.Called(ctx, exec, scenarioIterationId, configId)
	return args.Error(0)
}
