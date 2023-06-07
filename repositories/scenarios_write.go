package repositories

import (
	"context"
	"marble/marble-backend/models"
)

type ScenarioWriteRepository interface {
	CreateScenario(ctx context.Context, orgID string, scenario models.CreateScenarioInput) (models.Scenario, error)
	UpdateScenario(ctx context.Context, orgID string, scenario models.UpdateScenarioInput) (models.Scenario, error)
}
