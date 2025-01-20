package repositories

import (
	"context"
	"fmt"
	"strings"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"
)

func (*MarbleDbRepository) InsertResults(ctx context.Context, exec Executor,
	decision models.DecisionWithRuleExecutions,
) (models.SanctionCheckExecution, error) {
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
		return models.SanctionCheckExecution{}, err
	}

	if len(decision.SanctionCheckExecution.Matches) == 0 {
		return result, nil
	}

	matchSql := NewQueryBuilder().Insert(dbmodels.TABLE_SANCTION_CHECK_MATCHES).
		Columns("sanction_check_id", "opensanction_entity_id", "query_ids", "payload").
		Suffix(fmt.Sprintf("RETURNING %s", strings.Join(dbmodels.SelectSanctionCheckMatchesColumn, ",")))

	for _, match := range decision.SanctionCheckExecution.Matches {
		matchSql = matchSql.Values(result.Id, match.EntityId, match.QueryIds, match.Raw)
	}

	matches, err := SqlToListOfModels(ctx, exec, matchSql, dbmodels.AdaptSanctionCheckMatch)
	if err != nil {
		return models.SanctionCheckExecution{}, err
	}

	result.Matches = matches

	return result, nil
}
