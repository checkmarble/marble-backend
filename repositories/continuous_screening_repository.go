package repositories

import (
	"context"
	"fmt"
	"strings"
	"time"

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
		Select(dbmodels.SelectContinuousScreeningConfigColumnList...).
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
		Select(dbmodels.SelectContinuousScreeningConfigColumnList...).
		From(dbmodels.TABLE_CONTINUOUS_SCREENING_CONFIGS).
		Where(squirrel.Eq{"stable_id": stableId}).
		OrderBy("created_at DESC").
		Limit(1)

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptContinuousScreeningConfig)
}

// Fetch enabled continuous screening configs for a given object type and org id
func (repo *MarbleDbRepository) ListContinuousScreeningConfigByObjectType(
	ctx context.Context,
	exec Executor,
	orgId uuid.UUID,
	objectType string,
) ([]models.ContinuousScreeningConfig, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.SelectContinuousScreeningConfigColumnList...).
		From(dbmodels.TABLE_CONTINUOUS_SCREENING_CONFIGS).
		Where(squirrel.Eq{"org_id": orgId}).
		Where(squirrel.Expr("? = ANY(object_types)", objectType)).
		Where(squirrel.Eq{"enabled": true})

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptContinuousScreeningConfig)
}

func (repo *MarbleDbRepository) GetContinuousScreeningConfigsByOrgId(
	ctx context.Context,
	exec Executor,
	orgId uuid.UUID,
) ([]models.ContinuousScreeningConfig, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.SelectContinuousScreeningConfigColumnList...).
		From(dbmodels.TABLE_CONTINUOUS_SCREENING_CONFIGS).
		Where(squirrel.Eq{"org_id": orgId}).
		Where(squirrel.Eq{"enabled": true})

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptContinuousScreeningConfig)
}

func (repo *MarbleDbRepository) ListContinuousScreeningConfigs(
	ctx context.Context,
	exec Executor,
) ([]models.ContinuousScreeningConfig, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectContinuousScreeningConfigColumnList...).
		From(dbmodels.TABLE_CONTINUOUS_SCREENING_CONFIGS).
		Where(squirrel.Eq{"enabled": true}).
		OrderBy("id")

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptContinuousScreeningConfig)
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
func (repo *MarbleDbRepository) InsertContinuousScreening(
	ctx context.Context,
	exec Executor,
	screening models.ScreeningWithMatches,
	config models.ContinuousScreeningConfig,
	objectType string,
	objectId string,
	objectInternalId uuid.UUID,
	triggerType models.ContinuousScreeningTriggerType,
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
			"trigger_type",
			"search_input",
			"is_partial",
			"number_of_matches",
		).
		Values(
			id,
			config.OrgId,
			config.Id,
			config.StableId,
			objectType,
			objectId,
			objectInternalId,
			screening.Status.String(),
			triggerType.String(),
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

func (repo *MarbleDbRepository) InsertContinuousScreeningMatches(
	ctx context.Context,
	exec Executor,
	screeningId uuid.UUID,
	matches []models.ContinuousScreeningMatch,
) ([]models.ContinuousScreeningMatch, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	if len(matches) == 0 {
		return nil, nil
	}

	matchSql := NewQueryBuilder().
		Insert(dbmodels.TABLE_CONTINUOUS_SCREENING_MATCHES).
		Suffix("RETURNING *").
		Columns("continuous_screening_id", "opensanction_entity_id", "payload")

	for _, match := range matches {
		matchSql = matchSql.Values(screeningId, match.OpenSanctionEntityId, match.Payload)
	}

	return SqlToListOfModels(ctx, exec, matchSql, dbmodels.AdaptContinuousScreeningMatch)
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

func (repo *MarbleDbRepository) GetContinuousScreeningWithMatchesById(ctx context.Context,
	exec Executor, id uuid.UUID,
) (models.ContinuousScreeningWithMatches, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ContinuousScreeningWithMatches{}, err
	}

	query := selectContinuousScreeningWithMatches().
		Where(squirrel.Eq{"cs.id": id})

	return SqlToModel(ctx, exec, query, dbmodels.AdaptContinuousScreeningWithMatches)
}

func (repo *MarbleDbRepository) ListContinuousScreeningsWithMatchesByCaseId(
	ctx context.Context,
	exec Executor,
	caseId string,
) ([]models.ContinuousScreeningWithMatches, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := selectContinuousScreeningWithMatches().
		Where(squirrel.Eq{"cs.case_id": caseId})

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptContinuousScreeningWithMatches)
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

// This function is used to check if the screening result is the same as the existing one (if exists)
// Get the latest screening result in review and attached to a case for a given object id and object type
func (repo *MarbleDbRepository) GetContinuousScreeningByObjectId(
	ctx context.Context,
	exec Executor,
	objectId string,
	objectType string,
	orgId uuid.UUID,
	status *models.ScreeningStatus,
	inCase bool,
) (*models.ContinuousScreeningWithMatches, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := selectContinuousScreeningWithMatches().
		Where(squirrel.Eq{"cs.org_id": orgId}).
		Where(squirrel.Eq{"cs.object_type": objectType}).
		Where(squirrel.Eq{"cs.object_id": objectId}).
		OrderBy("cs.created_at DESC").
		Limit(1)

	if status != nil {
		query = query.Where(squirrel.Eq{"cs.status": status.String()})
	}
	if inCase {
		query = query.Where("cs.case_id IS NOT NULL")
	}

	return SqlToOptionalModel(ctx, exec, query, dbmodels.AdaptContinuousScreeningWithMatches)
}

func (repo *MarbleDbRepository) GetContinuousScreeningMatch(ctx context.Context, exec Executor, id uuid.UUID) (models.ContinuousScreeningMatch, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ContinuousScreeningMatch{}, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectContinuousScreeningMatchesColumn...).
		From(dbmodels.TABLE_CONTINUOUS_SCREENING_MATCHES).
		Where(squirrel.Eq{"id": id})

	return SqlToModel(ctx, exec, query, dbmodels.AdaptContinuousScreeningMatch)
}

