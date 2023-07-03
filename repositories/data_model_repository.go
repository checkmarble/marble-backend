package repositories

import (
	"encoding/json"
	"errors"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
)

type DataModelRepository interface {
	GetDataModel(tx Transaction, organizationID string) (models.DataModel, error)
	DeleteDataModel(tx Transaction, organizationID string) error
	CreateDataModel(tx Transaction, organizationID string, dataModel models.DataModel) error
}

type DataModelRepositoryPostgresql struct {
	transactionFactory TransactionFactory
}

func (repo *DataModelRepositoryPostgresql) GetDataModel(tx Transaction, organizationID string) (models.DataModel, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	if organizationID == "" {
		return models.DataModel{}, errors.New("organizationID is empty")
	}

	return SqlToModel(
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.SelectDataModelColumn...).
			From(dbmodels.TABLE_DATA_MODELS).
			Where(squirrel.Eq{"org_id": organizationID}),
		dbmodels.AdaptDataModel,
	)
}

func (repo *DataModelRepositoryPostgresql) DeleteDataModel(tx Transaction, organizationID string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		NewQueryBuilder().Delete(dbmodels.TABLE_DATA_MODELS).Where("org_id = ?", organizationID),
	)
	return err
}

func (repo *DataModelRepositoryPostgresql) CreateDataModel(tx Transaction, organizationID string, dataModel models.DataModel) error {
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
				organizationID,
				dataModel.Version,
				dataModel.Status.String(),
				tables,
			),
	)
	return err
}
