package repositories

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/google/uuid"
)

func (*MarbleDbRepository) GetActiveScreeningForDecision(
	ctx context.Context,
	exec Executor,
	screeningId string,
) (models.ScreeningWithMatches, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScreeningWithMatches{}, err
	}

	sql := selectScreeningsWithMatches().
		Where(squirrel.Eq{
			"sc.id": screeningId,
		})

	askedFor, err := SqlToModel(ctx, exec, sql, dbmodels.AdaptScreeningWithMatches)
	if err != nil {
		return models.ScreeningWithMatches{}, err
	}
	if !askedFor.IsArchived {
		return askedFor, nil
	}

	sql = selectScreeningsWithMatches().
		Where(squirrel.Eq{
			"sc.decision_id":              askedFor.DecisionId,
			"sc.sanction_check_config_id": askedFor.ScreeningConfigId,
			"sc.is_archived":              false,
		})

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptScreeningWithMatches)
}

func (*MarbleDbRepository) ListScreeningsForDecision(
	ctx context.Context,
	exec Executor,
	decisionId string,
	initialOnly bool,
) ([]models.ScreeningWithMatches, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	filters := squirrel.Eq{"sc.decision_id": decisionId}

	if initialOnly {
		filters["sc.is_manual"] = false
	} else {
		filters["sc.is_archived"] = false
	}

	sql := selectScreeningsWithMatches().
		Where(filters)

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptScreeningWithMatches)
}

func (*MarbleDbRepository) GetScreening(ctx context.Context, exec Executor, id string) (models.ScreeningWithMatches, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScreeningWithMatches{}, err
	}

	sql := selectScreeningsWithMatches().
		Where(squirrel.Eq{"sc.id": id})

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptScreeningWithMatches)
}

func (*MarbleDbRepository) GetScreeningWithoutMatches(ctx context.Context, exec Executor, id string) (models.Screening, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.Screening{}, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.SelectScreeningColumn...).
		From(dbmodels.TABLE_SCREENINGS).
		Where(squirrel.Eq{"id": id})

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptScreening)
}

func selectScreeningsWithMatches() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(columnsNames("sc", dbmodels.SelectScreeningColumn)...).
		Column(fmt.Sprintf("ARRAY_AGG(ROW(%s) ORDER BY array_position(array['confirmed_hit', 'pending', 'no_hit', 'skipped'], scm.status), scm.payload->>'score' DESC) FILTER (WHERE scm.id IS NOT NULL) AS matches",
			strings.Join(columnsNames("scm", dbmodels.SelectScreeningMatchesColumn), ","))).
		From(dbmodels.TABLE_SCREENINGS + " AS sc").
		LeftJoin(dbmodels.TABLE_SCREENING_MATCHES + " AS scm ON sc.id = scm.sanction_check_id").
		GroupBy("sc.id").
		OrderBy("sc.created_at")
}

func (*MarbleDbRepository) ArchiveScreening(ctx context.Context, exec Executor, id string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_SCREENINGS).
		Set("is_archived", true).
		Where(
			squirrel.Eq{"id": id, "is_archived": false},
		)

	return ExecBuilder(ctx, exec, sql)
}

func (*MarbleDbRepository) UpdateScreeningStatus(ctx context.Context, exec Executor, id string,
	status models.ScreeningStatus,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	return ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().
			Update(dbmodels.TABLE_SCREENINGS).
			Set("status", status.String()).
			Set("updated_at", "NOW()").
			Where(squirrel.Eq{"id": id}),
	)
}

func (*MarbleDbRepository) UpdateScreeningMatchPayload(ctx context.Context, exec Executor,
	match models.ScreeningMatch, newPayload []byte,
) (models.ScreeningMatch, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScreeningMatch{}, err
	}

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_SCREENING_MATCHES).
		Set("payload", newPayload).
		Set("enriched", true).
		Set("updated_at", "NOW()").
		Where(squirrel.Eq{"id": match.Id}).Suffix(fmt.Sprintf("RETURNING %s",
		strings.Join(dbmodels.SelectScreeningMatchesColumn, ",")))

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptScreeningMatch)
}

func (*MarbleDbRepository) ListScreeningMatches(
	ctx context.Context,
	exec Executor,
	screeningId string,
) ([]models.ScreeningMatch, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.SelectScreeningMatchesColumn...).
		From(dbmodels.TABLE_SCREENING_MATCHES).
		Where(squirrel.Eq{"sanction_check_id": screeningId})

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptScreeningMatch)
}

