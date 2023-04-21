package app

import (
	"context"
)

type RepositoryScenarioPublicationInterface interface {
	ReadScenarioPublications(ctx context.Context, orgID string, filters ReadScenarioPublicationsFilters) ([]ScenarioPublication, error)
	CreateScenarioPublication(ctx context.Context, orgID string, sp CreateScenarioPublicationInput) ([]ScenarioPublication, error)
	ReadScenarioPublication(ctx context.Context, orgID string, scenarioPublicationID string) (ScenarioPublication, error)
}

func (app *App) ReadScenarioPublications(ctx context.Context, orgID string, filters ReadScenarioPublicationsFilters) ([]ScenarioPublication, error) {
	return app.repository.ReadScenarioPublications(ctx, orgID, filters)
}

func (app *App) CreateScenarioPublication(ctx context.Context, orgID string, sp CreateScenarioPublicationInput) ([]ScenarioPublication, error) {
	return app.repository.CreateScenarioPublication(ctx, orgID, sp)
}

func (app *App) ReadScenarioPublication(ctx context.Context, orgID string, scenarioPublicationID string) (ScenarioPublication, error) {
	return app.repository.ReadScenarioPublication(ctx, orgID, scenarioPublicationID)
}
