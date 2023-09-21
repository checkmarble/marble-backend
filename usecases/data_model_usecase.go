package usecases

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/organization"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/transaction"
)

type DataModelUseCase struct {
	enforceSecurity            security.EnforceSecurityOrganization
	transactionFactory         transaction.TransactionFactory
	dataModelRepository        repositories.DataModelRepository
	populateOrganizationSchema organization.PopulateOrganizationSchema
}

func (usecase *DataModelUseCase) CreateDataModelTable(organizationID, name, description string) error {
	if err := usecase.enforceSecurity.WriteDataModel(); err != nil {
		return err
	}

	return usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		if err := usecase.dataModelRepository.CreateDataModelTable(tx, organizationID, name, description); err != nil {
			return err
		}
		return usecase.populateOrganizationSchema.CreateTable(tx, organizationID, name)
	})
}

func (usecase *DataModelUseCase) UpdateDataModelTable(tableID, description string) error {
	if err := usecase.enforceSecurity.WriteDataModel(); err != nil {
		return err
	}
	return usecase.dataModelRepository.UpdateDataModelTable(nil, tableID, description)
}

func (usecase *DataModelUseCase) CreateDataModelField(tableID string, field models.DataModelField) error {
	if err := usecase.enforceSecurity.WriteDataModel(); err != nil {
		return err
	}

	return usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		if err := usecase.dataModelRepository.CreateDataModelField(tx, tableID, field); err != nil {
			return err
		}

		table, err := usecase.dataModelRepository.GetDataModelTable(tx, tableID)
		if err != nil {
			return err
		}
		return usecase.populateOrganizationSchema.CreateField(tx, table.OrganizationID, table.Name, field)
	})
}

func (usecase *DataModelUseCase) UpdateDataModelField(fieldID, description string) error {
	if err := usecase.enforceSecurity.WriteDataModel(); err != nil {
		return err
	}
	return usecase.dataModelRepository.UpdateDataModelField(nil, fieldID, description)
}

func (usecase *DataModelUseCase) CreateDataModelLink(link models.DataModelLink) error {
	if err := usecase.enforceSecurity.WriteDataModel(); err != nil {
		return err
	}
	return usecase.dataModelRepository.CreateDataModelLink(nil, link)
}
