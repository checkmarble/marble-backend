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
	ListAllUniqueIndexes(ctx context.Context, exec repositories.Executor) ([]models.UnicityIndex, error)
	CreateIndexesAsync(ctx context.Context, exec repositories.Executor, indexes []models.ConcreteIndex) error
	CountPendingIndexes(ctx context.Context, exec repositories.Executor) (int, error)
	CreateUniqueIndexAsync(ctx context.Context, exec repositories.Executor, index models.UnicityIndex) error
	CreateUniqueIndex(ctx context.Context, exec repositories.Executor, index models.UnicityIndex) error
	DeleteUniqueIndex(ctx context.Context, exec repositories.Executor, index models.UnicityIndex) error
}

type ScenarioFetcher interface {
	FetchScenarioAndIteration(ctx context.Context, exec repositories.Executor, iterationId string) (models.ScenarioAndIteration, error)
}

type ClientDbIndexEditor struct {
	executorFactory               executor_factory.ExecutorFactory
	scenarioFetcher               ScenarioFetcher
	ingestedDataIndexesRepository IngestedDataIndexesRepository
	enforceSecurity               security.EnforceSecurityScenario
	enforceSecurityDataModel      security.EnforceSecurityOrganization
	organizationIdOfContext       func() (string, error)
}

func NewClientDbIndexEditor(
	executorFactory executor_factory.ExecutorFactory,
	scenarioFetcher ScenarioFetcher,
	ingestedDataIndexesRepository IngestedDataIndexesRepository,
	enforceSecurity security.EnforceSecurityScenario,
	enforceSecurityDataModel security.EnforceSecurityOrganization,
	organizationIdOfContext func() (string, error),
) ClientDbIndexEditor {
	return ClientDbIndexEditor{
		executorFactory:               executorFactory,
		scenarioFetcher:               scenarioFetcher,
		ingestedDataIndexesRepository: ingestedDataIndexesRepository,
		enforceSecurity:               enforceSecurity,
		enforceSecurityDataModel:      enforceSecurityDataModel,
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
		ctx,
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
	if err := editor.enforceSecurityDataModel.WriteDataModel(organizationId); err != nil {
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
		fmt.Sprintf("%d indexes pending creation in: %+v", len(indexes), indexes), "org_id", organizationId,
	)
	return nil
}

func (editor ClientDbIndexEditor) ListAllUniqueIndexes(ctx context.Context) ([]models.UnicityIndex, error) {
	if err := editor.enforceSecurityDataModel.ReadDataModel(); err != nil {
		return nil, err
	}
	organizationId, err := editor.organizationIdOfContext()
	if err != nil {
		return nil, err
	}

	db, err := editor.executorFactory.NewClientDbExecutor(ctx, organizationId)
	if err != nil {
		return nil, errors.Wrap(
			err,
			"Error while creating client schema executor in ListAllUniqueIndexes")
	}
	return editor.ingestedDataIndexesRepository.ListAllUniqueIndexes(ctx, db)
}

func (editor ClientDbIndexEditor) CreateUniqueIndexAsync(
	ctx context.Context,
	index models.UnicityIndex,
) error {
	logger := utils.LoggerFromContext(ctx)

	organizationId, err := editor.organizationIdOfContext()
	if err != nil {
		return err
	}
	if err := editor.enforceSecurityDataModel.WriteDataModel(organizationId); err != nil {
		return err
	}

	db, err := editor.executorFactory.NewClientDbExecutor(ctx, organizationId)
	if err != nil {
		return errors.Wrap(
			err,
			"Error while creating client schema executor in CreateUniqueIndexAsync")
	}
	err = editor.ingestedDataIndexesRepository.CreateUniqueIndexAsync(ctx, db, index)
	if err != nil {
		return errors.Wrap(err, "Error while creating unique index in CreateUniqueIndexAsync")
	}
	logger.InfoContext(
		ctx,
		fmt.Sprintf("Unique index pending creation asynchronously: %+v", index),
		"org_id", organizationId,
	)
	return nil
}

func (editor ClientDbIndexEditor) CreateUniqueIndex(
	ctx context.Context,
	exec repositories.Executor,
	index models.UnicityIndex,
) error {
	logger := utils.LoggerFromContext(ctx)
	organizationId, err := editor.organizationIdOfContext()
	if err != nil {
		return err
	}
	if err := editor.enforceSecurityDataModel.WriteDataModel(organizationId); err != nil {
		return err
	}

	if exec == nil {
		exec, err = editor.executorFactory.NewClientDbExecutor(ctx, organizationId)
		if err != nil {
			return errors.Wrap(
				err,
				"Error while creating client schema executor in CreateUniqueIndex")
		}
	}

	if err := editor.ingestedDataIndexesRepository.CreateUniqueIndex(ctx, exec, index); err != nil {
		return errors.Wrap(err, "Error while creating unique index in CreateUniqueIndex")
	}

	logger.InfoContext(ctx, fmt.Sprintf("Unique index pending created: %+v", index))
	return nil
}

func (editor ClientDbIndexEditor) DeleteUniqueIndex(
	ctx context.Context,
	index models.UnicityIndex,
) error {
	logger := utils.LoggerFromContext(ctx)
	organizationId, err := editor.organizationIdOfContext()
	if err != nil {
		return err
	}
	if err := editor.enforceSecurityDataModel.WriteDataModel(organizationId); err != nil {
		return err
	}

	db, err := editor.executorFactory.NewClientDbExecutor(ctx, organizationId)
	if err != nil {
		return errors.Wrap(
			err,
			"Error while creating client schema executor in DeleteUniqueIndex")
	}
	err = editor.ingestedDataIndexesRepository.DeleteUniqueIndex(ctx, db, index)
	if err != nil {
		return errors.Wrap(err, "Error while deleting unique index in DeleteUniqueIndex")
	}
	logger.InfoContext(
		ctx,
		fmt.Sprintf("Unique index deletion: %+v", index),
	)
	return nil
}
