package pg_repository

import (
	"context"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories/dbmodels"
	"marble/marble-backend/utils"

	"github.com/jackc/pgx/v5"
)

func (r *PGRepository) createScenarioIterationRules(ctx context.Context, tx pgx.Tx, organizationId string, scenarioIterationId string, rules []models.CreateRuleInput) ([]models.Rule, error) {
	if len(rules) == 0 {
		return nil, nil
	}

	sql, args, err := r.queryBuilder.
		Select("version IS NULL").
		From("scenario_iterations").
		Where("id = ?", scenarioIterationId).
		Where("org_id = ?", organizationId).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("unable to build scenario iteration rule query: %w", err)
	}

	var isDraft bool
	err = tx.QueryRow(ctx, sql, args...).Scan(&isDraft)
	if err != nil {
		return nil, fmt.Errorf("unable to check if scenario iteration is draft: %w", err)
	}
	if !isDraft {
		return nil, models.ErrScenarioIterationNotDraft
	}

	query := r.queryBuilder.
		Insert("scenario_iteration_rules").
		Columns(
			"id",
			"scenario_iteration_id",
			"org_id",
			"display_order",
			"name",
			"description",
			"formula_ast_expression",
			"score_modifier")

	for _, rule := range rules {

		formulaAstExpression, err := dbmodels.SerializeFormulaAstExpression(rule.FormulaAstExpression)
		if err != nil {
			return nil, fmt.Errorf("unable to marshal rule formula: %w", err)
		}

		// append all values to the query
		query = query.
			Values(
				utils.NewPrimaryKey(organizationId),
				scenarioIterationId,
				organizationId,
				rule.DisplayOrder,
				rule.Name,
				rule.Description,
				formulaAstExpression,
				rule.ScoreModifier,
			)
	}

	sql, args, err = query.Suffix("RETURNING *").ToSql()
	if err != nil {
		return nil, fmt.Errorf("unable to build rule query: %w", err)
	}

	rows, _ := tx.Query(ctx, sql, args...)
	createdRules, err := pgx.CollectRows(rows, pgx.RowToStructByName[dbmodels.DBRule])
	if err != nil {
		return nil, fmt.Errorf("unable to create rules: %w", err)
	}

	rulesDTOs := make([]models.Rule, len(createdRules))
	for i, createdRule := range createdRules {
		rulesDTOs[i], err = dbmodels.AdaptRule(createdRule)
		if err != nil {
			return nil, fmt.Errorf("dto issue: %w", err)
		}
	}
	return rulesDTOs, err
}
