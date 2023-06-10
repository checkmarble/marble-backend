package usecases

import (
	"context"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
)

type ScenarioUsecase struct {
	scenarioReadRepository  repositories.ScenarioReadRepository
	scenarioWriteRepository repositories.ScenarioWriteRepository
}

func (usecase *ScenarioUsecase) ListScenarios(ctx context.Context, organizationID string, filters models.ListScenariosFilters) ([]models.Scenario, error) {
	return usecase.scenarioReadRepository.ListScenarios(ctx, organizationID, filters)
}

func (usecase *ScenarioUsecase) GetScenario(ctx context.Context, organizationID string, scenarioID string) (models.Scenario, error) {
	return usecase.scenarioReadRepository.GetScenario(ctx, organizationID, scenarioID)
}

func (usecase *ScenarioUsecase) UpdateScenario(ctx context.Context, organizationID string, scenario models.UpdateScenarioInput) (models.Scenario, error) {
	return usecase.scenarioWriteRepository.UpdateScenario(ctx, organizationID, scenario)
}

func (usecase *ScenarioUsecase) CreateScenario(ctx context.Context, organizationID string, scenario models.CreateScenarioInput) (models.Scenario, error) {
	return usecase.scenarioWriteRepository.CreateScenario(ctx, organizationID, scenario)
}
