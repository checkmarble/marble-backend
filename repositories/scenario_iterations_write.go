package repositories

import (
	"context"
	"marble/marble-backend/models"
)

type ScenarioIterationWriteRepository interface {
	CreateScenarioIteration(ctx context.Context, organizationId string, scenarioIteration models.CreateScenarioIterationInput) (models.ScenarioIteration, error)
	UpdateScenarioIteration(ctx context.Context, organizationId string, scenarioIteration models.UpdateScenarioIterationInput) (models.ScenarioIteration, error)
}
