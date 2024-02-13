package repositories

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/Masterminds/squirrel"
)

func selectRules() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.SelectRulesColumn...).
		From(dbmodels.TABLE_RULES)
}

func (repo *MarbleDbRepository) GetRuleById(ctx context.Context, tx Transaction_deprec, ruleId string) (models.Rule, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	return SqlToModel(
		ctx,
		pgTx,
		selectRules().Where(squirrel.Eq{"id": ruleId}),
		dbmodels.AdaptRule,
	)
}

func (repo *MarbleDbRepository) ListRulesByIterationId(ctx context.Context, tx Transaction_deprec, iterationId string) ([]models.Rule, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	return SqlToListOfModels(
		ctx,
		pgTx,
		selectRules().
			Where(squirrel.Eq{"scenario_iteration_id": iterationId}).
			OrderBy("created_at DESC"),
		dbmodels.AdaptRule,
	)
}

func (repo *MarbleDbRepository) UpdateRule(ctx context.Context, tx Transaction_deprec, rule models.UpdateRuleInput) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	dbUpdateRuleInput, err := dbmodels.AdaptDBUpdateRuleInput(rule)
	if err != nil {
		return err
	}

	var updateRequest = NewQueryBuilder().
		Update(dbmodels.TABLE_RULES).
		SetMap(utils.ColumnValueMap(dbUpdateRuleInput)).
		Where("id = ?", rule.Id)

	_, err = pgTx.ExecBuilder(ctx, updateRequest)
	return err
}

func (repo *MarbleDbRepository) DeleteRule(ctx context.Context, tx Transaction_deprec, ruleID string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	_, err := pgTx.ExecBuilder(ctx, NewQueryBuilder().Delete(dbmodels.TABLE_RULES).Where("id = ?", ruleID))
	return err
}

func (repo *MarbleDbRepository) CreateRules(ctx context.Context, tx Transaction_deprec, rules []models.CreateRuleInput) ([]models.Rule, error) {
	if len(rules) == 0 {
		return []models.Rule{}, fmt.Errorf("no rule found")
	}

	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(ctx, tx)

	dbCreateRuleInputs, err := pure_utils.MapErr(rules, dbmodels.AdaptDBCreateRuleInput)
	if err != nil {
		return []models.Rule{}, err
	}

	query := NewQueryBuilder().
		Insert(dbmodels.TABLE_RULES).
		Columns(
			"id",
			"scenario_iteration_id",
			"org_id",
			"display_order",
			"name",
			"description",
			"formula_ast_expression",
			"score_modifier").
		Suffix("RETURNING *")

	for _, rule := range dbCreateRuleInputs {
		query = query.Values(
			rule.Id,
			rule.ScenarioIterationId,
			rule.OrganizationId,
			rule.DisplayOrder,
			rule.Name,
			rule.Description,
			rule.FormulaAstExpression,
			rule.ScoreModifier,
		)
	}

	return SqlToListOfModels(
		ctx,
		pgTx,
		query,
		dbmodels.AdaptRule,
	)
}

func (repo *MarbleDbRepository) CreateRule(ctx context.Context, tx Transaction_deprec, rule models.CreateRuleInput) (models.Rule, error) {
	rules, err := repo.CreateRules(ctx, tx, []models.CreateRuleInput{rule})
	if err != nil {
		return models.Rule{}, err
	}
	return rules[0], nil
}
