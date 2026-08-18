package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type GraphIncrementalRepository struct {
	mock.Mock
}

func (r *GraphIncrementalRepository) UpsertGraphRows(
	ctx context.Context,
	exec repositories.Executor,
	recordType string,
	fields []models.Field,
	objectIds []string,
) (int64, error) {
	args := r.Called(ctx, exec, recordType, fields, objectIds)
	return args.Get(0).(int64), args.Error(1)
}

func (r *GraphIncrementalRepository) RetractGraphRows(
	ctx context.Context,
	exec repositories.Executor,
	recordType string,
	fields []models.Field,
	objectIds []string,
) (int64, error) {
	args := r.Called(ctx, exec, recordType, fields, objectIds)
	return args.Get(0).(int64), args.Error(1)
}
