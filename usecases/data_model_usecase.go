package usecases

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"regexp"
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type dataModelUsecaseIndexEditor interface {
	ListAllUniqueIndexes(ctx context.Context, organizationId string) ([]models.UnicityIndex, error)
	ListAllIndexes(
		ctx context.Context,
		organizationId string,
		indexTypes ...models.IndexType,
	) ([]models.ConcreteIndex, error)
	CreateUniqueIndex(ctx context.Context, exec repositories.Executor, organizationId string, index models.UnicityIndex) error
	CreateUniqueIndexAsync(ctx context.Context, organizationId string, index models.UnicityIndex) error
	DeleteUniqueIndex(ctx context.Context, organizationId string, index models.UnicityIndex) error
	CreateIndexesAsync(
		ctx context.Context,
		organizationId string,
		indexes []models.ConcreteIndex,
	) error
}

type dataModelUsecaseIngestedDataReadRepo interface {
	GatherFieldStatistics(ctx context.Context, exec repositories.Executor, table models.Table,
		orgId string) ([]models.FieldStatistics, error)
}

type DataModelUseCase struct {
	clientDbIndexEditor           dataModelUsecaseIndexEditor
	dataModelRepository           repositories.DataModelRepository
	enforceSecurity               security.EnforceSecurityOrganization
	executorFactory               executor_factory.ExecutorFactory
	organizationSchemaRepository  repositories.OrganizationSchemaRepository
	transactionFactory            executor_factory.TransactionFactory
	dataModelIngestedDataReadRepo dataModelUsecaseIngestedDataReadRepo
}

var (
	uniqTypes      = []models.DataType{models.String, models.Int, models.Float}
	enumTypes      = []models.DataType{models.String, models.Int, models.Float}
	validNameRegex = regexp.MustCompile(`^[a-z]+[a-z0-9_]+$`)
)

func (usecase DataModelUseCase) GetDataModel(
	ctx context.Context,
	organizationID string,
	options models.DataModelReadOptions,
) (models.DataModel, error) {
	if err := usecase.enforceSecurity.ReadDataModel(); err != nil {
		return models.DataModel{}, err
	}
	exec := usecase.executorFactory.NewExecutor()

	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, exec, organizationID, options.IncludeEnums)
	if err != nil {
		return models.DataModel{}, err
	}

	if options.IncludeNavigationOptions {
		pivotsMeta, err := usecase.dataModelRepository.ListPivots(ctx, exec, organizationID, nil)
		if err != nil {
			return models.DataModel{}, err
		}

		pivots := make([]models.Pivot, len(pivotsMeta))
		for i, pivot := range pivotsMeta {
			pivots[i] = pivot.Enrich(dataModel)
		}

		indexes, err := usecase.clientDbIndexEditor.ListAllIndexes(ctx, organizationID, models.IndexTypeNavigation)
		if err != nil {
			return models.DataModel{}, err
		}
		dataModel = dataModel.AddNavigationOptionsToDataModel(indexes, pivots)
	}

	if options.IncludeUnicityConstraints {
		uniqueIndexes, err := usecase.clientDbIndexEditor.ListAllUniqueIndexes(ctx, organizationID)
		if err != nil {
			return models.DataModel{}, err
		}
		dataModel = dataModel.AddUnicityConstraintStatusToDataModel(uniqueIndexes)
	}

	if options.IncludeSamples {
		db, err := usecase.executorFactory.NewClientDbExecutor(ctx, organizationID)
		if err != nil {
			return models.DataModel{}, err
		}
		for tableName, table := range dataModel.Tables {
			fieldStats, err := usecase.dataModelIngestedDataReadRepo.GatherFieldStatistics(ctx, db, table, organizationID)
			if err != nil {
				return models.DataModel{}, err
			}
			dataModel.Tables[tableName] = table.WithFieldStatistics(fieldStats)
		}
	}

	return dataModel, nil
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
	// NB: even if we decide in the future to be more permissive on allowed field names, for instance if we escape them and allow special characters
	// (which I don't think we should), they MUST still remain lower case, unless we first migrate all non escaped fields&tables in data_model_fields/data_model_tables
	// to lower case and change logic in models/concrete_index.go ConcreteIndex.Covers()
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
	// Note for the future me: if we want to allow making a "not nullable" field to "nullable", we need to also have a routine
	// that removes the constraint on the DB if it exists, for backwards compatibility.
	// We currently no longer add those constraints on fields marked as required and their value is only enforced at ingestion time
	// in our code, as of early dec 2024.
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
	dataModel, err := usecase.GetDataModel(ctx, table.OrganizationID, models.DataModelReadOptions{
		IncludeUnicityConstraints: true,
	})
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
	dataModel, err := usecase.GetDataModel(ctx, link.OrganizationID, models.DataModelReadOptions{
		IncludeUnicityConstraints: true,
	})
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
			return pivotMeta.Enrich(dm), err
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
		pivots = append(pivots, pivot.Enrich(dm))
	}

	return pivots, nil
}

