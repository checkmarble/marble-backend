package app

import "context"

func (a *App) GetScenarios(ctx context.Context, organizationID string) ([]Scenario, error) {
	return a.repository.GetScenarios(ctx, organizationID)
}

func (a *App) CreateScenario(ctx context.Context, organizationID string, scenario CreateScenarioInput) (Scenario, error) {
	return a.repository.PostScenario(ctx, organizationID, scenario)
}

func (a *App) GetScenario(ctx context.Context, organizationID string, scenarioID string) (Scenario, error) {
	return a.repository.GetScenario(ctx, organizationID, scenarioID)
}

func (a *App) UpdateScenario(ctx context.Context, organizationID string, scenario UpdateScenarioInput) (Scenario, error) {
	return a.repository.UpdateScenario(ctx, organizationID, scenario)
}
