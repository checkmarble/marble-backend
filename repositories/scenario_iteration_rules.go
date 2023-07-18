package repositories

import (
	"context"
	"marble/marble-backend/models"
)

type ScenarioIterationRuleRepositoryLegacy interface {
	ListScenarioIterationRules(ctx context.Context, orgID string, filters models.GetScenarioIterationRulesFilters) ([]models.Rule, error)
	CreateScenarioIterationRule(ctx context.Context, orgID string, rule models.CreateRuleInput) (models.Rule, error)
	GetScenarioIterationRule(ctx context.Context, orgID string, ruleID string) (models.Rule, error)
	UpdateScenarioIterationRule(ctx context.Context, orgID string, rule models.UpdateRuleInput) (models.Rule, error)
}
