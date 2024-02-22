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
	GetTablesAndFields(ctx context.Context, exec Executor, organizationID string) ([]models.DataModelTableField, error)
	CreateDataModelTable(ctx context.Context, exec Executor, organizationID, tableID, name, description string) error
	UpdateDataModelTable(ctx context.Context, exec Executor, tableID, description string) error
	GetDataModelTable(ctx context.Context, exec Executor, tableID string) (models.DataModelTable, error)
	CreateDataModelField(ctx context.Context, exec Executor, tableID, fieldID string, field models.DataModelField) error
	UpdateDataModelField(
		ctx context.Context,
		exec Executor,
		field string,
		input models.UpdateDataModelFieldInput,
	) error
	CreateDataModelLink(ctx context.Context, exec Executor, link models.DataModelLink) error
	DeleteDataModel(ctx context.Context, exec Executor, organizationID string) error
	GetDataModelField(ctx context.Context, exec Executor, fieldId string) (models.Field, error)
}

type DataModelRepositoryPostgresql struct{}

func (repo *DataModelRepositoryPostgresql) GetDataModel(ctx context.Context, exec Executor,
	organizationID string, fetchEnumValues bool,
) (models.DataModel, error) {
	fields, err := repo.GetTablesAndFields(ctx, exec, organizationID)
	if err != nil {
		return models.DataModel{}, err
	}

	links, err := repo.GetLinks(ctx, exec, organizationID)
	if err != nil {
		return models.DataModel{}, err
	}

	dataModel := models.DataModel{
		Tables: make(map[models.TableName]models.Table),
	}

	for _, field := range fields {
		tableName := models.TableName(field.TableName)
		fieldName := models.FieldName(field.FieldName)

		var values []any
		if field.FieldIsEnum && fetchEnumValues {
			values, err = repo.GetEnumValues(ctx, exec, field.FieldID)
			if err != nil {
				return models.DataModel{}, err
			}
		}

		_, ok := dataModel.Tables[tableName]
		if ok {
			dataModel.Tables[tableName].Fields[fieldName] = models.Field{
				ID:          field.FieldID,
				Description: field.FieldDescription,
				DataType:    models.DataTypeFrom(field.FieldType),
				Nullable:    field.FieldNullable,
				IsEnum:      field.FieldIsEnum,
				Values:      values,
			}
		} else {
			dataModel.Tables[tableName] = models.Table{
				ID:          field.TableID,
				Name:        tableName,
				Description: field.TableDescription,
				Fields: map[models.FieldName]models.Field{
					fieldName: {
						ID:          field.FieldID,
						Description: field.FieldDescription,
						DataType:    models.DataTypeFrom(field.FieldType),
						Nullable:    field.FieldNullable,
						IsEnum:      field.FieldIsEnum,
						Values:      values,
					},
				},
				LinksToSingle: make(map[models.LinkName]models.LinkToSingle),
			}
		}
	}

	for _, link := range links {
		dataModel.Tables[link.ChildTable].LinksToSingle[link.Name] = models.LinkToSingle{
			LinkedTableName: link.ParentTable,
			ParentFieldName: link.ParentField,
			ChildFieldName:  link.ChildField,
		}
	}
	return dataModel, nil
}

func (repo *DataModelRepositoryPostgresql) CreateDataModelTable(ctx context.Context, exec Executor, organizationID, tableID, name, description string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := `
		INSERT INTO data_model_tables (id, organization_id, name, description)
		VALUES ($1, $2, $3, $4)`

	_, err := exec.Exec(ctx, query, tableID, organizationID, strings.ToLower(name), description)
	if IsUniqueViolationError(err) {
		return models.ConflictError
	}
	return err
}

func (repo *DataModelRepositoryPostgresql) GetDataModelTable(ctx context.Context, exec Executor, tableID string) (models.DataModelTable, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.DataModelTable{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.SelectDataModelTableColumns...).
			From(dbmodels.TableDataModelTable).
			Where(squirrel.Eq{"id": tableID}),
		dbmodels.AdaptDataModelTable,
	)
}

func (repo *DataModelRepositoryPostgresql) UpdateDataModelTable(ctx context.Context, exec Executor, tableID, description string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().
			Update(dbmodels.TableDataModelTable).
			Set("description", description).
			Where(squirrel.Eq{"id": tableID}),
	)
	return err
}

func (repo *DataModelRepositoryPostgresql) CreateDataModelField(ctx context.Context, exec Executor,
	tableID, fieldID string, field models.DataModelField,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := `
		INSERT INTO data_model_fields (id, table_id, name, type, nullable, description, is_enum)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`

	_, err := exec.Exec(ctx, query, fieldID, tableID, strings.ToLower(field.Name), field.Type,
		field.Nullable, field.Description, field.IsEnum)
	if IsUniqueViolationError(err) {
		return models.ConflictError
	}
	return err
}

