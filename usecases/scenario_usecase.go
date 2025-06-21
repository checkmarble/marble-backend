package usecases

import (
	"context"
	"slices"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/scenarios"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/tracking"

	"github.com/cockroachdb/errors"
)

type workflowRepository interface {
	ListWorkflowsForScenario(ctx context.Context, exec repositories.Executor, scenarioId string) ([]models.Workflow, error)
	GetWorkflowRule(ctx context.Context, exec repositories.Executor, id string) (models.WorkflowRule, error)
	GetWorkflowCondition(ctx context.Context, exec repositories.Executor, id string) (models.WorkflowCondition, error)
	GetWorkflowAction(ctx context.Context, exec repositories.Executor, id string) (models.WorkflowAction, error)
	InsertWorkflowRule(ctx context.Context, exec repositories.Executor, rule models.WorkflowRule) (models.WorkflowRule, error)
	InsertWorkflowCondition(ctx context.Context, exec repositories.Executor, rule models.WorkflowCondition) (models.WorkflowCondition, error)
	InsertWorkflowAction(ctx context.Context, exec repositories.Executor, rule models.WorkflowAction) (models.WorkflowAction, error)
}

type ScenarioUsecase struct {
	transactionFactory  executor_factory.TransactionFactory
	scenarioFetcher     scenarios.ScenarioFetcher
	validateScenarioAst scenarios.ValidateScenarioAst
	executorFactory     executor_factory.ExecutorFactory
	enforceSecurity     security.EnforceSecurityScenario
	repository          repositories.ScenarioUsecaseRepository
	workflowRepository  workflowRepository
}

func (usecase *ScenarioUsecase) ListScenarios(ctx context.Context, organizationId string) ([]models.Scenario, error) {
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

			// the DecisionToCaseInboxId and DecisionToCaseOutcomes settings are of higher criticity (they
			// influence how decisions are treated) so require a higher permission to update
			changeWorkflowSettings := scenarioInput.DecisionToCaseInboxId.Set ||
				scenarioInput.DecisionToCaseOutcomes != nil ||
				scenarioInput.DecisionToCaseWorkflowType != nil ||
				scenarioInput.DecisionToCaseNameTemplate != nil
			if changeWorkflowSettings {
				if err := usecase.enforceSecurity.PublishScenario(scenario); err != nil {
					return models.Scenario{}, err
				}
			}

			if err := validateScenarioUpdate(scenario, scenarioInput); err != nil {
				return models.Scenario{}, err
			}

			if scenarioInput.DecisionToCaseNameTemplate != nil {
				validation, err := usecase.ValidateScenarioAst(ctx, scenarioInput.Id,
					scenarioInput.DecisionToCaseNameTemplate, "string")
				if err != nil {
					return models.Scenario{}, err
				}
				if len(validation.Errors) > 0 || len(validation.Evaluation.FlattenErrors()) > 0 {
					errs := append(
						validation.Evaluation.FlattenErrors(),
						pure_utils.Map(
							validation.Errors, func(err models.ScenarioValidationError) error {
								return err.Error
							})...,
					)
					return models.Scenario{}, errors.Join(errs...)
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

func validateScenarioUpdate(scenario models.Scenario, input models.UpdateScenarioInput) error {
	// start by simple input sanity checks
	for _, outcome := range input.DecisionToCaseOutcomes {
		if !slices.Contains(models.ValidOutcomes, outcome) {
			return errors.Wrapf(
				models.BadParameterError,
				"Invalid input outcome: %s", outcome)
		}
	}
	workflowType := input.DecisionToCaseWorkflowType
	if workflowType != nil && !slices.Contains(models.ValidWorkflowTypes, *workflowType) {
		return errors.Wrapf(models.BadParameterError,
			"Invalid input workflow type: %s", *workflowType)
	}

	// next compute the new scenario, after updates
	if input.DecisionToCaseInboxId.Set {
		scenario.DecisionToCaseInboxId = input.DecisionToCaseInboxId.Ptr()
	}
	if input.DecisionToCaseOutcomes != nil {
		scenario.DecisionToCaseOutcomes = input.DecisionToCaseOutcomes
	}
	if input.DecisionToCaseWorkflowType != nil {
		scenario.DecisionToCaseWorkflowType = *input.DecisionToCaseWorkflowType
	}

	// now validate that the new scenario is valid
	if scenario.DecisionToCaseWorkflowType != models.WorkflowDisabled &&
		(scenario.DecisionToCaseInboxId == nil || len(scenario.DecisionToCaseOutcomes) == 0) {
		return errors.Wrap(models.BadParameterError,
			"DecisionToCaseInboxId and DecisionToCaseOutcomes are required when DecisionToCaseWorkflowType is not DISABLED")
	}

	return nil
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

func (uc *ScenarioUsecase) ListWorkflowsForScenario(ctx context.Context, scenarioId string) ([]models.Workflow, error) {
	exec := uc.executorFactory.NewExecutor()

	if err := uc.enforceSecurity.ListScenarios(uc.enforceSecurity.OrgId()); err != nil {
		return nil, err
	}

	rules, err := uc.workflowRepository.ListWorkflowsForScenario(ctx, exec, scenarioId)
	if err != nil {
		return nil, err
	}

	return rules, nil
}

func (uc *ScenarioUsecase) CreateWorkflowRule(ctx context.Context, orgId string, rule models.WorkflowRule) (models.WorkflowRule, error) {
	exec := uc.executorFactory.NewExecutor()

	scenario, err := uc.repository.GetScenarioById(ctx, exec, rule.ScenarioId)
	if err != nil {
		return models.WorkflowRule{}, err
	}

	if err := uc.enforceSecurity.PublishScenario(scenario); err != nil {
		return models.WorkflowRule{}, err
	}

	return uc.workflowRepository.InsertWorkflowRule(ctx, uc.executorFactory.NewExecutor(), rule)
}

func (uc *ScenarioUsecase) CreateWorkflowCondition(ctx context.Context, orgId string, cond models.WorkflowCondition) (models.WorkflowCondition, error) {
	exec := uc.executorFactory.NewExecutor()

	rule, err := uc.workflowRepository.GetWorkflowRule(ctx, exec, cond.RuleId)
	if err != nil {
		return models.WorkflowCondition{}, err
	}

	scenario, err := uc.repository.GetScenarioById(ctx, exec, rule.ScenarioId)
	if err != nil {
		return models.WorkflowCondition{}, err
	}

	if err := uc.enforceSecurity.PublishScenario(scenario); err != nil {
		return models.WorkflowCondition{}, err
	}

	return uc.workflowRepository.InsertWorkflowCondition(ctx, exec, cond)
}
func (uc *ScenarioUsecase) CreateWorkflowAction(ctx context.Context, orgId string, action models.WorkflowAction) (models.WorkflowAction, error) {
	exec := uc.executorFactory.NewExecutor()

	rule, err := uc.workflowRepository.GetWorkflowRule(ctx, exec, action.RuleId)
	if err != nil {
		return models.WorkflowAction{}, err
	}

	scenario, err := uc.repository.GetScenarioById(ctx, exec, rule.ScenarioId)
	if err != nil {
		return models.WorkflowAction{}, err
	}

	if err := uc.enforceSecurity.PublishScenario(scenario); err != nil {
		return models.WorkflowAction{}, err
	}

	return uc.workflowRepository.InsertWorkflowAction(ctx, exec, action)
}
