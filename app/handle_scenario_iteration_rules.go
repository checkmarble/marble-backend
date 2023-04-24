package app

import "context"

type RepositoryScenarioItertionRuleInterface interface {
	ListScenarioIterationRules(ctx context.Context, orgID string, filters GetScenarioIterationRulesFilters) ([]Rule, error)
	CreateScenarioIterationRule(ctx context.Context, orgID string, rule CreateRuleInput) (Rule, error)
	GetScenarioIterationRule(ctx context.Context, orgID string, ruleID string) (Rule, error)
	UpdateScenarioIterationRule(ctx context.Context, orgID string, rule UpdateRuleInput) (Rule, error)
}

func (app *App) ListScenarioIterationRules(ctx context.Context, organizationID string, filters GetScenarioIterationRulesFilters) ([]Rule, error) {
	return app.repository.ListScenarioIterationRules(ctx, organizationID, filters)
}

func (app *App) CreateScenarioIterationRule(ctx context.Context, organizationID string, rule CreateRuleInput) (Rule, error) {
	return app.repository.CreateScenarioIterationRule(ctx, organizationID, rule)
}

func (app *App) GetScenarioIterationRule(ctx context.Context, organizationID string, ruleID string) (Rule, error) {
	return app.repository.GetScenarioIterationRule(ctx, organizationID, ruleID)
}

func (app *App) UpdateScenarioIterationRule(ctx context.Context, organizationID string, rule UpdateRuleInput) (Rule, error) {
	return app.repository.UpdateScenarioIterationRule(ctx, organizationID, rule)
}
