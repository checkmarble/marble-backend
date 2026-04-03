package usecases

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"regexp"
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/indexes"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type dataModelUsecaseIndexEditor interface {
	ListAllUniqueIndexes(ctx context.Context, organizationId uuid.UUID) ([]models.UnicityIndex, error)
	ListAllIndexes(
		ctx context.Context,
		organizationId uuid.UUID,
		indexTypes ...models.IndexType,
	) ([]models.ConcreteIndex, error)
	CreateUniqueIndex(ctx context.Context, exec repositories.Executor, organizationId uuid.UUID, index models.UnicityIndex) error
	CreateUniqueIndexAsync(ctx context.Context, organizationId uuid.UUID, index models.UnicityIndex) error
	DeleteUniqueIndex(ctx context.Context, organizationId uuid.UUID, index models.UnicityIndex) error
	CreateIndexesBlocking(
		ctx context.Context,
		organizationId uuid.UUID,
		indexes []models.ConcreteIndex,
	) error
	CreateIndexesAsync(
		ctx context.Context,
		organizationId uuid.UUID,
		indexes []models.ConcreteIndex,
	) error
}

type dataModelUsecaseIngestedDataReadRepo interface {
	GatherFieldStatistics(ctx context.Context, exec repositories.Executor, table models.Table,
		orgId uuid.UUID) ([]models.FieldStatistics, error)
}

type usecase struct {
	clientDbIndexEditor           dataModelUsecaseIndexEditor
	dataModelRepository           repositories.DataModelRepository
	enforceSecurity               security.EnforceSecurityOrganization
	executorFactory               executor_factory.ExecutorFactory
	organizationSchemaRepository  repositories.OrganizationSchemaRepository
	transactionFactory            executor_factory.TransactionFactory
	dataModelIngestedDataReadRepo dataModelUsecaseIngestedDataReadRepo
	indexEditor                   indexes.ClientDbIndexEditor
	taskQueueRepository           repositories.TaskQueueRepository
	destroyUsecase                DataModelDestroyUsecase
}

var (
	uniqTypes      = []models.DataType{models.String, models.Int, models.Float}
	enumTypes      = []models.DataType{models.String, models.Int, models.Float}
	validNameRegex = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)
)

func (usecase usecase) getDataModelWithExec(
	ctx context.Context,
	exec repositories.Executor,
	organizationID uuid.UUID,
	options models.DataModelReadOptions,
	useCache bool,
) (models.DataModel, error) {
	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, exec, organizationID, options.IncludeEnums, useCache)
	if err != nil {
		return models.DataModel{}, err
	}

	if options.IncludeNavigationOptions {
		pivotsMeta, err := usecase.dataModelRepository.ListPivots(ctx, exec, organizationID, nil, useCache)
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

func (usecase usecase) GetDataModel(
	ctx context.Context,
	organizationID uuid.UUID,
	options models.DataModelReadOptions,
	useCache bool,
) (models.DataModel, error) {
	if err := usecase.enforceSecurity.ReadDataModel(); err != nil {
		return models.DataModel{}, err
	}

	exec := usecase.executorFactory.NewExecutor()
	return usecase.getDataModelWithExec(ctx, exec, organizationID, options, useCache)
}

// Better to use this method by providing the tableName instead of tableID to avoid using `AllTablesAsMap`
// which make a map of all tables for lookup by ID
func (usecase *usecase) validateTableSemanticType(
	ctx context.Context,
	exec repositories.Executor,
	organizationID uuid.UUID,
	tableName *string,
	tableID *string,
) error {
	if tableID == nil && tableName == nil {
		return errors.Wrap(models.BadParameterError, "table ID or table name is required")
	}

	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, exec, organizationID, false, false)
	if err != nil {
		return err
	}

	var table models.Table
	if tableID != nil {
		allTablesByID := dataModel.AllTablesAsMap()
		var ok bool
		table, ok = allTablesByID[*tableID]
		if !ok {
			return errors.Wrap(models.NotFoundError, "table not found in data model")
		}
	} else {
		var ok bool
		table, ok = dataModel.Tables[*tableName]
		if !ok {
			return errors.Wrap(models.NotFoundError, "table not found in data model")
		}
	}

	validationFunc, ok := TableSemanticTypeValidationFunctions[table.SemanticType]
	if !ok {
		return errors.Wrap(models.BadParameterError,
			"semantic type validation function not found")
	}

	return validationFunc(table.Name, dataModel)
}

func retrieveParentFieldIdForLink(parentTableId string, TablesById map[string]models.Table) (string, error) {
	parentTable, ok := TablesById[parentTableId]
	if !ok {
		return "", errors.Wrap(models.BadParameterError,
			"parent table not found in data model pointed by the link")
	}
	parentFieldId, ok := parentTable.Fields["object_id"]
	if !ok {
		// Should never happen since the `object_id` is a mandatory field for every table
		return "", errors.Wrap(models.BadParameterError,
			"parent field 'object_id' not found in parent table")
	}
	return parentFieldId.ID, nil
}

