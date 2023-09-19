package usecases

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/transaction"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/cockroachdb/errors"
)

type ScenarioUsecase struct {
	transactionFactory      transaction.TransactionFactory
	organizationIdOfContext func() (string, error)
	enforceSecurity         security.EnforceSecurityScenario
	scenarioReadRepository  repositories.ScenarioReadRepository
	scenarioWriteRepository repositories.ScenarioWriteRepository
}

func (usecase *ScenarioUsecase) ListScenarios() ([]models.Scenario, error) {
	organizationId, err := usecase.organizationIdOfContext()
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

func (usecase *ScenarioUsecase) GetScenario(scenarioId string) (models.Scenario, error) {
	scenario, err := usecase.scenarioReadRepository.GetScenarioById(nil, scenarioId)
	if err != nil {
		return models.Scenario{}, err
	}

	if err := usecase.enforceSecurity.ReadScenario(scenario); err != nil {
		return models.Scenario{}, err
	}

	return scenario, nil
}

func (usecase *ScenarioUsecase) UpdateScenario(scenarioInput models.UpdateScenarioInput) (models.Scenario, error) {
	return transaction.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) (models.Scenario, error) {

			scenario, err := usecase.scenarioReadRepository.GetScenarioById(tx, scenarioInput.Id)
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
			scenario, err = usecase.scenarioReadRepository.GetScenarioById(tx, scenario.Id)
			return scenario, errors.HandledWithMessage(err, "Error getting scenario after update")
		},
	)
}

func (usecase *ScenarioUsecase) CreateScenario(scenario models.CreateScenarioInput) (models.Scenario, error) {
	return transaction.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) (models.Scenario, error) {
			if err := usecase.enforceSecurity.CreateScenario(scenario.OrganizationId); err != nil {
				return models.Scenario{}, err
			}
			newScenarioId := utils.NewPrimaryKey(scenario.OrganizationId)
			if err := usecase.scenarioWriteRepository.CreateScenario(nil, scenario, newScenarioId); err != nil {
				return models.Scenario{}, err
			}
			scenario, err := usecase.scenarioReadRepository.GetScenarioById(tx, newScenarioId)
			return scenario, errors.HandledWithMessage(err, "Error getting scenario after update")
		},
	)
}
