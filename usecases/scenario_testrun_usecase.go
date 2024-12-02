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
	scenarioRepository  repositories.ScenarioUsecaseRepository
	clientDbIndexEditor clientDbIndexEditor
}

func (usecase *ScenarioTestRunUsecase) ActivateScenarioTestRun(ctx context.Context,
	organizationId string,
	input models.ScenarioTestRunInput,
) (models.ScenarioTestRun, error) {
	if err := usecase.enforceSecurity.CreateTestRun(organizationId); err != nil {
		return models.ScenarioTestRun{}, err
	}
	indexesToCreate, numPending, err := usecase.clientDbIndexEditor.GetIndexesToCreate(
		ctx,
		organizationId,
		input.PhantomIterationId,
	)
	if err != nil {
		return models.ScenarioTestRun{}, errors.Wrap(err,
			"Error while fetching indexes to create in ActivateScenarioTestRun")
	}

	if numPending > 0 {
		return models.ScenarioTestRun{}, models.ErrDataPreparationServiceUnavailable
	}

	if len(indexesToCreate) == 0 {
		return models.ScenarioTestRun{}, nil
	}
	exec := usecase.executorFactory.NewExecutor()
	errIdx := usecase.clientDbIndexEditor.CreateIndexesAsyncForScenarioWithCallback(ctx,
		organizationId, indexesToCreate, func(ctx context.Context, exec repositories.Executor, args ...interface{}) error {
			scenarioIterationId := args[0].(string)
			return usecase.repository.UpdateTestRunStatus(ctx, exec, scenarioIterationId, models.Up)
		}, input.PhantomIterationId)

	if errIdx != nil {
		return models.ScenarioTestRun{}, errors.Wrap(errIdx,
			"Error while creating indexes in ActivateScenarioTestRun")
	}

	// we should not have any existing testrun for this scenario
	existingTestrun, err := usecase.repository.GetActiveTestRunByScenarioIterationID(ctx, exec, input.PhantomIterationId)
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
	if *scenario.LiveVersionID == input.PhantomIterationId {
		return models.ScenarioTestRun{}, models.ErrWrongIterationForTestRun
	}

	// keep track of the live version associated to the current testrun
	input.LiveScenarioId = *scenario.LiveVersionID

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
			result.ScenarioId = scenario.Id
			return *result, err
		},
	)
}

func (usecase *ScenarioTestRunUsecase) ListTestRunByScenarioId(ctx context.Context,
	scenarioId string,
) ([]models.ScenarioTestRun, error) {
	testruns, err := usecase.repository.ListTestRunsByScenarioID(ctx,
		usecase.executorFactory.NewExecutor(), scenarioId)
	if err != nil {
		return nil, err
	}
	if len(testruns) > 0 {
		for _, testrun := range testruns {
			if err := usecase.enforceSecurity.ListTestRuns(testrun.OrganizationId); err != nil {
				return nil, err
			}
		}
	}
	return testruns, nil
}

func (usecase *ScenarioTestRunUsecase) GetTestRunById(ctx context.Context,
	testRunId, organizationId string,
) (models.ScenarioTestRun, error) {
	testrun, err := usecase.repository.GetTestRunByID(ctx,
		usecase.executorFactory.NewExecutor(), testRunId)
	if err != nil {
		return models.ScenarioTestRun{}, err
	}
	if testrun == nil {
		return models.ScenarioTestRun{}, nil
	}
	if err := usecase.enforceSecurity.ReadTestRun(testrun.OrganizationId); err != nil {
		return models.ScenarioTestRun{}, err
	}
	return *testrun, nil
}
