package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (*MarbleDbRepository) GetActiveSanctionCheckForDecision(
	ctx context.Context,
	exec Executor,
	decisionId string,
) (*models.SanctionCheckWithMatches, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := selectSanctionChecksWithMatches().
		Where(squirrel.Eq{"sc.decision_id": decisionId, "sc.is_archived": false})

	return SqlToOptionalModel(ctx, exec, sql, dbmodels.AdaptSanctionCheckWithMatches)
}

func (*MarbleDbRepository) GetSanctionChecksForDecision(
	ctx context.Context,
	exec Executor,
	decisionId string,
) ([]models.SanctionCheckWithMatches, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := selectSanctionChecksWithMatches().
		Where(squirrel.Eq{"sc.decision_id": decisionId, "sc.is_archived": false})

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptSanctionCheckWithMatches)
}

func (*MarbleDbRepository) GetSanctionCheck(ctx context.Context, exec Executor, id string) (models.SanctionCheckWithMatches, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.SanctionCheckWithMatches{}, err
	}

	sql := selectSanctionChecksWithMatches().
		Where(squirrel.Eq{"sc.id": id})

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptSanctionCheckWithMatches)
}

func (*MarbleDbRepository) GetSanctionCheckWithoutMatches(ctx context.Context, exec Executor, id string) (models.SanctionCheck, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.SanctionCheck{}, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.SelectSanctionChecksColumn...).
		From(dbmodels.TABLE_SANCTION_CHECKS).
		Where(squirrel.Eq{"id": id})

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptSanctionCheck)
}

func selectSanctionChecksWithMatches() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(columnsNames("sc", dbmodels.SelectSanctionChecksColumn)...).
		Column(fmt.Sprintf("ARRAY_AGG(ROW(%s)) FILTER (WHERE scm.id IS NOT NULL) AS matches",
			strings.Join(columnsNames("scm", dbmodels.SelectSanctionCheckMatchesColumn), ","))).
		From(dbmodels.TABLE_SANCTION_CHECKS + " AS sc").
		LeftJoin(dbmodels.TABLE_SANCTION_CHECK_MATCHES + " AS scm ON sc.id = scm.sanction_check_id").
		GroupBy("sc.id")
}

func (*MarbleDbRepository) ArchiveSanctionCheck(ctx context.Context, exec Executor, decisionId string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_SANCTION_CHECKS).
		Set("is_archived", true).
		Where(
			squirrel.Eq{"decision_id": decisionId, "is_archived": false},
		)

	return ExecBuilder(ctx, exec, sql)
}

func (*MarbleDbRepository) UpdateSanctionCheckStatus(ctx context.Context, exec Executor, id string,
	status models.SanctionCheckStatus,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	return ExecBuilder(
		ctx,
		exec,
		NewQueryBuilder().
			Update(dbmodels.TABLE_SANCTION_CHECKS).
			Set("status", status.String()).
			Set("updated_at", "NOW()").
			Where(squirrel.Eq{"id": id}),
	)
}

func (*MarbleDbRepository) ListSanctionCheckMatches(
	ctx context.Context,
	exec Executor,
	sanctionCheckId string,
) ([]models.SanctionCheckMatch, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.SelectSanctionCheckMatchesColumn...).
		From(dbmodels.TABLE_SANCTION_CHECK_MATCHES).
		Where(squirrel.Eq{"sanction_check_id": sanctionCheckId})

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptSanctionCheckMatch)
}

func (*MarbleDbRepository) GetSanctionCheckMatch(ctx context.Context, exec Executor,
	matchId string,
) (models.SanctionCheckMatch, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.SanctionCheckMatch{}, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.SelectSanctionCheckMatchesColumn...).
		From(dbmodels.TABLE_SANCTION_CHECK_MATCHES).
		Where(squirrel.Eq{"id": matchId})

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptSanctionCheckMatch)
}

