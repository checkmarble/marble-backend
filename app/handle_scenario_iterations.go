package app

import "context"

func (a *App) GetScenarioIterations(ctx context.Context, organizationID string, scenarioID string) ([]ScenarioIteration, error) {
	return a.repository.GetScenarioIterations(ctx, organizationID, scenarioID)
}

func (a *App) CreateScenarioIteration(ctx context.Context, organizationID string, scenarioIteration CreateScenarioIterationInput) (ScenarioIteration, error) {
	return a.repository.CreateScenarioIteration(ctx, organizationID, scenarioIteration)
}

func (a *App) GetScenarioIteration(ctx context.Context, organizationID string, scenarioIterationID string) (ScenarioIteration, error) {
	return a.repository.GetScenarioIteration(ctx, organizationID, scenarioIterationID)
}

func (a *App) UpdateScenarioIteration(ctx context.Context, organizationID string, scenarioIteration UpdateScenarioIterationInput) (ScenarioIteration, error) {
	return a.repository.UpdateScenarioIteration(ctx, organizationID, scenarioIteration)
}