// Table can have only one pivot because of the unique index on `organization_id + base_table_id`
func (usecase *usecase) ensureTableHasPivot(
	ctx context.Context,
	exec repositories.Executor,
	organizationId uuid.UUID,
	tableId string,
	fieldIdsByName map[string]string,
) error {
	pivots, err := usecase.dataModelRepository.ListPivots(ctx, exec, organizationId, utils.Ptr(tableId), false)
	if err != nil {
		return err
	}
	if len(pivots) == 0 {
		pathLinkEmpty := make([]string, 0)
		if _, err := usecase.CreatePivotWithExec(
			ctx,
			exec,
			models.CreatePivotInput{
				BaseTableId:    tableId,
				OrganizationId: organizationId,
				FieldId:        utils.Ptr(fieldIdsByName["object_id"]),
				PathLinkIds:    pathLinkEmpty,
			}); err != nil {
			return err
		}
	}
	return nil
}

func (usecase *usecase) CreateDataModelTable(
	ctx context.Context,
	organizationId uuid.UUID,
	input models.CreateTableInput,
) (string, error) {
	if err := usecase.enforceSecurity.WriteDataModel(organizationId); err != nil {
		return "", err
	}

	if !input.SemanticType.IsValid() {
		return "", errors.Wrap(models.BadParameterError, "invalid semantic type")
	}

	// Validate table name
	if !validNameRegex.MatchString(input.Name) {
		return "", errors.Wrap(models.BadParameterError,
			"table name must only contain lower case alphanumeric characters and underscores, and start by a letter")
	}

	oldDatamodel, err := usecase.GetDataModel(ctx, organizationId, models.DataModelReadOptions{}, false)
	if err != nil {
		return "", err
	}
	// input.Links miss the ParentFieldID since we automatically use the `object_id` field of the parent table. Need to retrieve the ID before creating the links
	tablesById := oldDatamodel.AllTablesAsMap()
	for i := range input.Links {
		parentFieldId, err := retrieveParentFieldIdForLink(
			input.Links[i].ParentTableID, tablesById)
		if err != nil {
			return "", err
		}
		input.Links[i].ParentFieldID = parentFieldId
	}

	// Generate IDs
	tableId := pure_utils.NewId().String()

	// Transaction: create everything in marble DB -> Validate -> org schema
	err = usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		if err := usecase.dataModelRepository.CreateDataModelTable(
			ctx, tx,
			organizationId,
			tableId,
			input,
		); err != nil {
			return err
		}

		newTableMetadata, err := usecase.dataModelRepository.GetDataModelTable(ctx, tx, tableId)
		if err != nil {
			return err
		}

		fieldsToCreate := make([]models.CreateFieldInput, len(input.Fields))
		fieldIdsByName := make(map[string]string)
		for i, f := range input.Fields {
			fieldsToCreate[i] = models.CreateFieldInput{
				TableId:      tableId,
				Name:         f.Name,
				Description:  f.Description,
				Alias:        f.Alias,
				DataType:     f.DataType,
				Nullable:     f.Nullable,
				IsEnum:       f.IsEnum,
				IsUnique:     f.IsUnique,
				FTMProperty:  f.FTMProperty,
				Metadata:     f.Metadata,
				SemanticType: f.SemanticType,
			}

			fieldId, err := usecase.createDataModelFieldWithExec(
				ctx, tx, newTableMetadata, fieldsToCreate[i],
			)
			if err != nil {
				return err
			}
			fieldIdsByName[fieldsToCreate[i].Name] = fieldId
		}

		// Validate required fields before they're used by ensureTableHasPivot and org schema creation
		for _, req := range []string{
			"object_id",
			"updated_at",
		} {
			if _, ok := fieldIdsByName[req]; !ok {
				return errors.Wrapf(models.BadParameterError,
					"required field %q is missing from the table fields", req)
			}
		}
		for _, f := range fieldsToCreate {
			if f.Name == "object_id" && (f.DataType != models.String || f.Nullable) {
				return errors.Wrap(models.BadParameterError,
					"field \"object_id\" must be of type String and non-nullable")
			}
			if f.Name == "updated_at" && (f.DataType != models.Timestamp || f.Nullable) {
				return errors.Wrap(models.BadParameterError,
					"field \"updated_at\" must be of type Timestamp and non-nullable")
			}
		}

		// Create links
		for _, l := range input.Links {
			childFieldId, ok := fieldIdsByName[l.ChildFieldName]
			if !ok {
				return errors.Wrap(models.BadParameterError,
					"child field not found in data model when creating link")
			}
			if _, err := usecase.createDataModelLinkWithExec(ctx, tx, models.DataModelLinkCreateInput{
				OrganizationID: organizationId,
				Name:           l.Name,
				LinkType:       l.LinkType,
				ParentTableID:  l.ParentTableID,
				ParentFieldID:  l.ParentFieldID,
				ChildTableID:   tableId,
				ChildFieldID:   childFieldId,
			}); err != nil {
				return err
			}
		}

		if err := usecase.validateTableSemanticType(ctx, tx, organizationId, &input.Name, nil); err != nil {
			return err
		}
		// Ensure the table has a pivot (default object_id pivot when none exist)
		if err := usecase.ensureTableHasPivot(ctx, tx, organizationId, tableId, fieldIdsByName); err != nil {
			return err
		}

		// Create org schema table + columns
		return usecase.transactionFactory.TransactionInOrgSchema(ctx, organizationId, func(orgTx repositories.Transaction) error {
			if err := usecase.organizationSchemaRepository.CreateSchemaIfNotExists(ctx, orgTx); err != nil {
				return err
			}
			if err := usecase.organizationSchemaRepository.CreateTable(ctx, orgTx, input.Name); err != nil {
				return err
			}

			for _, field := range fieldsToCreate {
				// Those fields are automatically created in the org schema
				if field.Name == "object_id" || field.Name == "updated_at" {
					continue
				}
				if err := usecase.organizationSchemaRepository.CreateField(ctx,
					orgTx, input.Name, field); err != nil {
					return err
				}
			}

			// Create unique index on object_id
			return usecase.clientDbIndexEditor.CreateUniqueIndex(
				ctx,
				orgTx,
				organizationId,
				getFieldUniqueIndex(input.Name, "object_id"),
			)
		})
	})
	if err != nil {
		return "", err
	}

	return tableId, nil
}

