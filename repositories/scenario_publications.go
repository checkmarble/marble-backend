package repositories

import (
	"context"
	"marble/marble-backend/models"
)

type ScenarioPublicationRepository interface {
	ListScenarioPublications(ctx context.Context, orgID string, filters models.ListScenarioPublicationsFilters) ([]models.ScenarioPublication, error)
	CreateScenarioPublication(ctx context.Context, orgID string, sp models.CreateScenarioPublicationInput, scenarioType models.ScenarioType) ([]models.ScenarioPublication, error)
	GetScenarioPublication(ctx context.Context, orgID string, scenarioPublicationID string) (models.ScenarioPublication, error)
}
