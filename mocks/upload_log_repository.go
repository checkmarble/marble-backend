package mocks

import (
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type UploadLogRepository struct {
	mock.Mock
}

func (r *UploadLogRepository) CreateUploadLog(tx repositories.Transaction, log models.UploadLog) error {
	args := r.Called(tx, log)
	return args.Error(0)
}

func (r *UploadLogRepository) UpdateUploadLog(tx repositories.Transaction, input models.UpdateUploadLogInput) error {
	args := r.Called(tx, input)
	return args.Error(0)
}

func (r *UploadLogRepository) UploadLogById(tx repositories.Transaction, id string) (models.UploadLog, error) {
	args := r.Called(tx, id)
	return args.Get(0).(models.UploadLog), args.Error(1)
}

func (r *UploadLogRepository) AllUploadLogsByStatus(tx repositories.Transaction, status models.UploadStatus) ([]models.UploadLog, error) {
	args := r.Called(tx, status)
	return args.Get(0).([]models.UploadLog), args.Error(1)
}
