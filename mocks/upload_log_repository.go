package mocks

import (
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type UploadLogRepository struct {
	mock.Mock
}

func (r *UploadLogRepository) CreateUploadLog(tx repositories.Transaction_deprec, log models.UploadLog) error {
	args := r.Called(tx, log)
	return args.Error(0)
}

func (r *UploadLogRepository) UpdateUploadLog(tx repositories.Transaction_deprec, input models.UpdateUploadLogInput) error {
	args := r.Called(tx, input)
	return args.Error(0)
}

func (r *UploadLogRepository) UploadLogById(tx repositories.Transaction_deprec, id string) (models.UploadLog, error) {
	args := r.Called(tx, id)
	return args.Get(0).(models.UploadLog), args.Error(1)
}

func (r *UploadLogRepository) AllUploadLogsByStatus(tx repositories.Transaction_deprec, status models.UploadStatus) ([]models.UploadLog, error) {
	args := r.Called(tx, status)
	return args.Get(0).([]models.UploadLog), args.Error(1)
}

func (r *UploadLogRepository) AllUploadLogsByTable(tx repositories.Transaction_deprec, organizationId, tableName string) ([]models.UploadLog, error) {
	args := r.Called(tx, organizationId, tableName)
	return args.Get(0).([]models.UploadLog), args.Error(1)
}
