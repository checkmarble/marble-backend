package usecases

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/scenarios"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

type WorkflowUsecase struct {
	executorFactory    executor_factory.ExecutorFactory
	enforceSecurity    security.EnforceSecurityScenario
	repository         workflowRepository
	scenarioRepository repositories.ScenarioUsecaseRepository

	validateScenarioAst scenarios.ValidateScenarioAst
}

type workflowRepository interface {
	ListWorkflowsForScenario(ctx context.Context, exec repositories.Executor, scenarioId uuid.UUID) ([]models.Workflow, error)
	GetWorkflowRule(ctx context.Context, exec repositories.Executor, id uuid.UUID) (models.WorkflowRule, error)
	GetWorkflowRuleDetails(ctx context.Context, exec repositories.Executor, id uuid.UUID) (models.Workflow, error)
	GetWorkflowCondition(ctx context.Context, exec repositories.Executor, id uuid.UUID) (models.WorkflowCondition, error)
	GetWorkflowAction(ctx context.Context, exec repositories.Executor, id uuid.UUID) (models.WorkflowAction, error)
	InsertWorkflowRule(ctx context.Context, exec repositories.Executor, rule models.WorkflowRule) (models.WorkflowRule, error)
	UpdateWorkflowRule(ctx context.Context, exec repositories.Executor, rule models.WorkflowRule) (models.WorkflowRule, error)
	DeleteWorkflowRule(ctx context.Context, exec repositories.Executor, ruleId uuid.UUID) error
	InsertWorkflowCondition(ctx context.Context, exec repositories.Executor, condition models.WorkflowCondition) (models.WorkflowCondition, error)
	UpdateWorkflowCondition(ctx context.Context, exec repositories.Executor, condition models.WorkflowCondition) (models.WorkflowCondition, error)
	DeleteWorkflowCondition(ctx context.Context, exec repositories.Executor, ruleId, conditionId uuid.UUID) error
	InsertWorkflowAction(ctx context.Context, exec repositories.Executor, action models.WorkflowAction) (models.WorkflowAction, error)
	UpdateWorkflowAction(ctx context.Context, exec repositories.Executor, action models.WorkflowAction) (models.WorkflowAction, error)
	DeleteWorkflowAction(ctx context.Context, exec repositories.Executor, ruleId, actionId uuid.UUID) error
	ReorderWorkflowRules(ctx context.Context, exec repositories.Executor, scenarioId uuid.UUID, ids []uuid.UUID) error
}

func (uc *WorkflowUsecase) ListWorkflowsForScenario(ctx context.Context, scenarioId uuid.UUID) ([]models.Workflow, error) {
	exec := uc.executorFactory.NewExecutor()

	scenario, err := uc.scenarioRepository.GetScenarioById(ctx, exec, scenarioId.String())
	if err != nil {
		return nil, err
	}

	if err := uc.enforceSecurity.ListScenarios(scenario.OrganizationId); err != nil {
		return nil, err
	}

	rules, err := uc.repository.ListWorkflowsForScenario(ctx, exec, scenarioId)
	if err != nil {
		return nil, err
	}

	return rules, nil
}

func (uc *WorkflowUsecase) GetWorkflowRule(ctx context.Context, ruleId uuid.UUID) (models.Workflow, error) {
	exec := uc.executorFactory.NewExecutor()

	workflow, err := uc.repository.GetWorkflowRuleDetails(ctx, exec, ruleId)
	if err != nil {
		return models.Workflow{}, err
	}

	scenario, err := uc.scenarioRepository.GetScenarioById(ctx, exec, workflow.ScenarioId.String())
	if err != nil {
		return models.Workflow{}, err
	}

	if err := uc.enforceSecurity.ReadScenario(scenario); err != nil {
		return models.Workflow{}, err
	}

	return workflow, nil
}

