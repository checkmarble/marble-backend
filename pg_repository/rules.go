package pg_repository

import (
	"context"
	"errors"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories/dbmodels"
	"marble/marble-backend/utils"

	"github.com/jackc/pgx/v5"
)

type dbCreateScenarioIterationRuleInput struct {
	Id                   string  `db:"id"`
	OrganizationId       string  `db:"org_id"`
	ScenarioIterationId  string  `db:"scenario_iteration_id"`
	DisplayOrder         int     `db:"display_order"`
	Name                 string  `db:"name"`
	Description          string  `db:"description"`
	ScoreModifier        int     `db:"score_modifier"`
	FormulaAstExpression *[]byte `db:"formula_ast_expression"`
}

func (r *PGRepository) CreateRule(ctx context.Context, organizationId string, rule models.CreateRuleInput) (models.Rule, error) {
	dbCreateRuleInput := dbCreateScenarioIterationRuleInput{
		Id:                  utils.NewPrimaryKey(organizationId),
		OrganizationId:      organizationId,
		ScenarioIterationId: rule.ScenarioIterationId,
		DisplayOrder:        rule.DisplayOrder,
		Name:                rule.Name,
		Description:         rule.Description,
		ScoreModifier:       rule.ScoreModifier,
	}

	var err error
	dbCreateRuleInput.FormulaAstExpression, err = dbmodels.SerializeFormulaAstExpression(rule.FormulaAstExpression)
	if err != nil {
		return models.Rule{}, fmt.Errorf("unable to marshal expression formula: %w", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return models.Rule{}, fmt.Errorf("unable to start a transaction: %w", err)
	}
	defer tx.Rollback(ctx) // safe to call even if tx commits

	sql, args, err := r.queryBuilder.
		Select("version IS NULL").
		From("scenario_iterations").
		Where("id = ?", rule.ScenarioIterationId).
		Where("org_id = ?", organizationId).
		ToSql()
	if err != nil {
		return models.Rule{}, fmt.Errorf("unable to build scenario iteration rule query: %w", err)
	}

	var isDraft bool
	err = tx.QueryRow(ctx, sql, args...).Scan(&isDraft)
	if err != nil {
		return models.Rule{}, fmt.Errorf("unable to check if scenario iteration is draft: %w", err)
	}
	if !isDraft {
		return models.Rule{}, models.ErrScenarioIterationNotDraft
	}

	sql, args, err = r.queryBuilder.
		Insert("scenario_iteration_rules").
		SetMap(ColumnValueMap(dbCreateRuleInput)).
		Suffix("RETURNING *").ToSql()
	if err != nil {
		return models.Rule{}, fmt.Errorf("unable to build rule query: %w", err)
	}

	rows, _ := tx.Query(ctx, sql, args...)
	createdRule, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbmodels.DBRule])
	if err != nil {
		return models.Rule{}, fmt.Errorf("unable to create rule: %w", err)
	}

	ruleDTO, err := dbmodels.AdaptRule(createdRule)
	if err != nil {
		return models.Rule{}, fmt.Errorf("dto issue: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return models.Rule{}, fmt.Errorf("transaction issue: %w", err)
	}

	return ruleDTO, err
}

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

type dbUpdateScenarioIterationRuleInput struct {
	Id                   string  `db:"id"`
	DisplayOrder         *int    `db:"display_order"`
	Name                 *string `db:"name"`
	Description          *string `db:"description"`
	ScoreModifier        *int    `db:"score_modifier"`
	FormulaAstExpression *[]byte `db:"formula_ast_expression"`
}

func (r *PGRepository) UpdateRule(ctx context.Context, organizationId string, rule models.UpdateRuleInput) (models.Rule, error) {
	dbUpdateRuleInput := dbUpdateScenarioIterationRuleInput{
		Id:            rule.Id,
		DisplayOrder:  rule.DisplayOrder,
		Name:          rule.Name,
		Description:   rule.Description,
		ScoreModifier: rule.ScoreModifier,
	}

	var err error
	dbUpdateRuleInput.FormulaAstExpression, err = dbmodels.SerializeFormulaAstExpression(rule.FormulaAstExpression)
	if err != nil {
		return models.Rule{}, fmt.Errorf("unable to marshal rule formula ast expression: %w", err)
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return models.Rule{}, fmt.Errorf("unable to start a transaction: %w", err)
	}
	defer tx.Rollback(ctx) // safe to call even if tx commits

	sql, args, err := r.queryBuilder.
		Select("si.version IS NULL").
		From("scenario_iteration_rules sir").
		Join("scenario_iterations si on si.id = sir.scenario_iteration_id").
		Where("sir.id = ?", rule.Id).
		Where("sir.org_id = ?", organizationId).
		ToSql()
	if err != nil {
		return models.Rule{}, fmt.Errorf("unable to build scenario iteration rule query: %w", err)
	}

	var isDraft bool
	err = tx.QueryRow(ctx, sql, args...).Scan(&isDraft)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Rule{}, models.NotFoundInRepositoryError
	} else if err != nil {
		return models.Rule{}, fmt.Errorf("unable to check if scenario iteration is draft: %w", err)
	}
	if !isDraft {
		return models.Rule{}, models.ErrScenarioIterationNotDraft
	}

	sql, args, err = r.queryBuilder.
		Update("scenario_iteration_rules").
		SetMap(ColumnValueMap(dbUpdateRuleInput)).
		Where("id = ?", rule.Id).
		Where("org_id = ?", organizationId).
		Suffix("RETURNING *").ToSql()
	if err != nil {
		return models.Rule{}, fmt.Errorf("unable to build scenario iteration rule query: %w", err)
	}

	rows, _ := tx.Query(ctx, sql, args...)
	updatedRule, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[dbmodels.DBRule])
	if errors.Is(err, pgx.ErrNoRows) {
		return models.Rule{}, models.NotFoundInRepositoryError
	} else if err != nil {
		return models.Rule{}, fmt.Errorf("unable to update rule(id: %s): %w", rule.Id, err)
	}

	ruleDTO, err := dbmodels.AdaptRule(updatedRule)
	if err != nil {
		return models.Rule{}, fmt.Errorf("dto issue: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return models.Rule{}, fmt.Errorf("transaction issue: %w", err)
	}

	return ruleDTO, err
}
