package usecases

import (
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/security"
	"marble/marble-backend/utils"
)

type ScenarioUsecase struct {
	transactionFactory      repositories.TransactionFactory
	OrganizationIdOfContext func() (string, error)
	enforceSecurity         security.EnforceSecurityScenario
	scenarioReadRepository  repositories.ScenarioReadRepository
	scenarioWriteRepository repositories.ScenarioWriteRepository
}

func (usecase *ScenarioUsecase) ListScenarios() ([]models.Scenario, error) {
	organizationId, err := usecase.OrganizationIdOfContext()
	if err != nil {
		return nil, err
	}
	scenarios, err := usecase.scenarioReadRepository.ListScenariosOfOrganization(nil, organizationId)
	if err != nil {
		return nil, err
	}

	for _, scenario := range scenarios {
		if err := usecase.enforceSecurity.ReadScenario(scenario); err != nil {
			return nil, err
		}
	}
	return scenarios, nil
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

func (usecase *ScenarioUsecase) UpdateScenario(scenarioInput models.UpdateScenarioInput) (models.Scenario, error) {
	return repositories.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) (models.Scenario, error) {

			scenario, err := usecase.scenarioReadRepository.GetScenarioById(tx, scenarioInput.ID)
			if err != nil {
				return models.Scenario{}, err
			}
			if err := usecase.enforceSecurity.UpdateScenario(scenario); err != nil {
				return models.Scenario{}, err
			}

			err = usecase.scenarioWriteRepository.UpdateScenario(tx, scenarioInput)
			if err != nil {
				return models.Scenario{}, err
			}
			return usecase.scenarioReadRepository.GetScenarioById(tx, scenario.ID)
		},
	)
}

func (usecase *ScenarioUsecase) CreateScenario(scenario models.CreateScenarioInput) (models.Scenario, error) {
	return repositories.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) (models.Scenario, error) {
			if err := usecase.enforceSecurity.CreateScenario(scenario.OrganizationID); err != nil {
				return models.Scenario{}, err
			}
			newScenarioId := utils.NewPrimaryKey(scenario.OrganizationID)
			err := usecase.scenarioWriteRepository.CreateScenario(nil, scenario, newScenarioId)
			if err != nil {
				return models.Scenario{}, err
			}
			return usecase.scenarioReadRepository.GetScenarioById(tx, newScenarioId)
		},
	)
}
