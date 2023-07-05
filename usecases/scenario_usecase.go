package usecases

import (
	"context"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/security"
)

type ScenarioUsecase struct {
	OrganizationIdOfContext string
	enforceSecurity         security.EnforceSecurity
	scenarioReadRepository  repositories.ScenarioReadRepository
	scenarioWriteRepository repositories.ScenarioWriteRepository
}

func (usecase *ScenarioUsecase) ListScenarios() ([]models.Scenario, error) {

	if err := usecase.enforceSecurity.ListScenarios(usecase.OrganizationIdOfContext); err != nil {
		return nil, err
	}
	return usecase.scenarioReadRepository.ListScenariosOfOrganization(nil, usecase.OrganizationIdOfContext)
}

func (usecase *ScenarioUsecase) GetScenario(scenarioID string) (models.Scenario, error) {

	scenario, err := usecase.scenarioReadRepository.GetScenarioById(nil, scenarioID)
	if err != nil {
		return models.Scenario{}, err
	}
	if err := usecase.enforceSecurity.ReadScenario(scenario); err != nil {
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
