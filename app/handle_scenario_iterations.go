package app

import "context"

type RepositoryScenarioItertionInterface interface {
	ListScenarioIterations(ctx context.Context, orgID string, filters GetScenarioIterationFilters) ([]ScenarioIteration, error)
	CreateScenarioIteration(ctx context.Context, orgID string, scenarioIteration CreateScenarioIterationInput) (ScenarioIteration, error)
	GetScenarioIteration(ctx context.Context, orgID string, scenarioIterationID string) (ScenarioIteration, error)
	UpdateScenarioIteration(ctx context.Context, orgID string, scenarioIteration UpdateScenarioIterationInput) (ScenarioIteration, error)
}

func (app *App) ListScenarioIterations(ctx context.Context, organizationID string, filters GetScenarioIterationFilters) ([]ScenarioIteration, error) {
	return app.repository.ListScenarioIterations(ctx, organizationID, filters)
}

func (app *App) CreateScenarioIteration(ctx context.Context, organizationID string, scenarioIteration CreateScenarioIterationInput) (ScenarioIteration, error) {
	return app.repository.CreateScenarioIteration(ctx, organizationID, scenarioIteration)
}

func (app *App) GetScenarioIteration(ctx context.Context, organizationID string, scenarioIterationID string) (ScenarioIteration, error) {
	return app.repository.GetScenarioIteration(ctx, organizationID, scenarioIterationID)
}

func (app *App) UpdateScenarioIteration(ctx context.Context, organizationID string, scenarioIteration UpdateScenarioIterationInput) (ScenarioIteration, error) {
	return app.repository.UpdateScenarioIteration(ctx, organizationID, scenarioIteration)
}
