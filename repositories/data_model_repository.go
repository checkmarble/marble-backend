package repositories

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
)

type DataModelRepository interface {
	GetDataModel(tx Transaction, organizationId string) (models.DataModel, error)
	GetTables(tx Transaction, organizationID string) ([]models.DataModelTableField, error)
	GetLinks(tx Transaction, organizationID string) ([]models.DataModelLink, error)
	DeleteDataModel(tx Transaction, organizationId string) error
	CreateDataModel(tx Transaction, organizationId string, dataModel models.DataModel) error
	CreateDataModelTable(tx Transaction, organizationID, name, description string) error
	UpdateDataModelTable(tx Transaction, tableID, description string) error
	GetDataModelTable(tx Transaction, tableID string) (models.DataModelTable, error)
	CreateDataModelField(tx Transaction, tableID string, field models.DataModelField) error
	UpdateDataModelField(tx Transaction, field, description string) error
	CreateDataModelLink(tx Transaction, link models.DataModelLink) error
}

type DataModelRepositoryPostgresql struct {
	transactionFactory TransactionFactoryPosgresql
}

func (repo *DataModelRepositoryPostgresql) GetDataModel(tx Transaction, organizationId string) (models.DataModel, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	if organizationId == "" {
		return models.DataModel{}, errors.New("organizationId is empty")
	}

	return SqlToModel(
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.SelectDataModelColumn...).
			From(dbmodels.TABLE_DATA_MODELS).
			Where(squirrel.Eq{"org_id": organizationId}),
		dbmodels.AdaptDataModel,
	)
}

func (repo *DataModelRepositoryPostgresql) DeleteDataModel(tx Transaction, organizationId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		NewQueryBuilder().Delete(dbmodels.TABLE_DATA_MODELS).Where("org_id = ?", organizationId),
	)
	return err
}

func (repo *DataModelRepositoryPostgresql) CreateDataModel(tx Transaction, organizationId string, dataModel models.DataModel) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	tables, err := json.Marshal(dataModel.Tables)
	if err != nil {
		return fmt.Errorf("unable to marshal tables: %w", err)
	}

	_, err = pgTx.ExecBuilder(
		NewQueryBuilder().Insert(dbmodels.TABLE_DATA_MODELS).
			Columns(
				"org_id",
				"version",
				"status",
				"tables",
			).
			Values(
				organizationId,
				dataModel.Version,
				dataModel.Status.String(),
				tables,
			),
	)
	return err
}

func (repo *DataModelRepositoryPostgresql) CreateDataModelTable(tx Transaction, organizationID, name, description string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		NewQueryBuilder().Insert("data_model_tables").
			Columns("organization_id", "name", "description").
			Values(organizationID, name, description),
	)
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

func (repo *DataModelRepositoryPostgresql) CreateDataModelField(tx Transaction, tableID string, field models.DataModelField) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		NewQueryBuilder().Insert("data_model_fields").
			Columns("table_id", "name", "type", "nullable", "description").
			Values(tableID, field.Name, field.Type, field.Nullable, field.Description),
	)
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
	return err
}

func (repo *DataModelRepositoryPostgresql) GetTables(tx Transaction, organizationID string) ([]models.DataModelTableField, error) {
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
			&dbModel.FieldDescription); err != nil {
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