func (repo *MarbleDbRepository) UpdateContinuousScreeningMatchStatus(
	ctx context.Context,
	exec Executor,
	id uuid.UUID,
	newStatus models.ScreeningMatchStatus,
	reviewedBy *uuid.UUID,
) (models.ContinuousScreeningMatch, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ContinuousScreeningMatch{}, err
	}

	query := NewQueryBuilder().
		Update(dbmodels.TABLE_CONTINUOUS_SCREENING_MATCHES).
		Where(squirrel.Eq{"id": id}).
		Set("status", newStatus).
		Set("reviewed_by", reviewedBy).
		Suffix("RETURNING *")

	return SqlToModel(ctx, exec, query, dbmodels.AdaptContinuousScreeningMatch)
}

func (repo *MarbleDbRepository) UpdateContinuousScreeningMatchStatusByBatch(
	ctx context.Context,
	exec Executor,
	ids []uuid.UUID,
	newStatus models.ScreeningMatchStatus,
	reviewedBy *uuid.UUID,
) ([]models.ContinuousScreeningMatch, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	if len(ids) == 0 {
		return nil, nil
	}

	query := NewQueryBuilder().
		Update(dbmodels.TABLE_CONTINUOUS_SCREENING_MATCHES).
		Where(squirrel.Eq{"id": ids}).
		Set("status", newStatus).
		Set("reviewed_by", reviewedBy).
		Suffix("RETURNING *")

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptContinuousScreeningMatch)
}

func (repo *MarbleDbRepository) UpdateContinuousScreeningStatus(
	ctx context.Context,
	exec Executor,
	id uuid.UUID,
	newStatus models.ScreeningStatus,
) (models.ContinuousScreening, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ContinuousScreening{}, err
	}

	query := NewQueryBuilder().
		Update(dbmodels.TABLE_CONTINUOUS_SCREENINGS).
		Where(squirrel.Eq{"id": id}).
		Set("status", newStatus).
		Suffix("RETURNING *")

	return SqlToModel(ctx, exec, query, dbmodels.AdaptContinuousScreening)
}

