package pg_repository

import (
	"context"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/utils"

	"github.com/jackc/pgx/v5"
)

func (r *PGRepository) createDecisionRules(ctx context.Context, tx pgx.Tx, orgID string, decisionID string, ruleExecutions []models.RuleExecution) error {
	query := r.queryBuilder.
		Insert("decision_rules").
		Columns(
			"id",
			"org_id",
			"decision_id",
			"name",
			"description",
			"score_modifier",
			"result",
			"error_code",
		)

	for _, re := range ruleExecutions {
		query = query.
			Values(
				utils.NewPrimaryKey(orgID),
				orgID,
				decisionID,
				re.Rule.Name,
				re.Rule.Description,
				re.ResultScoreModifier,
				re.Result,
				re.Error,
			)
	}

	sql, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("unable to build rule query: %w", err)
	}

	_, err = tx.Exec(ctx, sql, args...)
	return err
}
