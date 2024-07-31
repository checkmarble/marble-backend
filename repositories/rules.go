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

func (repo *MarbleDbRepository) GetRuleById(ctx context.Context, exec Executor, ruleId string) (models.Rule, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return models.Rule{}, err
	}

	return SqlToModel(
		ctx,
		exec,
		selectRules().Where(squirrel.Eq{"id": ruleId}),
		dbmodels.AdaptRule,
	)
}

func (repo *MarbleDbRepository) ListRulesByIterationId(ctx context.Context, exec Executor, iterationId string) ([]models.Rule, error) {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

	return SqlToListOfModels(
		ctx,
		exec,
		selectRules().
			Where(squirrel.Eq{"scenario_iteration_id": iterationId}).
			OrderBy("created_at DESC"),
		dbmodels.AdaptRule,
	)
}

func (repo *MarbleDbRepository) UpdateRule(ctx context.Context, exec Executor, rule models.UpdateRuleInput) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	dbUpdateRuleInput, err := dbmodels.AdaptDBUpdateRuleInput(rule)
	if err != nil {
		return err
	}

	updateRequest := NewQueryBuilder().
		Update(dbmodels.TABLE_RULES).
		SetMap(utils.ColumnValueMap(dbUpdateRuleInput)).
		Where("id = ?", rule.Id)

	err = ExecBuilder(ctx, exec, updateRequest)
	return err
}

func (repo *MarbleDbRepository) DeleteRule(ctx context.Context, exec Executor, ruleID string) error {
	if err := validateMarbleDbExecutor(exec); err != nil {
		return err
	}

	err := ExecBuilder(ctx, exec, NewQueryBuilder().Delete(dbmodels.TABLE_RULES).Where("id = ?", ruleID))
	return err
}

func (repo *MarbleDbRepository) CreateRules(ctx context.Context, exec Executor, rules []models.CreateRuleInput) ([]models.Rule, error) {
	if len(rules) == 0 {
		return []models.Rule{}, fmt.Errorf("no rule found")
	}

	if err := validateMarbleDbExecutor(exec); err != nil {
		return nil, err
	}

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
			"score_modifier",
			"rule_group",
			"snooze_group_id",
		).
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
			rule.RuleGroup,
			rule.SnoozeGroupId,
		)
	}

	return SqlToListOfModels(
		ctx,
		exec,
		query,
		dbmodels.AdaptRule,
	)
}

func (repo *MarbleDbRepository) CreateRule(ctx context.Context, exec Executor, rule models.CreateRuleInput) (models.Rule, error) {
	rules, err := repo.CreateRules(ctx, exec, []models.CreateRuleInput{rule})
	if err != nil {
		return models.Rule{}, err
	}
	return rules[0], nil
}