func (uc *WorkflowUsecase) CreateWorkflowRule(ctx context.Context, rule models.WorkflowRule) (models.WorkflowRule, error) {
	exec := uc.executorFactory.NewExecutor()

	scenario, err := uc.scenarioRepository.GetScenarioById(ctx, exec, rule.ScenarioId.String())
	if err != nil {
		return models.WorkflowRule{}, err
	}

	if err := uc.enforceSecurity.PublishScenario(scenario); err != nil {
		return models.WorkflowRule{}, err
	}

	return uc.repository.InsertWorkflowRule(ctx, exec, rule)
}

func (uc *WorkflowUsecase) UpdateWorkflowRule(ctx context.Context, ruleUpdate models.WorkflowRule) (models.WorkflowRule, error) {
	exec := uc.executorFactory.NewExecutor()

	rule, err := uc.repository.GetWorkflowRule(ctx, exec, ruleUpdate.Id)
	if err != nil {
		return models.WorkflowRule{}, err
	}

	scenario, err := uc.scenarioRepository.GetScenarioById(ctx, exec, rule.ScenarioId.String())
	if err != nil {
		return models.WorkflowRule{}, err
	}

	if err := uc.enforceSecurity.PublishScenario(scenario); err != nil {
		return models.WorkflowRule{}, err
	}

	return uc.repository.UpdateWorkflowRule(ctx, exec, ruleUpdate)
}

func (uc *WorkflowUsecase) DeleteWorkflowRule(ctx context.Context, ruleId uuid.UUID) error {
	exec := uc.executorFactory.NewExecutor()

	rule, err := uc.repository.GetWorkflowRule(ctx, exec, ruleId)
	if err != nil {
		return err
	}

	scenario, err := uc.scenarioRepository.GetScenarioById(ctx, exec, rule.ScenarioId.String())
	if err != nil {
		return err
	}

	if err := uc.enforceSecurity.PublishScenario(scenario); err != nil {
		return err
	}

	return uc.repository.DeleteWorkflowRule(ctx, exec, ruleId)
}

func (uc *WorkflowUsecase) CreateWorkflowCondition(ctx context.Context, cond models.WorkflowCondition) (models.WorkflowCondition, error) {
	exec := uc.executorFactory.NewExecutor()

	rule, err := uc.repository.GetWorkflowRule(ctx, exec, cond.RuleId)
	if err != nil {
		return models.WorkflowCondition{}, err
	}

	scenario, err := uc.scenarioRepository.GetScenarioById(ctx, exec, rule.ScenarioId.String())
	if err != nil {
		return models.WorkflowCondition{}, err
	}

	if err := uc.ValidateWorkflowCondition(ctx, scenario, cond); err != nil {
		return models.WorkflowCondition{}, err
	}

	if err := uc.enforceSecurity.PublishScenario(scenario); err != nil {
		return models.WorkflowCondition{}, err
	}

	return uc.repository.InsertWorkflowCondition(ctx, exec, cond)
}

func (uc *WorkflowUsecase) UpdateWorkflowCondition(ctx context.Context, cond models.WorkflowCondition) (models.WorkflowCondition, error) {
	exec := uc.executorFactory.NewExecutor()

	rule, err := uc.repository.GetWorkflowRule(ctx, exec, cond.RuleId)
	if err != nil {
		return models.WorkflowCondition{}, err
	}
	if cond.RuleId != rule.Id {
		return models.WorkflowCondition{}, errors.Wrap(models.NotFoundError, "could not find condition linked to rule")
	}

	scenario, err := uc.scenarioRepository.GetScenarioById(ctx, exec, rule.ScenarioId.String())
	if err != nil {
		return models.WorkflowCondition{}, err
	}

	if err := uc.ValidateWorkflowCondition(ctx, scenario, cond); err != nil {
		return models.WorkflowCondition{}, err
	}

	if err := uc.enforceSecurity.PublishScenario(scenario); err != nil {
		return models.WorkflowCondition{}, err
	}

	return uc.repository.UpdateWorkflowCondition(ctx, exec, cond)
}

