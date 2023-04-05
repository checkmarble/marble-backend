package pg_repository

import (
	"context"
	"fmt"
	"marble/marble-backend/app"
	"marble/marble-backend/app/operators"

	"github.com/jackc/pgx/v5"
)

type dbScenarioIterationRule struct {
	ID                  string `db:"id"`
	OrgID               string `db:"org_id"`
	ScenarioIterationID string `db:"scenario_iteration_id"`
	DisplayOrder        int    `db:"display_order"`
	Name                string `db:"name"`
	Description         string `db:"description"`
	ScoreModifier       int    `db:"score_modifier"`
	Formula             []byte `db:"formula"`
}

func (sir *dbScenarioIterationRule) dto() (app.Rule, error) {
	formula, err := operators.UnmarshalOperatorBool(sir.Formula)
	if err != nil {
		return app.Rule{}, fmt.Errorf("unable to unmarshal rule: %w", err)
	}

	return app.Rule{
		DisplayOrder:  sir.DisplayOrder,
		Name:          sir.Name,
		Description:   sir.Description,
		Formula:       formula,
		ScoreModifier: sir.ScoreModifier,
	}, nil
}

func (r *PGRepository) CreateScenarioIterationRule(scenarioIterationID string, orgID string, rule app.Rule) (app.Rule, error) {
	formulaBytes, err := rule.Formula.MarshalJSON()
	if err != nil {
		return app.Rule{}, fmt.Errorf("unable to marshal rule formula: %w", err)
	}

	sql, args, err := r.queryBuilder.
		Insert("scenario_iteration_rules").
		Columns(
			"scenario_iteration_id",
			"org_id",
			"display_order",
			"name",
			"description",
			"formula",
			"score_modifier").
		Values(
			scenarioIterationID,
			orgID,
			rule.DisplayOrder,
			rule.Name,
			rule.Description,
			formulaBytes,
			rule.ScoreModifier,
		).Suffix("RETURNING *").ToSql()
	if err != nil {
		return app.Rule{}, fmt.Errorf("unable to build rule query: %w", err)
	}

	rows, _ := r.db.Query(context.TODO(), sql, args...)
	createdRule, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbScenarioIterationRule])
	if err != nil {
		return app.Rule{}, fmt.Errorf("unable to create rule: %w", err)
	}

	ruleDTO, err := createdRule.dto()
	if err != nil {
		return app.Rule{}, fmt.Errorf("dto issue: %w", err)
	}

	return ruleDTO, err
}

func (r *PGRepository) createScenarioIterationRules(_ context.Context, tx pgx.Tx, orgID string, scenarioIterationID string, rules []app.Rule) ([]app.Rule, error) {
	query := r.queryBuilder.
		Insert("scenario_iteration_rules").
		Columns(
			"scenario_iteration_id",
			"org_id",
			"display_order",
			"name",
			"description",
			"formula",
			"score_modifier")

	for _, rule := range rules {
		formulaBytes, err := rule.Formula.MarshalJSON()
		if err != nil {
			return nil, fmt.Errorf("unable to marshal rule formula: %w", err)
		}

		// append all values to the query
		query = query.
			Values(
				scenarioIterationID,
				orgID,
				rule.DisplayOrder,
				rule.Name,
				rule.Description,
				string(formulaBytes),
				rule.ScoreModifier,
			)
	}

	sql, args, err := query.Suffix("RETURNING *").ToSql()
	if err != nil {
		return nil, fmt.Errorf("unable to build rule query: %w", err)
	}

	rows, _ := tx.Query(context.TODO(), sql, args...)
	createdRules, err := pgx.CollectRows(rows, pgx.RowToStructByName[dbScenarioIterationRule])
	if err != nil {
		return nil, fmt.Errorf("unable to create rules: %w", err)
	}

	rulesDTOs := make([]app.Rule, len(createdRules))
	for i, createdRule := range createdRules {
		rulesDTOs[i], err = createdRule.dto()
		if err != nil {
			return nil, fmt.Errorf("dto issue: %w", err)
		}
	}
	return rulesDTOs, err
}
