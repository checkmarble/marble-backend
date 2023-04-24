package app

import (
	"context"
)

type RepositoryScenarioPublicationInterface interface {
	ListScenarioPublications(ctx context.Context, orgID string, filters ListScenarioPublicationsFilters) ([]ScenarioPublication, error)
	CreateScenarioPublication(ctx context.Context, orgID string, sp CreateScenarioPublicationInput) ([]ScenarioPublication, error)
	GetScenarioPublication(ctx context.Context, orgID string, scenarioPublicationID string) (ScenarioPublication, error)
}

func (app *App) ListScenarioPublications(ctx context.Context, orgID string, filters ListScenarioPublicationsFilters) ([]ScenarioPublication, error) {
	return app.repository.ListScenarioPublications(ctx, orgID, filters)
}

func (app *App) CreateScenarioPublication(ctx context.Context, orgID string, sp CreateScenarioPublicationInput) ([]ScenarioPublication, error) {
	return app.repository.CreateScenarioPublication(ctx, orgID, sp)
}

func (app *App) GetScenarioPublication(ctx context.Context, orgID string, scenarioPublicationID string) (ScenarioPublication, error) {
	return app.repository.GetScenarioPublication(ctx, orgID, scenarioPublicationID)
}