func (usecase *DataModelUseCase) CreateNavigationOption(ctx context.Context, input models.CreateNavigationOptionInput) error {
	exec := usecase.executorFactory.NewExecutor()

	// Basic sanity checks on input
	if input.SourceTableId == input.TargetTableId && input.SourceFieldId != input.FilterFieldId {
		return errors.Wrap(
			models.BadParameterError,
			"if source and target tables are the same, source and filter fields must be the same",
		)
	}
	if input.FilterFieldId == input.OrderingFieldId {
		return errors.Wrap(
			models.BadParameterError,
			"filter and ordering fields must be different",
		)
	}

	// get data model for the org, with pivot definition
	sourceTableMeta, err := usecase.dataModelRepository.GetDataModelTable(ctx, exec, input.SourceTableId)
	if err != nil {
		return err
	}
	orgId := sourceTableMeta.OrganizationID
	if err := usecase.enforceSecurity.WriteDataModel(orgId); err != nil {
		return err
	}
	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, exec, orgId, true)
	if err != nil {
		return err
	}
	uniqueIndexes, err := usecase.clientDbIndexEditor.ListAllUniqueIndexes(ctx, orgId)
	if err != nil {
		return err
	}
	dataModel = dataModel.AddUnicityConstraintStatusToDataModel(uniqueIndexes)
	allTables := dataModel.AllTablesAsMap()

	pivotsMeta, err := usecase.dataModelRepository.ListPivots(ctx, exec, orgId, nil)
	if err != nil {
		return err
	}
	// Consider only the pivot defined on the input source table, if present. Other pivot values are irrelevant in this context.
	pivots := make([]models.Pivot, 0, 1)
	for _, pivot := range pivotsMeta {
		if pivot.BaseTableId == input.SourceTableId && pivot.FieldId != nil {
			pivots = append(pivots, pivot.Enrich(dataModel))
		}
	}

	// verify that the navigation option input matches one of the two cases where they can be created (reverse link or self-table pivot value)
	canCreateNavOption := false
	targetTable, ok := allTables[input.TargetTableId]
	if !ok {
		return errors.Wrap(
			models.NotFoundError,
			fmt.Sprintf("target table %s not found", input.TargetTableId),
		)
	}
	sourceTable, ok := allTables[input.SourceTableId]
	if !ok {
		return errors.Wrap(
			models.NotFoundError,
			fmt.Sprintf("source table %s not found", input.SourceTableId),
		)
	}
	sourceField, ok := sourceTable.GetFieldById(input.SourceFieldId)
	if !ok {
		return errors.Wrap(
			models.NotFoundError,
			fmt.Sprintf("source field %s not found in table %s", input.SourceFieldId, sourceTable.Name),
		)
	}
	filterField, ok := targetTable.GetFieldById(input.FilterFieldId)
	if !ok {
		return errors.Wrap(
			models.NotFoundError,
			fmt.Sprintf("filter field %s not found in table %s", input.FilterFieldId, targetTable.Name),
		)
	}
	orderingField, ok := targetTable.GetFieldById(input.OrderingFieldId)
	if !ok {
		return errors.Wrap(
			models.NotFoundError,
			fmt.Sprintf("ordering field %s not found in table %s", input.OrderingFieldId, targetTable.Name),
		)
	}

	for _, link := range targetTable.LinksToSingle {
		if link.ParentFieldId == input.SourceFieldId &&
			link.ParentTableId == input.SourceTableId {
			canCreateNavOption = true
			break
		}
	}
	for _, pivot := range pivots {
		// no navigation option on fields that are marked as unique
		if pivot.BaseTableId == input.SourceTableId && pivot.Field == sourceField.Name {
			if filterField.UnicityConstraint != models.NoUnicityConstraint {
				return errors.Wrap(
					models.BadParameterError,
					fmt.Sprintf("cannot create navigation option on unique field %s.%s",
						targetTable.Name, filterField.Name),
				)
			}
			canCreateNavOption = true
		}
	}

	if !canCreateNavOption {
		return errors.Wrap(
			models.UnprocessableEntityError,
			fmt.Sprintf("cannot create navigation option from %s.%s to %s.%s: must be a reverse link of self-table pivot value",
				sourceTable.Name, sourceField.Name,
				targetTable.Name, filterField.Name),
		)
	}

	// enrich data model with the existing navigation options to check against conflict
	indexes, err := usecase.clientDbIndexEditor.ListAllIndexes(ctx, orgId, models.IndexTypeNavigation)
	if err != nil {
		return err
	}
	dataModel = dataModel.AddNavigationOptionsToDataModel(indexes, pivots)

	// last, check if the navigation option already exists
	for _, navOption := range dataModel.AllTablesAsMap()[input.SourceTableId].NavigationOptions {
		if navOption.SourceFieldName == sourceField.Name &&
			navOption.FilterFieldName == filterField.Name &&
			navOption.OrderingFieldName == orderingField.Name {
			if navOption.Status == models.IndexStatusValid {
				return errors.Wrap(
					models.ConflictError,
					fmt.Sprintf("navigation option %s.%s -> %s.%s (order on %s) already exists",
						sourceTable.Name, sourceField.Name,
						targetTable.Name, filterField.Name, orderingField.Name),
				)
			}
			// index already pending creation, early return for noop
			if navOption.Status == models.IndexStatusPending {
				return nil
			}
		}
	}

	// Finally, create the index
	return usecase.clientDbIndexEditor.CreateIndexesAsync(ctx, orgId, []models.ConcreteIndex{
		{
			Type:      models.IndexTypeNavigation,
			TableName: targetTable.Name,
			Indexed:   []string{filterField.Name, orderingField.Name},
		},
	})
}

