package repositories

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
)

type DataModelRepository interface {
	GetDataModel(tx Transaction, organizationId string) (models.DataModel, error)
	DeleteDataModel(tx Transaction, organizationId string) error
	CreateDataModel(tx Transaction, organizationId string, dataModel models.DataModel) error
	CreateDataModelTable(tx Transaction, organizationID, name, description string) error
	UpdateDataModelTable(tx Transaction, tableID, description string) error
	GetDataModelTable(tx Transaction, tableID string) (models.DataModelTable, error)
	CreateDataModelField(tx Transaction, tableID string, field models.DataModelField) error
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

func (repo *DataModelRepositoryPostgresql) CreateDataModelLink(tx Transaction, link models.DataModelLink) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		NewQueryBuilder().
			Insert("data_model_links").
			Columns("name", "parent_table_id", "parent_field_id", "child_table_id", "child_field_id").
			Values(link.Name, link.ParentTableID, link.ParentFieldID, link.ChildTableID, link.ChildFieldID),
	)
	return err
}
