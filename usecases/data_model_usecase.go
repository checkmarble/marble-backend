package usecases

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/organization"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/google/uuid"
)

type DataModelUseCase struct {
	enforceSecurity            security.EnforceSecurityOrganization
	transactionFactory         executor_factory.TransactionFactory
	executorFactory            executor_factory.ExecutorFactory
	dataModelRepository        repositories.DataModelRepository
	populateOrganizationSchema organization.PopulateOrganizationSchema
}

func (usecase *DataModelUseCase) GetDataModel(ctx context.Context, organizationID string) (models.DataModel, error) {
	if err := usecase.enforceSecurity.ReadDataModel(); err != nil {
		return models.DataModel{}, err
	}

	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx,
		usecase.executorFactory.NewExecutor(), organizationID, true)
	if err != nil {
		return models.DataModel{}, err
	}
	return dataModel, nil
}

func (usecase *DataModelUseCase) CreateDataModelTable(ctx context.Context, organizationID, name, description string) (string, error) {
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
	err := usecase.transactionFactory.Transaction(ctx, func(tx repositories.Executor) error {
		err := usecase.dataModelRepository.CreateDataModelTable(ctx, tx, organizationID, tableID, name, description)
		if err != nil {
			return err
		}

		for _, field := range defaultFields {
			fieldID := uuid.New().String()
			err := usecase.dataModelRepository.CreateDataModelField(ctx, tx, tableID, fieldID, field)
			if err != nil {
				return err
			}
		}
		return usecase.populateOrganizationSchema.CreateTable(ctx, tx, organizationID, name)
	})
	if err != nil {
		return "", err
	}
	return tableID, nil
}

func (usecase *DataModelUseCase) UpdateDataModelTable(ctx context.Context, tableID, description string) error {
	if err := usecase.enforceSecurity.WriteDataModel(); err != nil {
		return err
	}
	return usecase.dataModelRepository.UpdateDataModelTable(ctx,
		usecase.executorFactory.NewExecutor(), tableID, description)
}

func (usecase *DataModelUseCase) CreateDataModelField(ctx context.Context, tableID string, field models.DataModelField) (string, error) {
	if err := usecase.enforceSecurity.WriteDataModel(); err != nil {
		return "", err
	}

	fieldID := uuid.New().String()
	err := usecase.transactionFactory.Transaction(ctx, func(tx repositories.Executor) error {
		err := usecase.dataModelRepository.CreateDataModelField(ctx, tx, tableID, fieldID, field)
		if err != nil {
			return err
		}

		table, err := usecase.dataModelRepository.GetDataModelTable(ctx, tx, tableID)
		if err != nil {
			return err
		}
		return usecase.populateOrganizationSchema.CreateField(ctx, tx, table.OrganizationID, table.Name, field)
	})
	if err != nil {
		return "", err
	}
	return fieldID, nil
}

func (usecase *DataModelUseCase) UpdateDataModelField(ctx context.Context, fieldID string, input models.UpdateDataModelFieldInput) error {
	exec := usecase.executorFactory.NewExecutor()
	if err := usecase.enforceSecurity.WriteDataModel(); err != nil {
		return err
	}

	field, err := usecase.dataModelRepository.GetDataModelField(ctx, exec, fieldID)
	if err != nil {
		return fmt.Errorf("repository.GetDataModelField error: %w", err)
	}

	if input.IsEnum != nil && *input.IsEnum &&
		(field.DataType != models.String &&
			field.DataType != models.Int &&
			field.DataType != models.Float) {
		return fmt.Errorf("enum fields can only be of type string or numeric: %w", models.BadParameterError)
	}

	return usecase.dataModelRepository.UpdateDataModelField(ctx, exec, fieldID, input)
}

func (usecase *DataModelUseCase) CreateDataModelLink(ctx context.Context, link models.DataModelLink) error {
	if err := usecase.enforceSecurity.WriteDataModel(); err != nil {
		return err
	}
	return usecase.dataModelRepository.CreateDataModelLink(ctx,
		usecase.executorFactory.NewExecutor(), link)
}

func (usecase *DataModelUseCase) DeleteDataModel(ctx context.Context, organizationID string) error {
	if err := usecase.enforceSecurity.WriteDataModel(); err != nil {
		return err
	}

	return usecase.transactionFactory.Transaction(ctx, func(tx repositories.Executor) error {
		err := usecase.dataModelRepository.DeleteDataModel(ctx, tx, organizationID)
		if err != nil {
			return err
		}

		schema, err := usecase.populateOrganizationSchema.OrganizationSchemaRepository.OrganizationSchemaOfOrganization(ctx, tx, organizationID)
		if err != nil {
			return err
		}

		return usecase.populateOrganizationSchema.OrganizationSchemaRepository.DeleteSchema(ctx, tx, schema.DatabaseSchema.Schema)
	})
}
