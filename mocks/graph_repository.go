package mocks

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type GraphRepository struct {
	mock.Mock
}

func (r *GraphRepository) FetchFields(
	ctx context.Context, exec repositories.Executor, recordType string, recordIds, fieldNames []string,
) ([]models.GraphRow, error) {
	args := r.Called(ctx, exec, recordType, recordIds, fieldNames)
	return args.Get(0).([]models.GraphRow), args.Error(1)
}

func (r *GraphRepository) FindByValues(
	ctx context.Context, exec repositories.Executor, recordType, fieldName string, values []string, perValueLimit int,
) ([]models.GraphMatch, error) {
	args := r.Called(ctx, exec, recordType, fieldName, values, perValueLimit)
	return args.Get(0).([]models.GraphMatch), args.Error(1)
}

func (r *GraphRepository) EstimateValueCount(
	ctx context.Context, exec repositories.Executor, recordType, fieldName, value string,
) (int, error) {
	args := r.Called(ctx, exec, recordType, fieldName, value)
	return args.Int(0), args.Error(1)
}

func (r *GraphRepository) GetNodeBatchMetadata(
	ctx context.Context, exec repositories.Executor, orgId uuid.UUID, records []models.ScoringRecordRef,
) ([]models.GraphResultNodeMetadata, error) {
	args := r.Called(ctx, exec, orgId, records)
	return args.Get(0).([]models.GraphResultNodeMetadata), args.Error(1)
}

func (r *GraphRepository) GetNodeBatchCaptions(
	ctx context.Context, exec repositories.Executor,
	captionFields map[string]string, records []models.ScoringRecordRef,
) ([]models.GraphResultNodeMetadata, error) {
	args := r.Called(ctx, exec, captionFields, records)
	return args.Get(0).([]models.GraphResultNodeMetadata), args.Error(1)
}
