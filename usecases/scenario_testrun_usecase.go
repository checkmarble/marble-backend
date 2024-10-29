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

func (usecase *ScenarioTestRunUsecase) GetByScenarioIterationID(ctx context.Context,
	scenarioIterationID string,
) (*models.ScenarioTestRun, error) {
	return usecase.repository.GetByScenarioIterationID(ctx, scenarioIterationID)
}

func (usecase *ScenarioTestRunUsecase) ActivateScenarioTestRun(ctx context.Context,
	organizationId string,
	input models.ScenarioTestRunInput,
) (models.ScenarioTestRun, error) {
	// we should not have any existing testrun for this scenario
	existingTestrun, err := usecase.GetByScenarioIterationID(ctx, input.ScenarioIterationId)
	if err != nil {
		return models.ScenarioTestRun{}, errors.Wrap(err,
			"error while fecthing entries to find an existing testrun")
	}
	if existingTestrun != nil && existingTestrun.Status == models.Up {
		return models.ScenarioTestRun{}, errors.Wrap(models.ErrTestRunAlreadyExist,
			fmt.Sprintf("the scenario %s has a running testrun", input.ScenarioId))
	}

	// we should have a live version running
	scenarioPublications, errScenarioPubs := usecase.scenarioPublicationsRepository.ListScenarioPublicationsOfOrganization(
		ctx, usecase.executorFactory.NewExecutor(), organizationId, models.ListScenarioPublicationsFilters{
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

	createdTestrun, err := executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Transaction) (models.ScenarioTestRun, error) {
			testrunID := pure_utils.NewPrimaryKey(organizationId)
			if err := usecase.repository.CreateTestRun(ctx, tx, testrunID, input); err != nil {
				return models.ScenarioTestRun{}, err
			}
			result, err := usecase.repository.GetByScenarioIterationID(ctx, input.ScenarioIterationId)
			return *result, err
		},
	)
	return createdTestrun, nil
}
