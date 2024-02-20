package indexes

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/pkg/errors"
)

type IngestedDataIndexesRepository interface {
	ListAllValidIndexes(ctx context.Context, exec repositories.Executor) ([]models.ConcreteIndex, error)
	CreateIndexesAsync(ctx context.Context, exec repositories.Executor, indexes []models.ConcreteIndex) (err error)
	CountPendingIndexes(ctx context.Context, exec repositories.Executor) (int, error)
}

type ScenarioFetcher interface {
	FetchScenarioAndIteration(
		ctx context.Context,
		exec repositories.Executor,
		scenarioIterationId string,
	) (models.ScenarioAndIteration, error)
}

type ClientDbIndexEditor struct {
	executorFactory               executor_factory.ExecutorFactory
	scenarioFetcher               ScenarioFetcher
	ingestedDataIndexesRepository IngestedDataIndexesRepository
	enforceSecurity               security.EnforceSecurityScenario
	organizationIdOfContext       func() (string, error)
}

func NewClientDbIndexEditor(
	executorFactory executor_factory.ExecutorFactory,
	scenarioFetcher ScenarioFetcher,
	ingestedDataIndexesRepository IngestedDataIndexesRepository,
	enforceSecurity security.EnforceSecurityScenario,
	organizationIdOfContext func() (string, error),
) ClientDbIndexEditor {
	return ClientDbIndexEditor{
		executorFactory:               executorFactory,
		scenarioFetcher:               scenarioFetcher,
		ingestedDataIndexesRepository: ingestedDataIndexesRepository,
		enforceSecurity:               enforceSecurity,
		organizationIdOfContext:       organizationIdOfContext,
	}
}

func (editor ClientDbIndexEditor) GetIndexesToCreate(
	ctx context.Context,
	scenarioIterationId string,
) (toCreate []models.ConcreteIndex, numPending int, err error) {
	exec := editor.executorFactory.NewExecutor()
	iterationToActivate, err := editor.scenarioFetcher.FetchScenarioAndIteration(ctx, exec, scenarioIterationId)
	if err != nil {
		return toCreate, numPending, err
	}
	if err := editor.enforceSecurity.PublishScenario(iterationToActivate.Scenario); err != nil {
		return toCreate, numPending, err
	}

	organizationId, err := editor.organizationIdOfContext()
	if err != nil {
		return toCreate, numPending, err
	}
	db, err := editor.executorFactory.NewClientDbExecutor(ctx, organizationId)
	if err != nil {
		return toCreate, numPending, errors.Wrap(err,
			"Error while creating client schema executor in CreateDatamodelIndexesForScenarioPublication")
	}

	existingIndexes, err := editor.ingestedDataIndexesRepository.ListAllValidIndexes(ctx, db)
	if err != nil {
		return toCreate, numPending, errors.Wrap(err,
			"Error while fetching existing indexes in CreateDatamodelIndexesForScenarioPublication")
	}

	toCreate, err = indexesToCreateFromScenarioIterations(
		[]models.ScenarioIteration{iterationToActivate.Iteration},
		existingIndexes,
	)
	if err != nil {
		return toCreate, numPending, errors.Wrap(err,
			"Error while finding indexes to create from scenario iterations in CreateDatamodelIndexesForScenarioPublication")
	}

	numPending, err = editor.ingestedDataIndexesRepository.CountPendingIndexes(ctx, db)
	if err != nil {
		return toCreate, numPending, errors.Wrap(err,
			"Error while counting pending indexes in CreateDatamodelIndexesForScenarioPublication")
	}

	return
}

func (editor ClientDbIndexEditor) CreateIndexesAsync(
	ctx context.Context,
	indexes []models.ConcreteIndex,
) error {
	logger := utils.LoggerFromContext(ctx)

	organizationId, err := editor.organizationIdOfContext()
	if err != nil {
		return err
	}
	db, err := editor.executorFactory.NewClientDbExecutor(ctx, organizationId)
	if err != nil {
		return errors.Wrap(
			err,
			"Error while creating client schema executor in StartPublicationPreparation")
	}
	err = editor.ingestedDataIndexesRepository.CreateIndexesAsync(ctx, db, indexes)
	if err != nil {
		return errors.Wrap(err, "Error while creating indexes in StartPublicationPreparation")
	}
	logger.InfoContext(
		ctx,
		fmt.Sprintf("%d indexes pending creation in: %+v\n", len(indexes), indexes), "org_id", organizationId,
	)
	return nil
}