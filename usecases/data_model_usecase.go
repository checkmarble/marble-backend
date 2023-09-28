package usecases

import (
	"fmt"

	"github.com/google/uuid"

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

func (usecase *DataModelUseCase) GetDataModel(organizationID string) (models.DataModel, error) {
	if err := usecase.enforceSecurity.ReadDataModel(); err != nil {
		return models.DataModel{}, err
	}

	dataModel, err := usecase.dataModelRepository.GetDataModel(organizationID)
	if err != nil {
		return models.DataModel{}, err
	}
	return dataModel, nil
}

func (usecase *DataModelUseCase) CreateDataModelTable(organizationID, name, description string) (string, error) {
	if err := usecase.enforceSecurity.WriteDataModel(); err != nil {
		return "", err
	}

	defaultFields := []models.DataModelField{
		{
			Name:        "object_id",
			Description: fmt.Sprintf("required id on all objects in the %s table", name),
			Type:        models.String.String(),
		},
		{
			Name:        "updated_at",
			Description: fmt.Sprintf("required timestamp on all objects in the %s table", name),
			Type:        models.Timestamp.String(),
		},
	}

	tableID := uuid.New().String()
	err := usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		err := usecase.dataModelRepository.CreateDataModelTable(tx, organizationID, tableID, name, description)
		if err != nil {
			return err
		}

		for _, field := range defaultFields {
			fieldID := uuid.New().String()
			err := usecase.dataModelRepository.CreateDataModelField(tx, tableID, fieldID, field)
			if err != nil {
				return err
			}
		}
		return usecase.populateOrganizationSchema.CreateTable(tx, organizationID, name)
	})
	if err != nil {
		return "", err
	}
	return tableID, nil
}

func (usecase *DataModelUseCase) UpdateDataModelTable(tableID, description string) error {
	if err := usecase.enforceSecurity.WriteDataModel(); err != nil {
		return err
	}
	return usecase.dataModelRepository.UpdateDataModelTable(nil, tableID, description)
}

func (usecase *DataModelUseCase) CreateDataModelField(tableID string, field models.DataModelField) (string, error) {
	if err := usecase.enforceSecurity.WriteDataModel(); err != nil {
		return "", err
	}

	fieldID := uuid.New().String()
	err := usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		err := usecase.dataModelRepository.CreateDataModelField(tx, tableID, fieldID, field)
		if err != nil {
			return err
		}

		table, err := usecase.dataModelRepository.GetDataModelTable(tx, tableID)
		if err != nil {
			return err
		}
		return usecase.populateOrganizationSchema.CreateField(tx, table.OrganizationID, table.Name, field)
	})
	if err != nil {
		return "", err
	}
	return fieldID, nil
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

func (usecase *DataModelUseCase) DeleteSchema(organizationID string) error {
	if err := usecase.enforceSecurity.WriteDataModel(); err != nil {
		return err
	}

	return usecase.transactionFactory.Transaction(models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) error {
		err := usecase.dataModelRepository.DeleteDataModel(tx, organizationID)
		if err != nil {
			return err
		}

		schema, err := usecase.populateOrganizationSchema.OrganizationSchemaRepository.OrganizationSchemaOfOrganization(tx, organizationID)
		if err != nil {
			return err
		}

		return usecase.transactionFactory.Transaction(schema.DatabaseSchema, func(tx repositories.Transaction) error {
			return usecase.populateOrganizationSchema.OrganizationSchemaRepository.DeleteSchema(tx, schema.DatabaseSchema.Schema)
		})
	})
}