// TODO: Change this method to accept all modification by batch (including fields and links)
func (usecase *usecase) UpdateDataModelTable(
	ctx context.Context,
	tableID string,
	description *string,
	ftmEntity pure_utils.Null[models.FollowTheMoneyEntity],
	alias pure_utils.Null[string],
	semanticType pure_utils.Null[models.SemanticType],
	captionField pure_utils.Null[string],
	primaryOrderingField pure_utils.Null[string],
) error {
	err := usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		table, err := usecase.dataModelRepository.GetDataModelTable(ctx, tx, tableID)
		if err != nil {
			return err
		}
		if err := usecase.enforceSecurity.WriteDataModel(table.OrganizationID); err != nil {
			return err
		}

		if captionField.Set && captionField.Valid {
			dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, tx, table.OrganizationID, false, false)
			if err != nil {
				return err
			}

			if _, ok := dataModel.Tables[table.Name].Fields[captionField.Value()]; !ok {
				return errors.Wrapf(models.BadParameterError,
					"field %s not found on table %s", captionField.Value(), table.Name)
			}
			if dataModel.Tables[table.Name].Fields[captionField.Value()].DataType != models.String {
				return errors.Wrap(models.BadParameterError,
					"a table caption field must be a string field")
			}

			indexExists, err := usecase.indexEditor.IngestedObjectsSearchIndexExists(ctx,
				usecase.enforceSecurity.OrgId(), table.Name, captionField.Value())
			if err != nil {
				return err
			}
			if !indexExists {
				index := models.ConcreteIndex{
					Type:      models.IndexTypeIngestedObjectsSearch,
					TableName: table.Name,
					Indexed:   []string{captionField.Value()},
				}

				if err := usecase.taskQueueRepository.EnqueueCreateIndexTask(ctx, tx,
					usecase.enforceSecurity.OrgId(), []models.ConcreteIndex{index}); err != nil {
					return err
				}
			}
		}

		if primaryOrderingField.Set && primaryOrderingField.Valid {
			dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, tx, table.OrganizationID, false, false)
			if err != nil {
				return err
			}

			field, ok := dataModel.Tables[table.Name].Fields[primaryOrderingField.Value()]
			if !ok {
				return errors.Wrapf(models.BadParameterError,
					"field %s not found on table %s", primaryOrderingField.Value(), table.Name)
			}
			if field.DataType != models.Timestamp {
				return errors.Wrap(models.BadParameterError,
					"primary ordering field must be a timestamp field")
			}
		}

		err = usecase.dataModelRepository.UpdateDataModelTable(ctx, tx, tableID, description,
			ftmEntity, alias, semanticType, captionField, primaryOrderingField)
		if err != nil {
			return err
		}

		// Validation after update
		if err := usecase.validateTableSemanticType(ctx, tx, table.OrganizationID, &table.Name, nil); err != nil {
			return err
		}
		return nil
	})
	return err
}

