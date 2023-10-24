package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
)

type DataModelUseCase struct {
	mock.Mock
}

func (m *DataModelUseCase) GetDataModel(ctx context.Context, organizationID string) (models.DataModel, error) {
	args := m.Called(ctx, organizationID)
	return args.Get(0).(models.DataModel), args.Error(1)
}

func (m *DataModelUseCase) CreateTable(ctx context.Context, organizationID, name, description string) (string, error) {
	args := m.Called(ctx, organizationID, name, description)
	return args.String(0), args.Error(1)
}

func (m *DataModelUseCase) UpdateDataModelTable(ctx context.Context, tableID, description string) error {
	args := m.Called(ctx, tableID, description)
	return args.Error(0)
}

func (m *DataModelUseCase) CreateField(ctx context.Context, organizationID, tableID string, field models.DataModelField) (string, error) {
	args := m.Called(ctx, organizationID, tableID, field)
	return args.String(0), args.Error(1)
}

func (m *DataModelUseCase) UpdateDataModelField(ctx context.Context, fieldID string, input models.UpdateDataModelFieldInput) error {
	args := m.Called(ctx, fieldID, input)
	return args.Error(0)
}

func (m *DataModelUseCase) CreateDataModelLink(ctx context.Context, link models.DataModelLink) error {
	args := m.Called(ctx, link)
	return args.Error(0)
}

func (m *DataModelUseCase) DeleteDataModel(ctx context.Context, organizationID string) error {
	args := m.Called(ctx, organizationID)
	return args.Error(0)
}
