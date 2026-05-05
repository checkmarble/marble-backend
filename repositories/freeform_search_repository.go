package repositories

import (
	"context"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
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
		From(dbmodels.TABLE_FREEFORM_SEARCHES).
		Where(squirrel.Eq{"org_id": orgIds}).
		Where(squirrel.Eq{"provider": providers}).
		Where(squirrel.GtOrEq{"created_at": from}).
		Where(squirrel.Lt{"created_at": to}).
		GroupBy("org_id", "provider")

	stringProviders := pure_utils.Map(providers, func(p models.ScreeningProvider) string { return string(p) })

	return countBy2Keys(ctx, exec, query, orgIds, stringProviders)
}
