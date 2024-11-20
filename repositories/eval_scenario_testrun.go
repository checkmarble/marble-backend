package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
)

type EvalScenarioRepository interface {
	GetScenarioIteration(ctx context.Context, exec Executor, scenarioIterationId string) (models.ScenarioIteration, error)
}

type EvalTestRunScenarioRepository interface {
	GetTestRunIterationByScenarioId(ctx context.Context, exec Executor, scenarioID string) (*models.ScenarioIteration, error)
}
