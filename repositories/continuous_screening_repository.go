package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func (repo *MarbleDbRepository) GetContinuousScreeningConfig(ctx context.Context, exec Executor, Id uuid.UUID) (models.ContinuousScreeningConfig, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ContinuousScreeningConfig{}, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.ContinuousScreeningConfigColumnList...).
		From(dbmodels.TABLE_CONTINUOUS_SCREENING_CONFIGS).
		Where(squirrel.Eq{"id": Id})

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptContinuousScreeningConfig)
}

// Get the latest continuous screening config by stable id
func (repo *MarbleDbRepository) GetContinuousScreeningConfigByStableId(ctx context.Context,
	exec Executor, stableId uuid.UUID,
) (models.ContinuousScreeningConfig, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ContinuousScreeningConfig{}, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.ContinuousScreeningConfigColumnList...).
		From(dbmodels.TABLE_CONTINUOUS_SCREENING_CONFIGS).
		Where(squirrel.Eq{"stable_id": stableId}).
		OrderBy("created_at DESC").
		Limit(1)

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptContinuousScreeningConfig)
}

func (repo *MarbleDbRepository) GetContinuousScreeningConfigsByOrgId(
	ctx context.Context,
	exec Executor,
	orgId string,
) ([]models.ContinuousScreeningConfig, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.ContinuousScreeningConfigColumnList...).
		From(dbmodels.TABLE_CONTINUOUS_SCREENING_CONFIGS).
		Where(squirrel.Eq{"org_id": orgId}).
		Where(squirrel.Eq{"enabled": true})

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptContinuousScreeningConfig)
}

// `enabled` is set to true by default (see: `20251105100110_continuous_screening_config.sql` migration)
func (repo *MarbleDbRepository) CreateContinuousScreeningConfig(ctx context.Context, exec Executor,
	input models.CreateContinuousScreeningConfig,
) (models.ContinuousScreeningConfig, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ContinuousScreeningConfig{}, err
	}

	if len(input.Datasets) == 0 {
		return models.ContinuousScreeningConfig{},
			errors.New("datasets are required for continuous screening config")
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_CONTINUOUS_SCREENING_CONFIGS).
		Suffix("RETURNING *").
		Columns(
			"org_id",
			"stable_id",
			"inbox_id",
			"name",
			"description",
			"algorithm",
			"datasets",
			"match_threshold",
			"match_limit",
			"object_types",
		).
		Values(
			input.OrgId,
			input.StableId,
			input.InboxId,
			input.Name,
			input.Description,
			input.Algorithm,
			input.Datasets,
			input.MatchThreshold,
			input.MatchLimit,
			input.ObjectTypes,
		)

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptContinuousScreeningConfig)
}

func (repo *MarbleDbRepository) UpdateContinuousScreeningConfig(
	ctx context.Context,
	exec Executor,
	id uuid.UUID,
	input models.UpdateContinuousScreeningConfig,
) (models.ContinuousScreeningConfig, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ContinuousScreeningConfig{}, err
	}

	countUpdate := 0

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_CONTINUOUS_SCREENING_CONFIGS).
		Suffix("RETURNING *").
		Set("updated_at", squirrel.Expr("NOW()")).
		Where(squirrel.Eq{"id": id})

	if input.Name != nil {
		sql = sql.Set("name", *input.Name)
		countUpdate++
	}
	if input.Description != nil {
		sql = sql.Set("description", *input.Description)
		countUpdate++
	}
	if input.Algorithm != nil {
		sql = sql.Set("algorithm", *input.Algorithm)
		countUpdate++
	}
	if input.Datasets != nil {
		sql = sql.Set("datasets", *input.Datasets)
		countUpdate++
	}
	if input.MatchThreshold != nil {
		sql = sql.Set("match_threshold", *input.MatchThreshold)
		countUpdate++
	}
	if input.MatchLimit != nil {
		sql = sql.Set("match_limit", *input.MatchLimit)
		countUpdate++
	}
	if input.Enabled != nil {
		sql = sql.Set("enabled", *input.Enabled)
		countUpdate++
	}
	if input.ObjectTypes != nil {
		sql = sql.Set("object_types", *input.ObjectTypes)
		countUpdate++
	}
	if input.InboxId != nil {
		sql = sql.Set("inbox_id", *input.InboxId)
		countUpdate++
	}

	if countUpdate == 0 {
		config, err := repo.GetContinuousScreeningConfig(ctx, exec, id)
		if err != nil {
			return models.ContinuousScreeningConfig{}, err
		}
		return config, nil
	}

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptContinuousScreeningConfig)
}

