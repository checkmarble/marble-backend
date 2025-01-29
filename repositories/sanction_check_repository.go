package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
)

func (*MarbleDbRepository) GetActiveSanctionCheckForDecision(ctx context.Context, exec Executor,
	decisionId string,
) (models.SanctionCheck, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.SanctionCheck{}, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.SelectSanctionChecksColumn...).
		From(dbmodels.TABLE_SANCTION_CHECKS).
		Where(squirrel.Eq{"decision_id": decisionId, "is_archived": false})

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptSanctionCheck)
}

func (*MarbleDbRepository) GetSanctionCheck(ctx context.Context, exec Executor, id string) (models.SanctionCheck, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.SanctionCheck{}, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.SelectSanctionChecksColumn...).
		From(dbmodels.TABLE_SANCTION_CHECKS).
		Where(squirrel.Eq{"id": id})

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptSanctionCheck)
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

func (*MarbleDbRepository) ListSanctionCheckMatches(ctx context.Context, exec Executor,
	sanctionCheckId string,
) ([]models.SanctionCheckMatch, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := NewQueryBuilder().
		Select(columnsNames("matches", dbmodels.SelectSanctionCheckMatchesColumn)...).
		Column("count(comments.id) AS comment_count").
		From(dbmodels.TABLE_SANCTION_CHECK_MATCHES + " matches").
		LeftJoin(dbmodels.TABLE_SANCTION_CHECK_MATCH_COMMENTS +
			" comments ON matches.id = comments.sanction_check_match_id").
		Where(squirrel.Eq{"sanction_check_id": sanctionCheckId}).
		GroupBy("matches.id")

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptSanctionCheckMatchWithComment)
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

func (*MarbleDbRepository) UpdateSanctionCheckMatchStatus(ctx context.Context, exec Executor,
	match models.SanctionCheckMatch, update models.SanctionCheckMatchUpdate,
) (models.SanctionCheckMatch, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.SanctionCheckMatch{}, err
	}

	sql := NewQueryBuilder().
		Update(dbmodels.TABLE_SANCTION_CHECK_MATCHES).
		SetMap(map[string]any{"status": update.Status, "reviewed_by": update.ReviewerId}).
		Where(squirrel.Eq{"id": match.Id}).
		Suffix(fmt.Sprintf("RETURNING %s", strings.Join(dbmodels.SelectSanctionCheckMatchesColumn, ",")))

	return SqlToModel(ctx, exec, sql, dbmodels.AdaptSanctionCheckMatch)
}

func (*MarbleDbRepository) InsertSanctionCheck(ctx context.Context, exec Executor, decisionId string,
	sanctionCheck models.SanctionCheck,
) (models.SanctionCheck, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return sanctionCheck, err
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_SANCTION_CHECKS).Columns(
		"decision_id",
		"search_input",
		"search_datasets",
		"search_threshold",
		"is_partial",
		"is_manual",
		"requested_by",
	).Values(
		decisionId,
		sanctionCheck.Query,
		sanctionCheck.Datasets,
		sanctionCheck.OrgConfig.MatchThreshold,
		sanctionCheck.Partial,
		sanctionCheck.IsManual,
		sanctionCheck.RequestedBy,
	).Suffix(fmt.Sprintf("RETURNING %s", strings.Join(dbmodels.SelectSanctionChecksColumn, ",")))

	result, err := SqlToModel(ctx, exec, sql, dbmodels.AdaptSanctionCheck)
	if err != nil {
		return models.SanctionCheck{}, err
	}

	if len(sanctionCheck.Matches) == 0 {
		return result, nil
	}

	matchSql := NewQueryBuilder().Insert(dbmodels.TABLE_SANCTION_CHECK_MATCHES).
		Columns("sanction_check_id", "opensanction_entity_id", "query_ids", "payload").
		Suffix(fmt.Sprintf("RETURNING %s", strings.Join(dbmodels.SelectSanctionCheckMatchesColumn, ",")))

	for _, match := range sanctionCheck.Matches {
		matchSql = matchSql.Values(result.Id, match.EntityId, match.QueryIds, match.Payload)
	}

	matches, err := SqlToListOfModels(ctx, exec, matchSql, dbmodels.AdaptSanctionCheckMatch)
	if err != nil {
		return models.SanctionCheck{}, err
	}

	result.Matches = matches

	return result, nil
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

func (*MarbleDbRepository) ListSanctionCheckMatchComments(ctx context.Context,
	exec Executor, matchId string,
) ([]models.SanctionCheckMatchComment, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.SelectSanctionCheckMatchCommentsColumn...).
		From(dbmodels.TABLE_SANCTION_CHECK_MATCH_COMMENTS).
		Where(squirrel.Eq{"sanction_check_match_id": matchId}).
		OrderBy("created_at ASC")

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptSanctionCheckMatchComment)
}
