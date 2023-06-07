package repositories

import (
	"context"
	"marble/marble-backend/models"
)

type ScenarioReadRepository interface {
	GetScenario(ctx context.Context, orgID string, scenarioID string) (models.Scenario, error)
	ListScenarios(ctx context.Context, orgID string) ([]models.Scenario, error)
}
