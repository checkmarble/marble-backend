package usecases

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/indexes"
	"github.com/checkmarble/marble-backend/usecases/scenarios"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
)

type scenarioListRepository interface {
	ListScenariosOfOrganization(ctx context.Context, exec repositories.Executor, organizationId string) ([]models.Scenario, error)
}

type IngestedDataIndexesRepository interface {
	ListAllValidIndexes(ctx context.Context, exec repositories.Executor) ([]models.ConcreteIndex, error)
	CreateIndexesAsync(ctx context.Context, exec repositories.Executor, indexes []models.ConcreteIndex) (numCreating int, err error)
}

type ScenarioPublicationUsecase struct {
	transactionFactory             executor_factory.TransactionFactory
	executorFactory                executor_factory.ExecutorFactory
	scenarioPublicationsRepository repositories.ScenarioPublicationRepository
	OrganizationIdOfContext        func() (string, error)
	enforceSecurity                security.EnforceSecurityScenario
	scenarioFetcher                scenarios.ScenarioFetcher
	scenarioPublisher              scenarios.ScenarioPublisher
	scenarioListRepository         scenarioListRepository
	ingestedDataIndexesRepository  IngestedDataIndexesRepository
}

func (usecase *ScenarioPublicationUsecase) GetScenarioPublication(ctx context.Context,
	scenarioPublicationID string,
) (models.ScenarioPublication, error) {
	scenarioPublication, err := usecase.scenarioPublicationsRepository.GetScenarioPublicationById(
		ctx, usecase.executorFactory.NewExecutor(), scenarioPublicationID)
	if err != nil {
		return models.ScenarioPublication{}, err
	}

	// Enforce permissions
	if err := usecase.enforceSecurity.ReadScenarioPublication(scenarioPublication); err != nil {
		return models.ScenarioPublication{}, err
	}
	return scenarioPublication, nil
}

func (usecase *ScenarioPublicationUsecase) ListScenarioPublications(ctx context.Context,
	filters models.ListScenarioPublicationsFilters,
) ([]models.ScenarioPublication, error) {
	organizationId, err := usecase.OrganizationIdOfContext()
	if err != nil {
		return nil, err
	}

	// Enforce permissions
	if err := usecase.enforceSecurity.ListScenarios(organizationId); err != nil {
		return nil, err
	}

	return usecase.scenarioPublicationsRepository.ListScenarioPublicationsOfOrganization(ctx,
		usecase.executorFactory.NewExecutor(), organizationId, filters)
}

func (usecase *ScenarioPublicationUsecase) ExecuteScenarioPublicationAction(ctx context.Context,
	input models.PublishScenarioIterationInput,
) ([]models.ScenarioPublication, error) {
	return executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Executor,
	) ([]models.ScenarioPublication, error) {
		scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(ctx, tx, input.ScenarioIterationId)
		if err != nil {
			return []models.ScenarioPublication{}, err
		}

		if err := usecase.enforceSecurity.PublishScenario(scenarioAndIteration.Scenario); err != nil {
			return []models.ScenarioPublication{}, err
		}

		return usecase.scenarioPublisher.PublishOrUnpublishIteration(ctx, tx,
			scenarioAndIteration, input.PublicationAction)
	})
}

func (usecase *ScenarioPublicationUsecase) CreateDatamodelIndexesForScenarioPublication(
	ctx context.Context, scenarioIterationId string,
) (ready bool, err error) {
	exec := usecase.executorFactory.NewExecutor()
	logger := utils.LoggerFromContext(ctx)
	iterationToActivate, err := usecase.scenarioFetcher.FetchScenarioAndIteration(ctx, exec, scenarioIterationId)
	if err != nil {
		return false, err
	}

	organizationId, err := usecase.OrganizationIdOfContext()
	if err != nil {
		return false, err
	}
	scenarios, err := usecase.scenarioListRepository.ListScenariosOfOrganization(ctx, exec, organizationId)
	if err != nil {
		return false, err
	}
	liveScenarios := utils.Filter(scenarios, func(scenario models.Scenario) bool {
		return scenario.LiveVersionID != nil
	})
	activeScenarioIterations, err := pure_utils.MapErr(liveScenarios, func(
		scenario models.Scenario,
	) (models.ScenarioIteration, error) {
		it, err := usecase.scenarioFetcher.FetchScenarioAndIteration(ctx, exec, *scenario.LiveVersionID)
		if err != nil {
			return models.ScenarioIteration{}, err
		}
		return it.Iteration, nil
	})
	if err != nil {
		return false, errors.Wrap(err, "Error while fetching active scenario iterations in CreateDatamodelIndexesForScenarioPublication")
	}

	db, err := usecase.executorFactory.NewClientDbExecutor(ctx, organizationId)
	if err != nil {
		return false, errors.Wrap(err, "Error while creating client schema executor in CreateDatamodelIndexesForScenarioPublication")
	}

	existingIndexes, err := usecase.ingestedDataIndexesRepository.ListAllValidIndexes(ctx, db)
	if err != nil {
		return false, errors.Wrap(err, "Error while fetching existing indexes in CreateDatamodelIndexesForScenarioPublication")
	}

	indexesToCreate, err := indexes.IndexesToCreateFromScenarioIterations(
		append(activeScenarioIterations, iterationToActivate.Iteration), existingIndexes)
	if err != nil {
		return false, errors.Wrap(err, "Error while finding indexes to create from scenario iterations in CreateDatamodelIndexesForScenarioPublication")
	}
	fmt.Printf("indexesToCreate: %+v\n", indexesToCreate)

	num, err := usecase.ingestedDataIndexesRepository.CreateIndexesAsync(ctx, db, indexesToCreate)
	if err != nil {
		return false, errors.Wrap(err, "Error while creating indexes in CreateDatamodelIndexesForScenarioPublication")
	}
	logger.Info(fmt.Sprintf("%d indexes pending creation in org %s", num, organizationId), "org_id", organizationId)

	return num == 0, nil
}
