package pg_repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"marble/marble-backend/app"
	"marble/marble-backend/app/operators"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type dbScenarioIterationRule struct {
	ID                  string      `db:"id"`
	OrgID               string      `db:"org_id"`
	ScenarioIterationID string      `db:"scenario_iteration_id"`
	DisplayOrder        int         `db:"display_order"`
	Name                string      `db:"name"`
	Description         string      `db:"description"`
	ScoreModifier       int         `db:"score_modifier"`
	Formula             []byte      `db:"formula"`
	CreatedAt           time.Time   `db:"created_at"`
	DeletedAt           pgtype.Time `db:"deleted_at"`
}

func (sir *dbScenarioIterationRule) dto() (app.Rule, error) {
	formula, err := operators.UnmarshalOperatorBool(sir.Formula)
	if err != nil {
		return app.Rule{}, fmt.Errorf("unable to unmarshal rule: %w", err)
	}

	return app.Rule{
		ID:                  sir.ID,
		ScenarioIterationID: sir.ScenarioIterationID,
		DisplayOrder:        sir.DisplayOrder,
		Name:                sir.Name,
		Description:         sir.Description,
		Formula:             formula,
		ScoreModifier:       sir.ScoreModifier,
		CreatedAt:           sir.CreatedAt,
	}, nil
}

func (r *PGRepository) GetScenarioIterationRule(ctx context.Context, orgID string, ruleID string) (app.Rule, error) {
	sql, args, err := r.queryBuilder.
		Select(columnList[dbScenarioIterationRule]()...).
		From("scenario_iteration_rules").
		Where("org_id = ?", orgID).
		Where("id= ?", ruleID).
		ToSql()
	if err != nil {
		return app.Rule{}, fmt.Errorf("unable to build rule query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	rule, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbScenarioIterationRule])
	if errors.Is(err, pgx.ErrNoRows) {
		return app.Rule{}, app.ErrNotFoundInRepository
	} else if err != nil {
		return app.Rule{}, fmt.Errorf("unable to get rule: %w", err)
	}

	ruleDTO, err := rule.dto()
	if err != nil {
		return app.Rule{}, fmt.Errorf("dto issue: %w", err)
	}

	return ruleDTO, err
}

type ListScenarioIterationRulesFilters struct {
	ScenarioIterationID *string `db:"scenario_iteration_id"`
}

func (r *PGRepository) ListScenarioIterationRules(ctx context.Context, orgID string, filters app.GetScenarioIterationRulesFilters) ([]app.Rule, error) {
	sql, args, err := r.queryBuilder.
		Select(columnList[dbScenarioIterationRule]()...).
		From("scenario_iteration_rules").
		Where("org_id = ?", orgID).
		Where(squirrel.Eq(columnValueMap(ListScenarioIterationRulesFilters{
			ScenarioIterationID: filters.ScenarioIterationID,
		}))).
		ToSql()
	if err != nil {
		return nil, fmt.Errorf("unable to build rule query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	rules, err := pgx.CollectRows(rows, pgx.RowToStructByName[dbScenarioIterationRule])
	if err != nil {
		return nil, fmt.Errorf("unable to get rules: %w", err)
	}

	var ruleDTOs []app.Rule
	for _, rule := range rules {
		ruleDTO, err := rule.dto()
		if err != nil {
			return nil, fmt.Errorf("dto issue: %w", err)
		}
		ruleDTOs = append(ruleDTOs, ruleDTO)
	}

	return ruleDTOs, err
}

type dbCreateScenarioIterationRuleInput struct {
	OrgID               string `db:"org_id"`
	ScenarioIterationID string `db:"scenario_iteration_id"`
	DisplayOrder        int    `db:"display_order"`
	Name                string `db:"name"`
	Description         string `db:"description"`
	ScoreModifier       int    `db:"score_modifier"`
	Formula             []byte `db:"formula"`
}

func (r *PGRepository) CreateScenarioIterationRule(ctx context.Context, orgID string, rule app.CreateRuleInput) (app.Rule, error) {
	dbCreateRuleInput := dbCreateScenarioIterationRuleInput{
		OrgID:               orgID,
		ScenarioIterationID: rule.ScenarioIterationID,
		DisplayOrder:        rule.DisplayOrder,
		Name:                rule.Name,
		Description:         rule.Description,
		ScoreModifier:       rule.ScoreModifier,
	}
	formulaBytes, err := rule.Formula.MarshalJSON()
	if err != nil {
		return app.Rule{}, fmt.Errorf("unable to marshal rule formula: %w", err)
	}
	dbCreateRuleInput.Formula = formulaBytes

	sql, args, err := r.queryBuilder.
		Insert("scenario_iteration_rules").
		SetMap(columnValueMap(dbCreateRuleInput)).
		Suffix("RETURNING *").ToSql()
	if err != nil {
		return app.Rule{}, fmt.Errorf("unable to build rule query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
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

func (r *PGRepository) createScenarioIterationRules(ctx context.Context, tx pgx.Tx, orgID string, scenarioIterationID string, rules []app.CreateRuleInput) ([]app.Rule, error) {
	if len(rules) == 0 {
		return nil, nil
	}
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

	rows, _ := tx.Query(ctx, sql, args...)
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

type dbUpdateScenarioIterationRuleInput struct {
	ID            string  `db:"id"`
	DisplayOrder  *int    `db:"display_order"`
	Name          *string `db:"name"`
	Description   *string `db:"description"`
	ScoreModifier *int    `db:"score_modifier"`
	Formula       *[]byte `db:"formula"`
}

func (r *PGRepository) UpdateScenarioIterationRule(ctx context.Context, orgID string, rule app.UpdateRuleInput) (app.Rule, error) {
	dbUpdateRuleInput := dbUpdateScenarioIterationRuleInput{
		ID:            rule.ID,
		DisplayOrder:  rule.DisplayOrder,
		Name:          rule.Name,
		Description:   rule.Description,
		ScoreModifier: rule.ScoreModifier,
	}
	if rule.Formula != nil {
		formulaBytes, err := json.Marshal(rule.Formula)
		if err != nil {
			return app.Rule{}, fmt.Errorf("unable to marshal rule formula: %w", err)
		}
		dbUpdateRuleInput.Formula = &formulaBytes
	}

	sql, args, err := r.queryBuilder.
		Update("scenario_iteration_rules").
		SetMap(columnValueMap(dbUpdateRuleInput)).
		Where("id = ?", rule.ID).
		Where("org_id = ?", orgID).
		Suffix("RETURNING *").ToSql()
	if err != nil {
		return app.Rule{}, fmt.Errorf("unable to build scenario query: %w", err)
	}

	rows, _ := r.db.Query(ctx, sql, args...)
	updatedRule, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbScenarioIterationRule])
	if errors.Is(err, pgx.ErrNoRows) {
		return app.Rule{}, app.ErrNotFoundInRepository
	} else if err != nil {
		return app.Rule{}, fmt.Errorf("unable to update rule(id: %s): %w", rule.ID, err)
	}

	ruleDTO, err := updatedRule.dto()
	if err != nil {
		return app.Rule{}, fmt.Errorf("dto issue: %w", err)
	}

	return ruleDTO, err
}
