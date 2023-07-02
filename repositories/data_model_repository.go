package repositories

import (
	"marble/marble-backend/models"
	"marble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
)

type DataModelRepository interface {
	GetDataModel(tx Transaction, organizationID string) (models.DataModel, error)
}

type DataModelRepositoryPostgresql struct {
	transactionFactory TransactionFactory
}

func (repo *DataModelRepositoryPostgresql) GetDataModel(tx Transaction, organizationID string) (models.DataModel, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToModel(
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.SelectDataModelColumn...).
			From(dbmodels.TABLE_DATA_MODELS).
			Where(squirrel.Eq{"org_id": organizationID}),
		dbmodels.AdaptDataModel,
	)
}