func (*MarbleDbRepository) GetScreeningMatch(ctx context.Context, exec Executor,
	matchId string,
) (models.ScreeningMatch, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScreeningMatch{}, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.SelectScreeningMatchesColumn...).
		From(dbmodels.TABLE_SCREENING_MATCHES).
		Where(squirrel.Eq{"id": matchId})

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptScreeningMatch)
}

func (*MarbleDbRepository) UpdateScreeningMatchStatus(
	ctx context.Context,
	exec Executor,
	match models.ScreeningMatch,
	update models.ScreeningMatchUpdate,
) (models.ScreeningMatch, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScreeningMatch{}, err
	}

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_SCREENING_MATCHES).
		SetMap(map[string]any{
			"status":      update.Status,
			"reviewed_by": update.ReviewerId,
			"updated_at":  "NOW()",
		}).
		Where(squirrel.Eq{"id": match.Id}).
		Suffix(fmt.Sprintf("RETURNING %s", strings.Join(dbmodels.SelectScreeningMatchesColumn, ",")))

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptScreeningMatch)
}

func (*MarbleDbRepository) InsertScreening(
	ctx context.Context,
	exec Executor,
	decisionId string,
	orgId string,
	screening models.ScreeningWithMatches,
	storeMatches bool,
) (models.ScreeningWithMatches, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return screening, err
	}

	scId := screening.Id
	if scId == "" {
		scId = uuid.NewString()
	}

	whitelistedEntities := make([]string, 0)

	if screening.WhitelistedEntities != nil {
		whitelistedEntities = screening.WhitelistedEntities
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_SCREENINGS).Columns(
		"id",
		"decision_id",
		"org_id",
		"sanction_check_config_id",
		"search_input",
		"initial_query",
		"search_datasets",
		"match_threshold",
		"match_limit",
		"is_partial",
		"is_manual",
		"initial_has_matches",
		"whitelisted_entities",
		"requested_by",
		"status",
		"error_codes",
		"number_of_matches",
	).Values(
		scId,
		decisionId,
		orgId,
		screening.ScreeningConfigId,
		screening.SearchInput,
		screening.InitialQuery,
		screening.Datasets,
		screening.EffectiveThreshold,
		screening.OrgConfig.MatchLimit,
		screening.Partial,
		screening.IsManual,
		screening.InitialHasMatches,
		whitelistedEntities,
		screening.RequestedBy,
		screening.Status.String(),
		screening.ErrorCodes,
		screening.NumberOfMatches,
	).Suffix(fmt.Sprintf("RETURNING %s", strings.Join(dbmodels.SelectScreeningColumn, ",")))

	result, err := SqlToModel(ctx, exec, sql, dbmodels.AdaptScreening)
	if err != nil {
		return models.ScreeningWithMatches{}, err
	}

	withMatches := models.ScreeningWithMatches{Screening: result}
	if !storeMatches || len(screening.Matches) == 0 {
		return withMatches, nil
	}

	matchSql := NewQueryBuilder().Insert(dbmodels.TABLE_SCREENING_MATCHES).
		Columns("sanction_check_id", "opensanction_entity_id", "query_ids", "payload", "counterparty_id").
		Suffix(fmt.Sprintf("RETURNING %s", strings.Join(dbmodels.SelectScreeningMatchesColumn, ",")))

	for _, match := range screening.Matches {
		matchSql = matchSql.Values(result.Id, match.EntityId, match.QueryIds, match.Payload, match.UniqueCounterpartyIdentifier)
	}

	matches, err := SqlToListOfModels(ctx, exec, matchSql, dbmodels.AdaptScreeningMatch)
	if err != nil {
		return models.ScreeningWithMatches{}, err
	}

	withMatches.Matches = matches

	return withMatches, nil
}

func (*MarbleDbRepository) ListScreeningCommentsByIds(ctx context.Context, exec Executor, ids []string) ([]models.ScreeningMatchComment, error) {
	sql := NewQueryBuilder().
		Select(dbmodels.SelectScreeningMatchCommentsColumn...).
		From(dbmodels.TABLE_SCREENING_MATCH_COMMENTS).
		Where("sanction_check_match_id = ANY(?)", ids)

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptScreeningMatchComment)
}

func (*MarbleDbRepository) AddScreeningMatchComment(ctx context.Context,
	exec Executor, comment models.ScreeningMatchComment,
) (models.ScreeningMatchComment, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScreeningMatchComment{}, err
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_SCREENING_MATCH_COMMENTS).
		Columns("sanction_check_match_id", "commented_by", "comment").
		Values(comment.MatchId, comment.CommenterId, comment.Comment).
		Suffix(fmt.Sprintf("RETURNING %s", strings.Join(dbmodels.SelectScreeningMatchCommentsColumn, ",")))

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptScreeningMatchComment)
}

