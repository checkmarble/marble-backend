package app

import (
	"context"
)

type RepositoryScenarioPublicationInterface interface {
	ReadScenarioPublications(ctx context.Context, orgID string, filters ReadScenarioPublicationsFilters) ([]ScenarioPublication, error)
	CreateScenarioPublication(ctx context.Context, orgID string, sp CreateScenarioPublicationInput) ([]ScenarioPublication, error)
}

func (app *App) ReadScenarioPublications(ctx context.Context, orgID string, filters ReadScenarioPublicationsFilters) ([]ScenarioPublication, error) {
	return app.repository.ReadScenarioPublications(ctx, orgID, filters)
}

func (app *App) CreateScenarioPublication(ctx context.Context, orgID string, sp CreateScenarioPublicationInput) ([]ScenarioPublication, error) {
	return app.repository.CreateScenarioPublication(ctx, orgID, sp)
}
