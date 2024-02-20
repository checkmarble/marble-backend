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