func (usecase *usecase) UpdateDataModelTableComposite(
	ctx context.Context,
	tableID string,
	input models.UpdateTableCompositeInput,
) (models.DataModelDeleteFieldReport, error) {
	conflictReport := models.NewDataModelDeleteFieldReport()

	err := usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		// 1. Get table metadata and enforce security
		table, err := usecase.dataModelRepository.GetDataModelTable(ctx, tx, tableID)
		if err != nil {
			return err
		}
		if err := usecase.enforceSecurity.WriteDataModel(table.OrganizationID); err != nil {
			return err
		}

		// 2. Conflict checking for link deletions
		for _, linkId := range input.LinksToDelete {
			canDelete, report, err := usecase.destroyUsecase.canDeleteLink(
				ctx, table.OrganizationID, tx, linkId)
			if err != nil {
				return err
			}
			if !canDelete || report.ArchivedIterations.Size() > 0 {
				conflictReport = report
				return errors.Wrap(models.ConflictError,
					fmt.Sprintf("link %s has conflicts and cannot be deleted", linkId))
			}
		}

		// 3. Conflict checking for field deletions
		// Also collect field metadata before deleting (needed for org schema operations later)
		deletedFields := make([]models.FieldMetadata, 0, len(input.FieldsToDelete))
		for _, fieldId := range input.FieldsToDelete {
			field, err := usecase.dataModelRepository.GetDataModelField(ctx, tx, fieldId)
			if err != nil {
				return err
			}
			if field.Name == "object_id" || field.Name == "updated_at" {
				return errors.Wrap(models.BadParameterError,
					"cannot delete reserved fields object_id and updated_at")
			}
			canDelete, report, err := usecase.destroyUsecase.canDeleteRef(
				ctx, table.OrganizationID, tx, table, &field)
			if err != nil {
				return err
			}
			if !canDelete || report.ArchivedIterations.Size() > 0 {
				conflictReport = report
				return errors.Wrap(models.ConflictError,
					fmt.Sprintf("field %s has conflicts and cannot be deleted", fieldId))
			}
			deletedFields = append(deletedFields, field)
		}

		// 4. Execute link deletions (cascade pivot for BelongsTo)
		links, err := usecase.dataModelRepository.GetLinks(ctx, tx, table.OrganizationID)
		if err != nil {
			return err
		}
		for _, linkId := range input.LinksToDelete {
			// Find the link to determine its type
			var linkType models.LinkType
			for _, l := range links {
				if l.Id == linkId {
					linkType = l.LinkType
					break
				}
			}

			// For BelongsTo links, cascade-delete associated pivot(s) found via PathLinkIds
			if linkType == models.LinkTypeBelongsTo {
				pivots, err := usecase.dataModelRepository.ListPivots(
					ctx, tx, table.OrganizationID, nil, false)
				if err != nil {
					return err
				}
				for _, pivot := range pivots {
					for _, pathLinkId := range pivot.PathLinkIds {
						if pathLinkId == linkId {
							if err := usecase.dataModelRepository.DeleteDataModelPivot(
								ctx, tx, pivot.Id.String()); err != nil {
								return err
							}
							break
						}
					}
				}
			}

			if err := usecase.dataModelRepository.DeleteDataModelLink(ctx, tx, linkId); err != nil {
				return err
			}
		}

		// 5. Execute field deletions (metadata only — org schema handled in step 11)
		for _, field := range deletedFields {
			if err := usecase.dataModelRepository.DeleteDataModelField(ctx, tx, table, field); err != nil {
				return err
			}
		}

		// 6. Add new fields
		// Refresh table metadata after deletions (FTMEntity may have changed for validation)
		table, err = usecase.dataModelRepository.GetDataModelTable(ctx, tx, tableID)
		if err != nil {
			return err
		}

		dataModel, err := usecase.dataModelRepository.GetDataModel(
			ctx, tx, table.OrganizationID, false, false)
		if err != nil {
			return err
		}
		fieldIdsByName := make(map[string]string)
		for name, field := range dataModel.Tables[table.Name].Fields {
			fieldIdsByName[name] = field.ID
		}

		for _, f := range input.FieldsToAdd {
			f.TableId = tableID
			fieldId, err := usecase.createDataModelFieldWithExec(ctx, tx, table, f)
			if err != nil {
				return err
			}
			fieldIdsByName[f.Name] = fieldId
		}

		// 7. Add new links
		tablesById := dataModel.AllTablesAsMap()
		for _, l := range input.LinksToAdd {
			parentFieldId, err := retrieveParentFieldIdForLink(l.ParentTableID, tablesById)
			if err != nil {
				return err
			}

			childFieldId, ok := fieldIdsByName[l.ChildFieldName]
			if !ok {
				return errors.Wrap(models.BadParameterError,
					fmt.Sprintf("child field %q not found when creating link", l.ChildFieldName))
			}

			if _, err := usecase.createDataModelLinkWithExec(ctx, tx, models.DataModelLinkCreateInput{
				OrganizationID: table.OrganizationID,
				Name:           l.Name,
				LinkType:       l.LinkType,
				ParentTableID:  l.ParentTableID,
				ParentFieldID:  parentFieldId,
				ChildTableID:   tableID,
				ChildFieldID:   childFieldId,
			}); err != nil {
				return errors.Wrap(err, "failed to create link")
			}
		}

		// 8. Modify existing fields
		for _, f := range input.FieldsToUpdate {
			field, err := usecase.dataModelRepository.GetDataModelField(ctx, tx, f.ID)
			if err != nil {
				return err
			}

			if field.Name == "object_id" || field.Name == "updated_at" {
				if f.IsEnum != nil || f.IsNullable != nil || f.IsUnique != nil || f.FTMProperty.Set {
					return errors.Wrap(models.BadParameterError,
						"only the description of the `object_id` and `updated_at` fields can be updated")
				}
			}

			if err := validateFTMProperty(table, f.FTMProperty); err != nil {
				return err
			}

			if err := usecase.dataModelRepository.UpdateDataModelField(
				ctx, tx, f.ID, f.UpdateFieldInput); err != nil {
				return err
			}
		}

		// 9. Update table properties
		if input.CaptionField.Set && input.CaptionField.Valid {
			// Re-fetch data model to include fields added/updated in steps 6-8
			freshDataModel, err := usecase.dataModelRepository.GetDataModel(
				ctx, tx, table.OrganizationID, false, false)
			if err != nil {
				return err
			}

			if _, ok := freshDataModel.Tables[table.Name].Fields[input.CaptionField.Value()]; !ok {
				return errors.Wrapf(models.BadParameterError,
					"field %s not found on table %s", input.CaptionField.Value(), table.Name)
			}
			if freshDataModel.Tables[table.Name].Fields[input.CaptionField.Value()].DataType != models.String {
				return errors.Wrap(models.BadParameterError,
					"a table caption field must be a string field")
			}

			indexExists, err := usecase.indexEditor.IngestedObjectsSearchIndexExists(ctx,
				usecase.enforceSecurity.OrgId(), table.Name, input.CaptionField.Value())
			if err != nil {
				return err
			}
			if !indexExists {
				index := models.ConcreteIndex{
					Type:      models.IndexTypeIngestedObjectsSearch,
					TableName: table.Name,
					Indexed:   []string{input.CaptionField.Value()},
				}

				if err := usecase.taskQueueRepository.EnqueueCreateIndexTask(ctx, tx,
					usecase.enforceSecurity.OrgId(), []models.ConcreteIndex{index}); err != nil {
					return err
				}
			}
		}

		if err := usecase.dataModelRepository.UpdateDataModelTable(ctx, tx, tableID,
			input.Description, input.FTMEntity, input.Alias, input.SemanticType,
			input.CaptionField, input.PrimaryOrderingField); err != nil {
			return err
		}

		// 10. Semantic validation (single check at the end, after all mutations)
		if err := usecase.validateTableSemanticType(
			ctx, tx, table.OrganizationID, &table.Name, nil); err != nil {
			return err
		}

		// Ensure the table has a pivot (default object_id pivot when none exist)
		if err := usecase.ensureTableHasPivot(
			ctx, tx, table.OrganizationID, tableID, fieldIdsByName); err != nil {
			return err
		}

		// 11. Org schema mutations (add/delete physical columns)
		return usecase.transactionFactory.TransactionInOrgSchema(
			ctx, table.OrganizationID, func(orgTx repositories.Transaction) error {
				for _, f := range input.FieldsToAdd {
					if f.Name == "object_id" || f.Name == "updated_at" {
						continue
					}
					if err := usecase.organizationSchemaRepository.CreateField(
						ctx, orgTx, table.Name, f); err != nil {
						return err
					}
				}

				for _, field := range deletedFields {
					if err := usecase.organizationSchemaRepository.DeleteField(
						ctx, orgTx, table.Name, field.Name); err != nil {
						return err
					}
				}

				return nil
			},
		)
	})
	if err != nil {
		return conflictReport, err
	}

	return models.DataModelDeleteFieldReport{Performed: true}, nil
}

