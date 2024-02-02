package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (repo *MarbleDbRepository) GetApiKeyById(ctx context.Context, tx Transaction, apiKeyId string) (models.ApiKey, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	return SqlToModel(
		ctx,
		pgTx,
		NewQueryBuilder().
			Select(dbmodels.ApiKeyFields...).
			From(dbmodels.TABLE_APIKEYS).
			Where("id = ?", apiKeyId).
			Where("deleted_at IS NULL"),
		dbmodels.AdaptApikey,
	)
}

func (repo *MarbleDbRepository) GetApiKeyByKey(ctx context.Context, tx Transaction, key string) (models.ApiKey, error) {
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

func (repo *MarbleDbRepository) ListApiKeys(ctx context.Context, tx Transaction, organizationId string) ([]models.ApiKey, error) {
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

func (repo *MarbleDbRepository) CreateApiKey(ctx context.Context, tx Transaction, apiKey models.CreateApiKey) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	_, err := pgTx.ExecBuilder(
		ctx,
		NewQueryBuilder().
			Insert(dbmodels.TABLE_APIKEYS).
			Columns(
				"id",
				"org_id",
				"key",
				"description",
				"role",
			).
			Values(
				apiKey.Id,
				apiKey.OrganizationId,
				apiKey.Hash,
				apiKey.Description,
				apiKey.Role,
			),
	)
	return err
}

func (repo *MarbleDbRepository) SoftDeleteApiKey(ctx context.Context, tx Transaction, apiKeyId string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	_, err := pgTx.ExecBuilder(
		ctx,
		NewQueryBuilder().
			Update(dbmodels.TABLE_APIKEYS).
			Where(squirrel.Eq{"id": apiKeyId}).
			Set("deleted_at", squirrel.Expr("NOW()")),
	)
	return err
}
