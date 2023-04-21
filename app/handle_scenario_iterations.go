package app

import "context"

func (app *App) GetScenarioIterations(ctx context.Context, organizationID string, filters GetScenarioIterationFilters) ([]ScenarioIteration, error) {
	return app.repository.GetScenarioIterations(ctx, organizationID, filters)
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