func (repo *DataModelRepositoryPostgresql) UpdateDataModelField(
	ctx context.Context,
	exec Executor,
	fieldID string,
	input models.UpdateDataModelFieldInput,
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

func (repo *DataModelRepositoryPostgresql) CreateDataModelLink(ctx context.Context, exec Executor, link models.DataModelLink) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().
			Insert("data_model_links").
			Columns("organization_id", "name", "parent_table_id", "parent_field_id", "child_table_id", "child_field_id").
			Values(link.OrganizationID, strings.ToLower(string(link.Name)), link.ParentTableID,
				link.ParentFieldID, link.ChildTableID, link.ChildFieldID),
	)
	if IsUniqueViolationError(err) {
		return models.ConflictError
	}
	return err
}

func (repo *DataModelRepositoryPostgresql) GetTablesAndFields(ctx context.Context, exec Executor,
	organizationID string,
) ([]models.DataModelTableField, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query, args, err := NewQueryBuilder().
		Select(dbmodels.SelectDataModelFieldColumns...).
		From(dbmodels.TableDataModelTable).
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

	fields, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.DataModelTableField, error) {
		var dbModel dbmodels.DbDataModelField
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
			return models.DataModelTableField{}, err
		}
		return dbmodels.AdaptDataModelTableField(dbModel), err
	})
	return fields, err
}

func (repo *DataModelRepositoryPostgresql) GetLinks(ctx context.Context, exec Executor, organizationID string) ([]models.DataModelLink, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := `
		SELECT data_model_links.id, data_model_links.name, parent_table.name, parent_field.name, child_table.name, child_field.name FROM data_model_links
    	JOIN data_model_tables AS parent_table ON (data_model_links.parent_table_id = parent_table.id)
    	JOIN data_model_fields AS parent_field ON (data_model_links.parent_field_id = parent_field.id)
    	JOIN data_model_tables AS child_table ON (data_model_links.child_table_id = child_table.id)
    	JOIN data_model_fields AS child_field ON (data_model_links.child_field_id = child_field.id)
    	WHERE data_model_links.organization_id = $1`

	rows, err := exec.Query(ctx, query, organizationID)
	if err != nil {
		return nil, err
	}

	links, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.DataModelLink, error) {
		var dbLinks dbmodels.DataModelLink
		if err := rows.Scan(&dbLinks.ID,
			&dbLinks.Name,
			&dbLinks.ParentTable,
			&dbLinks.ParentField,
			&dbLinks.ChildTable,
			&dbLinks.ChildField); err != nil {
			return models.DataModelLink{}, err
		}
		return dbmodels.AdaptDataModelLink(dbLinks), err
	})
	return links, nil
}

func (repo *DataModelRepositoryPostgresql) DeleteDataModel(ctx context.Context, exec Executor, organizationID string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().
			Delete(dbmodels.TableDataModelTable).
			Where(squirrel.Eq{"organization_id": organizationID}),
	)
	if err != nil {
		return err
	}

	err = ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().
			Delete(dbmodels.TABLE_DATA_MODELS).
			Where(squirrel.Eq{"org_id": organizationID}),
	)
	if err != nil {
		return err
	}
	return err
}

func (repo *DataModelRepositoryPostgresql) GetEnumValues(ctx context.Context, exec Executor, fieldID string) ([]any, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query, args, err := NewQueryBuilder().
		Select("text_value", "float_value").
		From("data_model_enum_values").
		Where(squirrel.Eq{"field_id": fieldID}).
		Where("(text_value IS NOT NULL OR float_value IS NOT NULL)").
		OrderBy("last_seen DESC").
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

func (repo *DataModelRepositoryPostgresql) GetDataModelField(ctx context.Context, exec Executor, fieldId string) (models.Field, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.Field{}, err
	}

	query := `
		SELECT
			data_model_fields.description,
			data_model_fields.is_enum,
			data_model_fields.nullable,
			data_model_fields.table_id,
			data_model_fields.type
		FROM data_model_fields
		WHERE id = $1
	`

	row := exec.QueryRow(ctx, query, fieldId)

	var field models.Field
	var dataType string
	if err := row.Scan(
		&field.Description,
		&field.IsEnum,
		&field.Nullable,
		&field.TableId,
		&dataType,
	); errors.Is(err, pgx.ErrNoRows) {
		return models.Field{}, fmt.Errorf("error in GetDataModelField: %w", models.NotFoundError)
	} else if err != nil {
		return models.Field{}, err
	}
	field.ID = fieldId
	field.DataType = models.DataTypeFrom(dataType)

	return field, nil
}