func (uc DataModelUseCase) GetDataModelOptions(ctx context.Context, orgId, tableId string) (models.DataModelOptions, error) {
	exec := uc.executorFactory.NewExecutor()

	if err := uc.enforceSecurity.ReadDataModel(); err != nil {
		return models.DataModelOptions{}, err
	}

	tableMeta, err := uc.dataModelRepository.GetDataModelTable(ctx, exec, tableId)
	if err != nil {
		return models.DataModelOptions{}, err
	}
	if tableMeta.OrganizationID != orgId {
		return models.DataModelOptions{}, errors.Wrap(models.NotFoundError, "table not found")
	}

	dataModel, err := uc.dataModelRepository.GetDataModel(ctx, exec, tableMeta.OrganizationID, false)
	if err != nil {
		return models.DataModelOptions{}, err
	}

	opts, err := uc.dataModelRepository.GetDataModelOptionsForTable(ctx, exec, tableId)
	if err != nil {
		return models.DataModelOptions{}, err
	}

	if opts == nil {
		opts = &models.DataModelOptions{}
	}

	opts.FieldOrder = sortTableFieldsForDisplay(
		dataModel.Tables[tableMeta.Name].Fields,
		*opts,
	)

	return *opts, nil
}

func (uc DataModelUseCase) UpdateDataModelOptions(ctx context.Context,
	orgId string,
	req models.UpdateDataModelOptionsRequest,
) (models.DataModelOptions, error) {
	exec := uc.executorFactory.NewExecutor()

	if err := uc.enforceSecurity.WriteDataModel(orgId); err != nil {
		return models.DataModelOptions{}, err
	}

	tableMeta, err := uc.dataModelRepository.GetDataModelTable(ctx, exec, req.TableId)
	if err != nil {
		return models.DataModelOptions{}, err
	}
	if tableMeta.OrganizationID != orgId {
		return models.DataModelOptions{}, errors.Wrap(models.NotFoundError, "table not found")
	}
	dataModel, err := uc.dataModelRepository.GetDataModel(ctx, exec, tableMeta.OrganizationID, false)
	if err != nil {
		return models.DataModelOptions{}, err
	}

	table, ok := dataModel.Tables[tableMeta.Name]
	if !ok {
		return models.DataModelOptions{}, errors.Wrap(
			models.UnprocessableEntityError, "table not found")
	}

	if len(req.DisplayedFields) > 0 {
		for _, fieldId := range req.DisplayedFields {
			fieldFound := false

			for _, tableField := range table.Fields {
				if tableField.ID == fieldId {
					fieldFound = true
					break
				}
			}

			if !fieldFound {
				return models.DataModelOptions{}, errors.Wrap(
					models.UnprocessableEntityError, "provided field does not exist on the table")
			}
		}
	}

	opts, err := uc.dataModelRepository.UpsertDataModelOptions(ctx, exec, req)
	if err != nil {
		return models.DataModelOptions{}, err
	}

	return opts, nil
}

func sortTableFieldsForDisplay(fieldsMeta map[string]models.Field, opts models.DataModelOptions) []string {
	// Build the full ordered list of fields by appending the ordered fields from
	// the options and appending the rest of the (unordered) fields as they came from
	// the database.
	// Leftover fields can happen if a field was added to the table after the order was
	// set.
	dbFields := slices.Collect(maps.Values(fieldsMeta))

	slices.SortFunc(dbFields, func(lhs, rhs models.Field) int {
		return cmp.Compare(lhs.Name, rhs.Name)
	})

	orderedFields := make([]string, len(opts.FieldOrder), len(dbFields))

	for idx, field := range opts.FieldOrder {
		// In the case of manually deleted fields, omit them from the order (will be empty string)
		if !slices.ContainsFunc(dbFields, func(f models.Field) bool {
			return f.ID == field
		}) {
			continue
		}

		orderedFields[idx] = opts.FieldOrder[idx]
	}
	for _, field := range dbFields {
		if slices.Contains(opts.FieldOrder, field.ID) {
			continue
		}

		orderedFields = append(orderedFields, field.ID)
	}

	// Delete any empty string from the ordered fields (those represent fields present in the
	// order, that were deleted from the database).
	orderedFields = slices.DeleteFunc(orderedFields, func(f string) bool {
		return f == ""
	})

	return orderedFields
}
