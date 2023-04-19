package app

import "context"

func (app *App) GetScenarios(ctx context.Context, organizationID string) ([]Scenario, error) {
	return app.repository.GetScenarios(ctx, organizationID)
}

func (app *App) CreateScenario(ctx context.Context, organizationID string, scenario CreateScenarioInput) (Scenario, error) {
	return app.repository.PostScenario(ctx, organizationID, scenario)
}

func (app *App) GetScenario(ctx context.Context, organizationID string, scenarioID string) (Scenario, error) {
	return app.repository.GetScenario(ctx, organizationID, scenarioID)
}

func (app *App) UpdateScenario(ctx context.Context, organizationID string, scenario UpdateScenarioInput) (Scenario, error) {
	return app.repository.UpdateScenario(ctx, organizationID, scenario)
}
