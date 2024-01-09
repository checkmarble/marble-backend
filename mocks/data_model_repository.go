package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type DataModelRepository struct {
	mock.Mock
}

func (d *DataModelRepository) GetDataModel(ctx context.Context, organizationId string, fetchEnumValues bool) (models.DataModel, error) {
	args := d.Called(organizationId, fetchEnumValues)
	return args.Get(0).(models.DataModel), args.Error(1)
}

func (d *DataModelRepository) GetTablesAndFields(ctx context.Context, tx repositories.Transaction, organizationID string) ([]models.DataModelTableField, error) {
	args := d.Called(tx, organizationID)
	return args.Get(0).([]models.DataModelTableField), args.Error(1)
}

func (d *DataModelRepository) DeleteDataModel(ctx context.Context, tx repositories.Transaction, organizationId string) error {
	args := d.Called(tx, organizationId)
	return args.Error(0)
}

func (d *DataModelRepository) CreateDataModelTable(ctx context.Context, tx repositories.Transaction, organizationID, tableID, name, description string) error {
	args := d.Called(tx, organizationID, tableID, name, description)
	return args.Error(0)
}

func (d *DataModelRepository) UpdateDataModelTable(ctx context.Context, tx repositories.Transaction, tableID, description string) error {
	args := d.Called(tx, tableID, description)
	return args.Error(0)
}

func (d *DataModelRepository) CreateDataModelField(ctx context.Context, tx repositories.Transaction, tableID, fieldID string, field models.DataModelField) error {
	args := d.Called(tx, tableID, fieldID, field)
	return args.Error(0)
}

func (d *DataModelRepository) GetDataModelTable(ctx context.Context, tx repositories.Transaction, tableID string) (models.DataModelTable, error) {
	args := d.Called(tx, tableID)
	return args.Get(0).(models.DataModelTable), args.Error(1)
}

func (d *DataModelRepository) CreateDataModelLink(ctx context.Context, tx repositories.Transaction, link models.DataModelLink) error {
	args := d.Called(tx, link)
	return args.Error(0)
}

func (d *DataModelRepository) UpdateDataModelField(ctx context.Context, tx repositories.Transaction, fieldID, description string) error {
	args := d.Called(tx, fieldID, description)
	return args.Error(0)
}
