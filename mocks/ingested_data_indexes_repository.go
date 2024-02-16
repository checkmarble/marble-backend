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
) ([]models.ConcreteIndex, error) {
	args := m.Called(ctx, exec)
	return args.Get(0).([]models.ConcreteIndex), args.Error(1)
}

func (m *IngestedDataIndexesRepository) CreateIndexesAsync(
	ctx context.Context,
	exec repositories.Executor, indexes []models.ConcreteIndex,
) (int, error) {
	args := m.Called(ctx, exec, indexes)
	return args.Int(0), args.Error(1)
}
