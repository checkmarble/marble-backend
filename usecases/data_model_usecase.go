package usecases

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type DataModelUseCase struct {
	clientDbIndexEditor          clientDbIndexEditor
	dataModelRepository          repositories.DataModelRepository
	enforceSecurity              security.EnforceSecurityOrganization
	executorFactory              executor_factory.ExecutorFactory
	organizationSchemaRepository repositories.OrganizationSchemaRepository
	transactionFactory           executor_factory.TransactionFactory
}

func (usecase *DataModelUseCase) GetDataModel(ctx context.Context, organizationID string) (models.DataModel, error) {
	if err := usecase.enforceSecurity.ReadDataModel(); err != nil {
		return models.DataModel{}, err
	}

	dataModel, err := usecase.dataModelRepository.GetDataModel(
		ctx,
		usecase.executorFactory.NewExecutor(),
		organizationID,
		true,
	)
	if err != nil {
		return models.DataModel{}, err
	}

	uniqueIndexes, err := usecase.clientDbIndexEditor.ListAllUniqueIndexes(ctx)
	if err != nil {
		return models.DataModel{}, err
	}
	dataModel = addUnicityConstraintStatusToDataModel(dataModel, uniqueIndexes)

	return dataModel, nil
}

func addUnicityConstraintStatusToDataModel(dataModel models.DataModel, uniqueIndexes []models.UnicityIndex) models.DataModel {
	for _, index := range uniqueIndexes {
		// here we only care about single fields with a unicity constraint
		if len(index.Fields) != 1 {
			continue
		}
		table, ok := dataModel.Tables[index.TableName]
		if !ok {
			continue
		}
		field, ok := table.Fields[index.Fields[0]]
		if !ok {
			continue
		}

		if field.Name == index.Fields[0] {
			if index.CreationInProcess && field.UnicityConstraint != models.ActiveUniqueConstraint {
				field.UnicityConstraint = models.PendingUniqueConstraint
			} else {
				field.UnicityConstraint = models.ActiveUniqueConstraint
			}
			// cannot directly modify the struct field in the map, so we need to reassign it
			dataModel.Tables[index.TableName].Fields[index.Fields[0]] = field
		}
	}
	return dataModel
}

func (usecase *DataModelUseCase) CreateDataModelTable(ctx context.Context, organizationId, name, description string) (string, error) {
	if err := usecase.enforceSecurity.WriteDataModel(organizationId); err != nil {
		return "", err
	}
	tableId := uuid.New().String()

	defaultFields := []models.CreateFieldInput{
		{
			TableId:     tableId,
			DataType:    models.String,
			Description: fmt.Sprintf("required id on all objects in the %s table", name),
			Name:        "object_id",
			Nullable:    false,
		},
		{
			TableId:     tableId,
			DataType:    models.Timestamp,
			Description: fmt.Sprintf("required timestamp on all objects in the %s table", name),
			Name:        "updated_at",
			Nullable:    false,
		},
	}

	err := usecase.transactionFactory.Transaction(ctx, func(tx repositories.Executor) error {
		err := usecase.dataModelRepository.CreateDataModelTable(ctx, tx, organizationId, tableId, name, description)
		if err != nil {
			return err
		}

		for _, field := range defaultFields {
			fieldId := uuid.New().String()
			err := usecase.dataModelRepository.CreateDataModelField(ctx, tx, fieldId, field)
			if err != nil {
				return err
			}
		}

		// if it returns an error, rolls back the other transaction
		return usecase.transactionFactory.TransactionInOrgSchema(ctx, organizationId, func(orgTx repositories.Executor) error {
			if err := usecase.organizationSchemaRepository.CreateSchemaIfNotExists(ctx, orgTx); err != nil {
				return err
			}
			if err := usecase.organizationSchemaRepository.CreateTable(ctx, orgTx, name); err != nil {
				return err
			}
			// the unique index on object_id will serve both to enforce unicity and to speed up ingestion queries
			return usecase.clientDbIndexEditor.CreateUniqueIndex(
				ctx,
				orgTx,
				models.UnicityIndex{
					TableName: models.TableName(name),
					Fields:    []models.FieldName{"object_id"},
					Included:  []models.FieldName{"updated_at", "id"},
				})
		})
	})
	return tableId, err
}

func (usecase *DataModelUseCase) UpdateDataModelTable(ctx context.Context, tableID, description string) error {
	if table, err := usecase.dataModelRepository.GetDataModelTable(
		ctx,
		usecase.executorFactory.NewExecutor(),
		tableID,
	); err != nil {
		return err
	} else if err := usecase.enforceSecurity.WriteDataModel(table.OrganizationID); err != nil {
		return err
	}

	return usecase.dataModelRepository.UpdateDataModelTable(
		ctx,
		usecase.executorFactory.NewExecutor(),
		tableID,
		description,
	)
}

