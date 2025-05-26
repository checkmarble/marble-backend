package usecases

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
)

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
		exec repositories.Transaction,
		scenarioAndIteration models.ScenarioAndIteration,
		publicationAction models.PublicationAction,
	) ([]models.ScenarioPublication, error)
}

type clientDbIndexEditor interface {
	GetIndexesToCreate(ctx context.Context, organizationId string, scenarioIterationId string) (
		toCreate []models.ConcreteIndex, numPending int, err error,
	)
	CreateIndexesAsync(ctx context.Context, organizationId string, indexes []models.ConcreteIndex) error
	CreateIndexesAsyncForScenarioWithCallback(
		ctx context.Context,
		organizationId string,
		indexes []models.ConcreteIndex,
		onSuccess models.OnCreateIndexesSuccess) error
	ListAllUniqueIndexes(ctx context.Context, organizationId string) ([]models.UnicityIndex, error)
	CreateUniqueIndex(ctx context.Context, exec repositories.Executor, organizationId string, index models.UnicityIndex) error
	CreateUniqueIndexAsync(ctx context.Context, organizationId string, index models.UnicityIndex) error
	DeleteUniqueIndex(ctx context.Context, organizationId string, index models.UnicityIndex) error
}

type PublicationUsecaseFeatureAccessReader interface {
	GetOrganizationFeatureAccess(
		ctx context.Context,
		organizationId string,
	) (models.OrganizationFeatureAccess, error)
}

type SanctionCheckRequirementChecker interface {
	IsConfigured(context.Context) (bool, error)
}

type ScenarioPublicationUsecase struct {
	transactionFactory             executor_factory.TransactionFactory
	executorFactory                executor_factory.ExecutorFactory
	scenarioPublicationsRepository repositories.ScenarioPublicationRepository
	taskQueueRepository            repositories.TaskQueueRepository
	enforceSecurity                security.EnforceSecurityScenario
	scenarioFetcher                ScenarioFetcher
	scenarioPublisher              ScenarioPublisher
	clientDbIndexEditor            clientDbIndexEditor
	featureAccessReader            PublicationUsecaseFeatureAccessReader
	sanctionCheckRequirements      SanctionCheckRequirementChecker
}

func NewScenarioPublicationUsecase(
	transactionFactory executor_factory.TransactionFactory,
	executorFactory executor_factory.ExecutorFactory,
	scenarioPublicationsRepository repositories.ScenarioPublicationRepository,
	taskQueueRepository repositories.TaskQueueRepository,
	enforceSecurity security.EnforceSecurityScenario,
	scenarioFetcher ScenarioFetcher,
	scenarioPublisher ScenarioPublisher,
	clientDbIndexEditor clientDbIndexEditor,
	featureAccessReader PublicationUsecaseFeatureAccessReader,
	sanctionCheckRequirements SanctionCheckRequirementChecker,
) *ScenarioPublicationUsecase {
	return &ScenarioPublicationUsecase{
		transactionFactory:             transactionFactory,
		executorFactory:                executorFactory,
		scenarioPublicationsRepository: scenarioPublicationsRepository,
		taskQueueRepository:            taskQueueRepository,
		enforceSecurity:                enforceSecurity,
		scenarioFetcher:                scenarioFetcher,
		scenarioPublisher:              scenarioPublisher,
		clientDbIndexEditor:            clientDbIndexEditor,
		featureAccessReader:            featureAccessReader,
		sanctionCheckRequirements:      sanctionCheckRequirements,
	}
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
	organizationId string,
	filters models.ListScenarioPublicationsFilters,
) ([]models.ScenarioPublication, error) {
	// Enforce permissions
	if err := usecase.enforceSecurity.ListScenarios(organizationId); err != nil {
		return nil, err
	}

	return usecase.scenarioPublicationsRepository.ListScenarioPublicationsOfOrganization(ctx,
		usecase.executorFactory.NewExecutor(), organizationId, filters)
}

func (usecase *ScenarioPublicationUsecase) ExecuteScenarioPublicationAction(
	ctx context.Context,
	organizationId string,
	input models.PublishScenarioIterationInput,
) ([]models.ScenarioPublication, error) {
	indexesToCreate, _, err := usecase.clientDbIndexEditor.GetIndexesToCreate(
		ctx,
		organizationId,
		input.ScenarioIterationId,
	)
	if err != nil {
		return nil, errors.Wrap(err, "Error while fetching indexes to create in ExecuteScenarioPublicationAction")
	}
	if len(indexesToCreate) > 0 && input.PublicationAction == models.Publish {
		return nil, errors.Wrap(
			models.ErrScenarioIterationRequiresPreparation,
			fmt.Sprintf("Cannot publish the scenario iteration: it requires data preparation to be run first for %d indexes", len(indexesToCreate)),
		)
	}

	return executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Transaction,
		) ([]models.ScenarioPublication, error) {
			scenarioAndIteration, err := usecase.scenarioFetcher.FetchScenarioAndIteration(ctx, tx, input.ScenarioIterationId)
			if err != nil {
				return nil, err
			}
			if len(scenarioAndIteration.Iteration.SanctionCheckConfigs) > 0 {
				featureAccess, err := usecase.featureAccessReader.GetOrganizationFeatureAccess(ctx, organizationId)
				if err != nil {
					return nil, err
				}
				if !featureAccess.Sanctions.IsAllowed() {
					return nil, errors.Wrapf(models.ForbiddenError,
						"Sanction check feature access is missing: status is %s", featureAccess.Sanctions)
				}

				if isConfigured, err := usecase.sanctionCheckRequirements.IsConfigured(ctx); !isConfigured {
					return nil, err
				}
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
	organizationId string,
	scenarioIterationId string,
) (status models.PublicationPreparationStatus, err error) {
	logger := utils.LoggerFromContext(ctx)

	indexesToCreate, numPending, err := usecase.clientDbIndexEditor.GetIndexesToCreate(ctx, organizationId, scenarioIterationId)
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
	organizationId string,
	scenarioIterationId string,
) error {
	indexesToCreate, numPending, err := usecase.clientDbIndexEditor.GetIndexesToCreate(ctx, organizationId, scenarioIterationId)
	if err != nil {
		return errors.Wrap(err, "Error while fetching indexes to create in StartPublicationPreparation")
	}

	if len(indexesToCreate) == 0 {
		return nil
	}

	if numPending > 0 {
		return models.ErrDataPreparationServiceUnavailable
	}

	if err := usecase.taskQueueRepository.EnqueueCreateIndexTask(ctx,
		organizationId, indexesToCreate); err != nil {
		return err
	}

	return err
}
