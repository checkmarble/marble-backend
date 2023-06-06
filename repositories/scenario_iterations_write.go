package repositories

import (
	"context"
	"marble/marble-backend/app"
)

type ScenarioIterationWriteRepository interface {
	CreateScenarioIteration(ctx context.Context, orgID string, scenarioIteration app.CreateScenarioIterationInput) (app.ScenarioIteration, error)
	UpdateScenarioIteration(ctx context.Context, orgID string, scenarioIteration app.UpdateScenarioIterationInput) (app.ScenarioIteration, error)
}
