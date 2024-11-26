package usecases

import (
	"context"
	"fmt"
	"regexp"
	"slices"

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
	uniqTypes      = []models.DataType{models.String, models.Int, models.Float}
	enumTypes      = []models.DataType{models.String, models.Int, models.Float}
	validNameRegex = regexp.MustCompile(`^[a-z]+[a-z0-9_]+$`)
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

	uniqueIndexes, err := usecase.clientDbIndexEditor.ListAllUniqueIndexes(ctx, organizationID)
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
	if !validNameRegex.MatchString(name) {
		return "", errors.Wrap(models.BadParameterError,
			"table name must only contain lower case alphanumeric characters and underscores, and start by a letter")
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

	err := usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
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
		return usecase.transactionFactory.TransactionInOrgSchema(ctx, organizationId, func(orgTx repositories.Transaction) error {
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
				organizationId,
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
	if field.Name == "id" {
		return "", errors.Wrap(models.BadParameterError, "field name 'id' is reserved")
	}
	if !validNameRegex.MatchString(field.Name) {
		return "", errors.Wrap(models.BadParameterError,
			"field name must only contain lower case alphanumeric characters and underscores, and start by a letter")
	}

	fieldId := uuid.New().String()
	var tableName string
	var organizationId string
	err := usecase.transactionFactory.Transaction(
		ctx,
		func(tx repositories.Transaction) error {
			table, err := usecase.dataModelRepository.GetDataModelTable(ctx, tx, field.TableId)
			if err != nil {
				return err
			}
			organizationId = table.OrganizationID
			if err := usecase.enforceSecurity.WriteDataModel(table.OrganizationID); err != nil {
				return err
			}

			tableName = table.Name

			if err := usecase.dataModelRepository.CreateDataModelField(ctx, tx, fieldId, field); err != nil {
				return err
			}

			db, err := usecase.executorFactory.NewClientDbExecutor(ctx, organizationId)
			if err != nil {
				return err
			}
			// if it returns an error, automatically rolls back the other transaction
			return usecase.organizationSchemaRepository.CreateField(ctx, db, table.Name, field)
		},
	)
	if err != nil {
		return "", err
	}

	if field.IsUnique {
		err := usecase.clientDbIndexEditor.CreateUniqueIndexAsync(
			ctx,
			organizationId,
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
			table.OrganizationID,
			getFieldUniqueIndex(table.Name, field.Name),
		)
	}

	// delete the unique index if required
	if makeNotUnique {
		return usecase.clientDbIndexEditor.DeleteUniqueIndex(
			ctx,
			table.OrganizationID,
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
			if link.ParentTableName == tableName &&
				link.ParentFieldName == fieldName {
				links = append(links, link)
			}
		}
	}

	return links
}

func (usecase *DataModelUseCase) CreateDataModelLink(ctx context.Context, link models.DataModelLinkCreateInput) (string, error) {
	if err := usecase.enforceSecurity.WriteDataModel(link.OrganizationID); err != nil {
		return "", err
	}
	if !validNameRegex.MatchString(link.Name) {
		return "", errors.Wrap(models.BadParameterError,
			"field name must only contain lower case alphanumeric characters and underscores, and start by a letter")
	}
	exec := usecase.executorFactory.NewExecutor()

	// check existence of tables
	if _, err := usecase.dataModelRepository.GetDataModelTable(ctx, exec, link.ChildTableID); err != nil {
		return "", err
	}
	table, err := usecase.dataModelRepository.GetDataModelTable(ctx, exec, link.ParentTableID)
	if err != nil {
		return "", err
	}

	// check existence of fields
	if _, err := usecase.dataModelRepository.GetDataModelField(ctx, exec, link.ChildFieldID); err != nil {
		return "", err
	}
	field, err := usecase.dataModelRepository.GetDataModelField(ctx, exec, link.ParentFieldID)
	if err != nil {
		return "", err
	}

	// Check that the parent field is unique by getting the full data model
	dataModel, err := usecase.GetDataModel(ctx, link.OrganizationID)
	if err != nil {
		return "", err
	}
	parentTable := dataModel.Tables[table.Name]
	parentField := parentTable.Fields[field.Name]
	if parentField.UnicityConstraint != models.ActiveUniqueConstraint {
		return "", errors.Wrap(models.BadParameterError,
			fmt.Sprintf("parent field must be unique: field %s is not", field.Name))
	}

	linkId := uuid.NewString()
	return linkId, usecase.dataModelRepository.CreateDataModelLink(ctx, exec, linkId, link)
}

func (usecase *DataModelUseCase) DeleteDataModel(ctx context.Context, organizationID string) error {
	if err := usecase.enforceSecurity.WriteDataModel(organizationID); err != nil {
		return err
	}

	return usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		if err := usecase.dataModelRepository.DeleteDataModel(ctx, tx, organizationID); err != nil {
			return err
		}

		db, err := usecase.executorFactory.NewClientDbExecutor(ctx, organizationID)
		if err != nil {
			return err
		}
		// if it returns an error, automatically rolls back the other transaction
		return usecase.organizationSchemaRepository.DeleteSchema(ctx, db)
	})
}

// data model pivot methods

func (usecase *DataModelUseCase) CreatePivot(ctx context.Context, input models.CreatePivotInput) (models.Pivot, error) {
	if err := usecase.enforceSecurity.WriteDataModel(input.OrganizationId); err != nil {
		return models.Pivot{}, err
	}

	exec := usecase.executorFactory.NewExecutor()
	dm, err := usecase.dataModelRepository.GetDataModel(ctx, exec, input.OrganizationId, false)
	if err != nil {
		return models.Pivot{}, err
	}

	if err := validatePivotCreateInput(input, dm); err != nil {
		return models.Pivot{}, err
	}

	id := uuid.New().String()
	return executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Transaction) (models.Pivot, error) {
			err := usecase.dataModelRepository.CreatePivot(ctx, tx, id, input)
			if err != nil {
				return models.Pivot{}, err
			}
			pivotMeta, err := usecase.dataModelRepository.GetPivot(ctx, tx, id)
			return models.AdaptPivot(pivotMeta, dm), err
		},
	)
}

