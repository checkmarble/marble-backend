package repositories

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

type ApiKeyRepository interface {
	GetApiKeysOfOrganization(tx Transaction, organizationId string) ([]models.ApiKey, error)
	GetApiKeyByKey(tx Transaction, apiKey string) (models.ApiKey, error)
	CreateApiKey(tx Transaction, apiKey models.CreateApiKeyInput) error
}

type ApiKeyRepositoryImpl struct {
	transactionFactory TransactionFactory
}

func (repo *ApiKeyRepositoryImpl) GetApiKeyByKey(tx Transaction, key string) (models.ApiKey, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToModel(
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.ApiKeyFields...).
			From(dbmodels.TABLE_APIKEYS).
			Where("key = ?", key),
		dbmodels.AdaptApikey,
	)
}

func (repo *ApiKeyRepositoryImpl) GetApiKeysOfOrganization(tx Transaction, organizationId string) ([]models.ApiKey, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToListOfModels(
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.ApiKeyFields...).
			From(dbmodels.TABLE_APIKEYS).
			Where("org_id = ?", organizationId),
		dbmodels.AdaptApikey,
	)

}

func (repo *ApiKeyRepositoryImpl) CreateApiKey(tx Transaction, apiKey models.CreateApiKeyInput) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(
		NewQueryBuilder().
			Insert(dbmodels.TABLE_APIKEYS).
			Columns(
				"org_id",
				"key",
			).
			Values(
				apiKey.OrganizationId,
				apiKey.Key,
			),
	)
	return err
}
