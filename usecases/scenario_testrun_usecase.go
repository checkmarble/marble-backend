package usecases

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/pkg/errors"
)

type ScenarioTestRunUsecase struct {
	transactionFactory  executor_factory.TransactionFactory
	executorFactory     executor_factory.ExecutorFactory
	enforceSecurity     security.EnforceSecurityTestRun
	repository          repositories.ScenarioTestRunRepository
	scenarioRepository  ScenarioUsecaseRepository
	clientDbIndexEditor clientDbIndexEditor
}

func (usecase *ScenarioTestRunUsecase) ActivateScenarioTestRun(ctx context.Context,
	organizationId string,
	input models.ScenarioTestRunInput,
) (models.ScenarioTestRun, error) {
	if err := usecase.enforceSecurity.CreateTestRun(organizationId); err != nil {
		return models.ScenarioTestRun{}, err
	}
	indexesToCreate, _, err := usecase.clientDbIndexEditor.GetIndexesToCreate(
		ctx,
		organizationId,
		input.ScenarioIterationId,
	)
	if err != nil {
		return models.ScenarioTestRun{}, errors.Wrap(err,
			"Error while fetching indexes to create in ActivateScenarioTestRun")
	}
	if len(indexesToCreate) > 0 {
		return models.ScenarioTestRun{}, errors.Wrap(
			models.ErrScenarioIterationRequiresPreparation,
			fmt.Sprintf("Cannot activate the scenario in test mode: it requires data preparation to be run first for %d indexes", len(indexesToCreate)),
		)
	}

	errIdx := usecase.clientDbIndexEditor.CreateIndexesAsync(ctx, organizationId, indexesToCreate)
	if errIdx != nil {
		return models.ScenarioTestRun{}, errors.Wrap(errIdx,
			"Error while creating indexes in ActivateScenarioTestRun")
	}
	exec := usecase.executorFactory.NewExecutor()
	// we should not have any existing testrun for this scenario
	existingTestrun, err := usecase.repository.GetActiveTestRunByScenarioIterationID(ctx, exec, input.ScenarioIterationId)
	if err != nil {
		return models.ScenarioTestRun{}, errors.Wrap(err,
			"error while fecthing entries to find an existing testrun")
	}
	if existingTestrun != nil {
		return models.ScenarioTestRun{}, errors.Wrap(models.ErrTestRunAlreadyExist,
			fmt.Sprintf("the scenario %s has a running testrun", input.ScenarioId))
	}

	// we should have a live version running
	scenario, errScenario := usecase.scenarioRepository.GetScenarioById(ctx, exec, input.ScenarioId)
	if errScenario != nil {
		return models.ScenarioTestRun{}, errScenario
	}
	if scenario.LiveVersionID == nil {
		return models.ScenarioTestRun{}, models.ErrScenarioHasNoLiveVersion
	}

	// the live version must not be the one on which we want to start a testrun
	if *scenario.LiveVersionID == input.ScenarioIterationId {
		return models.ScenarioTestRun{}, models.ErrWrongIterationForTestRun
	}

	return executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Transaction) (models.ScenarioTestRun, error) {
			testrunID := pure_utils.NewPrimaryKey(organizationId)
			if err := usecase.repository.CreateTestRun(ctx, tx, testrunID, input); err != nil {
				return models.ScenarioTestRun{}, err
			}
			result, err := usecase.repository.GetTestRunByID(ctx, exec, testrunID)
			if err != nil {
				return models.ScenarioTestRun{}, err
			}
			return *result, err
		},
	)
}
