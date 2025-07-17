package repositories

import (
	"context"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
)

func selectMetadata() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.MetadataFields...).
		From(dbmodels.TABLE_METADATA)
}

func (repo *MarbleDbRepository) GetMetadata(ctx context.Context, exec Executor, orgID *uuid.UUID, key models.MetadataKey) (models.Metadata, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.Metadata{}, err
	}

	query := selectMetadata().Where(squirrel.Eq{"key": string(key)})

	if orgID != nil {
		query = query.Where(squirrel.Eq{"org_id": *orgID})
	} else {
		query = query.Where(squirrel.Eq{"org_id": nil})
	}

	return SqlToModel(
		ctx,
		exec,
		query,
		dbmodels.AdaptMetadata,
	)
}
