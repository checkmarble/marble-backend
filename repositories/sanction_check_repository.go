package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/squirrel"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"
)

func (*MarbleDbRepository) ListSanctionChecksForDecision(ctx context.Context, exec Executor,
	decisionId string,
) ([]models.SanctionCheck, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	sql := NewQueryBuilder().
		Select(dbmodels.SelectSanctionChecksColumn...).
		From(dbmodels.TABLE_SANCTION_CHECKS).
		Where(squirrel.Eq{"decision_id": decisionId})

	return SqlToListOfModels(ctx, exec, sql, dbmodels.AdaptSanctionCheck)
}

func (*MarbleDbRepository) ListSanctionCheckMatches(ctx context.Context, exec Executor,
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

func (*MarbleDbRepository) InsertSanctionCheck(ctx context.Context, exec Executor,
	decision models.DecisionWithRuleExecutions,
) (models.SanctionCheck, error) {
	utils.LoggerFromContext(ctx).Debug("SANCTION CHECK: inserting matches in database")

	if err := validateMarbleDbExecutor(exec); err != nil {
		return *decision.SanctionCheckExecution, err
	}

	sql := NewQueryBuilder().
		Insert(dbmodels.TABLE_SANCTION_CHECKS).Columns(
		"decision_id",
		"search_input",
		"search_datasets",
		"search_threshold",
		"is_partial",
	).Values(
		decision.DecisionId,
		decision.SanctionCheckExecution.Query,
		decision.SanctionCheckExecution.Query.OrgConfig.Datasets,
		decision.SanctionCheckExecution.Query.OrgConfig.MatchThreshold,
		decision.SanctionCheckExecution.Partial,
	).Suffix(fmt.Sprintf("RETURNING %s", strings.Join(dbmodels.SelectSanctionChecksColumn, ",")))

	result, err := SqlToModel(ctx, exec, sql, dbmodels.AdaptSanctionCheck)
	if err != nil {
		return models.SanctionCheck{}, err
	}

	if len(decision.SanctionCheckExecution.Matches) == 0 {
		return result, nil
	}

	matchSql := NewQueryBuilder().Insert(dbmodels.TABLE_SANCTION_CHECK_MATCHES).
		Columns("sanction_check_id", "opensanction_entity_id", "query_ids", "payload").
		Suffix(fmt.Sprintf("RETURNING %s", strings.Join(dbmodels.SelectSanctionCheckMatchesColumn, ",")))

	for _, match := range decision.SanctionCheckExecution.Matches {
		matchSql = matchSql.Values(result.Id, match.EntityId, match.QueryIds, match.Payload)
	}

	matches, err := SqlToListOfModels(ctx, exec, matchSql, dbmodels.AdaptSanctionCheckMatch)
	if err != nil {
		return models.SanctionCheck{}, err
	}

	result.Matches = matches

	return result, nil
}
