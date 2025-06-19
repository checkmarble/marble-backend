package repositories

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
)

type EvalScenarioRepository interface {
	GetScenarioIteration(ctx context.Context, exec Executor, scenarioIterationId string) (models.ScenarioIteration, error)
}

type EvalScreeningConfigRepository interface {
	ListScreeningConfigs(ctx context.Context, exec Executor, scenarioIterationId string) ([]models.ScreeningConfig, error)
}
