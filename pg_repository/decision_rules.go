package pg_repository

import (
	"context"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/utils"

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

func (dr *dbDecisionRule) toDomain() models.RuleExecution {
	return models.RuleExecution{
		Rule: models.Rule{
			Name:        dr.Name,
			Description: dr.Description,
		},
		Result:              dr.Result,
		ResultScoreModifier: dr.ScoreModifier,
		Error:               models.RuleExecutionError(dr.ErrorCode),
	}
}

func (r *PGRepository) createDecisionRules(ctx context.Context, tx pgx.Tx, orgID string, decisionID string, ruleExecutions []models.RuleExecution) ([]models.RuleExecution, error) {
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

	sql, args, err := query.Suffix("RETURNING *").ToSql()
	if err != nil {
		return nil, fmt.Errorf("unable to build rule query: %w", err)
	}

	rows, _ := tx.Query(ctx, sql, args...)
	createdDecisionRules, err := pgx.CollectRows(rows, pgx.RowToStructByName[dbDecisionRule])
	if err != nil {
		return nil, fmt.Errorf("unable to create rules: %w", err)
	}

	decisionRulesDTOs := make([]models.RuleExecution, len(createdDecisionRules))
	for i, decisionRule := range createdDecisionRules {
		decisionRulesDTOs[i] = decisionRule.toDomain()
	}
	return decisionRulesDTOs, err
}
