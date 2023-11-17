package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/checkmarble/marble-backend/models"
)

func (tx *Transaction) getTableName(ctx context.Context, tableID string) (string, error) {
	query := `
		SELECT name
		FROM data_model_tables
		WHERE id = $1
	`

	var name string
	err := tx.QueryRow(ctx, query, tableID).Scan(&name)
	if err != nil {
		return "", fmt.Errorf("tx.QueryRow error: %w", err)
	}
	return name, nil
}

func (tx *Transaction) addDataModelTable(ctx context.Context, organizationID, name, description string) (string, error) {
	query := `
		INSERT INTO data_model_tables (organization_id, name, description)
		VALUES ($1, $2, $3)
		RETURNING id
	`

	var ID string
	err := tx.QueryRow(ctx, query, organizationID, name, description).Scan(&ID)
	if err != nil {
		return "", fmt.Errorf("tx.QueryRow error: %w", err)
	}
	return ID, nil
}

func (tx *Transaction) addDataModelField(ctx context.Context, tableID string, field models.DataModelField) (string, error) {
	query := `
		INSERT INTO data_model_fields (table_id, name, type, nullable, description, is_enum)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`

	var fieldID string
	err := tx.QueryRow(ctx, query, tableID, field.Name, field.Type, field.Nullable, field.Description, field.IsEnum).Scan(&fieldID)
	if err != nil {
		return "", fmt.Errorf("tx.QueryRow error: %w", err)
	}
	return fieldID, nil
}

func (db *Database) CreateDataModelLink(ctx context.Context, link models.DataModelLink) error {
	query := `
		INSERT INTO data_model_links (organization_id, name, parent_table_id, parent_field_id, child_table_id, child_field_id)
		VALUES ($1, $2, $3, $4, $5, $6)
	`

	_, err := db.pool.Exec(ctx, query, link.OrganizationID, link.Name, link.ParentTableID, link.ParentFieldID, link.ChildTableID, link.ChildFieldID)
	if err != nil {
		return fmt.Errorf("tx.Exec error: %w", err)
	}
	return err
}

func (db *Database) DeleteDataModel(ctx context.Context, organizationID string) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}

	dataModelQuery := `
		DELETE FROM data_model_tables
		WHERE organization_id = $1
	`

	_, err = tx.Exec(ctx, dataModelQuery, organizationID)
	if err != nil {
		return err
	}

	schema, err := tx.OrganizationSchemaOfOrganization(ctx, organizationID)
	if err != nil {
		return err
	}

	sanitizedSchema := pgx.Identifier.Sanitize([]string{schema})
	schemaQuery := fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", sanitizedSchema)
	if _, err := tx.Exec(ctx, schemaQuery); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (db *Database) CreateDataModelTable(ctx context.Context, organizationID, name, description string, defaultFields []models.DataModelField) (string, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	schema, err := tx.OrganizationSchemaOfOrganization(ctx, organizationID)
	if err != nil {
		return "", err
	}

	tableID, err := tx.addDataModelTable(ctx, organizationID, name, description)
	if err != nil {
		return "", err
	}

	for _, field := range defaultFields {
		if _, err := tx.addDataModelField(ctx, tableID, field); err != nil {
			return "", err
		}
	}

	if err := tx.createSchema(ctx, schema); err != nil {
		return "", err
	}

	if err := tx.addTableToSchema(ctx, schema, name); err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("tx.Commit error: %w", err)
	}
	return tableID, nil
}

func (db *Database) UpdateDataModelTable(ctx context.Context, tableID, description string) error {
	query := `
		UPDATE data_model_tables
		SET description = $2
		WHERE id = $1
	`

	_, err := db.pool.Exec(ctx, query, tableID, description)
	if err != nil {
		return fmt.Errorf("pool.Exec error: %w", err)
	}
	return nil
}

func (db *Database) CreateDataModelField(ctx context.Context, organizationID, tableID string, field models.DataModelField) (string, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	schema, err := tx.OrganizationSchemaOfOrganization(ctx, organizationID)
	if err != nil {
		return "", err
	}
	fieldID, err := tx.addDataModelField(ctx, tableID, field)
	if err != nil {
		return "", err
	}
	tableName, err := tx.getTableName(ctx, tableID)
	if err != nil {
		return "", err
	}
	if err := tx.addDataModelFieldToSchema(ctx, schema, tableName, field); err != nil {
		return "", err
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return fieldID, nil
}

func (db *Database) UpdateDataModelField(ctx context.Context, fieldID string, input models.UpdateDataModelFieldInput) error {
	query := NewQueryBuilder().
		Update("data_model_fields").
		Where(squirrel.Eq{"id": fieldID})
	if input.Description != nil {
		query = query.Set("description", *input.Description)
	}
	if input.IsEnum != nil {
		query = query.Set("is_enum", *input.IsEnum)
	}
	sql, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("query.ToSql error: %w", err)
	}

	_, err = db.pool.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("pool.Exec error: %w", err)
	}
	return nil
}

