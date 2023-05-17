package app

import "context"

type RepositoryScenarios interface {
	ListScenarios(ctx context.Context, orgID string) ([]Scenario, error)
	CreateScenario(ctx context.Context, orgID string, scenario CreateScenarioInput) (Scenario, error)
	GetScenario(ctx context.Context, orgID string, scenarioID string) (Scenario, error)
	UpdateScenario(ctx context.Context, orgID string, scenario UpdateScenarioInput) (Scenario, error)
}

func (app *App) ListScenarios(ctx context.Context, organizationID string) ([]Scenario, error) {
	return app.repository.ListScenarios(ctx, organizationID)
}

func (app *App) CreateScenario(ctx context.Context, organizationID string, scenario CreateScenarioInput) (Scenario, error) {
	return app.repository.CreateScenario(ctx, organizationID, scenario)
}

func (app *App) GetScenario(ctx context.Context, organizationID string, scenarioID string) (Scenario, error) {
	return app.repository.GetScenario(ctx, organizationID, scenarioID)
}

func (app *App) UpdateScenario(ctx context.Context, organizationID string, scenario UpdateScenarioInput) (Scenario, error) {
	return app.repository.UpdateScenario(ctx, organizationID, scenario)
}
