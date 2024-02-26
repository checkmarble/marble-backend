package mocks

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/stretchr/testify/mock"
)

type ClientDbIndexEditor struct {
	mock.Mock
}

func (editor *ClientDbIndexEditor) GetIndexesToCreate(
	ctx context.Context,
	scenarioIterationId string,
) (toCreate []models.ConcreteIndex, numPending int, err error) {
	args := editor.Called(ctx, scenarioIterationId)
	return args.Get(0).([]models.ConcreteIndex), args.Int(1), args.Error(2)
}

func (editor *ClientDbIndexEditor) CreateIndexesAsync(
	ctx context.Context,
	indexes []models.ConcreteIndex,
) error {
	args := editor.Called(ctx, indexes)
	return args.Error(0)
}

func (editor *ClientDbIndexEditor) ListAllUniqueIndexes(ctx context.Context) ([]models.UnicityIndex, error) {
	args := editor.Called(ctx)
	return args.Get(0).([]models.UnicityIndex), args.Error(1)
}

func (editor *ClientDbIndexEditor) CreateUniqueIndexAsync(ctx context.Context, index models.UnicityIndex) error {
	args := editor.Called(ctx, index)
	return args.Error(0)
}
