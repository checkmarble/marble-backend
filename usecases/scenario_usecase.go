package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/scenarios"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/tracking"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
)

type ScenarioUsecase struct {
	transactionFactory  executor_factory.TransactionFactory
	scenarioFetcher     scenarios.ScenarioFetcher
	validateScenarioAst scenarios.ValidateScenarioAst
	executorFactory     executor_factory.ExecutorFactory
	enforceSecurity     security.EnforceSecurityScenario
	repository          repositories.ScenarioUsecaseRepository
	workflowRepository  workflowRepository
}

func (usecase *ScenarioUsecase) ListScenarios(ctx context.Context, organizationId uuid.UUID) ([]models.Scenario, error) {
	scenarios, err := usecase.repository.ListScenariosOfOrganization(ctx,
		usecase.executorFactory.NewExecutor(), organizationId)
	if err != nil {
		return nil, err
	}

	for _, scenario := range scenarios {
		if err := usecase.enforceSecurity.ReadScenario(scenario); err != nil {
			return nil, err
		}
	}
	return scenarios, nil
}

func (usecase *ScenarioUsecase) GetScenario(ctx context.Context, scenarioId string) (models.Scenario, error) {
	scenario, err := usecase.repository.GetScenarioById(ctx,
		usecase.executorFactory.NewExecutor(), scenarioId)
	if err != nil {
		return models.Scenario{}, err
	}

	if err := usecase.enforceSecurity.ReadScenario(scenario); err != nil {
		return models.Scenario{}, err
	}

	return scenario, nil
}

func (usecase *ScenarioUsecase) UpdateScenario(
	ctx context.Context,
	scenarioInput models.UpdateScenarioInput,
) (models.Scenario, error) {
	return executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Transaction) (models.Scenario, error) {
			scenario, err := usecase.repository.GetScenarioById(ctx, tx, scenarioInput.Id)
			if err != nil {
				return models.Scenario{}, err
			}

			if err := usecase.enforceSecurity.UpdateScenario(scenario); err != nil {
				return models.Scenario{}, err
			}

			err = usecase.repository.UpdateScenario(ctx, tx, scenarioInput)
			if err != nil {
				return models.Scenario{}, err
			}
			scenario, err = usecase.repository.GetScenarioById(ctx, tx, scenario.Id)
			return scenario, errors.HandledWithMessage(err, "Error getting scenario after update")
		},
	)
}

func (usecase *ScenarioUsecase) ValidateScenarioAst(ctx context.Context,
	scenarioId string, astNode *ast.Node, expectedReturnType ...string,
) (validation models.AstValidation, err error) {
	scenario, err := usecase.scenarioFetcher.FetchScenario(ctx,
		usecase.executorFactory.NewExecutor(), scenarioId)
	if err != nil {
		return validation, err
	}

	if err := usecase.enforceSecurity.ReadScenario(scenario); err != nil {
		return validation, err
	}

	validation = usecase.validateScenarioAst.Validate(ctx, scenario, astNode, expectedReturnType...)

	return validation, nil
}

func (usecase *ScenarioUsecase) CreateScenario(
	ctx context.Context,
	scenario models.CreateScenarioInput,
) (models.Scenario, error) {
	if err := usecase.enforceSecurity.CreateScenario(scenario.OrganizationId); err != nil {
		return models.Scenario{}, err
	}

	createdScenario, err := executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Transaction) (models.Scenario, error) {
			newScenarioId := pure_utils.NewPrimaryKey(scenario.OrganizationId)
			if err := usecase.repository.CreateScenario(ctx, tx, scenario.OrganizationId, scenario, newScenarioId); err != nil {
				return models.Scenario{}, err
			}
			scenario, err := usecase.repository.GetScenarioById(ctx, tx, newScenarioId)
			return scenario, errors.HandledWithMessage(err, "Error getting scenario after update")
		},
	)
	if err != nil {
		return models.Scenario{}, err
	}

	tracking.TrackEvent(ctx, models.AnalyticsScenarioCreated, map[string]interface{}{
		"scenario_id": createdScenario.Id,
	})
	return createdScenario, nil
}

func (usecase *ScenarioUsecase) ListLatestRules(ctx context.Context, scenarioId string) ([]models.ScenarioRuleLatestVersion, error) {
	scenario, err := usecase.repository.GetScenarioById(ctx,
		usecase.executorFactory.NewExecutor(), scenarioId)
	if err != nil {
		return nil, err
	}

	if err := usecase.enforceSecurity.ReadScenario(scenario); err != nil {
		return nil, err
	}

	return usecase.repository.ListScenarioLatestRuleVersions(ctx,
		usecase.executorFactory.NewExecutor(), scenarioId)
}
