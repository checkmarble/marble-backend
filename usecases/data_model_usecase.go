package usecases

import (
	"context"
	"fmt"
	"slices"
	"time"

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

var (
	uniqTypes = []models.DataType{models.String, models.Int, models.Float}
	enumTypes = []models.DataType{models.String, models.Int, models.Float}
)

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
	dm := dataModel.Copy()
	for _, index := range uniqueIndexes {
		// here we only care about single fields with a unicity constraint
		if len(index.Fields) != 1 {
			continue
		}
		table, ok := dm.Tables[index.TableName]
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
			dm.Tables[index.TableName].Fields[index.Fields[0]] = field
		}
	}
	return dm
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
				getFieldUniqueIndex(name, "object_id"),
			)
		})
	})
	return tableId, err
}

func (usecase *DataModelUseCase) UpdateDataModelTable(ctx context.Context, tableID, description string) error {
	exec := usecase.executorFactory.NewExecutor()
	if table, err := usecase.dataModelRepository.GetDataModelTable(ctx, exec, tableID); err != nil {
		return err
	} else if err := usecase.enforceSecurity.WriteDataModel(table.OrganizationID); err != nil {
		return err
	}

	return usecase.dataModelRepository.UpdateDataModelTable(ctx, exec, tableID, description)
}

func (usecase *DataModelUseCase) CreateDataModelField(ctx context.Context, field models.CreateFieldInput) (string, error) {
	fieldId := uuid.New().String()
	var tableName string
	err := usecase.transactionFactory.Transaction(ctx, func(tx repositories.Executor) error {
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
	})
	if err != nil {
		return "", err
	}

	if field.IsUnique {
		err := usecase.clientDbIndexEditor.CreateUniqueIndexAsync(
			ctx,
			getFieldUniqueIndex(tableName, field.Name),
		)
		if err != nil {
			return "", err
		}
	}

	return fieldId, nil
}

func getFieldUniqueIndex(tableName string, fieldName string) models.UnicityIndex {
	// the unique index on object_id will serve both to enforce unicity and to speed up ingestion queries
	// which is why we include the updated_at and id fields
	if fieldName == "object_id" {
		return models.UnicityIndex{
			TableName: tableName,
			Fields:    []string{"object_id"},
			Included:  []string{"updated_at", "id"},
		}
	}
	return models.UnicityIndex{
		TableName: tableName,
		Fields:    []string{fieldName},
	}
}

func (usecase *DataModelUseCase) UpdateDataModelField(ctx context.Context, fieldID string, input models.UpdateFieldInput) error {
	exec := usecase.executorFactory.NewExecutor()
	// permission and input validation
	field, err := usecase.dataModelRepository.GetDataModelField(ctx, exec, fieldID)
	if err != nil {
		return err
	}
	table, err := usecase.dataModelRepository.GetDataModelTable(ctx, exec, field.TableId)
	if err != nil {
		return err
	} else if err := usecase.enforceSecurity.WriteDataModel(table.OrganizationID); err != nil {
		return err
	}
	dataModel, err := usecase.GetDataModel(ctx, table.OrganizationID)
	if err != nil {
		return err
	}

	makeUnique, makeNotUnique, err := validateFieldUpdateRules(dataModel, field, table, input)
	if err != nil {
		return err
	}

	// update the field (data_model_field row)
	if err := usecase.dataModelRepository.UpdateDataModelField(ctx, exec, fieldID, input); err != nil {
		return err
	}

	// asynchronously create the unique index if required
	if makeUnique {
		return usecase.clientDbIndexEditor.CreateUniqueIndexAsync(
			ctx,
			getFieldUniqueIndex(table.Name, field.Name),
		)
	}

	// delete the unique index if required
	if makeNotUnique {
		return usecase.clientDbIndexEditor.DeleteUniqueIndex(
			ctx,
			getFieldUniqueIndex(table.Name, field.Name),
		)
	}

	return nil
}

