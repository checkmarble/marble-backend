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

func (r *SanctionCheckConfigRepository) GetSanctionCheckConfig(
	ctx context.Context,
	exec repositories.Executor,
	scenarioIterationId string,
) (*models.SanctionCheckConfig, error) {
	args := r.Called(ctx, exec, scenarioIterationId)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.SanctionCheckConfig), args.Error(1)
}

func (r *SanctionCheckConfigRepository) UpdateSanctionCheckConfig(
	ctx context.Context,
	exec repositories.Executor,
	scenarioIterationId string,
	sanctionCheckConfig models.UpdateSanctionCheckConfigInput,
) (models.SanctionCheckConfig, error) {
	args := r.Called(ctx, exec, scenarioIterationId, sanctionCheckConfig)
	return args.Get(0).(models.SanctionCheckConfig), args.Error(1)
}
