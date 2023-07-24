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
	ListScenarioIterationRules(ctx context.Context, orgID string, filters models.GetScenarioIterationRulesFilters) ([]models.Rule, error)
	CreateScenarioIterationRule(ctx context.Context, orgID string, rule models.CreateRuleInput) (models.Rule, error)
	GetScenarioIterationRule(ctx context.Context, orgID string, ruleID string) (models.Rule, error)
	UpdateScenarioIterationRule(ctx context.Context, orgID string, rule models.UpdateRuleInput) (models.Rule, error)
}

type RuleRepository interface {
	UpdateRuleWithAstExpression(tx Transaction, ruleId string, expression ast.Node) error
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
