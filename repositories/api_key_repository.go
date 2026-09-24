package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
)

func (repo *MarbleDbRepository) GetApiKeyById(ctx context.Context, exec Executor, apiKeyId string) (models.ApiKey, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ApiKey{}, err
	}

	apiKey, err := SqlToModel(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.ApiKeyFields...).
			From(dbmodels.TABLE_APIKEYS).
			Where("id = ?", apiKeyId).
			Where("deleted_at IS NULL"),
		dbmodels.AdaptApikey,
	)
	if err != nil {
		return apiKey, err
	}
	bindings, err := repo.ListApiKeyRoleBindings(ctx, exec, apiKey.Id)
	if err != nil {
		return apiKey, err
	}
	apiKey.RoleBindings = bindings
	return apiKey, nil
}

func (repo *MarbleDbRepository) GetApiKeyByHash(ctx context.Context, exec Executor, hash []byte) (models.ApiKey, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ApiKey{}, err
	}

	apiKey, err := SqlToModel(
		ctx,
		exec,
		NewQueryBuilder().
			Select(dbmodels.ApiKeyFields...).
			From(dbmodels.TABLE_APIKEYS).
			Where("key_hash = ?", hash).
			Where("deleted_at IS NULL"),
		dbmodels.AdaptApikey,
	)
	if err != nil {
		return apiKey, err
	}
	bindings, err := repo.ListApiKeyRoleBindings(ctx, exec, apiKey.Id)
	if err != nil {
		return apiKey, err
	}
	apiKey.RoleBindings = bindings
	return apiKey, nil
}

func (repo *MarbleDbRepository) ListApiKeys(ctx context.Context, exec Executor, organizationId uuid.UUID) ([]models.ApiKey, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	apiKeys, err := SqlToListOfModels(
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
	if err != nil {
		return nil, err
	}
	for idx := range apiKeys {
		bindings, err := repo.ListApiKeyRoleBindings(ctx, exec, apiKeys[idx].Id)
		if err != nil {
			return nil, err
		}
		apiKeys[idx].RoleBindings = bindings
	}
	return apiKeys, nil
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
				"prefix",
				"key_hash",
				"description",
				"role",
			).
			Values(
				apiKey.Id,
				apiKey.OrganizationId,
				apiKey.Prefix,
				apiKey.Hash,
				apiKey.Description,
				models.LegacyRoleValue(apiKey.RoleBindings),
			),
	)
	if err != nil {
		return err
	}

	return repo.ReplaceApiKeyRoleBindings(ctx, exec, apiKey.OrganizationId, apiKey.Id, apiKey.RoleBindings)
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