// For this method, we use the ScreeningWithMatches struct to insert the continuous screening.
// The struct contains all information we need and we reuse the struct which is built from OpenSanction Search Response.
func (*MarbleDbRepository) InsertContinuousScreening(
	ctx context.Context,
	exec Executor,
	screening models.ScreeningWithMatches,
	orgId uuid.UUID,
	configId uuid.UUID,
	configStableId uuid.UUID,
	objectType string,
	objectId string,
	objectInternalId uuid.UUID,
) (models.ContinuousScreeningWithMatches, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ContinuousScreeningWithMatches{}, err
	}

	id := uuid.New()

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_CONTINUOUS_SCREENINGS).
		Suffix("RETURNING *").
		Columns(
			"id",
			"org_id",
			"continuous_screening_config_id",
			"continuous_screening_config_stable_id",
			"object_type",
			"object_id",
			"object_internal_id",
			"status",
			"search_input",
			"is_partial",
			"number_of_matches",
		).
		Values(
			id,
			orgId,
			configId,
			configStableId,
			objectType,
			objectId,
			objectInternalId,
			screening.Status.String(),
			screening.SearchInput,
			screening.Partial,
			screening.NumberOfMatches,
		)

	cs, err := SqlToModel(ctx, exec, sql, dbmodels.AdaptContinuousScreening)
	if err != nil {
		return models.ContinuousScreeningWithMatches{}, err
	}

	if len(screening.Matches) == 0 {
		return models.ContinuousScreeningWithMatches{ContinuousScreening: cs}, nil
	}

	matchSql := NewQueryBuilder().
		Insert(dbmodels.TABLE_CONTINUOUS_SCREENING_MATCHES).
		Suffix("RETURNING *").
		Columns("continuous_screening_id", "opensanction_entity_id", "payload")

	for _, match := range screening.Matches {
		matchSql = matchSql.Values(id, match.EntityId, match.Payload)
	}

	matches, err := SqlToListOfModels(ctx, exec, matchSql, dbmodels.AdaptContinuousScreeningMatch)
	if err != nil {
		return models.ContinuousScreeningWithMatches{}, err
	}

	return models.ContinuousScreeningWithMatches{ContinuousScreening: cs, Matches: matches}, nil
}

func (repo *MarbleDbRepository) GetContinuousScreeningById(ctx context.Context, exec Executor, id string) (models.ContinuousScreening, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ContinuousScreening{}, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectContinuousScreeningColumn...).
		From(dbmodels.TABLE_CONTINUOUS_SCREENINGS).
		Where(squirrel.Eq{"id": id})

	return SqlToModel(ctx, exec, query, dbmodels.AdaptContinuousScreening)
}

func (repo *MarbleDbRepository) ListContinuousScreeningsByCaseId(
	ctx context.Context,
	exec Executor,
	caseId string,
) ([]models.ContinuousScreening, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectContinuousScreeningColumn...).
		From(dbmodels.TABLE_CONTINUOUS_SCREENINGS).
		Where(squirrel.Eq{"case_id": caseId})

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptContinuousScreening)
}

func (repo *MarbleDbRepository) ListContinuousScreeningsByIds(ctx context.Context, exec Executor, ids []uuid.UUID) ([]models.ContinuousScreening, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectContinuousScreeningColumn...).
		From(dbmodels.TABLE_CONTINUOUS_SCREENINGS).
		Where(squirrel.Eq{"id": ids})

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptContinuousScreening)
}