func (*MarbleDbRepository) UpdateSanctionCheckMatchStatus(
	ctx context.Context,
	exec Executor,
	match models.SanctionCheckMatch,
	update models.SanctionCheckMatchUpdate,
) (models.SanctionCheckMatch, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.SanctionCheckMatch{}, err
	}

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_SANCTION_CHECK_MATCHES).
		SetMap(map[string]any{
			"status":      update.Status,
			"reviewed_by": update.ReviewerId,
			"updated_at":  "NOW()",
		}).
		Where(squirrel.Eq{"id": match.Id}).
		Suffix(fmt.Sprintf("RETURNING %s", strings.Join(dbmodels.SelectSanctionCheckMatchesColumn, ",")))

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptSanctionCheckMatch)
}

func (*MarbleDbRepository) InsertSanctionCheck(
	ctx context.Context,
	exec Executor,
	decisionId string,
	sanctionCheck models.SanctionCheckWithMatches,
	storeMatches bool,
) (models.SanctionCheckWithMatches, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return sanctionCheck, err
	}

	whitelistedEntities := make([]string, 0)

	if sanctionCheck.WhitelistedEntities != nil {
		whitelistedEntities = sanctionCheck.WhitelistedEntities
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_SANCTION_CHECKS).Columns(
		"decision_id",
		"search_input",
		"search_datasets",
		"match_threshold",
		"match_limit",
		"is_partial",
		"is_manual",
		"initial_has_matches",
		"whitelisted_entities",
		"requested_by",
		"status",
	).Values(
		decisionId,
		sanctionCheck.SearchInput,
		sanctionCheck.Datasets,
		sanctionCheck.OrgConfig.MatchThreshold,
		sanctionCheck.OrgConfig.MatchLimit,
		sanctionCheck.Partial,
		sanctionCheck.IsManual,
		sanctionCheck.InitialHasMatches,
		whitelistedEntities,
		sanctionCheck.RequestedBy,
		sanctionCheck.Status.String(),
	).Suffix(fmt.Sprintf("RETURNING %s", strings.Join(dbmodels.SelectSanctionChecksColumn, ",")))

	result, err := SqlToModel(ctx, exec, sql, dbmodels.AdaptSanctionCheck)
	if err != nil {
		return models.SanctionCheckWithMatches{}, err
	}

	withMatches := models.SanctionCheckWithMatches{SanctionCheck: result}
	if !storeMatches || len(sanctionCheck.Matches) == 0 {
		return withMatches, nil
	}

	matchSql := NewQueryBuilder().Insert(dbmodels.TABLE_SANCTION_CHECK_MATCHES).
		Columns("sanction_check_id", "opensanction_entity_id", "query_ids", "payload", "counterparty_id").
		Suffix(fmt.Sprintf("RETURNING %s", strings.Join(dbmodels.SelectSanctionCheckMatchesColumn, ",")))

	for _, match := range sanctionCheck.Matches {
		matchSql = matchSql.Values(result.Id, match.EntityId, match.QueryIds, match.Payload, match.UniqueCounterpartyIdentifier)
	}

	matches, err := SqlToListOfModels(ctx, exec, matchSql, dbmodels.AdaptSanctionCheckMatch)
	if err != nil {
		return models.SanctionCheckWithMatches{}, err
	}

	withMatches.Matches = matches

	return withMatches, nil
}

func (*MarbleDbRepository) ListSanctionCheckCommentsByIds(ctx context.Context, exec Executor, ids []string) ([]models.SanctionCheckMatchComment, error) {
	sql := NewQueryBuilder().
		Select(dbmodels.SelectSanctionCheckMatchCommentsColumn...).
		From(dbmodels.TABLE_SANCTION_CHECK_MATCH_COMMENTS).
		Where("sanction_check_match_id = ANY(?)", ids)

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptSanctionCheckMatchComment)
}

func (*MarbleDbRepository) AddSanctionCheckMatchComment(ctx context.Context,
	exec Executor, comment models.SanctionCheckMatchComment,
) (models.SanctionCheckMatchComment, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.SanctionCheckMatchComment{}, err
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_SANCTION_CHECK_MATCH_COMMENTS).
		Columns("sanction_check_match_id", "commented_by", "comment").
		Values(comment.MatchId, comment.CommenterId, comment.Comment).
		Suffix(fmt.Sprintf("RETURNING %s", strings.Join(dbmodels.SelectSanctionCheckMatchCommentsColumn, ",")))

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptSanctionCheckMatchComment)
}

