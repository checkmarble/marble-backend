package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (*MarbleDbRepository) InsertFreeformSearch(
	ctx context.Context,
	exec Executor,
	h models.FreeformSearch,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_FREEFORM_SEARCHES).
		Columns(
			"id",
			"org_id",
			"user_id",
			"api_key_id",
			"provider",
			"search_input",
		).
		Values(
			h.Id,
			h.OrgId,
			h.UserId,
			h.ApiKeyId,
			h.Provider,
			h.SearchInput,
		)

	return ExecBuilder(ctx, exec, sql)
}

func (repo *MarbleDbRepository) CountFreeformSearchesByProvider(ctx context.Context, exec Executor,
	orgIds []string, providers []models.ScreeningProvider, from, to time.Time,
) (models.ByOrgByProviderCounter, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select("org_id, provider, count(*) as count").
		From(dbmodels.TABLE_FREEFORM_SEARCH).
		Where(squirrel.Eq{"org_id": orgIds}).
		// TODO: TBD - uncomment or replace by the right field to filter by provider
		// Where(squirrel.Eq{"provider": providers}).
		Where(squirrel.GtOrEq{"created_at": from}).
		Where(squirrel.Lt{"created_at": to}).
		// TODO: TBD - uncomment or replace by the right field to group by provider
		// GroupBy("org_id", "provider")
		GroupBy("org_id")

	return countBy2Dimensions(ctx, exec, query, orgIds, providers)
}