func (usecase *usecase) createDataModelFieldWithExec(ctx context.Context,
	exec repositories.Executor, table models.TableMetadata, field models.CreateFieldInput,
) (string, error) {
	fieldId := pure_utils.NewId().String()

	if !validNameRegex.MatchString(field.Name) {
		return "", errors.Wrapf(models.BadParameterError,
			"field name %q must only contain lower case alphanumeric characters and underscores, and start by a letter", field.Name)
	}
	if models.DataModelReservedFieldNames[field.Name] {
		return "", errors.Wrap(models.BadParameterError,
			"field name is reserved and cannot be used")
	}
	if field.DataType == models.UnknownDataType {
		return "", errors.Wrapf(models.BadParameterError,
			"invalid data type for field %q", field.Name)
	}
	if err := validateFTMProperty(table, pure_utils.NullFromPtr(field.FTMProperty)); err != nil {
		return "", errors.Wrap(models.BadParameterError, err.Error())
	}
	if err := models.ValidateField(models.Field{
		Name:         field.Name,
		Description:  field.Description,
		Alias:        field.Alias,
		DataType:     field.DataType,
		Nullable:     field.Nullable,
		IsEnum:       field.IsEnum,
		FTMProperty:  field.FTMProperty,
		SemanticType: field.SemanticType,
	}); err != nil {
		return "", errors.Wrap(models.BadParameterError,
			fmt.Sprintf("invalid field %q: %s", field.Name, err.Error()))
	}

	if err := usecase.dataModelRepository.CreateDataModelField(ctx, exec, table.OrganizationID, fieldId, field); err != nil {
		return "", err
	}

	return fieldId, nil
}

