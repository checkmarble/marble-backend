package usecases

import (
	"context"
	"fmt"
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/tracking"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/cockroachdb/errors"
)

type ScenarioUsecaseRepository interface {
	GetScenarioById(ctx context.Context, exec repositories.Executor, scenarioId string) (models.Scenario, error)
	ListScenariosOfOrganization(ctx context.Context, exec repositories.Executor, organizationId string) ([]models.Scenario, error)
	CreateScenario(ctx context.Context, exec repositories.Executor, organizationId string,
		scenario models.CreateScenarioInput, newScenarioId string) error
	UpdateScenario(ctx context.Context, exec repositories.Executor,
		scenario models.UpdateScenarioInput) error
}

type ScenarioUsecase struct {
	transactionFactory      executor_factory.TransactionFactory
	executorFactory         executor_factory.ExecutorFactory
	organizationIdOfContext func() (string, error)
	enforceSecurity         security.EnforceSecurityScenario
	repository              ScenarioUsecaseRepository
}

func (usecase *ScenarioUsecase) ListScenarios(ctx context.Context) ([]models.Scenario, error) {
	organizationId, err := usecase.organizationIdOfContext()
	if err != nil {
		return nil, err
	}
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
	for _, outcome := range scenarioInput.DecisionToCaseOutcomes {
		if !slices.Contains(models.ValidOutcomes, outcome) {
			return models.Scenario{}, errors.Wrap(
				models.BadParameterError,
				fmt.Sprintf("Invalid input outcome: %s", outcome))
		}
	}

	return executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Executor) (models.Scenario, error) {
			scenario, err := usecase.repository.GetScenarioById(ctx, tx, scenarioInput.Id)
			if err != nil {
				return models.Scenario{}, err
			}
			if err := usecase.enforceSecurity.UpdateScenario(scenario); err != nil {
				return models.Scenario{}, err
			}
			// the DecisionToCaseInboxId and DecisionToCaseOutcomes settings are of higher criticity (they
			// influence how decisions are treated) so require a higher permission to update
			if scenarioInput.DecisionToCaseInboxId.Valid ||
				scenarioInput.DecisionToCaseOutcomes != nil {
				if err := usecase.enforceSecurity.PublishScenario(scenario); err != nil {
					return models.Scenario{}, err
				}
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

func (usecase *ScenarioUsecase) CreateScenario(ctx context.Context,
	scenario models.CreateScenarioInput,
) (models.Scenario, error) {
	organizationId, err := usecase.organizationIdOfContext()
	if err != nil {
		return models.Scenario{}, err
	}

	if err := usecase.enforceSecurity.CreateScenario(organizationId); err != nil {
		return models.Scenario{}, err
	}

	cratedScenario, err := executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Executor) (models.Scenario, error) {
			newScenarioId := utils.NewPrimaryKey(organizationId)
			if err := usecase.repository.CreateScenario(ctx, tx, organizationId, scenario, newScenarioId); err != nil {
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
		"scenario_id": cratedScenario.Id,
	})
	return cratedScenario, nil
}
