package app

import "context"

func (app *App) GetScenarioIterationRules(ctx context.Context, organizationID string, filters GetScenarioIterationRulesFilters) ([]Rule, error) {
	return app.repository.GetScenarioIterationRules(ctx, organizationID, filters)
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
