package repositories

import (
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories/dbmodels"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/Masterminds/squirrel"
)

type RuleRepository interface {
	GetRuleById(tx Transaction, ruleId string) (models.Rule, error)
	ListRulesByIterationId(tx Transaction, iterationId string) ([]models.Rule, error)
	UpdateRule(tx Transaction, rule models.UpdateRuleInput) error
	DeleteRule(tx Transaction, ruleID string) error
	CreateRules(tx Transaction, rules []models.CreateRuleInput) ([]models.Rule, error)
	CreateRule(tx Transaction, rule models.CreateRuleInput) (models.Rule, error)
}

type RuleRepositoryPostgresql struct {
	transactionFactory TransactionFactoryPosgresql
}

func selectRules() squirrel.SelectBuilder {
	return NewQueryBuilder().
		Select(dbmodels.SelectRulesColumn...).
		From(dbmodels.TABLE_RULES)
}

func (repo *RuleRepositoryPostgresql) GetRuleById(tx Transaction, ruleId string) (models.Rule, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToModelAdapterWithErr(
		pgTx,
		selectRules().Where(squirrel.Eq{"id": ruleId}),
		dbmodels.AdaptRule,
	)
}

func (repo *RuleRepositoryPostgresql) ListRulesByIterationId(tx Transaction, iterationId string) ([]models.Rule, error) {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	return SqlToListOfModelsAdapterWithErr(
		pgTx,
		selectRules().Where(squirrel.Eq{"scenario_iteration_id": iterationId}),
		dbmodels.AdaptRule,
	)
}

func (repo *RuleRepositoryPostgresql) UpdateRule(tx Transaction, rule models.UpdateRuleInput) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	dbUpdateRuleInput, err := dbmodels.AdaptDBUpdateRuleInput(rule)
	if err != nil {
		return err
	}

	var updateRequest = NewQueryBuilder().
		Update(dbmodels.TABLE_RULES).
		SetMap(utils.ColumnValueMap(dbUpdateRuleInput)).
		Where("id = ?", rule.Id)

	_, err = pgTx.ExecBuilder(updateRequest)
	return err
}

func (repo *RuleRepositoryPostgresql) DeleteRule(tx Transaction, ruleID string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(NewQueryBuilder().Delete(dbmodels.TABLE_RULES).Where("id = ?", ruleID))
	return err
}

func (repo *RuleRepositoryPostgresql) CreateRules(tx Transaction, rules []models.CreateRuleInput) ([]models.Rule, error) {
	if len(rules) == 0 {
		return []models.Rule{}, fmt.Errorf("No rule found")
	}

	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	dbCreateRuleInputs, err := utils.MapErr(rules, dbmodels.AdaptDBCreateRuleInput)
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

	return SqlToListOfModelsAdapterWithErr(
		pgTx,
		query,
		dbmodels.AdaptRule,
	)
}

func (repo *RuleRepositoryPostgresql) CreateRule(tx Transaction, rule models.CreateRuleInput) (models.Rule, error) {
	rules, err := repo.CreateRules(tx, []models.CreateRuleInput{rule})
	if err != nil {
		return models.Rule{}, err
	}
	return rules[0], nil
}