func (uc *WorkflowUsecase) DeleteWorkflowCondition(ctx context.Context, ruleId, conditionId uuid.UUID) error {
	exec := uc.executorFactory.NewExecutor()

	condition, err := uc.repository.GetWorkflowCondition(ctx, exec, conditionId)
	if err != nil {
		return err
	}

	rule, err := uc.repository.GetWorkflowRule(ctx, exec, condition.RuleId)
	if err != nil {
		return err
	}
	if ruleId != rule.Id {
		return errors.Wrap(models.NotFoundError, "could not find condition linked to rule")
	}

	scenario, err := uc.scenarioRepository.GetScenarioById(ctx, exec, rule.ScenarioId.String())
	if err != nil {
		return err
	}

	if err := uc.enforceSecurity.PublishScenario(scenario); err != nil {
		return err
	}

	return uc.repository.DeleteWorkflowCondition(ctx, exec, ruleId, conditionId)
}

func (uc *WorkflowUsecase) CreateWorkflowAction(ctx context.Context, action models.WorkflowAction) (models.WorkflowAction, error) {
	exec := uc.executorFactory.NewExecutor()

	rule, err := uc.repository.GetWorkflowRule(ctx, exec, action.RuleId)
	if err != nil {
		return models.WorkflowAction{}, err
	}

	scenario, err := uc.scenarioRepository.GetScenarioById(ctx, exec, rule.ScenarioId.String())
	if err != nil {
		return models.WorkflowAction{}, err
	}

	if err := uc.ValidateWorkflowAction(ctx, scenario, action); err != nil {
		return models.WorkflowAction{}, err
	}

	if err := uc.enforceSecurity.PublishScenario(scenario); err != nil {
		return models.WorkflowAction{}, err
	}

	return uc.repository.InsertWorkflowAction(ctx, exec, action)
}

func (uc *WorkflowUsecase) UpdateWorkflowAction(ctx context.Context, action models.WorkflowAction) (models.WorkflowAction, error) {
	exec := uc.executorFactory.NewExecutor()

	rule, err := uc.repository.GetWorkflowRule(ctx, exec, action.RuleId)
	if err != nil {
		return models.WorkflowAction{}, err
	}
	if action.RuleId != rule.Id {
		return models.WorkflowAction{}, errors.Wrap(models.NotFoundError, "could not find condition linked to rule")
	}

	scenario, err := uc.scenarioRepository.GetScenarioById(ctx, exec, rule.ScenarioId.String())
	if err != nil {
		return models.WorkflowAction{}, err
	}

	if err := uc.ValidateWorkflowAction(ctx, scenario, action); err != nil {
		return models.WorkflowAction{}, err
	}

	if err := uc.enforceSecurity.PublishScenario(scenario); err != nil {
		return models.WorkflowAction{}, err
	}

	return uc.repository.UpdateWorkflowAction(ctx, exec, action)
}

func (uc *WorkflowUsecase) DeleteWorkflowAction(ctx context.Context, ruleId, actionId uuid.UUID) error {
	exec := uc.executorFactory.NewExecutor()

	condition, err := uc.repository.GetWorkflowAction(ctx, exec, actionId)
	if err != nil {
		return err
	}

	rule, err := uc.repository.GetWorkflowRule(ctx, exec, condition.RuleId)
	if err != nil {
		return err
	}
	if ruleId != rule.Id {
		return errors.Wrap(models.NotFoundError, "could not find condition linked to rule")
	}

	scenario, err := uc.scenarioRepository.GetScenarioById(ctx, exec, rule.ScenarioId.String())
	if err != nil {
		return err
	}

	if err := uc.enforceSecurity.PublishScenario(scenario); err != nil {
		return err
	}

	return uc.repository.DeleteWorkflowAction(ctx, exec, ruleId, actionId)
}

func (uc *WorkflowUsecase) ReorderWorkflowRules(ctx context.Context, scenarioId uuid.UUID, ids []uuid.UUID) error {
	exec := uc.executorFactory.NewExecutor()

	scenario, err := uc.scenarioRepository.GetScenarioById(ctx, exec, scenarioId.String())
	if err != nil {
		return err
	}

	if err := uc.enforceSecurity.PublishScenario(scenario); err != nil {
		return err
	}

	return uc.repository.ReorderWorkflowRules(ctx, exec, scenarioId, ids)
}

