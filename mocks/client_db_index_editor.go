package mocks

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

type ClientDbIndexEditor struct {
	mock.Mock
}

func (editor *ClientDbIndexEditor) GetIndexesToCreate(
	ctx context.Context,
	organizationId uuid.UUID,
	scenarioIterationId string,
) (toCreate []models.ConcreteIndex, numPending int, err error) {
	args := editor.Called(ctx, organizationId, scenarioIterationId)
	return args.Get(0).([]models.ConcreteIndex), args.Int(1), args.Error(2)
}

func (editor *ClientDbIndexEditor) CreateIndexesAsync(
	ctx context.Context,
	organizationId uuid.UUID,
	indexes []models.ConcreteIndex,
) error {
	args := editor.Called(ctx, organizationId, indexes)
	return args.Error(0)
}

func (editor *ClientDbIndexEditor) CreateIndexesAsyncForScenarioWithCallback(
	ctx context.Context,
	organizationId uuid.UUID,
	indexes []models.ConcreteIndex,
	onSuccess models.OnCreateIndexesSuccess,
) error {
	calls := editor.Called(ctx, organizationId, indexes, onSuccess)
	return calls.Error(0)
}

func (editor *ClientDbIndexEditor) CreateIndexes(
	ctx context.Context,
	organizationId uuid.UUID,
	indexes []models.ConcreteIndex,
) error {
	args := editor.Called(ctx, organizationId, indexes)
	return args.Error(0)
}

func (editor *ClientDbIndexEditor) ListAllUniqueIndexes(ctx context.Context, organizationId uuid.UUID) ([]models.UnicityIndex, error) {
	args := editor.Called(ctx, organizationId)
	return args.Get(0).([]models.UnicityIndex), args.Error(1)
}

func (editor *ClientDbIndexEditor) ListAllIndexes(
	ctx context.Context,
	organizationId uuid.UUID,
	indexTypes ...models.IndexType,
) ([]models.ConcreteIndex, error) {
	callArgs := []any{ctx, organizationId}
	for _, indexType := range indexTypes {
		callArgs = append(callArgs, indexType)
	}
	args := editor.Called(callArgs...)
	if args.Error(1) != nil {
		return nil, args.Error(1)
	}
	if args.Get(0) == nil {
		return nil, nil
	}
	return args.Get(0).([]models.ConcreteIndex), args.Error(1)
}

func (editor *ClientDbIndexEditor) CreateUniqueIndex(
	ctx context.Context,
	exec repositories.Executor,
	organizationId uuid.UUID,
	index models.UnicityIndex,
) error {
	args := editor.Called(ctx, exec, organizationId, index)
	return args.Error(0)
}

func (editor *ClientDbIndexEditor) CreateUniqueIndexAsync(ctx context.Context,
	organizationId uuid.UUID, index models.UnicityIndex,
) error {
	args := editor.Called(ctx, organizationId, index)
	return args.Error(0)
}

func (editor *ClientDbIndexEditor) DeleteUniqueIndex(ctx context.Context, organizationId uuid.UUID, index models.UnicityIndex) error {
	args := editor.Called(ctx, organizationId, index)
	return args.Error(0)
}

func (editor *ClientDbIndexEditor) GetRequiredIndices(ctx context.Context, organizationId uuid.UUID) (required []models.AggregateQueryFamily, err error) {
	args := editor.Called(ctx, organizationId)
	return []models.AggregateQueryFamily{}, args.Error(1)
}
