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

func (d *DataModelRepository) DeleteDataModel(tx repositories.Transaction, organizationId string) error {
	args := d.Called(tx, organizationId)
	return args.Error(0)
}

func (d *DataModelRepository) CreateDataModel(tx repositories.Transaction, organizationId string, dataModel models.DataModel) error {
	args := d.Called(tx, organizationId, dataModel)
	return args.Error(0)
}
