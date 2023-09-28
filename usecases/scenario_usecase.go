package usecases

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/transaction"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/cockroachdb/errors"
)

type ScenarioUsecaseRepository interface {
	GetScenarioById(tx repositories.Transaction, scenarioId string) (models.Scenario, error)
	ListScenariosOfOrganization(tx repositories.Transaction, organizationId string) ([]models.Scenario, error)
	CreateScenario(tx repositories.Transaction, organizationId string, scenario models.CreateScenarioInput, newScenarioId string) error
	UpdateScenario(tx repositories.Transaction, scenario models.UpdateScenarioInput) error
}

type ScenarioUsecase struct {
	transactionFactory      transaction.TransactionFactory
	organizationIdOfContext func() (string, error)
	enforceSecurity         security.EnforceSecurityScenario
	repository              ScenarioUsecaseRepository
}

func (usecase *ScenarioUsecase) ListScenarios() ([]models.Scenario, error) {
	organizationId, err := usecase.organizationIdOfContext()
	if err != nil {
		return nil, err
	}
	scenarios, err := usecase.repository.ListScenariosOfOrganization(nil, organizationId)
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
	scenario, err := usecase.repository.GetScenarioById(nil, scenarioId)
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

			scenario, err := usecase.repository.GetScenarioById(tx, scenarioInput.Id)
			if err != nil {
				return models.Scenario{}, err
			}
			if err := usecase.enforceSecurity.UpdateScenario(scenario); err != nil {
				return models.Scenario{}, err
			}

			err = usecase.repository.UpdateScenario(tx, scenarioInput)
			if err != nil {
				return models.Scenario{}, err
			}
			scenario, err = usecase.repository.GetScenarioById(tx, scenario.Id)
			return scenario, errors.HandledWithMessage(err, "Error getting scenario after update")
		},
	)
}

func (usecase *ScenarioUsecase) CreateScenario(scenario models.CreateScenarioInput) (models.Scenario, error) {
	organizationId, err := usecase.organizationIdOfContext()
	if err != nil {
		return models.Scenario{}, err
	}

	if err := usecase.enforceSecurity.CreateScenario(organizationId); err != nil {
		return models.Scenario{}, err
	}

	return transaction.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) (models.Scenario, error) {
			newScenarioId := utils.NewPrimaryKey(organizationId)
			if err := usecase.repository.CreateScenario(tx, organizationId, scenario, newScenarioId); err != nil {
				return models.Scenario{}, err
			}
			scenario, err := usecase.repository.GetScenarioById(tx, newScenarioId)
			return scenario, errors.HandledWithMessage(err, "Error getting scenario after update")
		},
	)
}
