package repositories

import (
	"fmt"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

type DataModelRepository interface {
	GetDataModel(organizationID string, fetchEnumValues bool) (models.DataModel, error)
	GetTablesAndFields(tx Transaction, organizationID string) ([]models.DataModelTableField, error)
	CreateDataModelTable(tx Transaction, organizationID, tableID, name, description string) error
	UpdateDataModelTable(tx Transaction, tableID, description string) error
	GetDataModelTable(tx Transaction, tableID string) (models.DataModelTable, error)
	CreateDataModelField(tx Transaction, tableID, fieldID string, field models.DataModelField) error
	UpdateDataModelField(tx Transaction, field, description string) error
	CreateDataModelLink(tx Transaction, link models.DataModelLink) error
	DeleteDataModel(tx Transaction, organizationID string) error
}

type DataModelRepositoryPostgresql struct {
	transactionFactory TransactionFactoryPosgresql
}

func (repo *DataModelRepositoryPostgresql) GetDataModel(organizationID string, fetchEnumValues bool) (models.DataModel, error) {
	fields, err := repo.GetTablesAndFields(nil, organizationID)
	if err != nil {
		return models.DataModel{}, err
	}

	links, err := repo.GetLinks(nil, organizationID)
	if err != nil {
		return models.DataModel{}, err
	}

	dataModel := models.DataModel{
		Tables: make(map[models.TableName]models.Table),
	}

	for _, field := range fields {
		tableName := models.TableName(field.TableName)
		fieldName := models.FieldName(field.FieldName)

		var values []string
		if field.FieldIsEnum && fetchEnumValues {
			values, err = repo.GetEnumValues(nil, field.FieldID)
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

func (repo *DataModelRepositoryPostgresql) CreateDataModelTable(tx Transaction, organizationID, tableID, name, description string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	query := `
		INSERT INTO data_model_tables (id, organization_id, name, description)
		VALUES ($1, $2, $3, $4)`

	_, err := pgTx.exec.Exec(pgTx.ctx, query, tableID, organizationID, name, description)
	if IsUniqueViolationError(err) {
		return models.DuplicateValueError
	}
	return err
}

func (repo *DataModelRepositoryPostgresql) GetDataModelTable(tx Transaction, tableID string) (models.DataModelTable, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToModel(
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.SelectDataModelTableColumns...).
			From(dbmodels.TableDataModelTable).
			Where(squirrel.Eq{"id": tableID}),
		dbmodels.AdaptDataModelTable,
	)
}

func (repo *DataModelRepositoryPostgresql) UpdateDataModelTable(tx Transaction, tableID, description string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		NewQueryBuilder().
			Update(dbmodels.TableDataModelTable).
			Set("description", description).
			Where(squirrel.Eq{"id": tableID}),
	)
	return err
}

func (repo *DataModelRepositoryPostgresql) CreateDataModelField(tx Transaction, tableID, fieldID string, field models.DataModelField) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	query := `
		INSERT INTO data_model_fields (id, table_id, name, type, nullable, description, is_enum)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`

	_, err := pgTx.exec.Exec(pgTx.ctx, query, fieldID, tableID, field.Name, field.Type, field.Nullable, field.Description, field.IsEnum)
	if IsUniqueViolationError(err) {
		return models.DuplicateValueError
	}
	return err
}

func (repo *DataModelRepositoryPostgresql) UpdateDataModelField(tx Transaction, fieldID, description string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		NewQueryBuilder().
			Update(dbmodels.TableDataModelFields).
			Set("description", description).
			Where(squirrel.Eq{"id": fieldID}),
	)
	return err
}

func (repo *DataModelRepositoryPostgresql) CreateDataModelLink(tx Transaction, link models.DataModelLink) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		NewQueryBuilder().
			Insert("data_model_links").
			Columns("organization_id", "name", "parent_table_id", "parent_field_id", "child_table_id", "child_field_id").
			Values(link.OrganizationID, link.Name, link.ParentTableID, link.ParentFieldID, link.ChildTableID, link.ChildFieldID),
	)
	if IsUniqueViolationError(err) {
		return models.DuplicateValueError
	}
	return err
}

func (repo *DataModelRepositoryPostgresql) GetTablesAndFields(tx Transaction, organizationID string) ([]models.DataModelTableField, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	query, args, err := NewQueryBuilder().
		Select(dbmodels.SelectDataModelFieldColumns...).
		From(dbmodels.TableDataModelTable).
		Join(fmt.Sprintf("%s ON (data_model_tables.id = data_model_fields.table_id)", dbmodels.TableDataModelFields)).
		Where(squirrel.Eq{"organization_id": organizationID}).
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := pgTx.exec.Query(pgTx.ctx, query, args...)
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

func (repo *DataModelRepositoryPostgresql) GetLinks(tx Transaction, organizationID string) ([]models.DataModelLink, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	query := `
		SELECT data_model_links.id, data_model_links.name, parent_table.name, parent_field.name, child_table.name, child_field.name FROM data_model_links
    	JOIN data_model_tables AS parent_table ON (data_model_links.parent_table_id = parent_table.id)
    	JOIN data_model_fields AS parent_field ON (data_model_links.parent_field_id = parent_field.id)
    	JOIN data_model_tables AS child_table ON (data_model_links.child_table_id = child_table.id)
    	JOIN data_model_fields AS child_field ON (data_model_links.child_field_id = child_field.id)
    	WHERE data_model_links.organization_id = $1`

	rows, err := pgTx.exec.Query(pgTx.ctx, query, organizationID)
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

func (repo *DataModelRepositoryPostgresql) DeleteDataModel(tx Transaction, organizationID string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		NewQueryBuilder().
			Delete(dbmodels.TableDataModelTable).
			Where(squirrel.Eq{"organization_id": organizationID}),
	)
	if err != nil {
		return err
	}

	_, err = pgTx.ExecBuilder(
		NewQueryBuilder().
			Delete(dbmodels.TABLE_DATA_MODELS).
			Where(squirrel.Eq{"org_id": organizationID}),
	)
	if err != nil {
		return err
	}
	return err
}

func (repo *DataModelRepositoryPostgresql) GetEnumValues(tx Transaction, fieldID string) ([]string, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	query, args, err := NewQueryBuilder().
		Select("value").
		From("data_model_enum_values").
		Where(squirrel.Eq{"field_id": fieldID}).
		OrderBy("last_seen DESC").
		Limit(100).
		ToSql()
	if err != nil {
		return nil, err
	}

	rows, err := pgTx.exec.Query(pgTx.ctx, query, args...)
	if err != nil {
		return nil, err
	}

	values, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (string, error) {
		var value string
		if err := rows.Scan(&value); err != nil {
			return "", err
		}
		return value, err
	})
	return values, nil
}
