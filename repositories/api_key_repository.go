package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

type ApiKeyRepository interface {
	GetApiKeysOfOrganization(ctx context.Context, tx Transaction, organizationId string) ([]models.ApiKey, error)
	GetApiKeyByKey(ctx context.Context, tx Transaction, apiKey string) (models.ApiKey, error)
	CreateApiKey(ctx context.Context, tx Transaction, apiKey models.CreateApiKeyInput) error
}

type ApiKeyRepositoryImpl struct {
	transactionFactory TransactionFactoryPosgresql
}

func (repo *ApiKeyRepositoryImpl) GetApiKeyByKey(ctx context.Context, tx Transaction, key string) (models.ApiKey, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	return SqlToModel(
		ctx,
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.ApiKeyFields...).
			From(dbmodels.TABLE_APIKEYS).
			Where("key = ?", key).
			Where("deleted_at IS NULL"),
		dbmodels.AdaptApikey,
	)
}

func (repo *ApiKeyRepositoryImpl) GetApiKeysOfOrganization(ctx context.Context, tx Transaction, organizationId string) ([]models.ApiKey, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	return SqlToListOfModels(
		ctx,
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.ApiKeyFields...).
			From(dbmodels.TABLE_APIKEYS).
			Where("org_id = ?", organizationId).
			Where("deleted_at IS NULL"),
		dbmodels.AdaptApikey,
	)

}

func (repo *ApiKeyRepositoryImpl) CreateApiKey(ctx context.Context, tx Transaction, apiKey models.CreateApiKeyInput) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	_, err := pgTx.ExecBuilder(
		ctx,
		NewQueryBuilder().
			Insert(dbmodels.TABLE_APIKEYS).
			Columns(
				"org_id",
				"key",
				"description",
			).
			Values(
				apiKey.OrganizationId,
				apiKey.Key,
				apiKey.Description,
			),
	)
	return err
}
