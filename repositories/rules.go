package repositories

import (
	"context"
	"encoding/json"
	"marble/marble-backend/dto"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/repositories/dbmodels"
)

type ScenarioIterationRuleRepositoryLegacy interface {
	ListRules(ctx context.Context, organizationId string, filters models.GetRulesFilters) ([]models.Rule, error)
	CreateRule(ctx context.Context, organizationId string, rule models.CreateRuleInput) (models.Rule, error)
	GetRule(ctx context.Context, organizationId string, ruleID string) (models.Rule, error)
	UpdateRule(ctx context.Context, organizationId string, rule models.UpdateRuleInput) (models.Rule, error)
}

type RuleRepository interface {
	UpdateRuleWithAstExpression(tx Transaction, ruleId string, expression ast.Node) error
	DeleteRule(ctx context.Context, ruleID string) error
}

type RuleRepositoryPostgresql struct {
	transactionFactory TransactionFactory
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

func (repo *RuleRepositoryPostgresql) DeleteRule(ctx context.Context, ruleID string) error {
	pgTx := repo.transactionFactory.adaptMarbleDatabaseTransaction(nil)

	_, err := pgTx.ExecBuilder(NewQueryBuilder().Delete(dbmodels.TABLE_RULES).Where("id = ?", ruleID))
	return err
}