func validateFieldUpdateRules(
	dataModel models.DataModel,
	field models.FieldMetadata,
	table models.TableMetadata,
	input models.UpdateFieldInput,
) (makeUnique, makeNotUnique bool, err error) {
	makeEnum := input.IsEnum != nil && *input.IsEnum && !field.IsEnum
	makeNotEnum := input.IsEnum != nil && !*input.IsEnum && field.IsEnum
	if makeEnum && !slices.Contains(enumTypes, field.DataType) {
		return false, false, errors.Wrap(
			models.BadParameterError,
			"enum fields can only be of type string or numeric")
	}

	currentField := dataModel.Tables[table.Name].Fields[field.Name]
	isUnique := currentField.UnicityConstraint != models.NoUnicityConstraint

	makeUnique = input.IsUnique != nil &&
		*input.IsUnique &&
		currentField.UnicityConstraint == models.NoUnicityConstraint
	if makeUnique && !slices.Contains(uniqTypes, field.DataType) {
		return false, false, errors.Wrap(
			models.BadParameterError,
			"unique fields can only be of type string, int or float")
	}

	linksToField := findLinksToField(dataModel, table.Name, field.Name)
	makeNotUnique = input.IsUnique != nil &&
		!*input.IsUnique &&
		currentField.UnicityConstraint != models.NoUnicityConstraint
	if makeNotUnique && len(linksToField) > 0 {
		return false, false, errors.Wrap(
			models.BadParameterError,
			"cannot remove unicity constraint on a field that is linked to another table")
	}
	if makeNotUnique && field.Name == "object_id" {
		return false, false, errors.Wrap(
			models.BadParameterError,
			"cannot remove unicity constraint on the object_id field")
	}

	if makeUnique && (makeEnum || (field.IsEnum && !makeNotEnum)) {
		return false, false, errors.Wrap(
			models.BadParameterError,
			"cannot make a field unique if it is an enum")
	}
	// if makeEnum && !(currentField.UnicityConstraint == models.NoUnicityConstraint || makeNotUnique) {
	if makeEnum && (makeUnique || (isUnique && !makeNotUnique)) {
		return false, false, errors.Wrap(
			models.BadParameterError,
			"cannot make a field an enum if it is unique or has a pending unique constraint")
	}

	return
}

func findLinksToField(dataModel models.DataModel, tableName string, fieldName string) []models.LinkToSingle {
	var links []models.LinkToSingle
	for _, table := range dataModel.Tables {
		for _, link := range table.LinksToSingle {
			if string(link.LinkedTableName) == tableName &&
				string(link.ParentFieldName) == fieldName {
				links = append(links, link)
			}
		}
	}

	return links
}

func (usecase *DataModelUseCase) CreateDataModelLink(ctx context.Context, link models.DataModelLinkCreateInput) error {
	if err := usecase.enforceSecurity.WriteDataModel(link.OrganizationID); err != nil {
		return err
	}
	exec := usecase.executorFactory.NewExecutor()

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
	parentTable := dataModel.Tables[table.Name]
	parentField := parentTable.Fields[field.Name]
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

func (usecase *DataModelUseCase) CreatePivot(ctx context.Context, organizationID string, input models.CreatePivotInput) (models.Pivot, error) {
	if err := usecase.enforceSecurity.WriteDataModel(organizationID); err != nil {
		return models.Pivot{}, err
	}

	// return  dummy pivot for now
	return models.Pivot{
		Id:        uuid.New().String(),
		CreatedAt: time.Now(),

		BaseTableId: input.BaseTableId,
		BaseTable:   "dummy_table",
		Links:       []string{},
		LinkIds:     input.LinkIds,
	}, nil
}

func (usecase *DataModelUseCase) ListPivots(ctx context.Context, organizationID string, tableID *string) ([]models.Pivot, error) {
	if err := usecase.enforceSecurity.ReadDataModel(); err != nil {
		return nil, err
	}

	// return dummy pivots for now
	return []models.Pivot{
		{
			Id:        uuid.New().String(),
			CreatedAt: time.Now(),

			BaseTableId: *tableID,
			BaseTable:   "dummy_table",
			Links:       []string{},
			LinkIds:     []string{"dummy_link_id"},
		},
	}, nil
}
