package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

type DataModelRepository interface {
	GetDataModel(ctx context.Context, exec Executor, organizationID string, fetchEnumValues bool) (models.DataModel, error)
	CreateDataModelTable(ctx context.Context, exec Executor, organizationID, tableID, name, description string) error
	UpdateDataModelTable(ctx context.Context, exec Executor, tableID, description string) error
	GetDataModelTable(ctx context.Context, exec Executor, tableID string) (models.TableMetadata, error)
	CreateDataModelField(ctx context.Context, exec Executor, fieldId string, field models.CreateFieldInput) error
	UpdateDataModelField(
		ctx context.Context,
		exec Executor,
		field string,
		input models.UpdateFieldInput,
	) error
	CreateDataModelLink(
		ctx context.Context,
		exec Executor,
		id string,
		link models.DataModelLinkCreateInput,
	) error
	GetLinks(ctx context.Context, exec Executor, organizationId string) ([]models.LinkToSingle, error)
	DeleteDataModel(ctx context.Context, exec Executor, organizationID string) error
	GetDataModelField(ctx context.Context, exec Executor, fieldId string) (models.FieldMetadata, error)
	BatchInsertEnumValues(ctx context.Context, exec Executor, enumValues models.EnumValues, table models.Table) error

	CreatePivot(ctx context.Context, exec Executor, id string, pivot models.CreatePivotInput) error
	ListPivots(ctx context.Context, exec Executor, organization_id string, tableId *string) ([]models.PivotMetadata, error)
	GetPivot(ctx context.Context, exec Executor, pivotId string) (models.PivotMetadata, error)

	GetDataModelOptionsForTable(ctx context.Context, exec Executor, tableId string) (*models.DataModelOptions, error)
	UpsertDataModelOptions(ctx context.Context, exec Executor,
		req models.UpdateDataModelOptionsRequest) (models.DataModelOptions, error)
}

type DataModelRepositoryPostgresql struct{}

func (repo MarbleDbRepository) GetDataModel(
	ctx context.Context,
	exec Executor,
	organizationID string,
	fetchEnumValues bool,
) (models.DataModel, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.DataModel{}, err
	}

	fields, err := repo.getTablesAndFields(ctx, exec, organizationID)
	if err != nil {
		return models.DataModel{}, err
	}

	links, err := repo.GetLinks(ctx, exec, organizationID)
	if err != nil {
		return models.DataModel{}, err
	}

	dataModel := models.DataModel{
		Tables: make(map[string]models.Table),
	}

	for _, field := range fields {
		var values []any
		if field.FieldIsEnum && fetchEnumValues {
			values, err = repo.GetEnumValues(ctx, exec, field.FieldID)
			if err != nil {
				return models.DataModel{}, err
			}
		}

		_, ok := dataModel.Tables[field.TableName]
		if !ok {
			dataModel.Tables[field.TableName] = models.Table{
				ID:            field.TableID,
				Name:          field.TableName,
				Description:   field.TableDescription,
				Fields:        map[string]models.Field{},
				LinksToSingle: make(map[string]models.LinkToSingle),
			}
		}
		dataModel.Tables[field.TableName].Fields[field.FieldName] = models.Field{
			ID:          field.FieldID,
			DataType:    models.DataTypeFrom(field.FieldType),
			Description: field.FieldDescription,
			Name:        field.FieldName,
			Nullable:    field.FieldNullable,
			IsEnum:      field.FieldIsEnum,
			TableId:     field.TableID,
			Values:      values,
		}

	}

	for _, link := range links {
		dataModel.Tables[link.ChildTableName].LinksToSingle[link.Name] = link
	}
	return dataModel, nil
}

func (repo MarbleDbRepository) CreateDataModelTable(ctx context.Context, exec Executor, organizationID, tableID, name, description string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := `
		INSERT INTO data_model_tables (id, organization_id, name, description)
		VALUES ($1, $2, $3, $4)`

	_, err := exec.Exec(ctx, query, tableID, organizationID, name, description)
	if IsUniqueViolationError(err) {
		return models.ConflictError
	}
	return err
}

func (repo MarbleDbRepository) GetDataModelTable(ctx context.Context, exec Executor, tableID string) (models.TableMetadata, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.TableMetadata{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.SelectDataModelTableColumns...).
			From(dbmodels.TableDataModelTables).
			Where(squirrel.Eq{"id": tableID}),
		dbmodels.AdaptTableMetadata,
	)
}

func (repo MarbleDbRepository) UpdateDataModelTable(ctx context.Context, exec Executor, tableID, description string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().
			Update(dbmodels.TableDataModelTables).
			Set("description", description).
			Where(squirrel.Eq{"id": tableID}),
	)
	return err
}