func (repo *MarbleDbRepository) UpdateContinuousScreening(
	ctx context.Context,
	exec Executor,
	id uuid.UUID,
	input models.UpdateContinuousScreeningInput,
) (models.ContinuousScreening, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ContinuousScreening{}, err
	}

	updated := false
	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_CONTINUOUS_SCREENINGS).
		Where(squirrel.Eq{"id": id}).
		Suffix("RETURNING *")

	if input.Status != nil {
		sql = sql.Set("status", *input.Status)
		updated = true
	}
	if input.IsPartial != nil {
		sql = sql.Set("is_partial", *input.IsPartial)
		updated = true
	}
	if input.NumberOfMatches != nil {
		sql = sql.Set("number_of_matches", *input.NumberOfMatches)
		updated = true
	}
	if input.CaseId != nil {
		sql = sql.Set("case_id", *input.CaseId)
		updated = true
	}

	if !updated {
		return repo.GetContinuousScreeningById(ctx, exec, id.String())
	}

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptContinuousScreening)
}

func (repo *MarbleDbRepository) GetLastProcessedVersion(
	ctx context.Context,
	exec Executor,
	datasetName string,
) (models.ContinuousScreeningDatasetUpdate, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ContinuousScreeningDatasetUpdate{}, err
	}

	// Use order by version descending to get the latest version because the version from OpenSanctions use date based versioning
	query := NewQueryBuilder().
		Select(dbmodels.SelectContinuousScreeningDatasetUpdateColumn...).
		From(dbmodels.TABLE_CONTINUOUS_SCREENING_DATASET_UPDATES).
		Where(squirrel.Eq{"dataset_name": datasetName}).
		OrderBy("version DESC").
		Limit(1)

	return SqlToModel(ctx, exec, query, dbmodels.AdaptContinuousScreeningDatasetUpdate)
}

func (repo *MarbleDbRepository) CreateContinuousScreeningDatasetUpdate(
	ctx context.Context,
	exec Executor,
	input models.CreateContinuousScreeningDatasetUpdate,
) (models.ContinuousScreeningDatasetUpdate, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ContinuousScreeningDatasetUpdate{}, err
	}

	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_CONTINUOUS_SCREENING_DATASET_UPDATES).
		Suffix("RETURNING *").
		Columns(
			"dataset_name",
			"version",
			"delta_file_path",
			"total_items",
		).
		Values(
			input.DatasetName,
			input.Version,
			input.DeltaFilePath,
			input.TotalItems,
		)

	return SqlToModel(ctx, exec, query, dbmodels.AdaptContinuousScreeningDatasetUpdate)
}

func (repo *MarbleDbRepository) CreateContinuousScreeningUpdateJob(
	ctx context.Context,
	exec Executor,
	input models.CreateContinuousScreeningUpdateJob,
) (models.ContinuousScreeningUpdateJob, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ContinuousScreeningUpdateJob{}, err
	}

	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_CONTINUOUS_SCREENING_UPDATE_JOBS).
		Suffix("RETURNING *").
		Columns(
			"continuous_screening_dataset_update_id",
			"continuous_screening_config_id",
			"org_id",
			"status",
		).
		Values(
			input.DatasetUpdateId,
			input.ConfigId,
			input.OrgId,
			models.ContinuousScreeningUpdateJobStatusPending.String(),
		)

	return SqlToModel(ctx, exec, query, dbmodels.AdaptContinuousScreeningUpdateJob)
}

func (repo *MarbleDbRepository) GetEnrichedContinuousScreeningUpdateJob(
	ctx context.Context,
	exec Executor,
	id uuid.UUID,
) (models.EnrichedContinuousScreeningUpdateJob, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.EnrichedContinuousScreeningUpdateJob{}, err
	}

	query := NewQueryBuilder().
		Select(fmt.Sprintf("ROW(%s) AS update_job", strings.Join(
			columnsNames("ucs", dbmodels.SelectContinuousScreeningUpdateJobColumn), ","))).
		Column(fmt.Sprintf("ROW(%s) AS config", strings.Join(columnsNames("cs",
			dbmodels.SelectContinuousScreeningConfigColumnList), ","))).
		Column(fmt.Sprintf("ROW(%s) AS dataset_update", strings.Join(columnsNames("ds",
			dbmodels.SelectContinuousScreeningDatasetUpdateColumn), ","))).
		From(dbmodels.TABLE_CONTINUOUS_SCREENING_UPDATE_JOBS + " AS ucs").
		LeftJoin(dbmodels.TABLE_CONTINUOUS_SCREENING_CONFIGS +
			" AS cs ON (ucs.continuous_screening_config_id = cs.id)").
		LeftJoin(dbmodels.TABLE_CONTINUOUS_SCREENING_DATASET_UPDATES +
			" AS ds ON (ucs.continuous_screening_dataset_update_id = ds.id)").
		Where(squirrel.Eq{"ucs.id": id})

	return SqlToModel(ctx, exec, query, dbmodels.AdaptEnrichedContinuousScreeningUpdateJob)
}

