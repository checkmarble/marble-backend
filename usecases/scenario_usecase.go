package usecases

import (
	"context"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/utils"
)

type ScenarioUsecase struct {
	scenarioReadRepository  repositories.ScenarioReadRepository
	scenarioWriteRepository repositories.ScenarioWriteRepository
}

func (usecase *ScenarioUsecase) ListScenarios(ctx context.Context, organizationID string) ([]models.Scenario, error) {
	if err := utils.EnforceOrganizationAccess(utils.MustCredentialsFromCtx(ctx), organizationID); err != nil {
		return nil, err
	}
	return usecase.scenarioReadRepository.ListScenariosOfOrganization(nil, organizationID)
}

func (usecase *ScenarioUsecase) ListAllScenarios() ([]models.Scenario, error) {
	return usecase.scenarioReadRepository.ListAllScenarios(nil)
}

func (usecase *ScenarioUsecase) GetScenario(ctx context.Context, scenarioID string) (models.Scenario, error) {
	scenario, err := usecase.scenarioReadRepository.GetScenarioById(nil, scenarioID)
	if err != nil {
		return models.Scenario{}, err
	}
	if err := utils.EnforceOrganizationAccess(utils.MustCredentialsFromCtx(ctx), scenario.OrganizationID); err != nil {
		return models.Scenario{}, err
	}
	return scenario, nil
}

func (usecase *ScenarioUsecase) UpdateScenario(ctx context.Context, organizationID string, scenario models.UpdateScenarioInput) (models.Scenario, error) {
	return usecase.scenarioWriteRepository.UpdateScenario(ctx, organizationID, scenario)
}

func (usecase *ScenarioUsecase) CreateScenario(ctx context.Context, organizationID string, scenario models.CreateScenarioInput) (models.Scenario, error) {
	return usecase.scenarioWriteRepository.CreateScenario(ctx, organizationID, scenario)
}
