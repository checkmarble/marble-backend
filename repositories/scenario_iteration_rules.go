package repositories

import (
	"context"
	"marble/marble-backend/app"
)

type ScenarioIterationRuleRepository interface {
	ListScenarioIterationRules(ctx context.Context, orgID string, filters app.GetScenarioIterationRulesFilters) ([]app.Rule, error)
	CreateScenarioIterationRule(ctx context.Context, orgID string, rule app.CreateRuleInput) (app.Rule, error)
	GetScenarioIterationRule(ctx context.Context, orgID string, ruleID string) (app.Rule, error)
	UpdateScenarioIterationRule(ctx context.Context, orgID string, rule app.UpdateRuleInput) (app.Rule, error)
}
