package repositories

import (
	"context"
	"marble/marble-backend/app"
)

type ScenarioPublicationRepository interface {
	ListScenarioPublications(ctx context.Context, orgID string, filters app.ListScenarioPublicationsFilters) ([]app.ScenarioPublication, error)
	CreateScenarioPublication(ctx context.Context, orgID string, sp app.CreateScenarioPublicationInput) ([]app.ScenarioPublication, error)
	GetScenarioPublication(ctx context.Context, orgID string, scenarioPublicationID string) (app.ScenarioPublication, error)
}
