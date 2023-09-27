package mocks

import (
	"github.com/stretchr/testify/mock"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
)

type DataModelRepository struct {
	mock.Mock
}

func (d *DataModelRepository) GetDataModel(tx repositories.Transaction, organizationId string) (models.DataModel, error) {
	args := d.Called(tx, organizationId)
	return args.Get(0).(models.DataModel), args.Error(1)
}

func (d *DataModelRepository) GetTables(tx repositories.Transaction, organizationID string) ([]models.DataModelTableField, error) {
	args := d.Called(tx, organizationID)
	return args.Get(0).([]models.DataModelTableField), args.Error(1)
}

func (d *DataModelRepository) GetLinks(tx repositories.Transaction, organizationID string) ([]models.DataModelLink, error) {
	args := d.Called(tx, organizationID)
	return args.Get(0).([]models.DataModelLink), args.Error(1)
}

func (d *DataModelRepository) DeleteDataModel(tx repositories.Transaction, organizationId string) error {
	args := d.Called(tx, organizationId)
	return args.Error(0)
}

func (d *DataModelRepository) CreateDataModel(tx repositories.Transaction, organizationId string, dataModel models.DataModel) error {
	args := d.Called(tx, organizationId, dataModel)
	return args.Error(0)
}

func (d *DataModelRepository) CreateDataModelTable(tx repositories.Transaction, organizationID, tableID, name, description string) error {
	args := d.Called(tx, organizationID, tableID, name, description)
	return args.Error(0)
}

func (d *DataModelRepository) UpdateDataModelTable(tx repositories.Transaction, tableID, description string) error {
	args := d.Called(tx, tableID, description)
	return args.Error(0)
}

func (d *DataModelRepository) CreateDataModelField(tx repositories.Transaction, tableID, fieldID string, field models.DataModelField) error {
	args := d.Called(tx, tableID, fieldID, field)
	return args.Error(0)
}

func (d *DataModelRepository) GetDataModelTable(tx repositories.Transaction, tableID string) (models.DataModelTable, error) {
	args := d.Called(tx, tableID)
	return args.Get(0).(models.DataModelTable), args.Error(1)
}

func (d *DataModelRepository) CreateDataModelLink(tx repositories.Transaction, link models.DataModelLink) error {
	args := d.Called(tx, link)
	return args.Error(0)
}

func (d *DataModelRepository) UpdateDataModelField(tx repositories.Transaction, fieldID, description string) error {
	args := d.Called(tx, fieldID, description)
	return args.Error(0)
}