func (usecase *usecase) CreateDataModelField(ctx context.Context, field models.CreateFieldInput) (string, error) {
	var fieldId string
	err := usecase.transactionFactory.Transaction(
		ctx,
		func(tx repositories.Transaction) error {
			var err error
			table, err := usecase.dataModelRepository.GetDataModelTable(ctx, tx, field.TableId)
			if err != nil {
				return err
			}
			if err := usecase.enforceSecurity.WriteDataModel(table.OrganizationID); err != nil {
				return err
			}

			fieldId, err = usecase.createDataModelFieldWithExec(
				ctx, tx, table, field,
			)
			if err != nil {
				return err
			}

			// validation before creating field in org schema
			if err := usecase.validateTableSemanticType(ctx, tx, table.OrganizationID, &table.Name, nil); err != nil {
				return err
			}

			db, err := usecase.executorFactory.NewClientDbExecutor(ctx, table.OrganizationID)
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

func (usecase *usecase) UpdateDataModelField(ctx context.Context, fieldID string, input models.UpdateFieldInput) error {
	// Note for the future me: if we want to allow making a "not nullable" field to "nullable", we need to also have a routine
	// that removes the constraint on the DB if it exists, for backwards compatibility.
	// We currently no longer add those constraints on fields marked as required and their value is only enforced at ingestion time
	// in our code, as of early dec 2024.
	err := usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		// permission and input validation
		field, err := usecase.dataModelRepository.GetDataModelField(ctx, tx, fieldID)
		if err != nil {
			return err
		}
		table, err := usecase.dataModelRepository.GetDataModelTable(ctx, tx, field.TableId)
		if err != nil {
			return err
		} else if err := usecase.enforceSecurity.WriteDataModel(table.OrganizationID); err != nil {
			return err
		}

		if field.Name == "object_id" || field.Name == "updated_at" {
			if input.IsEnum != nil || input.IsNullable != nil || input.IsUnique != nil || input.FTMProperty.Set {
				return errors.Wrap(models.BadParameterError,
					"only the description of the `object_id` and `updated_at` fields can be updated")
			}
		}

		dataModel, err := usecase.getDataModelWithExec(ctx, tx, table.OrganizationID,
			models.DataModelReadOptions{IncludeUnicityConstraints: true}, false)
		if err != nil {
			return err
		}

		makeUnique, makeNotUnique, err := validateFieldUpdateRules(dataModel, field, table, input)
		if err != nil {
			return err
		}

		// Check if the FTM property is valid and supported by the FTM entity defined in the table
		if err := validateFTMProperty(table, input.FTMProperty); err != nil {
			return err
		}

		// update the field (data_model_field row)
		if err := usecase.dataModelRepository.UpdateDataModelField(ctx, tx, fieldID, input); err != nil {
			return err
		}

		// Validation after update
		if err := usecase.validateTableSemanticType(ctx, tx, table.OrganizationID, &table.Name, nil); err != nil {
			return err
		}

		// NOTE: I decided to not touch the Unique index management here, but I think we can at least remove the `makeUnique` part
		// since we don't allow the user to create unique field anymore.
		// We could keep the `makeNotUnique` part for older fields that were unique before this change and want to remove the unicity constraint
		//
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
	})
	return err
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

func validateFTMProperty(table models.TableMetadata, property pure_utils.Null[models.FollowTheMoneyProperty]) error {
	// Check if the FTM property is valid and supported by the FTM entity defined in the table
	if property.Set {
		if property.Valid {
			if table.FTMEntity == nil {
				return fmt.Errorf("FTM entity not defined for table %s", table.Name)
			}
			ok := slices.Contains(models.FollowTheMoneyEntityProperties[*table.FTMEntity], property.Value())
			if !ok {
				return fmt.Errorf(
					"invalid FTM property for entity %s: %s",
					*table.FTMEntity,
					property.Value(),
				)
			}
		}
	}
	return nil
}

func (usecase *usecase) createDataModelLinkWithExec(ctx context.Context, exec repositories.Executor, link models.DataModelLinkCreateInput) (string, error) {
	dataModel, err := usecase.getDataModelWithExec(ctx, exec, link.OrganizationID, models.DataModelReadOptions{}, false)
	if err != nil {
		return "", err
	}
	allTables := dataModel.AllTablesAsMap()
	allFields := dataModel.AllFieldsAsMap()

	if !validNameRegex.MatchString(link.Name) {
		return "", errors.Wrap(models.BadParameterError,
			"link name must only contain lower case alphanumeric characters and underscores, and start by a letter")
	}

	// check existence of tables and fields
	if _, ok := allTables[link.ChildTableID]; !ok {
		return "", errors.Wrap(models.NotFoundError,
			fmt.Sprintf("child table %s not found", link.ChildTableID))
	}
	if _, ok := allTables[link.ParentTableID]; !ok {
		return "", errors.Wrap(models.NotFoundError,
			fmt.Sprintf("parent table %s not found", link.ParentTableID))
	}
	childField, ok := allFields[link.ChildFieldID]
	if !ok {
		return "", errors.Wrap(models.NotFoundError,
			fmt.Sprintf("child field %s not found", link.ChildFieldID))
	}
	if childField.TableId != link.ChildTableID {
		// Should not occur if the ID is correct, but just in case
		return "", errors.Wrap(models.BadParameterError,
			fmt.Sprintf("child field %s does not belong to child table %s", link.ChildFieldID, link.ChildTableID))
	}
	parentField, ok := allFields[link.ParentFieldID]
	if !ok {
		return "", errors.Wrap(models.NotFoundError,
			fmt.Sprintf("parent field %s not found", link.ParentFieldID))
	}
	if parentField.TableId != link.ParentTableID {
		// Should not occur if the ID is correct, but just in case
		return "", errors.Wrap(models.BadParameterError,
			fmt.Sprintf("parent field %s does not belong to parent table %s",
				link.ParentFieldID, link.ParentTableID))
	}

	if parentField.Name != "object_id" {
		return "", errors.Wrap(models.BadParameterError,
			"parent field must be the object_id field")
	}

	if childField.DataType != models.String {
		return "", errors.Wrap(models.BadParameterError,
			fmt.Sprintf("child field must be a string, field %s is %s", childField.Name, childField.DataType.String()))
	}
	if childField.Name == "object_id" {
		return "", errors.Wrap(models.BadParameterError,
			"child field cannot be object_id")
	}

	// Can only have one BelongsTo link on the child Table
	if link.LinkType == models.LinkTypeBelongsTo {
		for _, l := range allTables[link.ChildTableID].LinksToSingle {
			if l.LinkType == models.LinkTypeBelongsTo {
				return "", errors.Wrap(models.BadParameterError,
					fmt.Sprintf("child table %s already has a belongs_to link", allTables[link.ChildTableID].Name))
			}
		}

		// Delete the existing pivot if exists, as we will create a new one
		// Can have a pivot without a belongs_to link because we create a default pivot for every table which refer to
		// itself
		pivots, err := usecase.dataModelRepository.ListPivots(ctx, exec,
			link.OrganizationID, utils.Ptr(link.ChildTableID), false)
		if err != nil {
			return "", err
		}
		// Use loop to not handle the empty pivot case differently. But in practice can have only 0 or 1 pivot on the table
		for _, pivot := range pivots {
			if err := usecase.dataModelRepository.DeleteDataModelPivot(ctx, exec, pivot.Id.String()); err != nil {
				return "", err
			}
		}
	}

	linkId := pure_utils.NewId().String()
	if err := usecase.dataModelRepository.CreateDataModelLink(ctx, exec, linkId, link); err != nil {
		return "", err
	}

	// BelongsTo with different tables: also create a path-based pivot
	if link.LinkType == models.LinkTypeBelongsTo {
		_, err = usecase.CreatePivotWithExec(ctx, exec, models.CreatePivotInput{
			OrganizationId: link.OrganizationID,
			BaseTableId:    link.ChildTableID,
			PathLinkIds:    []string{linkId},
		})
		if err != nil {
			return linkId, err
		}
	}

	return linkId, nil
}

// This method handles links between data model tables.
// A link can be between different tables or a self-link.
// `related` type will create a link without creating a pivot, while `belongs_to` will create a pivot and a link if the
// link is between different tables.
func (usecase *usecase) CreateDataModelLink(ctx context.Context, link models.DataModelLinkCreateInput) (string, error) {
	if err := usecase.enforceSecurity.WriteDataModel(link.OrganizationID); err != nil {
		return "", err
	}
	return executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Transaction) (string, error) {
			linkId, err := usecase.createDataModelLinkWithExec(ctx, tx, link)
			if err != nil {
				return "", err
			}

			if err := usecase.validateTableSemanticType(ctx, tx, link.OrganizationID, nil, &link.ChildTableID); err != nil {
				return "", err
			}

			return linkId, nil
		},
	)
}

