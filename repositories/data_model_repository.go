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
}

type DataModelRepositoryPostgresql struct {
	transactionFactory TransactionFactory
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
