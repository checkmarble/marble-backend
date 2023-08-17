package repositories

import (
	"context"
	"encoding/json"
	"marble/marble-backend/dto"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/repositories/dbmodels"

	"github.com/Masterminds/squirrel"
)

type ScenarioIterationRuleRepositoryLegacy interface {
	CreateRule(ctx context.Context, organizationId string, rule models.CreateRuleInput) (models.Rule, error)
	UpdateRule(ctx context.Context, organizationId string, rule models.UpdateRuleInput) (models.Rule, error)
}

type RuleRepository interface {
	GetRuleById(tx Transaction, ruleId string) (models.Rule, error)
	ListRulesByIterationId(tx Transaction, iterationId string) ([]models.Rule, error)
	UpdateRuleWithAstExpression(tx Transaction, ruleId string, expression ast.Node) error
	DeleteRule(tx Transaction, ruleID string) error
}

type RuleRepositoryPostgresql struct {
	transactionFactory TransactionFactory
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
		selectRules().Where(squirrel.Eq{"scenarioIterationId": iterationId}),
		dbmodels.AdaptRule,
	)
}

func (repo *RuleRepositoryPostgresql) UpdateRuleWithAstExpression(tx Transaction, ruleId string, expression ast.Node) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	nodeDto, err := dto.AdaptNodeDto(expression)
	if err != nil {
		return err
	}
	serializedExpression, err := json.Marshal(nodeDto)
	if err != nil {
		return err
	}

	var updateRequest = NewQueryBuilder().Update(dbmodels.TABLE_RULES)
	updateRequest = updateRequest.Set("formula_ast_expression", serializedExpression)
	updateRequest = updateRequest.Where("id = ?", ruleId)

	_, err = pgTx.ExecBuilder(updateRequest)
	return err
}

func (repo *RuleRepositoryPostgresql) DeleteRule(tx Transaction, ruleID string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(tx)

	_, err := pgTx.ExecBuilder(NewQueryBuilder().Delete(dbmodels.TABLE_RULES).Where("id = ?", ruleID))
	return err
}
