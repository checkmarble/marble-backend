package usecases

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/indexes"
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
	CountPendingIndexes(ctx context.Context, exec repositories.Executor) (int, error)
}

type ScenarioFetcher interface {
	FetchScenarioAndIteration(
		ctx context.Context,
		exec repositories.Executor,
		scenarioIterationId string,
	) (models.ScenarioAndIteration, error)
}

type ScenarioPublisher interface {
	PublishOrUnpublishIteration(
		ctx context.Context,
		exec repositories.Executor,
		scenarioAndIteration models.ScenarioAndIteration,
		publicationAction models.PublicationAction,
	) ([]models.ScenarioPublication, error)
}

type ScenarioPublicationUsecase struct {
	transactionFactory             executor_factory.TransactionFactory
	executorFactory                executor_factory.ExecutorFactory
	scenarioPublicationsRepository repositories.ScenarioPublicationRepository
	OrganizationIdOfContext        func() (string, error)
	enforceSecurity                security.EnforceSecurityScenario
	scenarioFetcher                ScenarioFetcher
	scenarioPublisher              ScenarioPublisher
	scenarioListRepository         scenarioListRepository
	ingestedDataIndexesRepository  IngestedDataIndexesRepository
}

func (usecase *ScenarioPublicationUsecase) GetScenarioPublication(
	ctx context.Context,
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

func (usecase *ScenarioPublicationUsecase) ListScenarioPublications(
	ctx context.Context,
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

func (usecase *ScenarioPublicationUsecase) ExecuteScenarioPublicationAction(
	ctx context.Context,
	input models.PublishScenarioIterationInput,
) ([]models.ScenarioPublication, error) {
	return executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Executor,
		) ([]models.ScenarioPublication, error) {
			scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(ctx, tx, input.ScenarioIterationId)
			if err != nil {
				return nil, err
			}

			if err := usecase.enforceSecurity.PublishScenario(scenarioAndIteration.Scenario); err != nil {
				return nil, err
			}

			return usecase.scenarioPublisher.PublishOrUnpublishIteration(
				ctx,
				tx,
				scenarioAndIteration,
				input.PublicationAction,
			)
		})
}

func (usecase *ScenarioPublicationUsecase) GetPublicationPreparationStatus(
	ctx context.Context,
	scenarioIterationId string,
) (status models.PublicationPreparationStatus, err error) {
	logger := utils.LoggerFromContext(ctx)

	indexesToCreate, numPending, err := usecase.getIndexesToCreate(ctx, scenarioIterationId)
	if err != nil {
		return status, errors.Wrap(err, "Error while fetching indexes to create in GetPublicationPreparationStatus")
	}

	if len(indexesToCreate) == 0 {
		status.PreparationStatus = models.PreparationStatusReadyToActivate
	} else {
		logger.InfoContext(ctx, fmt.Sprintf("Found %d indexes to create in GetPublicationPreparationStatus: %+v\n", len(indexesToCreate), indexesToCreate))
		status.PreparationStatus = models.PreparationStatusRequired
	}

	if numPending == 0 {
		status.PreparationServiceStatus = models.PreparationServiceStatusAvailable
	} else {
		status.PreparationServiceStatus = models.PreparationServiceStatusOccupied
	}

	return
}

func (usecase *ScenarioPublicationUsecase) StartPublicationPreparation(
	ctx context.Context,
	scenarioIterationId string,
) error {
	logger := utils.LoggerFromContext(ctx)

	indexesToCreate, numPending, err := usecase.getIndexesToCreate(ctx, scenarioIterationId)
	if err != nil {
		return errors.Wrap(err, "Error while fetching indexes to create in StartPublicationPreparation")
	}

	if len(indexesToCreate) == 0 {
		return nil
	}

	if numPending > 0 {
		return errors.Wrap(
			models.ConflictError, // return 409 if the db is busy creating indexes in this schema
			"There are still pending indexes in the schema")
	}

	organizationId, err := usecase.OrganizationIdOfContext()
	if err != nil {
		return err
	}
	db, err := usecase.executorFactory.NewClientDbExecutor(ctx, organizationId)
	if err != nil {
		return errors.Wrap(
			err,
			"Error while creating client schema executor in StartPublicationPreparation")
	}
	num, err := usecase.ingestedDataIndexesRepository.CreateIndexesAsync(ctx, db, indexesToCreate)
	if err != nil {
		return errors.Wrap(err, "Error while creating indexes in StartPublicationPreparation")
	}
	logger.InfoContext(
		ctx,
		fmt.Sprintf("%d indexes pending creation in: %+v\n", num, indexesToCreate), "org_id", organizationId,
	)
	return nil
}

func (usecase *ScenarioPublicationUsecase) getIndexesToCreate(
	ctx context.Context,
	scenarioIterationId string,
) (toCreate []models.ConcreteIndex, numPending int, err error) {
	exec := usecase.executorFactory.NewExecutor()
	iterationToActivate, err := usecase.scenarioFetcher.FetchScenarioAndIteration(ctx, exec, scenarioIterationId)
	if err != nil {
		return toCreate, numPending, err
	}
	if err := usecase.enforceSecurity.PublishScenario(iterationToActivate.Scenario); err != nil {
		return toCreate, numPending, err
	}

	organizationId, err := usecase.OrganizationIdOfContext()
	if err != nil {
		return toCreate, numPending, err
	}
	db, err := usecase.executorFactory.NewClientDbExecutor(ctx, organizationId)
	if err != nil {
		return toCreate, numPending, errors.Wrap(err,
			"Error while creating client schema executor in CreateDatamodelIndexesForScenarioPublication")
	}

	existingIndexes, err := usecase.ingestedDataIndexesRepository.ListAllValidIndexes(ctx, db)
	if err != nil {
		return toCreate, numPending, errors.Wrap(err,
			"Error while fetching existing indexes in CreateDatamodelIndexesForScenarioPublication")
	}

	toCreate, err = indexes.IndexesToCreateFromScenarioIterations(
		[]models.ScenarioIteration{iterationToActivate.Iteration},
		existingIndexes,
	)
	if err != nil {
		return toCreate, numPending, errors.Wrap(err,
			"Error while finding indexes to create from scenario iterations in CreateDatamodelIndexesForScenarioPublication")
	}

	numPending, err = usecase.ingestedDataIndexesRepository.CountPendingIndexes(ctx, db)
	if err != nil {
		return toCreate, numPending, errors.Wrap(err,
			"Error while counting pending indexes in CreateDatamodelIndexesForScenarioPublication")
	}

	return
}