func (repo *MarbleDbRepository) ListContinuousScreeningsForOrg(
	ctx context.Context,
	exec Executor,
	orgId uuid.UUID,
	paginationAndSorting models.PaginationAndSorting,
) ([]models.ContinuousScreeningWithMatches, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	if paginationAndSorting.Sorting != models.SortingFieldCreatedAt {
		return nil, errors.Wrapf(models.BadParameterError, "invalid sorting field: %s", paginationAndSorting.Sorting)
	}

	orderCond := fmt.Sprintf("cs.%s %s, cs.id %s", paginationAndSorting.Sorting,
		paginationAndSorting.Order, paginationAndSorting.Order)

	query := selectContinuousScreeningWithMatches().
		Where(squirrel.Eq{"cs.org_id": orgId}).
		OrderBy(orderCond).
		Limit(uint64(paginationAndSorting.Limit))

	var offset models.ContinuousScreening
	if paginationAndSorting.OffsetId != "" {
		var err error
		offset, err = repo.GetContinuousScreeningById(ctx, exec, paginationAndSorting.OffsetId)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errors.Wrap(models.NotFoundError,
				"No row found matching the provided offsetId")
		} else if err != nil {
			return nil, errors.Wrap(err,
				"failed to fetch decision corresponding to the provided offsetId")
		}
	}
	var err error
	query, err = applyPaginationFilters(query, paginationAndSorting, offset)
	if err != nil {
		return nil, err
	}

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptContinuousScreeningWithMatches)
}

func selectContinuousScreeningWithMatches() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(columnsNames("cs", dbmodels.SelectContinuousScreeningColumn)...).
		Column(fmt.Sprintf("ARRAY_AGG(ROW(%s) ORDER BY array_position(array['confirmed_hit', 'pending', 'no_hit', 'skipped'], csm.status), csm.payload->>'score' DESC) FILTER (WHERE csm.id IS NOT NULL) AS matches",
			strings.Join(columnsNames("csm", dbmodels.SelectContinuousScreeningMatchesColumn), ","))).
		From(dbmodels.TABLE_CONTINUOUS_SCREENINGS + " AS cs").
		LeftJoin(dbmodels.TABLE_CONTINUOUS_SCREENING_MATCHES +
			" AS csm ON (cs.id = csm.continuous_screening_id)").
		GroupBy(columnsNames("cs", dbmodels.SelectContinuousScreeningColumn)...)
}

func applyPaginationFilters(query squirrel.SelectBuilder, p models.PaginationAndSorting,
	offset models.ContinuousScreening,
) (squirrel.SelectBuilder, error) {
	if p.OffsetId == "" {
		return query, nil
	}

	var offsetValue any
	switch p.Sorting {
	case models.ContinuousScreeningSortingCreatedAt:
		offsetValue = offset.CreatedAt
	default:
		// only ordering and pagination by created_at is allowed for now
		return query, fmt.Errorf("invalid sorting field: %w", models.BadParameterError)
	}

	args := []any{offsetValue, p.OffsetId}
	if p.Order == models.SortingOrderDesc {
		query = query.Where(fmt.Sprintf("(cs.%s, cs.id) < (?, ?)", p.Sorting), args...)
	} else {
		query = query.Where(fmt.Sprintf("(cs.%s, cs.id) > (?, ?)", p.Sorting), args...)
	}

	return query, nil
}

func (repo *MarbleDbRepository) UpdateContinuousScreeningsCaseId(ctx context.Context, exec Executor, ids []uuid.UUID, caseId string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := NewQueryBuilder().
		Update(dbmodels.TABLE_CONTINUOUS_SCREENINGS).
		Where(squirrel.Eq{"id": ids}).
		Set("case_id", caseId)

	return ExecBuilder(ctx, exec, query)
}