func (db *Database) GetTablesAndFields(ctx context.Context, organizationID string) ([]models.DataModelTableField, error) {
	query := `
		SELECT
			data_model_tables.id,
			data_model_tables.organization_id,
			data_model_tables.name,
			data_model_tables.description,
			data_model_fields.id,
			data_model_fields.name,
			data_model_fields.type,
			data_model_fields.nullable,
			data_model_fields.description,
			data_model_fields.is_enum
		FROM data_model_tables
		JOIN data_model_fields ON (data_model_tables.id = data_model_fields.table_id)
		WHERE organization_id = $1
	`

	rows, err := db.pool.Query(ctx, query, organizationID)
	if err != nil {
		return nil, err
	}

	fields, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.DataModelTableField, error) {
		var model models.DataModelTableField
		if err := rows.Scan(&model.TableID,
			&model.OrganizationID,
			&model.TableName,
			&model.TableDescription,
			&model.FieldID,
			&model.FieldName,
			&model.FieldType,
			&model.FieldNullable,
			&model.FieldDescription,
			&model.FieldIsEnum,
		); err != nil {
			return models.DataModelTableField{}, err
		}
		return model, nil
	})
	return fields, err
}

func (db *Database) GetLinks(ctx context.Context, organizationID string) ([]models.DataModelLink, error) {
	query := `
		SELECT data_model_links.id, data_model_links.name, parent_table.name, parent_field.name, child_table.name, child_field.name FROM data_model_links
    	JOIN data_model_tables AS parent_table ON (data_model_links.parent_table_id = parent_table.id)
    	JOIN data_model_fields AS parent_field ON (data_model_links.parent_field_id = parent_field.id)
    	JOIN data_model_tables AS child_table ON (data_model_links.child_table_id = child_table.id)
    	JOIN data_model_fields AS child_field ON (data_model_links.child_field_id = child_field.id)
    	WHERE data_model_links.organization_id = $1
	`

	rows, err := db.pool.Query(ctx, query, organizationID)
	if err != nil {
		return nil, fmt.Errorf("pgx.Query error: %w", err)
	}

	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (models.DataModelLink, error) {
		var link models.DataModelLink
		if err := rows.Scan(&link.ID,
			&link.Name,
			&link.ParentTable,
			&link.ParentField,
			&link.ChildTable,
			&link.ChildField); err != nil {
			return models.DataModelLink{}, err
		}
		return link, nil
	})
}

func (db *Database) GetEnumValues(ctx context.Context, fieldID string) ([]any, error) {
	query := `
		SELECT text_value, float_value
		FROM data_model_enum_values
		WHERE field_id = $1
		AND (text_value IS NOT NULL OR float_value IS NOT NULL)
		ORDER BY last_seen DESC
		LIMIT 100
	`

	rows, err := db.pool.Query(ctx, query, fieldID)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (any, error) {
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
}

func (db *Database) GetDataModelField(ctx context.Context, fieldId string) (models.Field, error) {
	query := `
		SELECT
			data_model_fields.id,
			data_model_fields.type,
			data_model_fields.nullable,
			data_model_fields.description,
			data_model_fields.is_enum
		FROM data_model_fields
		WHERE id = $1
	`

	row := db.pool.QueryRow(ctx, query, fieldId)

	var field models.Field
	var dataType string
	if err := row.Scan(
		&field.ID,
		&dataType,
		&field.Nullable,
		&field.Description,
		&field.IsEnum,
	); errors.Is(err, pgx.ErrNoRows) {
		return models.Field{}, fmt.Errorf("error in GetDataModelField: %w", models.NotFoundError)
	} else if err != nil {
		return models.Field{}, err
	}
	field.DataType = models.DataTypeFrom(dataType)

	return field, nil
}

func (db *Database) GetDataModel(ctx context.Context, organizationID string, fetchEnumValues bool) (models.DataModel, error) {
	fields, err := db.GetTablesAndFields(ctx, organizationID)
	if err != nil {
		return models.DataModel{}, err
	}

	links, err := db.GetLinks(ctx, organizationID)
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
			values, err = db.GetEnumValues(ctx, field.FieldID)
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
