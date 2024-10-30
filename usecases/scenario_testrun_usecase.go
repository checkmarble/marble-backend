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
	transactionFactory             executor_factory.TransactionFactory
	executorFactory                executor_factory.ExecutorFactory
	scenarioPublicationsRepository repositories.ScenarioPublicationRepository
	enforceSecurity                security.EnforceSecurityScenario
	clientDbIndexEditor            clientDbIndexEditor
	repository                     repositories.ScenarioTestRunRepository
}

func (usecase *ScenarioTestRunUsecase) ActivateScenarioTestRun(ctx context.Context,
	organizationId string,
	input models.ScenarioTestRunInput,
) (models.ScenarioTestRun, error) {
	exec := usecase.executorFactory.NewExecutor()
	// we should not have any existing testrun for this scenario
	existingTestrun, err := usecase.repository.GetByScenarioIterationID(ctx, exec, input.ScenarioIterationId)
	if err != nil {
		return models.ScenarioTestRun{}, errors.Wrap(err,
			"error while fecthing entries to find an existing testrun")
	}
	if existingTestrun.ScenarioIterationId != "" && existingTestrun.Status == models.Up {
		return models.ScenarioTestRun{}, errors.Wrap(models.ErrTestRunAlreadyExist,
			fmt.Sprintf("the scenario %s has a running testrun", input.ScenarioId))
	}

	// we should have a live version running
	scenarioPublications, errScenarioPubs := usecase.scenarioPublicationsRepository.ListScenarioPublicationsOfOrganization(
		ctx, exec, organizationId, models.ListScenarioPublicationsFilters{
			ScenarioId: &input.ScenarioId,
		})
	if errScenarioPubs != nil {
		return models.ScenarioTestRun{}, errScenarioPubs
	}
	if len(scenarioPublications) == 0 {
		return models.ScenarioTestRun{}, models.ErrScenarioHasNoLiveVersion
	}
	for _, scenarioPublication := range scenarioPublications {
		if scenarioPublication.ScenarioIterationId == input.ScenarioIterationId {
			return models.ScenarioTestRun{}, models.ErrWrongIterationForTestRun
		}
	}

	return executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Transaction) (models.ScenarioTestRun, error) {
			testrunID := pure_utils.NewPrimaryKey(organizationId)
			if err := usecase.repository.CreateTestRun(ctx, tx, testrunID, input); err != nil {
				return models.ScenarioTestRun{}, err
			}
			result, err := usecase.repository.GetByID(ctx, exec, testrunID)
			return result, err
		},
	)
}
