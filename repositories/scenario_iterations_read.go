package repositories

import (
	"context"
	"marble/marble-backend/models"
)

type ScenarioIterationReadRepository interface {
	GetScenarioIteration(ctx context.Context, orgID string, scenarioIterationID string) (models.ScenarioIteration, error)
	ListScenarioIterations(ctx context.Context, orgID string, filters models.GetScenarioIterationFilters) ([]models.ScenarioIteration, error)
}
