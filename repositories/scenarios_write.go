package repositories

import (
	"context"
	"marble/marble-backend/app"
)

type ScenarioWriteRepository interface {
	CreateScenario(ctx context.Context, orgID string, scenario app.CreateScenarioInput) (app.Scenario, error)
	UpdateScenario(ctx context.Context, orgID string, scenario app.UpdateScenarioInput) (app.Scenario, error)
}