func (repo *MarbleDbRepository) UpdateContinuousScreeningUpdateJob(
	ctx context.Context,
	exec Executor,
	updateId uuid.UUID,
	status models.ContinuousScreeningUpdateJobStatus,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := NewQueryBuilder().
		Update(dbmodels.TABLE_CONTINUOUS_SCREENING_UPDATE_JOBS).
		Where(squirrel.Eq{"id": updateId}).
		Set("status", status.String()).
		Set("updated_at", "NOW()")

	return ExecBuilder(ctx, exec, query)
}

func (repo *MarbleDbRepository) GetContinuousScreeningJobOffset(
	ctx context.Context,
	exec Executor,
	updateJobId uuid.UUID,
) (*models.ContinuousScreeningJobOffset, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectContinuousScreeningJobOffsetColumn...).
		From(dbmodels.TABLE_CONTINUOUS_SCREENING_JOB_OFFSETS).
		Where(squirrel.Eq{"continuous_screening_update_job_id": updateJobId}).
		Limit(1)

	return SqlToOptionalModel(ctx, exec, query, dbmodels.AdaptContinuousScreeningJobOffset)
}

func (repo *MarbleDbRepository) UpsertContinuousScreeningJobOffset(
	ctx context.Context,
	exec Executor,
	input models.CreateContinuousScreeningJobOffset,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_CONTINUOUS_SCREENING_JOB_OFFSETS).
		Columns(
			"continuous_screening_update_job_id",
			"byte_offset",
			"items_processed",
		).
		Values(
			input.UpdateJobId,
			input.ByteOffset,
			input.ItemsProcessed,
		).
		Suffix(
			"ON CONFLICT (continuous_screening_update_job_id) DO UPDATE SET " +
				"byte_offset = EXCLUDED.byte_offset, " +
				"items_processed = EXCLUDED.items_processed, " +
				"updated_at = NOW()",
		)

	return ExecBuilder(ctx, exec, query)
}

func (repo *MarbleDbRepository) CreateContinuousScreeningJobError(
	ctx context.Context,
	exec Executor,
	input models.CreateContinuousScreeningJobError,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_CONTINUOUS_SCREENING_JOB_ERRORS).
		Columns(
			"continuous_screening_update_job_id",
			"details",
		).
		Values(
			input.UpdateJobId,
			input.Details,
		)

	return ExecBuilder(ctx, exec, query)
}

func (repo *MarbleDbRepository) CreateContinuousScreeningDeltaTrack(
	ctx context.Context,
	exec Executor,
	input models.CreateContinuousScreeningDeltaTrack,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_CONTINUOUS_SCREENING_DELTA_TRACKS).
		Columns(
			"id",
			"org_id",
			"object_type",
			"object_id",
			"object_internal_id",
			"entity_id",
			"operation",
		).
		Values(
			uuid.Must(uuid.NewV7()),
			input.OrgId,
			input.ObjectType,
			input.ObjectId,
			input.ObjectInternalId,
			input.EntityId,
			input.Operation.String(),
		)

	return ExecBuilder(ctx, exec, query)
}

// Fetch entity IDs which have been changed and not processed yet.
// Only fetch the last change for each entity ID, and consider previous changes have been processed in the same version.
// CursorEntityId is the entity ID of the last change that has been processed.
func (repo *MarbleDbRepository) ListContinuousScreeningLastChangeByEntityIds(
	ctx context.Context,
	exec Executor,
	orgId uuid.UUID,
	limit uint64,
	toDate time.Time,
	cursorEntityId string,
) ([]models.ContinuousScreeningDeltaTrack, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectContinuousScreeningDeltaTrackColumn...).
		Options("DISTINCT ON (entity_id)").
		From(dbmodels.TABLE_CONTINUOUS_SCREENING_DELTA_TRACKS).
		Where(squirrel.Eq{"dataset_file_id": nil}).
		Where(squirrel.Eq{"org_id": orgId}).
		Where(squirrel.Lt{"created_at": toDate})

	if cursorEntityId != "" {
		query = query.Where(squirrel.Gt{"entity_id": cursorEntityId})
	}

	query = query.OrderBy("entity_id", "created_at DESC").
		Limit(limit)

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptContinuousScreeningDeltaTrack)
}

