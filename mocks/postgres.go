package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
)

type Database struct {
	mock.Mock
}

func (m *Database) UserByEmail(ctx context.Context, email string) (models.User, error) {
	args := m.Called(ctx, email)
	return args.Get(0).(models.User), args.Error(1)
}

func (m *Database) GetOrganizationByID(ctx context.Context, organizationID string) (models.Organization, error) {
	args := m.Called(ctx, organizationID)
	return args.Get(0).(models.Organization), args.Error(1)
}

func (m *Database) GetApiKeyByKey(ctx context.Context, key string) (models.ApiKey, error) {
	args := m.Called(ctx, key)
	return args.Get(0).(models.ApiKey), args.Error(1)
}

func (m *Database) GetDataModel(ctx context.Context, organizationID string, fetchEnumValues bool) (models.DataModel, error) {
	args := m.Called(ctx, organizationID, fetchEnumValues)
	return args.Get(0).(models.DataModel), args.Error(1)
}

func (m *Database) GetDataModelField(ctx context.Context, fieldID string) (models.Field, error) {
	args := m.Called(ctx, fieldID)
	return args.Get(0).(models.Field), args.Error(1)
}

func (m *Database) CreateDataModelTable(ctx context.Context,
	organizationID, name, description string, defaultFields []models.DataModelField,
) (string, error) {
	args := m.Called(ctx, organizationID, name, description, defaultFields)
	return args.String(0), args.Error(1)
}

func (m *Database) CreateDataModelField(ctx context.Context, organizationID, tableID string, field models.DataModelField) (string, error) {
	args := m.Called(ctx, organizationID, tableID, field)
	return args.String(0), args.Error(1)
}

func (m *Database) DeleteDataModel(ctx context.Context, organizationID string) error {
	args := m.Called(ctx, organizationID)
	return args.Error(0)
}

func (m *Database) CreateDataModelLink(ctx context.Context, link models.DataModelLink) error {
	args := m.Called(ctx, link)
	return args.Error(0)
}

func (m *Database) UpdateDataModelTable(ctx context.Context, tableID, description string) error {
	args := m.Called(ctx, tableID, description)
	return args.Error(0)
}

func (m *Database) UpdateDataModelField(ctx context.Context, fieldID string, input models.UpdateDataModelFieldInput) error {
	args := m.Called(ctx, fieldID, input)
	return args.Error(0)
}
