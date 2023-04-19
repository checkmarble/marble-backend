package app

import (
	"context"
)

type RepositoryScenarioPublicationInterface interface {
	ReadScenarioPublications(ctx context.Context, orgID string, scenarioID string) ([]ScenarioPublication, error)
	ReadScenarioIterationPublications(ctx context.Context, orgID string, scenarioIterationID string) ([]ScenarioPublication, error)
	ReadScenarioPublication(ctx context.Context, orgID string, scenarioPublicationID string) (ScenarioPublication, error)
	CreateScenarioPublication(ctx context.Context, orgID string, sp CreateScenarioPublication) ([]ScenarioPublication, error)
}

func (app *App) GetScenarioPublications(ctx context.Context, orgID string, scenarioID string) ([]ScenarioPublication, error) {
	return app.repository.ReadScenarioPublications(ctx, orgID, scenarioID)
}

func (app *App) GetScenarioIterationPublications(ctx context.Context, orgID string, scenarioIterationID string) ([]ScenarioPublication, error) {
	return app.repository.ReadScenarioIterationPublications(ctx, orgID, scenarioIterationID)
}

func (app *App) GetScenarioPublication(ctx context.Context, orgID string, scenarioPublicationID string) (ScenarioPublication, error) {
	return app.repository.ReadScenarioPublication(ctx, orgID, scenarioPublicationID)
}

func (app *App) CreateScenarioPublication(ctx context.Context, orgID string, sp CreateScenarioPublication) ([]ScenarioPublication, error) {
	return app.repository.CreateScenarioPublication(ctx, orgID, sp)
}
