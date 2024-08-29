package mocks

import (
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type UploadLogRepository struct {
	mock.Mock
}

func (r *UploadLogRepository) CreateUploadLog(exec repositories.Executor, log models.UploadLog) error {
	args := r.Called(exec, log)
	return args.Error(0)
}

func (r *UploadLogRepository) UpdateUploadLogStatus(exec repositories.Executor, input models.UpdateUploadLogStatusInput) error {
	args := r.Called(exec, input)
	return args.Error(0)
}

func (r *UploadLogRepository) UploadLogById(exec repositories.Executor, id string) (models.UploadLog, error) {
	args := r.Called(exec, id)
	return args.Get(0).(models.UploadLog), args.Error(1)
}

func (r *UploadLogRepository) AllUploadLogsByStatus(exec repositories.Executor, status models.UploadStatus) ([]models.UploadLog, error) {
	args := r.Called(exec, status)
	return args.Get(0).([]models.UploadLog), args.Error(1)
}

func (r *UploadLogRepository) AllUploadLogsByTable(exec repositories.Executor,
	organizationId, tableName string,
) ([]models.UploadLog, error) {
	args := r.Called(exec, organizationId, tableName)
	return args.Get(0).([]models.UploadLog), args.Error(1)
}