func (repo *MarbleDbRepository) GetContinuousScreeningLatestDatasetFileByOrgId(
	ctx context.Context,
	exec Executor,
	orgId uuid.UUID,
	fileType models.ContinuousScreeningDatasetFileType,
) (*models.ContinuousScreeningDatasetFile, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectContinuousScreeningDatasetFileColumn...).
		From(dbmodels.TABLE_CONTINUOUS_SCREENING_DATASET_FILES).
		Where(squirrel.Eq{"org_id": orgId}).
		Where(squirrel.Eq{"file_type": fileType.String()}).
		OrderBy("version DESC").
		Limit(1)

	return SqlToOptionalModel(ctx, exec, query, dbmodels.AdaptContinuousScreeningDatasetFile)
}

func (repo *MarbleDbRepository) ListContinuousScreeningLatestFullFiles(
	ctx context.Context,
	exec Executor,
) ([]models.ContinuousScreeningDatasetFile, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectContinuousScreeningDatasetFileColumn...).
		Options("DISTINCT ON (org_id)").
		From(dbmodels.TABLE_CONTINUOUS_SCREENING_DATASET_FILES).
		Where(squirrel.Eq{"file_type": models.ContinuousScreeningDatasetFileTypeFull.String()}).
		OrderBy("org_id", "version DESC")

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptContinuousScreeningDatasetFile)
}

// List the limit latest delta files by org
func (repo *MarbleDbRepository) ListContinuousScreeningLatestDeltaFiles(
	ctx context.Context,
	exec Executor,
	orgId uuid.UUID,
	limit uint64,
) ([]models.ContinuousScreeningDatasetFile, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectContinuousScreeningDatasetFileColumn...).
		From(dbmodels.TABLE_CONTINUOUS_SCREENING_DATASET_FILES).
		Where(squirrel.Eq{"org_id": orgId}).
		Where(squirrel.Eq{"file_type": models.ContinuousScreeningDatasetFileTypeDelta.String()}).
		OrderBy("version DESC").
		Limit(limit)

	return SqlToListOfModels(ctx, exec, query, dbmodels.AdaptContinuousScreeningDatasetFile)
}

func (repo *MarbleDbRepository) GetContinuousScreeningDatasetFileById(
	ctx context.Context,
	exec Executor,
	id uuid.UUID,
) (models.ContinuousScreeningDatasetFile, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ContinuousScreeningDatasetFile{}, err
	}

	query := NewQueryBuilder().
		Select(dbmodels.SelectContinuousScreeningDatasetFileColumn...).
		From(dbmodels.TABLE_CONTINUOUS_SCREENING_DATASET_FILES).
		Where(squirrel.Eq{"id": id})

	return SqlToModel(ctx, exec, query, dbmodels.AdaptContinuousScreeningDatasetFile)
}

func (repo *MarbleDbRepository) CreateContinuousScreeningDatasetFile(
	ctx context.Context,
	exec Executor,
	input models.CreateContinuousScreeningDatasetFile,
) (models.ContinuousScreeningDatasetFile, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ContinuousScreeningDatasetFile{}, err
	}

	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_CONTINUOUS_SCREENING_DATASET_FILES).
		Suffix("RETURNING *").
		Columns(
			"org_id",
			"file_type",
			"version",
			"file_path",
		).
		Values(
			input.OrgId,
			input.FileType.String(),
			input.Version,
			input.FilePath,
		)

	return SqlToModel(ctx, exec, query, dbmodels.AdaptContinuousScreeningDatasetFile)
}

func (repo *MarbleDbRepository) UpdateDeltaTracksDatasetFileId(
	ctx context.Context,
	exec Executor,
	orgId uuid.UUID,
	datasetFileId uuid.UUID,
	toDate time.Time,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	query := NewQueryBuilder().
		Update(dbmodels.TABLE_CONTINUOUS_SCREENING_DELTA_TRACKS).
		Set("dataset_file_id", datasetFileId).
		Set("updated_at", squirrel.Expr("NOW()")).
		Where(squirrel.Eq{"org_id": orgId}).
		Where(squirrel.Eq{"dataset_file_id": nil}).
		Where(squirrel.Lt{"created_at": toDate})

	return ExecBuilder(ctx, exec, query)
}
