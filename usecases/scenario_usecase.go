package usecases

import (
	"context"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/security"
)

type ScenarioUsecase struct {
	OrganizationIdOfContext func() (string, error)
	enforceSecurity         security.EnforceSecurity
	scenarioReadRepository  repositories.ScenarioReadRepository
	scenarioWriteRepository repositories.ScenarioWriteRepository
}

func (usecase *ScenarioUsecase) ListScenarios() ([]models.Scenario, error) {

	if err := usecase.enforceReadScenarioPermission(); err != nil {
		return nil, err
	}
	organizationId, err := usecase.OrganizationIdOfContext()
	if err != nil {
		return nil, err
	}
	return usecase.scenarioReadRepository.ListScenariosOfOrganization(nil, organizationId)
}

func (usecase *ScenarioUsecase) GetScenario(scenarioID string) (models.Scenario, error) {

	if err := usecase.enforceReadScenarioPermission(); err != nil {
		return models.Scenario{}, err
	}

	scenario, err := usecase.scenarioReadRepository.GetScenarioById(nil, scenarioID)
	if err != nil {
		return models.Scenario{}, err
	}

	if err := usecase.enforceSecurity.ReadOrganization(scenario.OrganizationID); err != nil {
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

func (usecase *ScenarioUsecase) enforceReadScenarioPermission() error {
	return usecase.enforceSecurity.Permission(models.SCENARIO_READ)
}
