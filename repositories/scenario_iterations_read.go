package repositories

import (
	"context"
	"marble/marble-backend/app"
)

type ScenarioIterationReadRepository interface {
	GetScenarioIteration(ctx context.Context, orgID string, scenarioIterationID string) (app.ScenarioIteration, error)
	ListScenarioIterations(ctx context.Context, orgID string, filters app.GetScenarioIterationFilters) ([]app.ScenarioIteration, error)
}
