package usecases

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/scenarios"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/transaction"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
)

type scenarioListRepository interface {
	ListScenariosOfOrganization(ctx context.Context, tx repositories.Transaction, organizationId string) ([]models.Scenario, error)
}

type ScenarioPublicationUsecase struct {
	transactionFactory             transaction.TransactionFactory
	scenarioPublicationsRepository repositories.ScenarioPublicationRepository
	OrganizationIdOfContext        func() (string, error)
	enforceSecurity                security.EnforceSecurityScenario
	scenarioFetcher                scenarios.ScenarioFetcher
	scenarioPublisher              scenarios.ScenarioPublisher
	scenarioListRepository         scenarioListRepository
}

func (usecase *ScenarioPublicationUsecase) GetScenarioPublication(ctx context.Context, scenarioPublicationID string) (models.ScenarioPublication, error) {
	scenarioPublication, err := usecase.scenarioPublicationsRepository.GetScenarioPublicationById(ctx, nil, scenarioPublicationID)
	if err != nil {
		return models.ScenarioPublication{}, err
	}

	// Enforce permissions
	if err := usecase.enforceSecurity.ReadScenarioPublication(scenarioPublication); err != nil {
		return models.ScenarioPublication{}, err
	}
	return scenarioPublication, nil
}

func (usecase *ScenarioPublicationUsecase) ListScenarioPublications(ctx context.Context, filters models.ListScenarioPublicationsFilters) ([]models.ScenarioPublication, error) {
	organizationId, err := usecase.OrganizationIdOfContext()
	if err != nil {
		return nil, err
	}

	// Enforce permissions
	if err := usecase.enforceSecurity.ListScenarios(organizationId); err != nil {
		return nil, err
	}

	return usecase.scenarioPublicationsRepository.ListScenarioPublicationsOfOrganization(ctx, nil, organizationId, filters)
}

func (usecase *ScenarioPublicationUsecase) ExecuteScenarioPublicationAction(ctx context.Context, input models.PublishScenarioIterationInput) ([]models.ScenarioPublication, error) {
	return transaction.TransactionReturnValue(ctx, usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) ([]models.ScenarioPublication, error) {

		scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(ctx, tx, input.ScenarioIterationId)
		if err != nil {
			return []models.ScenarioPublication{}, err
		}

		if err := usecase.enforceSecurity.PublishScenario(scenarioAndIteration.Scenario); err != nil {
			return []models.ScenarioPublication{}, err
		}

		return usecase.scenarioPublisher.PublishOrUnpublishIteration(ctx, tx, scenarioAndIteration, input.PublicationAction)
	})

}

func (usecase *ScenarioPublicationUsecase) CreateDatamodelIndexesForScenarioPublication(ctx context.Context, scenarioIterationId string) (bool, error) {
	iterationToActivate, err := usecase.scenarioFetcher.FetchScenarioAndIteration(ctx, nil, scenarioIterationId)
	if err != nil {
		return false, err
	}

	organizationId, err := usecase.OrganizationIdOfContext()
	if err != nil {
		return false, err
	}
	scenarios, err := usecase.scenarioListRepository.ListScenariosOfOrganization(ctx, nil, organizationId)
	if err != nil {
		return false, err
	}
	liveScenarios := utils.Filter(scenarios, func(scenario models.Scenario) bool {
		return scenario.LiveVersionID != nil
	})
	activeScenarioIterations, err := utils.MapErr(liveScenarios, func(scenario models.Scenario) (models.ScenarioIteration, error) {
		it, err := usecase.scenarioFetcher.FetchScenarioAndIteration(ctx, nil, *scenario.LiveVersionID)
		if err != nil {
			return models.ScenarioIteration{}, err
		}
		return it.Iteration, nil
	})
	if err != nil {
		return false, errors.Wrap(err, "Error while fetching active scenario iterations in CreateDatamodelIndexesForScenarioPublication")
	}

	indexesToCreate, err := models.IndexesToCreateFromScenarioIterations(append(activeScenarioIterations, iterationToActivate.Iteration), nil)
	if err != nil {
		return false, errors.Wrap(err, "Error while finding indexes to create from scenario iterations in CreateDatamodelIndexesForScenarioPublication")
	}
	fmt.Printf("indexesToCreate: %+v\n", indexesToCreate)

	return false, nil
}