func (repo MarbleDbRepository) CreateDataModelField(
	ctx context.Context,
	exec Executor,
	fieldId string,
	field models.CreateFieldInput,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := `
		INSERT INTO data_model_fields (id, table_id, name, type, nullable, description, is_enum)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`

	_, err := exec.Exec(ctx,
		query,
		fieldId,
		field.TableId,
		field.Name,
		field.DataType.String(),
		field.Nullable,
		field.Description,
		field.IsEnum,
	)
	if IsUniqueViolationError(err) {
		return models.ConflictError
	}
	return err
}

func (repo MarbleDbRepository) UpdateDataModelField(
	ctx context.Context,
	exec Executor,
	fieldID string,
	input models.UpdateFieldInput,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}
	query := NewQueryBuilder().
		Update(dbmodels.TableDataModelFields).
		Where(squirrel.Eq{"id": fieldID})

	if input.Description != nil {
		query = query.Set("description", *input.Description)
	}
	if input.IsEnum != nil {
		query = query.Set("is_enum", *input.IsEnum)
	}

	err := ExecBuilder(
		ctx,
		exec,
		query,
	)
	return err
}

func (repo MarbleDbRepository) CreateDataModelLink(ctx context.Context, exec Executor, id string, link models.DataModelLinkCreateInput) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().
			Insert("data_model_links").
			Columns(
				"id",
				"organization_id",
				"name",
				"parent_table_id",
				"parent_field_id",
				"child_table_id",
				"child_field_id",
			).
			Values(
				id,
				link.OrganizationID,
				strings.ToLower(link.Name),
				link.ParentTableID,
				link.ParentFieldID,
				link.ChildTableID,
				link.ChildFieldID,
			),
	)
	if IsUniqueViolationError(err) {
		return models.ConflictError
	}
	return err
}

func (repo MarbleDbRepository) getTablesAndFields(ctx context.Context, exec Executor,
	organizationID string,
) ([]dbmodels.DbDataModelTableJoinField, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query, args, err := NewQueryBuilder().
		Select(dbmodels.SelectDataModelTableJoinFieldColumns...).
		From(dbmodels.TableDataModelTables).
		Join(fmt.Sprintf("%s ON (data_model_tables.id = data_model_fields.table_id)", dbmodels.TableDataModelFields)).
		Where(squirrel.Eq{"organization_id": organizationID}).
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := exec.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	fields, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (
		dbmodels.DbDataModelTableJoinField, error,
	) {
		var dbModel dbmodels.DbDataModelTableJoinField
		if err := rows.Scan(&dbModel.TableID,
			&dbModel.OrganizationID,
			&dbModel.TableName,
			&dbModel.TableDescription,
			&dbModel.FieldID,
			&dbModel.FieldName,
			&dbModel.FieldType,
			&dbModel.FieldNullable,
			&dbModel.FieldDescription,
			&dbModel.FieldIsEnum,
		); err != nil {
			return dbmodels.DbDataModelTableJoinField{}, err
		}
		return dbModel, nil
	})
	return fields, err
}

func (repo MarbleDbRepository) GetLinks(ctx context.Context, exec Executor, organizationID string) ([]models.LinkToSingle, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := `
	SELECT
		links.id,
		links.organization_id,
		links.name,
		parent_table.name,
		parent_table.id,
		parent_field.name,
		parent_field.id,
		child_table.name,
		child_table.id,
		child_field.name,
		child_field.id
	FROM data_model_links AS links
    	JOIN data_model_tables AS parent_table ON (links.parent_table_id = parent_table.id)
    	JOIN data_model_fields AS parent_field ON (links.parent_field_id = parent_field.id)
    	JOIN data_model_tables AS child_table ON (links.child_table_id = child_table.id)
    	JOIN data_model_fields AS child_field ON (links.child_field_id = child_field.id)
    	WHERE links.organization_id = $1`

	rows, err := exec.Query(ctx, query, organizationID)
	if err != nil {
		return nil, err
	}

	links, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.LinkToSingle, error) {
		var dbLinks dbmodels.DbDataModelLink
		if err := rows.Scan(
			&dbLinks.Id,
			&dbLinks.OrganizationId,
			&dbLinks.Name,
			&dbLinks.ParentTableName,
			&dbLinks.ParentTableId,
			&dbLinks.ParentFieldName,
			&dbLinks.ParentFieldId,
			&dbLinks.ChildTableName,
			&dbLinks.ChildTableId,
			&dbLinks.ChildFieldName,
			&dbLinks.ChildFieldId,
		); err != nil {
			return models.LinkToSingle{}, err
		}
		return dbmodels.AdaptLinkToSingle(dbLinks), err
	})
	return links, nil
}

func (repo MarbleDbRepository) DeleteDataModel(ctx context.Context, exec Executor, organizationID string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	return ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().
			Delete(dbmodels.TableDataModelTables).
			Where(squirrel.Eq{"organization_id": organizationID}),
	)
}