func (usecase *DataModelUseCase) CreateDataModelField(ctx context.Context, field models.CreateFieldInput) (string, error) {
	fieldId := uuid.New().String()
	var tableName string
	if err := usecase.transactionFactory.Transaction(ctx, func(tx repositories.Executor) error {
		table, err := usecase.dataModelRepository.GetDataModelTable(ctx, tx, field.TableId)
		if err != nil {
			return err
		}
		if err := usecase.enforceSecurity.WriteDataModel(table.OrganizationID); err != nil {
			return err
		}

		tableName = table.Name

		if err := usecase.dataModelRepository.CreateDataModelField(ctx, tx, fieldId, field); err != nil {
			return err
		}

		// if it returns an error, automatically rolls back the other transaction
		return usecase.transactionFactory.TransactionInOrgSchema(
			ctx,
			table.OrganizationID,
			func(orgTx repositories.Executor) error {
				return usecase.organizationSchemaRepository.CreateField(ctx, orgTx, table.Name, field)
			},
		)
	}); err != nil {
		return "", err
	}

	if field.IsUnique {
		if err := usecase.clientDbIndexEditor.CreateUniqueIndexAsync(
			ctx,
			models.UnicityIndex{
				TableName: models.TableName(tableName),
				Fields:    []models.FieldName{field.Name},
			}); err != nil {
			return "", err
		}
	}

	return fieldId, nil
}

func (usecase *DataModelUseCase) UpdateDataModelField(ctx context.Context, fieldID string, input models.UpdateFieldInput) error {
	exec := usecase.executorFactory.NewExecutor()
	field, err := usecase.dataModelRepository.GetDataModelField(ctx, exec, fieldID)
	if err != nil {
		return err
	}

	if table, err := usecase.dataModelRepository.GetDataModelTable(ctx, exec, field.TableId); err != nil {
		return err
	} else if err := usecase.enforceSecurity.WriteDataModel(table.OrganizationID); err != nil {
		return err
	}

	if input.IsEnum != nil && *input.IsEnum &&
		(field.DataType != models.String &&
			field.DataType != models.Int &&
			field.DataType != models.Float) {
		return fmt.Errorf("enum fields can only be of type string or numeric: %w", models.BadParameterError)
	}

	return usecase.dataModelRepository.UpdateDataModelField(ctx, exec, fieldID, input)
}

func (usecase *DataModelUseCase) CreateDataModelLink(ctx context.Context, link models.DataModelLinkCreateInput) error {
	exec := usecase.executorFactory.NewExecutor()
	if err := usecase.enforceSecurity.WriteDataModel(link.OrganizationID); err != nil {
		return err
	}

	// check existence of tables
	if _, err := usecase.dataModelRepository.GetDataModelTable(ctx, exec, link.ChildTableID); err != nil {
		return err
	}
	table, err := usecase.dataModelRepository.GetDataModelTable(ctx, exec, link.ParentTableID)
	if err != nil {
		return err
	}

	// check existence of fields
	if _, err := usecase.dataModelRepository.GetDataModelField(ctx, exec, link.ChildFieldID); err != nil {
		return err
	}
	field, err := usecase.dataModelRepository.GetDataModelField(ctx, exec, link.ParentFieldID)
	if err != nil {
		return err
	}

	// Check that the parent field is unique by getting the full data model
	dataModel, err := usecase.GetDataModel(ctx, link.OrganizationID)
	if err != nil {
		return err
	}
	parentTable := dataModel.Tables[models.TableName(table.Name)]
	parentField := parentTable.Fields[models.FieldName(field.Name)]
	if parentField.UnicityConstraint != models.ActiveUniqueConstraint {
		return errors.Wrap(models.BadParameterError,
			fmt.Sprintf("parent field must be unique: field %s is not", field.Name))
	}

	return usecase.dataModelRepository.CreateDataModelLink(ctx, exec, link)
}

func (usecase *DataModelUseCase) DeleteDataModel(ctx context.Context, organizationID string) error {
	if err := usecase.enforceSecurity.WriteDataModel(organizationID); err != nil {
		return err
	}

	return usecase.transactionFactory.Transaction(ctx, func(tx repositories.Executor) error {
		if err := usecase.dataModelRepository.DeleteDataModel(ctx, tx, organizationID); err != nil {
			return err
		}

		// if it returns an error, automatically rolls back the other transaction
		return usecase.transactionFactory.TransactionInOrgSchema(ctx, organizationID, func(orgTx repositories.Executor) error {
			return usecase.organizationSchemaRepository.DeleteSchema(ctx, orgTx)
		})
	})
}
