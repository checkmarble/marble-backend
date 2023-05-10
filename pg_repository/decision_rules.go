package pg_repository

import (
	"context"
	"fmt"
	"marble/marble-backend/app"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type dbDecisionRule struct {
	ID            string      `db:"id"`
	OrgID         string      `db:"org_id"`
	DecisionID    string      `db:"decision_id"`
	Name          string      `db:"name"`
	Description   string      `db:"description"`
	ScoreModifier int         `db:"score_modifier"`
	Result        bool        `db:"result"`
	ErrorCode     int         `db:"error_code"`
	DeletedAt     pgtype.Time `db:"deleted_at"`
}

func (dr *dbDecisionRule) toDomain() app.RuleExecution {
	return app.RuleExecution{
		Rule: app.Rule{
			Name:        dr.Name,
			Description: dr.Description,
		},
		Result:              dr.Result,
		ResultScoreModifier: dr.ScoreModifier,
		Error:               app.RuleExecutionError(dr.ErrorCode),
	}
}

func (r *PGRepository) createDecisionRules(ctx context.Context, tx pgx.Tx, orgID string, decisionID string, ruleExecutions []app.RuleExecution) ([]app.RuleExecution, error) {
	query := r.queryBuilder.
		Insert("decision_rules").
		Columns(
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
				orgID,
				decisionID,
				re.Rule.Name,
				re.Rule.Description,
				re.ResultScoreModifier,
				re.Result,
				re.Error,
			)
	}

	sql, args, err := query.Suffix("RETURNING *").ToSql()
	if err != nil {
		return nil, fmt.Errorf("unable to build rule query: %w", err)
	}

	rows, _ := tx.Query(ctx, sql, args...)
	createdDecisionRules, err := pgx.CollectRows(rows, pgx.RowToStructByName[dbDecisionRule])
	if err != nil {
		return nil, fmt.Errorf("unable to create rules: %w", err)
	}

	decisionRulesDTOs := make([]app.RuleExecution, len(createdDecisionRules))
	for i, decisionRule := range createdDecisionRules {
		decisionRulesDTOs[i] = decisionRule.toDomain()
	}
	return decisionRulesDTOs, err
}