func (repo MarbleDbRepository) GetEnumValues(ctx context.Context, exec Executor, fieldID string) ([]any, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query, args, err := NewQueryBuilder().
		Select("text_value", "float_value").
		From("data_model_enum_values").
		Where(squirrel.Eq{"field_id": fieldID}).
		Where("(text_value IS NOT NULL OR float_value IS NOT NULL)").
		Limit(100).
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := exec.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	values, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (any, error) {
		var valueString, valueFloat any
		if err := rows.Scan(&valueString, &valueFloat); err != nil {
			return "", err
		}
		// presumably if there is a row, one of the values should be non-nil
		if valueString != nil {
			return valueString, nil
		}
		return valueFloat, err
	})
	return values, nil
}

func (repo MarbleDbRepository) GetDataModelField(ctx context.Context, exec Executor, fieldId string) (models.FieldMetadata, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.FieldMetadata{}, err
	}

	query := `
		SELECT
			data_model_fields.description,
			data_model_fields.is_enum,
			data_model_fields.name,
			data_model_fields.nullable,
			data_model_fields.table_id,
			data_model_fields.type
		FROM data_model_fields
		WHERE id = $1
	`

	row := exec.QueryRow(ctx, query, fieldId)

	var field models.FieldMetadata
	var dataType string
	if err := row.Scan(
		&field.Description,
		&field.IsEnum,
		&field.Name,
		&field.Nullable,
		&field.TableId,
		&dataType,
	); errors.Is(err, pgx.ErrNoRows) {
		return models.FieldMetadata{}, fmt.Errorf("error in GetDataModelField: %w", models.NotFoundError)
	} else if err != nil {
		return models.FieldMetadata{}, err
	}
	field.ID = fieldId
	field.DataType = models.DataTypeFrom(dataType)

	return field, nil
}

// ///////////////////////////////
// Data table pivot methods
// ///////////////////////////////

func (repo MarbleDbRepository) CreatePivot(
	ctx context.Context,
	exec Executor,
	id string,
	pivot models.CreatePivotInput,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().
			Insert(dbmodels.TABLE_DATA_MODEL_PIVOTS).
			Columns("id", "organization_id", "base_table_id", "field_id", "path_link_ids").
			Values(id, pivot.OrganizationId, pivot.BaseTableId, pivot.FieldId, pivot.PathLinkIds),
	)

	if IsUniqueViolationError(err) {
		return errors.Wrap(
			models.ConflictError,
			fmt.Sprintf("Conflict on creating pivot for table %s in repository CreatePivot", pivot.BaseTableId),
		)
	}
	return err
}

func (repo MarbleDbRepository) ListPivots(
	ctx context.Context,
	exec Executor,
	organizationId string,
	tableId *string,
) ([]models.PivotMetadata, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectPivotColumns...).
		From(dbmodels.TABLE_DATA_MODEL_PIVOTS).
		Where(squirrel.Eq{"organization_id": organizationId}).
		OrderBy("created_at DESC")

	if tableId != nil {
		query = query.Where(squirrel.Eq{"base_table_id": *tableId})
	}

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptPivotMetadata)
}

func (repo MarbleDbRepository) GetPivot(ctx context.Context, exec Executor, pivotId string) (models.PivotMetadata, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.PivotMetadata{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.SelectPivotColumns...).
			From(dbmodels.TABLE_DATA_MODEL_PIVOTS).
			Where(squirrel.Eq{"id": pivotId}),
		dbmodels.AdaptPivotMetadata,
	)
}

func (repo MarbleDbRepository) BatchInsertEnumValues(ctx context.Context, exec Executor, enumValues models.EnumValues, table models.Table) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	// This has to be done in 2 queries because there cannot be multiple ON CONFLICT clauses per query
	textQuery := NewQueryBuilder().
		Insert("data_model_enum_values").
		Columns("field_id", "text_value").
		Suffix("ON CONFLICT ON CONSTRAINT unique_data_model_enum_text_values_field_id_value DO NOTHING")

	floatQuery := NewQueryBuilder().
		Insert("data_model_enum_values").
		Columns("field_id", "float_value").
		Suffix("ON CONFLICT ON CONSTRAINT unique_data_model_enum_float_values_field_id_value DO NOTHING")

	// Hack to avoid empty query, which would cause an execution error
	var shouldInsertTextValues bool
	var shouldInsertFloatValues bool

	for fieldName, values := range enumValues {
		fieldId := table.Fields[fieldName].ID
		dataType := table.Fields[fieldName].DataType

		for value := range values {
			switch dataType {
			case models.String:
				textQuery = textQuery.Values(fieldId, value)
				shouldInsertTextValues = true
			case models.Float:
				floatQuery = floatQuery.Values(fieldId, value)
				shouldInsertFloatValues = true
			}
		}
	}

	if shouldInsertTextValues {
		err := ExecBuilder(ctx, exec, textQuery)
		if err != nil {
			return err
		}
	}
	if shouldInsertFloatValues {
		err := ExecBuilder(ctx, exec, floatQuery)
		if err != nil {
			return err
		}
	}

	return nil
}
