package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (*MarbleDbRepository) InsertFreeformSearch(
	ctx context.Context,
	exec Executor,
	s models.FreeformSearch,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	searchInputBytes, err := json.Marshal(dbmodels.AdaptDBScreeningRefineRequest(s.SearchInput))
	if err != nil {
		return err
	}
	configBytes, err := json.Marshal(s.SearchConfig)
	if err != nil {
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
			"search_config",
		).
		Values(
			s.Id,
			s.OrgId,
			s.UserId,
			s.ApiKeyId,
			s.Provider,
			searchInputBytes,
			configBytes,
		)

	return ExecBuilder(ctx, exec, sql)
}

func (*MarbleDbRepository) ListFreeformSearches(
	ctx context.Context,
	exec Executor,
	filters models.ScreeningFreeformSearchFilters,
	pagination models.PaginationAndSorting,
) ([]models.FreeformSearch, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	orderCondition := fmt.Sprintf("%s %s", pagination.Sorting, pagination.Order)
	query := NewQueryBuilder().
		Select(dbmodels.SelectFreeformSearchColumn...).
		From(dbmodels.TABLE_FREEFORM_SEARCHES).
		Where(squirrel.Eq{"org_id": filters.OrgId}).
		OrderBy(orderCondition).
		Limit(uint64(pagination.Limit))

	return SqlToListOfModels(
		ctx,
		exec,
		query,
		dbmodels.AdaptFreeformSearch,
	)
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