func (repo *MarbleDbRepository) CreateSanctionCheckFile(ctx context.Context, exec Executor,
	input models.SanctionCheckFileInput,
) (models.SanctionCheckFile, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.SanctionCheckFile{}, err
	}

	file, err := SqlToModel(
		ctx,
		exec,
		NewQueryBuilder().Insert(dbmodels.TABLE_SANCTION_CHECK_FILES).
			Columns(
				"bucket_name",
				"sanction_check_id",
				"file_name",
				"file_reference",
			).
			Values(
				input.BucketName,
				input.SanctionCheckId,
				input.FileName,
				input.FileReference,
			).
			Suffix(fmt.Sprintf("RETURNING %s", strings.Join(dbmodels.SelectSanctionCheckFileColumn, ","))),
		dbmodels.AdaptSanctionCheckFile,
	)

	return file, err
}

func (repo *MarbleDbRepository) ListSanctionCheckFiles(ctx context.Context, exec Executor,
	sanctionCheckId string,
) ([]models.SanctionCheckFile, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	files, err := SqlToListOfModels(
		ctx,
		exec,
		NewQueryBuilder().Select(dbmodels.SelectSanctionCheckFileColumn...).
			From(dbmodels.TABLE_SANCTION_CHECK_FILES).
			Where(squirrel.Eq{"sanction_check_id": sanctionCheckId}),
		dbmodels.AdaptSanctionCheckFile,
	)

	return files, err
}

func (repo *MarbleDbRepository) GetSanctionCheckFile(ctx context.Context, exec Executor,
	sanctionCheckId, fileId string,
) (models.SanctionCheckFile, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.SanctionCheckFile{}, err
	}

	file, err := SqlToModel(
		ctx,
		exec,
		NewQueryBuilder().Select(dbmodels.SelectSanctionCheckFileColumn...).
			From(dbmodels.TABLE_SANCTION_CHECK_FILES).
			Where(squirrel.Eq{"sanction_check_id": sanctionCheckId, "id": fileId}),
		dbmodels.AdaptSanctionCheckFile,
	)

	return file, err
}

func (repo *MarbleDbRepository) CopySanctionCheckFiles(ctx context.Context, exec Executor, sanctionCheckId, newSanctionCheckId string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_SANCTION_CHECK_FILES).
		Columns("bucket_name", "file_reference", "file_name", "sanction_check_id").
		Select(squirrel.
			Select("bucket_name", "file_reference", "file_name").
			Column("?", newSanctionCheckId).
			From(dbmodels.TABLE_SANCTION_CHECK_FILES).
			Where(squirrel.Eq{"sanction_check_id": sanctionCheckId}))

	if err := ExecBuilder(ctx, exec, sql); err != nil {
		return err
	}

	return nil
}

func (repo *MarbleDbRepository) AddSanctionCheckMatchWhitelist(ctx context.Context, exec Executor,
	orgId, counterpartyId string, entityId string, reviewerId models.UserId,
) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_SANCTION_CHECK_WHITELISTS).
		Columns("org_id", "counterparty_id", "entity_id", "whitelisted_by").
		Values(orgId, counterpartyId, entityId, reviewerId).
		Suffix("ON CONFLICT (org_id, counterparty_id, entity_id) DO NOTHING")

	return ExecBuilder(ctx, exec, sql)
}

func (repo *MarbleDbRepository) IsSanctionCheckMatchWhitelisted(ctx context.Context, exec Executor,
	orgId, counterpartyId string, entityIds []string,
) ([]models.SanctionCheckWhitelist, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.SanctionCheckWhitelistColumnList...).
		From(dbmodels.TABLE_SANCTION_CHECK_WHITELISTS).
		Where(squirrel.And{
			squirrel.Eq{
				"org_id":          orgId,
				"counterparty_id": counterpartyId,
			},
			squirrel.Expr("entity_id = ANY(?)", entityIds),
		})

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptSanctionCheckWhitelist)
}

func (repo *MarbleDbRepository) CountWhitelistsForCounterpartyId(ctx context.Context, exec Executor,
	orgId, counterpartyId string,
) (int, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return 0, err
	}

	query := NewQueryBuilder().
		Select("COUNT(*)").
		From(dbmodels.TABLE_SANCTION_CHECK_WHITELISTS).
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
