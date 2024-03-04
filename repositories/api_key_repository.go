package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (repo *MarbleDbRepository) GetApiKeyById(ctx context.Context, exec Executor, apiKeyId string) (models.ApiKey, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ApiKey{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.ApiKeyFields...).
			From(dbmodels.TABLE_APIKEYS).
			Where("id = ?", apiKeyId).
			Where("deleted_at IS NULL"),
		dbmodels.AdaptApikey,
	)
}

func (repo *MarbleDbRepository) GetApiKeyByKey(ctx context.Context, exec Executor, key string) (models.ApiKey, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ApiKey{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.ApiKeyFields...).
			From(dbmodels.TABLE_APIKEYS).
			Where("key = ?", key).
			Where("deleted_at IS NULL"),
		dbmodels.AdaptApikey,
	)
}

func (repo *MarbleDbRepository) ListApiKeys(ctx context.Context, exec Executor, organizationId string) ([]models.ApiKey, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	return SqlToListOfModels(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.ApiKeyFields...).
			From(dbmodels.TABLE_APIKEYS).
			Where("org_id = ?", organizationId).
			Where("deleted_at IS NULL").
			OrderBy("created_at DESC"),
		dbmodels.AdaptApikey,
	)
}

func (repo *MarbleDbRepository) CreateApiKey(ctx context.Context, exec Executor, apiKey models.ApiKey) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().
			Insert(dbmodels.TABLE_APIKEYS).
			Columns(
				"id",
				"org_id",
				"key",
				"key_hash",
				"description",
				"role",
			).
			Values(
				apiKey.Id,
				apiKey.OrganizationId,
				apiKey.Key,
				apiKey.Hash,
				apiKey.Description,
				apiKey.Role,
			),
	)
	return err
}

func (repo *MarbleDbRepository) SoftDeleteApiKey(ctx context.Context, exec Executor, apiKeyId string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().
			Update(dbmodels.TABLE_APIKEYS).
			Where(squirrel.Eq{"id": apiKeyId}).
			Set("deleted_at", squirrel.Expr("NOW()")),
	)
	return err
}
