package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type IngestedDataIndexesRepository struct {
	mock.Mock
}

func (m *IngestedDataIndexesRepository) ListAllValidIndexes(
	ctx context.Context,
	exec repositories.Executor,
	indexTypes ...models.IndexType,
) ([]models.ConcreteIndex, error) {
	callArgs := []any{ctx, exec}
	for _, indexType := range indexTypes {
		callArgs = append(callArgs, indexType)
	}
	args := m.Called(callArgs...)
	if args.Error(1) != nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ConcreteIndex), args.Error(1)
}

func (m *IngestedDataIndexesRepository) ListAllIndexes(
	ctx context.Context,
	exec repositories.Executor,
	indexTypes ...models.IndexType,
) ([]models.ConcreteIndex, error) {
	callArgs := []any{ctx, exec}
	for _, indexType := range indexTypes {
		callArgs = append(callArgs, indexType)
	}
	args := m.Called(callArgs...)
	if args.Error(1) != nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ConcreteIndex), args.Error(1)
}

func (m *IngestedDataIndexesRepository) CreateIndexesWithCallback(ctx context.Context, exec repositories.Executor,
	indexes []models.ConcreteIndex, onSuccess models.OnCreateIndexesSuccess,
) error {
	calls := m.Called(ctx, exec, indexes, onSuccess)
	return calls.Error(0)
}

func (m *IngestedDataIndexesRepository) CreateIndexesAsync(
	ctx context.Context,
	exec repositories.Executor, indexes []models.ConcreteIndex,
) error {
	args := m.Called(ctx, exec, indexes)
	return args.Error(0)
}

func (m *IngestedDataIndexesRepository) CreateIndexes(
	ctx context.Context,
	exec repositories.Executor, indexes []models.ConcreteIndex,
) error {
	args := m.Called(ctx, exec, indexes)
	return args.Error(0)
}

func (m *IngestedDataIndexesRepository) CountPendingIndexes(
	ctx context.Context,
	exec repositories.Executor,
) (int, error) {
	args := m.Called(ctx, exec)
	return args.Int(0), args.Error(1)
}

func (m *IngestedDataIndexesRepository) ListAllUniqueIndexes(
	ctx context.Context,
	exec repositories.Executor,
) ([]models.UnicityIndex, error) {
	args := m.Called(ctx, exec)
	if args.Error(1) != nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.UnicityIndex), args.Error(1)
}

func (m *IngestedDataIndexesRepository) CreateUniqueIndexAsync(
	ctx context.Context,
	exec repositories.Executor,
	index models.UnicityIndex,
) error {
	args := m.Called(ctx, exec, index)
	return args.Error(0)
}

func (m *IngestedDataIndexesRepository) CreateUniqueIndex(
	ctx context.Context,
	exec repositories.Executor,
	index models.UnicityIndex,
) error {
	args := m.Called(ctx, exec, index)
	return args.Error(0)
}

func (m *IngestedDataIndexesRepository) DeleteUniqueIndex(
	ctx context.Context,
	exec repositories.Executor,
	index models.UnicityIndex,
) error {
	args := m.Called(ctx, exec, index)
	return args.Error(0)
}

func (m *IngestedDataIndexesRepository) ListIndicesPendingCreation(
	ctx context.Context,
	exec repositories.Executor,
) ([]string, error) {
	args := m.Called(ctx, exec)
	return args.Get(0).([]string), args.Error(1)
}

func (m *IngestedDataIndexesRepository) ListInvalidIndices(
	ctx context.Context,
	exec repositories.Executor,
) ([]string, error) {
	args := m.Called(ctx, exec)
	return args.Get(0).([]string), args.Error(1)
}

func (m *IngestedDataIndexesRepository) DeleteIndex(
	ctx context.Context,
	exec repositories.Executor,
	indexName string,
) error {
	args := m.Called(ctx, exec)
	return args.Error(0)
}