func (uc *WorkflowUsecase) ValidateWorkflowCondition(ctx context.Context, scenario models.Scenario, cond models.WorkflowCondition) error {
	switch cond.Function {
	case
		models.WorkflowConditionAlways,
		models.WorkflowConditionNever:

		if cond.Params != nil {
			return errors.Wrapf(models.BadParameterError, "workflow condition %s does not take parameters", cond.Function)
		}
	case models.WorkflowConditionOutcomeIn:
		var params []string

		if err := json.Unmarshal(cond.Params, &params); err != nil {
			return errors.Wrap(models.BadParameterError, err.Error())
		}

		if len(params) == 0 {
			return errors.Wrap(models.BadParameterError, "at least one outcome must be provided")
		}
		for _, outcome := range params {
			if models.OutcomeFrom(outcome) == models.UnknownOutcome {
				return errors.Wrap(models.BadParameterError, fmt.Sprintf("invalid outcome '%s'", outcome))
			}
		}
	case models.WorkflowConditionRuleHit:
		var params dto.WorkflowConditionRuleHitParams

		if err := json.Unmarshal(cond.Params, &params); err != nil {
			return errors.Wrap(models.BadParameterError, err.Error())
		}
		if err := binding.Validator.Engine().(*validator.Validate).Struct(params); err != nil {
			return errors.Wrap(models.BadParameterError, err.Error())
		}
	case models.WorkflowPayloadEvaluates:
		var params dto.WorkflowConditionEvaluatesParams

		if err := json.Unmarshal(cond.Params, &params); err != nil {
			return errors.Wrap(models.BadParameterError, err.Error())
		}
		if err := binding.Validator.Engine().(*validator.Validate).Struct(params); err != nil {
			return errors.Wrap(models.BadParameterError, err.Error())
		}

		astNode, err := dto.AdaptASTNode(params.Expression)
		if err != nil {
			return errors.Wrap(models.BadParameterError, err.Error())
		}

		validation := uc.validateScenarioAst.Validate(ctx, scenario, &astNode, "bool")

		if len(validation.Errors) > 0 {
			return errors.Wrap(errors.Join(
				pure_utils.Map(validation.Errors, func(e models.ScenarioValidationError) error { return e.Error })...),
				"invalid AST in field 'expression'")
		}
	default:
		return errors.Wrapf(models.BadParameterError, "unknown workflow condition type: %s", cond.Function)
	}

	return nil
}

func (uc *WorkflowUsecase) ValidateWorkflowAction(ctx context.Context, scenario models.Scenario, cond models.WorkflowAction) error {
	switch cond.Action {
	case models.WorkflowCreateCase, models.WorkflowAddToCaseIfPossible:
		var params dto.WorkflowActionCaseParams

		if err := json.Unmarshal(cond.Params, &params); err != nil {
			return errors.Wrap(models.BadParameterError, err.Error())
		}
		if err := binding.Validator.Engine().(*validator.Validate).Struct(params); err != nil {
			return errors.Wrap(models.BadParameterError, err.Error())
		}

		if params.TitleTemplate != nil {
			astNode, err := dto.AdaptASTNode(*params.TitleTemplate)
			if err != nil {
				return errors.Wrap(models.BadParameterError, err.Error())
			}

			validation := uc.validateScenarioAst.Validate(ctx, scenario, &astNode, "string")

			if len(validation.Errors) > 0 {
				return errors.Wrap(errors.Join(
					pure_utils.Map(validation.Errors, func(e models.ScenarioValidationError) error { return e.Error })...),
					"invalid AST in field 'title_template'")
			}
		}
	case models.WorkflowDisabled:
		return nil
	default:
		return errors.Wrapf(models.BadParameterError, "unknown workflow action type: %s", cond.Action)
	}

	return nil
}
