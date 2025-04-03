package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
)

type EvalScenarioRepository interface {
	GetScenarioIteration(ctx context.Context, exec Executor, scenarioIterationId string) (models.ScenarioIteration, error)
}

type EvalSanctionCheckConfigRepository interface {
	GetSanctionCheckConfig(ctx context.Context, exec Executor, scenarioIterationId string) (*models.SanctionCheckConfig, error)
}