func (usecase *usecase) DeleteDataModel(ctx context.Context, organizationID uuid.UUID) error {
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

func (usecase *usecase) CreatePivotWithExec(ctx context.Context, exec repositories.Executor, input models.CreatePivotInput) (models.Pivot, error) {
	if err := usecase.enforceSecurity.WriteDataModel(input.OrganizationId); err != nil {
		return models.Pivot{}, err
	}

	dm, err := usecase.dataModelRepository.GetDataModel(ctx, exec, input.OrganizationId, false, false)
	if err != nil {
		return models.Pivot{}, err
	}

	if err := validatePivotCreateInput(input, dm); err != nil {
		return models.Pivot{}, err
	}

	id := pure_utils.NewId().String()
	err = usecase.dataModelRepository.CreatePivot(ctx, exec, id, input)
	if err != nil {
		return models.Pivot{}, err
	}
	pivotMeta, err := usecase.dataModelRepository.GetPivot(ctx, exec, id)
	return pivotMeta.Enrich(dm), err
}

func (usecase *usecase) CreatePivot(ctx context.Context, input models.CreatePivotInput) (models.Pivot, error) {
	return executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Transaction) (models.Pivot, error) {
			return usecase.CreatePivotWithExec(ctx, tx, input)
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

func (usecase *usecase) ListPivots(ctx context.Context, organizationId uuid.UUID, tableID *string) ([]models.Pivot, error) {
	if err := usecase.enforceSecurity.ReadDataModel(); err != nil {
		return nil, err
	}

	exec := usecase.executorFactory.NewExecutor()

	dm, err := usecase.dataModelRepository.GetDataModel(ctx, exec, organizationId, false, false)
	if err != nil {
		return nil, err
	}

	pivotsMeta, err := usecase.dataModelRepository.ListPivots(ctx, exec, organizationId, tableID, false)
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

func (usecase *usecase) CreateNavigationOption(ctx context.Context, input models.CreateNavigationOptionInput) error {
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
	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, exec, orgId, true, false)
	if err != nil {
		return err
	}
	uniqueIndexes, err := usecase.clientDbIndexEditor.ListAllUniqueIndexes(ctx, orgId)
	if err != nil {
		return err
	}
	dataModel = dataModel.AddUnicityConstraintStatusToDataModel(uniqueIndexes)
	allTables := dataModel.AllTablesAsMap()

	pivotsMeta, err := usecase.dataModelRepository.ListPivots(ctx, exec, orgId, nil, false)
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
		// WARNING: Pivot check seems different from the AddNavigationOptionsToDataModel method.
		// Mismatch between SourceTableId and TargetTableId because the index.TableName represent the TargetTableId
		// and not the SourceTableId.
		// TODO: Fix it or delete this part if we don't want to allow navigation option on pivot field.
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
			navOption.OrderingFieldName == orderingField.Name &&
			navOption.TargetTableName == targetTable.Name {
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
	switch input.Blocking {
	case true:
		return usecase.clientDbIndexEditor.CreateIndexesBlocking(ctx, orgId, []models.ConcreteIndex{
			{
				Type:      models.IndexTypeNavigation,
				TableName: targetTable.Name,
				Indexed:   []string{filterField.Name, orderingField.Name},
			},
		})

	case false:
		return usecase.clientDbIndexEditor.CreateIndexesAsync(ctx, orgId, []models.ConcreteIndex{
			{
				Type:      models.IndexTypeNavigation,
				TableName: targetTable.Name,
				Indexed:   []string{filterField.Name, orderingField.Name},
			},
		})
	}

	return nil
}

// TODO: We should probably remove the DataModelOptions since the frontend use the table/field metadata for
// display setting like hidden and field order
func (usecase usecase) GetDataModelOptions(ctx context.Context, orgId uuid.UUID, tableId string) (models.DataModelOptions, error) {
	exec := usecase.executorFactory.NewExecutor()

	if err := usecase.enforceSecurity.ReadDataModel(); err != nil {
		return models.DataModelOptions{}, err
	}

	tableMeta, err := usecase.dataModelRepository.GetDataModelTable(ctx, exec, tableId)
	if err != nil {
		return models.DataModelOptions{}, err
	}
	if tableMeta.OrganizationID != orgId {
		return models.DataModelOptions{}, errors.Wrap(models.NotFoundError, "table not found")
	}

	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, exec, tableMeta.OrganizationID, false, false)
	if err != nil {
		return models.DataModelOptions{}, err
	}

	opts, err := usecase.dataModelRepository.GetDataModelOptionsForTable(ctx, exec, tableId)
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

// TODO: DataModelOptions is deprecated since the options will be moved to the table/field metadata.
// Confirm with the frontend team
func (usecase usecase) UpdateDataModelOptions(ctx context.Context,
	orgId uuid.UUID,
	req models.UpdateDataModelOptionsRequest,
) (models.DataModelOptions, error) {
	exec := usecase.executorFactory.NewExecutor()

	if err := usecase.enforceSecurity.WriteDataModel(orgId); err != nil {
		return models.DataModelOptions{}, err
	}

	tableMeta, err := usecase.dataModelRepository.GetDataModelTable(ctx, exec, req.TableId)
	if err != nil {
		return models.DataModelOptions{}, err
	}
	if tableMeta.OrganizationID != orgId {
		return models.DataModelOptions{}, errors.Wrap(models.NotFoundError, "table not found")
	}
	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, exec, tableMeta.OrganizationID, false, false)
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

	opts, err := usecase.dataModelRepository.UpsertDataModelOptions(ctx, exec, req)
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
