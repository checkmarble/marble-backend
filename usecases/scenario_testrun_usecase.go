package usecases

import (
	"context"

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

func (usecases *UsecasesWithCreds) NewScenarioTestRunUseCase() ScenarioTestRunUsecase {
	return ScenarioTestRunUsecase{
		transactionFactory:  usecases.NewTransactionFactory(),
		executorFactory:     usecases.NewExecutorFactory(),
		enforceSecurity:     usecases.NewEnforceTestRunScenarioSecurity(),
		repository:          &usecases.Repositories.MarbleDbRepository,
		clientDbIndexEditor: usecases.NewClientDbIndexEditor(),
		scenarioRepository:  &usecases.Repositories.MarbleDbRepository,
	}
}

func (usecase *ScenarioTestRunUsecase) CreateScenarioTestRun(
	ctx context.Context,
	organizationId string,
	input models.ScenarioTestRunInput,
) (models.ScenarioTestRun, error) {
	if err := usecase.enforceSecurity.CreateTestRun(organizationId); err != nil {
		return models.ScenarioTestRun{}, err
	}
	exec := usecase.executorFactory.NewExecutor()

	// we should have a live version running
	scenario, err := usecase.scenarioRepository.GetScenarioById(ctx, exec, input.ScenarioId)
	if err != nil {
		return models.ScenarioTestRun{}, err
	}
	if scenario.LiveVersionID == nil {
		return models.ScenarioTestRun{}, models.ErrScenarioHasNoLiveVersion
	}
	// the live version must not be the one on which we want to start a testrun
	if *scenario.LiveVersionID == input.PhantomIterationId {
		return models.ScenarioTestRun{}, models.ErrWrongIterationForTestRun
	}

	// we should not have any existing testrun for this scenario
	testRuns, err := usecase.repository.ListRunningTestRun(ctx, exec, organizationId)
	if err != nil {
		return models.ScenarioTestRun{}, err
	}
	if len(testRuns) > 0 {
		return models.ScenarioTestRun{}, errors.Wrapf(models.ErrTestRunAlreadyExist,
			"the scenario %s has a running testrun", input.ScenarioId)
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

	// keep track of the live version associated to the current testrun
	repoInput := input.CreateDbInput(*scenario.LiveVersionID)
	testRunId := pure_utils.NewPrimaryKey(organizationId)

	tr, err := executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Transaction) (models.ScenarioTestRun, error) {
			if err := usecase.repository.CreateTestRun(ctx, tx, testRunId, repoInput); err != nil {
				return models.ScenarioTestRun{}, err
			}
			result, err := usecase.repository.GetTestRunByID(ctx, tx, testRunId)
			if err != nil {
				return models.ScenarioTestRun{}, err
			}
			return result, nil
		},
	)
	if err != nil {
		return models.ScenarioTestRun{}, err
	}

	// finally, create the indexes, and update the status asynchronously. This is error prone, and an improvement is planned (creating
	// the test run in a task queue)
	err = usecase.clientDbIndexEditor.CreateIndexesAsyncForScenarioWithCallback(
		ctx,
		organizationId,
		indexesToCreate,
		func(ctx context.Context) error {
			return usecase.repository.UpdateTestRunStatus(ctx, exec, testRunId, models.Up)
		})
	if err != nil {
		return models.ScenarioTestRun{}, errors.Wrap(err,
			"Error while creating indexes in ActivateScenarioTestRun")
	}

	return tr, nil
}

func (usecase *ScenarioTestRunUsecase) ListTestRunByScenarioId(ctx context.Context,
	scenarioId string,
) ([]models.ScenarioTestRun, error) {
	exec := usecase.executorFactory.NewExecutor()
	testruns, err := usecase.repository.ListTestRunsByScenarioID(ctx, exec, scenarioId)
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
	testRunId string,
) (models.ScenarioTestRun, error) {
	testrun, err := usecase.repository.GetTestRunByID(
		ctx,
		usecase.executorFactory.NewExecutor(),
		testRunId)
	if err != nil {
		return models.ScenarioTestRun{}, err
	}
	if err := usecase.enforceSecurity.ReadTestRun(testrun.OrganizationId); err != nil {
		return models.ScenarioTestRun{}, err
	}
	return testrun, nil
}

func (usecase *ScenarioTestRunUsecase) CancelTestRunById(ctx context.Context,
	testRunId string,
) (models.ScenarioTestRun, error) {
	exec := usecase.executorFactory.NewExecutor()
	testRun, err := usecase.repository.GetTestRunByID(
		ctx,
		exec,
		testRunId)
	if err != nil {
		return models.ScenarioTestRun{}, err
	}
	if err := usecase.enforceSecurity.ReadTestRun(testRun.OrganizationId); err != nil {
		return models.ScenarioTestRun{}, err
	}
	if testRun.Status != models.Down {
		if err := usecase.repository.UpdateTestRunStatus(ctx,
			exec, testRunId, models.Down); err != nil {
			return models.ScenarioTestRun{}, err
		}
		updatedTestRun, err := usecase.repository.GetTestRunByID(
			ctx,
			exec,
			testRunId)
		if err != nil {
			return models.ScenarioTestRun{}, err
		}
		return updatedTestRun, nil
	}
	return testRun, nil
}
