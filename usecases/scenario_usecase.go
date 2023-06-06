package usecases

import (
	"context"
	"marble/marble-backend/app"
	"marble/marble-backend/repositories"
)

type ScenarioUsecase struct {
	scenarioReadRepository  repositories.ScenarioReadRepository
	scenarioWriteRepository repositories.ScenarioWriteRepository
}

func (usecase *ScenarioUsecase) ListScenarios(ctx context.Context, organizationID string) ([]app.Scenario, error) {
	return usecase.scenarioReadRepository.ListScenarios(ctx, organizationID)
}

func (usecase *ScenarioUsecase) GetScenario(ctx context.Context, organizationID string, scenarioID string) (app.Scenario, error) {
	return usecase.scenarioReadRepository.GetScenario(ctx, organizationID, scenarioID)
}

func (usecase *ScenarioUsecase) UpdateScenario(ctx context.Context, organizationID string, scenario app.UpdateScenarioInput) (app.Scenario, error) {
	return usecase.scenarioWriteRepository.UpdateScenario(ctx, organizationID, scenario)
}

func (usecase *ScenarioUsecase) CreateScenario(ctx context.Context, organizationID string, scenario app.CreateScenarioInput) (app.Scenario, error) {
	return usecase.scenarioWriteRepository.CreateScenario(ctx, organizationID, scenario)
}