func validatePivotCreateInput(input models.CreatePivotInput, dm models.DataModel) error {
	hasField := input.FieldId != nil
	hasPath := len(input.PathLinkIds) > 0
	// For now we choose not to allow to select a field and a path at the same time
	// (a field can only be selected if the pivot is a field from the base table)
	// By convention, the pivot field in the case of a pivot defined with a path is the
	// parent field of the last link in the path.
	// This is susceptible to change in the future.
	if hasField == hasPath {
		return errors.Wrap(
			models.BadParameterError,
			"either field_id or path_link_ids must be provided",
		)
	}

	// check existence of base table
	var table models.Table
	for _, t := range dm.Tables {
		if t.ID == input.BaseTableId {
			table = t
			break
		}
	}
	if table.ID == "" {
		return errors.Wrap(
			models.NotFoundError,
			fmt.Sprintf("base table %s not found", input.BaseTableId),
		)
	}
	if hasField && dm.AllFieldsAsMap()[*input.FieldId].DataType != models.String {
		return errors.Wrap(
			models.BadParameterError,
			"pivot field must be of type string",
		)
	}

	// check existence of links
	linksMap := dm.AllLinksAsMap()
	for _, linkId := range input.PathLinkIds {
		link := linksMap[linkId]
		if link.Id == "" {
			return errors.Wrap(
				models.NotFoundError,
				fmt.Sprintf("link %s not found", linkId),
			)
		}
	}

	// verify that the links are chained consistently
	if hasPath {
		err := models.ValidatePathPivot(dm, input.PathLinkIds, table.Name)
		if err != nil {
			return err
		}
		field := models.FieldFromPath(dm, input.PathLinkIds)
		if field.DataType != models.String {
			return errors.Wrap(
				models.BadParameterError,
				"pivot field must be of type string",
			)
		}
	}

	return nil
}

func (usecase *DataModelUseCase) ListPivots(ctx context.Context, organizationId string, tableID *string) ([]models.Pivot, error) {
	if err := usecase.enforceSecurity.ReadDataModel(); err != nil {
		return nil, err
	}

	exec := usecase.executorFactory.NewExecutor()

	dm, err := usecase.dataModelRepository.GetDataModel(ctx, exec, organizationId, false)
	if err != nil {
		return nil, err
	}

	pivotsMeta, err := usecase.dataModelRepository.ListPivots(ctx, exec, organizationId, tableID)
	if err != nil {
		return nil, err
	}

	pivots := make([]models.Pivot, 0, len(pivotsMeta))
	for _, pivot := range pivotsMeta {
		err = usecase.enforceSecurity.ReadOrganization(pivot.OrganizationId)
		if err != nil {
			return nil, err
		}
		pivots = append(pivots, models.AdaptPivot(pivot, dm))
	}

	return pivots, nil
}
