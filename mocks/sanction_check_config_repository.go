package mocks

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/stretchr/testify/mock"
)

type SanctionCheckConfigRepository struct {
	mock.Mock
}

func (r *SanctionCheckConfigRepository) ListSanctionCheckConfigs(
	ctx context.Context,
	exec repositories.Executor,
	scenarioIterationId string,
) ([]models.SanctionCheckConfig, error) {
	args := r.Called(ctx, exec, scenarioIterationId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.SanctionCheckConfig), args.Error(1)
}

func (r *SanctionCheckConfigRepository) GetSanctionCheckConfig(
	ctx context.Context,
	exec repositories.Executor,
	scenarioIterationId string,
	sanctionCheckId string,
) (*models.SanctionCheckConfig, error) {
	args := r.Called(ctx, exec, scenarioIterationId, sanctionCheckId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.SanctionCheckConfig), args.Error(1)
}

func (r *SanctionCheckConfigRepository) CreateSanctionCheckConfig(
	ctx context.Context,
	exec repositories.Executor,
	scenarioIterationId string,
	sanctionCheckConfig models.UpdateSanctionCheckConfigInput,
) (models.SanctionCheckConfig, error) {
	args := r.Called(ctx, exec, scenarioIterationId, sanctionCheckConfig)
	return args.Get(0).(models.SanctionCheckConfig), args.Error(1)
}

func (r *SanctionCheckConfigRepository) UpdateSanctionCheckConfig(
	ctx context.Context,
	exec repositories.Executor,
	scenarioIterationId string,
	sanctionCheckId string,
	sanctionCheckConfig models.UpdateSanctionCheckConfigInput,
) (models.SanctionCheckConfig, error) {
	args := r.Called(ctx, exec, scenarioIterationId, sanctionCheckId, sanctionCheckConfig)
	return args.Get(0).(models.SanctionCheckConfig), args.Error(1)
}

func (r *SanctionCheckConfigRepository) DeleteSanctionCheckConfig(
	ctx context.Context,
	exec repositories.Executor,
	scenarioIterationId string,
) error {
	args := r.Called(ctx, exec, scenarioIterationId)
	return args.Error(0)
}
