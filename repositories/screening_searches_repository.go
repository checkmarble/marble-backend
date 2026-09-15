package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
)

func (repo *MarbleDbRepository) ListSavedScreeningSearches(ctx context.Context, exec Executor, orgId uuid.UUID, providerName models.ScreeningProvider) ([]models.ScreeningSavedSearch, error) {
	query := NewQueryBuilder().
		Select(dbmodels.ScreeningSavedSearchesColumns...).
		From(dbmodels.TABLE_SCREENING_SAVED_SEARCHES).
		Where(squirrel.Eq{"org_id": orgId, "provider": providerName, "deleted_at": nil})

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptScreeningSavedSearch)
}

func (repo *MarbleDbRepository) SaveScreeningSearch(
	ctx context.Context,
	exec Executor,
	orgId uuid.UUID,
	provider models.ScreeningProvider,
	name string,
	cfg dto.ScreeningFreeformDto,
) (models.ScreeningSavedSearch, error) {
	cfgJson, err := json.Marshal(cfg)
	if err != nil {
		return models.ScreeningSavedSearch{}, err
	}

	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_SCREENING_SAVED_SEARCHES).
		Columns("id", "org_id", "provider", "name", "config").
		Values(
			pure_utils.NewId(),
			orgId,
			provider,
			name,
			cfgJson,
		).
		Suffix(fmt.Sprintf("returning %s", strings.Join(dbmodels.ScreeningSavedSearchesColumns, ",")))

	return SqlToModel(ctx, exec, query, dbmodels.AdaptScreeningSavedSearch)
}

func (repo *MarbleDbRepository) DeleteSavedSearch(ctx context.Context, exec Executor, orgId, id uuid.UUID) error {
	query := NewQueryBuilder().
		Update(dbmodels.TABLE_SCREENING_SAVED_SEARCHES).
		Set("deleted_at", squirrel.Expr("now()")).
		Where(squirrel.Eq{"org_id": orgId, "id": id, "deleted_at": nil})

	return ExecBuilder(ctx, exec, query)
}
