package app

import "context"

func (a *App) GetScenarioIterationRules(ctx context.Context, organizationID string, scenarioIterationID string) ([]Rule, error) {
	return a.repository.GetScenarioIterationRules(ctx, organizationID, scenarioIterationID)
}

func (a *App) CreateScenarioIterationRule(ctx context.Context, organizationID string, rule CreateRuleInput) (Rule, error) {
	return a.repository.CreateScenarioIterationRule(ctx, organizationID, rule)
}

func (a *App) GetScenarioIterationRule(ctx context.Context, organizationID string, ruleID string) (Rule, error) {
	return a.repository.GetScenarioIterationRule(ctx, organizationID, ruleID)
}

func (a *App) UpdateScenarioIterationRule(ctx context.Context, organizationID string, rule UpdateRuleInput) (Rule, error) {
	return a.repository.UpdateScenarioIterationRule(ctx, organizationID, rule)
}
