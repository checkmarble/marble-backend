package mocks

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/stretchr/testify/mock"
)

type ClientDbIndexEditor struct {
	mock.Mock
}

func (editor *ClientDbIndexEditor) GetIndexesToCreate(
	ctx context.Context,
	organizationId string,
	scenarioIterationId string,
) (toCreate []models.ConcreteIndex, numPending int, err error) {
	args := editor.Called(ctx, organizationId, scenarioIterationId)
	return args.Get(0).([]models.ConcreteIndex), args.Int(1), args.Error(2)
}

func (editor *ClientDbIndexEditor) CreateIndexesAsync(
	ctx context.Context,
	organizationId string,
	indexes []models.ConcreteIndex,
) error {
	args := editor.Called(ctx, organizationId, indexes)
	return args.Error(0)
}

func (editor *ClientDbIndexEditor) ListAllUniqueIndexes(ctx context.Context, organizationId string) ([]models.UnicityIndex, error) {
	args := editor.Called(ctx, organizationId)
	return args.Get(0).([]models.UnicityIndex), args.Error(1)
}

func (editor *ClientDbIndexEditor) CreateUniqueIndex(
	ctx context.Context,
	exec repositories.Executor,
	organizationId string,
	index models.UnicityIndex,
) error {
	args := editor.Called(ctx, exec, organizationId, index)
	return args.Error(0)
}

func (editor *ClientDbIndexEditor) CreateUniqueIndexAsync(ctx context.Context, organizationId string, index models.UnicityIndex) error {
	args := editor.Called(ctx, organizationId, index)
	return args.Error(0)
}

func (editor *ClientDbIndexEditor) DeleteUniqueIndex(ctx context.Context, organizationId string, index models.UnicityIndex) error {
	args := editor.Called(ctx, organizationId, index)
	return args.Error(0)
}
