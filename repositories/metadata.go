package repositories

import (
	"context"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
)

func selectMetadata() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.MetadataFields...).
		From(dbmodels.TABLE_METADATA)
}

func (repo *MarbleDbRepository) GetMetadata(ctx context.Context, exec Executor, orgID *uuid.UUID, key models.MetadataKey) (*models.Metadata, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := selectMetadata().Where(squirrel.Eq{"key": string(key)})

	if orgID != nil {
		query = query.Where(squirrel.Eq{"org_id": *orgID})
	} else {
		query = query.Where(squirrel.Eq{"org_id": nil})
	}

	return SqlToOptionalModel(
		ctx,
		exec,
		query,
		dbmodels.AdaptMetadata,
	)
}

func (repo *MarbleDbRepository) CreateMetadata(ctx context.Context, exec Executor, metadata models.Metadata) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_METADATA).
		Columns("id", "org_id", "key", "value").
		Values(metadata.ID, metadata.OrgID, string(metadata.Key), metadata.Value)

	return ExecBuilder(ctx, exec, query)
}

func (repo *MarbleDbRepository) ListFeatureFlags(ctx context.Context, exec Executor) (map[string]string, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	prefix := string(models.MetadataKeyFeatureFlag) + ":"

	query := selectMetadata().
		Where(squirrel.Eq{"org_id": nil}).
		Where(squirrel.Expr("starts_with(key, ?)", prefix))

	rows, err := SqlToListOfModels(ctx, exec, query, func(db dbmodels.DBMetadata) (dbmodels.DBMetadata, error) {
		return db, nil
	})
	if err != nil {
		return nil, err
	}

	flags := make(map[string]string, len(rows))

	for _, row := range rows {
		flags[strings.TrimPrefix(row.Key, prefix)] = row.Value
	}

	return flags, nil
}

func (repo *MarbleDbRepository) ReplaceFeatureFlags(ctx context.Context, exec Transaction, flags map[string]string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	if err := GetAdvisoryLockTx(ctx, exec, "feature_flags_replace"); err != nil {
		return err
	}

	prefix := string(models.MetadataKeyFeatureFlag) + ":"

	deleteQuery := NewQueryBuilder().
		Delete(dbmodels.TABLE_METADATA).
		Where(squirrel.Eq{"org_id": nil}).
		Where(squirrel.Expr("starts_with(key, ?)", prefix))

	if err := ExecBuilder(ctx, exec, deleteQuery); err != nil {
		return err
	}

	if len(flags) == 0 {
		return nil
	}

	insertQuery := NewQueryBuilder().
		Insert(dbmodels.TABLE_METADATA).
		Columns("id", "org_id", "key", "value")

	for flag, value := range flags {
		insertQuery = insertQuery.Values(pure_utils.NewId(), nil, prefix+flag, value)
	}

	return ExecBuilder(ctx, exec, insertQuery)
}

func (repo *MarbleDbRepository) UpsertMetadata(ctx context.Context, exec Executor, metadata models.Metadata) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	id := metadata.ID

	if id == uuid.Nil {
		id = pure_utils.NewId()
	}

	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_METADATA).
		Columns("id", "org_id", "key", "value").
		Values(id, metadata.OrgID, string(metadata.Key), metadata.Value).
		SuffixExpr(squirrel.Expr("on conflict (org_id, key) do update set value = ?", metadata.Value))

	return ExecBuilder(ctx, exec, query)
}
