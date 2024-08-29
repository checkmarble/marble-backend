package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type UploadLogRepository struct {
	mock.Mock
}

func (r *UploadLogRepository) CreateUploadLog(ctx context.Context, exec repositories.Executor, log models.UploadLog) error {
	args := r.Called(ctx, exec, log)
	return args.Error(0)
}

func (r *UploadLogRepository) UpdateUploadLogStatus(ctx context.Context, exec repositories.Executor, input models.UpdateUploadLogStatusInput) (bool, error) {
	args := r.Called(ctx, exec, input)
	return true, args.Error(0)
}

func (r *UploadLogRepository) UploadLogById(ctx context.Context, exec repositories.Executor, id string) (models.UploadLog, error) {
	args := r.Called(ctx, exec, id)
	return args.Get(0).(models.UploadLog), args.Error(1)
}

func (r *UploadLogRepository) AllUploadLogsByStatus(ctx context.Context, exec repositories.Executor, status models.UploadStatus) ([]models.UploadLog, error) {
	args := r.Called(ctx, exec, status)
	return args.Get(0).([]models.UploadLog), args.Error(1)
}

func (r *UploadLogRepository) AllUploadLogsByTable(
	ctx context.Context,
	exec repositories.Executor,
	organizationId, tableName string,
) ([]models.UploadLog, error) {
	args := r.Called(ctx, exec, organizationId, tableName)
	return args.Get(0).([]models.UploadLog), args.Error(1)
}