func (repo *MarbleDbRepository) CreateScreeningFile(ctx context.Context, exec Executor,
	input models.ScreeningFileInput,
) (models.ScreeningFile, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScreeningFile{}, err
	}

	file, err := SqlToModel(
		ctx,
		exec,
		NewQueryBuilder().Insert(dbmodels.TABLE_SCREENING_FILES).
			Columns(
				"bucket_name",
				"sanction_check_id",
				"file_name",
				"file_reference",
			).
			Values(
				input.BucketName,
				input.ScreeningId,
				input.FileName,
				input.FileReference,
			).
			Suffix(fmt.Sprintf("RETURNING %s", strings.Join(dbmodels.SelectScreeningFileColumn, ","))),
		dbmodels.AdaptScreeningFile,
	)

	return file, err
}

func (repo *MarbleDbRepository) ListScreeningFiles(ctx context.Context, exec Executor,
	screeningId string,
) ([]models.ScreeningFile, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	files, err := SqlToListOfModels(
		ctx,
		exec,
		NewQueryBuilder().Select(dbmodels.SelectScreeningFileColumn...).
			From(dbmodels.TABLE_SCREENING_FILES).
			Where(squirrel.Eq{"sanction_check_id": screeningId}),
		dbmodels.AdaptScreeningFile,
	)

	return files, err
}

func (repo *MarbleDbRepository) GetScreeningFile(ctx context.Context, exec Executor,
	screeningId, fileId string,
) (models.ScreeningFile, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.ScreeningFile{}, err
	}

	file, err := SqlToModel(
		ctx,
		exec,
		NewQueryBuilder().Select(dbmodels.SelectScreeningFileColumn...).
			From(dbmodels.TABLE_SCREENING_FILES).
			Where(squirrel.Eq{"sanction_check_id": screeningId, "id": fileId}),
		dbmodels.AdaptScreeningFile,
	)

	return file, err
}

func (repo *MarbleDbRepository) CopyScreeningFiles(ctx context.Context, exec Executor, screeningId, newScreeningId string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_SCREENING_FILES).
		Columns("bucket_name", "file_reference", "file_name", "sanction_check_id").
		Select(squirrel.
			Select("bucket_name", "file_reference", "file_name").
			Column("?", newScreeningId).
			From(dbmodels.TABLE_SCREENING_FILES).
			Where(squirrel.Eq{"sanction_check_id": screeningId}))

	if err := ExecBuilder(ctx, exec, sql); err != nil {
		return err
	}

	return nil
}

func (repo *MarbleDbRepository) CountWhitelistsForCounterpartyId(ctx context.Context, exec Executor,
	orgId, counterpartyId string,
) (int, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return 0, err
	}

	query := NewQueryBuilder().
		Select("COUNT(*)").
		From(dbmodels.TABLE_SCREENING_WHITELISTS).
		Where(squirrel.And{
			squirrel.Eq{
				"org_id":          orgId,
				"counterparty_id": counterpartyId,
			},
		})

	sql, args, err := query.ToSql()
	if err != nil {
		return 0, err
	}

	row := exec.QueryRow(ctx, sql, args...)

	var count int

	if err := row.Scan(&count); err != nil {
		return 0, err
	}

	return count, nil
}

func (repo *MarbleDbRepository) CountScreeningsByOrg(ctx context.Context, exec Executor,
	orgIds []string, from, to time.Time,
) (map[string]int, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	query := NewQueryBuilder().
		Select("org_id, count(*) as count").
		From(dbmodels.TABLE_SCREENINGS).
		Where(squirrel.Eq{"org_id": orgIds}).
		Where(squirrel.GtOrEq{"created_at": from}).
		Where(squirrel.Lt{"created_at": to}).
		GroupBy("org_id")

	return countByHelper(ctx, exec, query, orgIds)
}

func (repo *MarbleDbRepository) screeningsWithoutHitsOfDecision(
	ctx context.Context,
	exec Executor,
	decisionIds []string,
) (map[string][]models.Screening, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.SelectScreeningColumn...).
		From(dbmodels.TABLE_SCREENINGS).
		Where(squirrel.Eq{
			"decision_id": decisionIds,
			"is_archived": false,
		})

	screenings, err := SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptScreening)
	if err != nil {
		return nil, err
	}

	screeningsAsMap := make(map[string][]models.Screening, len(decisionIds))
	for _, screening := range screenings {
		screeningsAsMap[screening.DecisionId] = append(
			screeningsAsMap[screening.DecisionId], screening)
	}
	return screeningsAsMap, nil
}
