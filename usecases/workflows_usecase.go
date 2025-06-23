package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
	"github.com/pkg/errors"
)

type workflowRepository interface {
	ListWorkflowsForScenario(ctx context.Context, exec repositories.Executor, scenarioId string) ([]models.Workflow, error)
	GetWorkflowRule(ctx context.Context, exec repositories.Executor, id string) (models.WorkflowRule, error)
	GetWorkflowCondition(ctx context.Context, exec repositories.Executor, id string) (models.WorkflowCondition, error)
	GetWorkflowAction(ctx context.Context, exec repositories.Executor, id string) (models.WorkflowAction, error)
	InsertWorkflowRule(ctx context.Context, exec repositories.Executor, rule models.WorkflowRule) (models.WorkflowRule, error)
	UpdateWorkflowRule(ctx context.Context, exec repositories.Executor, rule models.WorkflowRule) (models.WorkflowRule, error)
	DeleteWorkflowRule(ctx context.Context, exec repositories.Executor, ruleId string) error
	InsertWorkflowCondition(ctx context.Context, exec repositories.Executor, condition models.WorkflowCondition) (models.WorkflowCondition, error)
	UpdateWorkflowCondition(ctx context.Context, exec repositories.Executor, condition models.WorkflowCondition) (models.WorkflowCondition, error)
	DeleteWorkflowCondition(ctx context.Context, exec repositories.Executor, ruleId, conditionId string) error
	InsertWorkflowAction(ctx context.Context, exec repositories.Executor, action models.WorkflowAction) (models.WorkflowAction, error)
	UpdateWorkflowAction(ctx context.Context, exec repositories.Executor, action models.WorkflowAction) (models.WorkflowAction, error)
	DeleteWorkflowAction(ctx context.Context, exec repositories.Executor, ruleId, actionId string) error
	ReorderWorkflowRules(ctx context.Context, exec repositories.Executor, scenarioId string, ids []uuid.UUID) error
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

func (uc *ScenarioUsecase) CreateWorkflowRule(ctx context.Context, rule models.WorkflowRule) (models.WorkflowRule, error) {
	exec := uc.executorFactory.NewExecutor()

	scenario, err := uc.repository.GetScenarioById(ctx, exec, rule.ScenarioId)
	if err != nil {
		return models.WorkflowRule{}, err
	}

	if err := uc.enforceSecurity.PublishScenario(scenario); err != nil {
		return models.WorkflowRule{}, err
	}

	return uc.workflowRepository.InsertWorkflowRule(ctx, exec, rule)
}

func (uc *ScenarioUsecase) UpdateWorkflowRule(ctx context.Context, ruleUpdate models.WorkflowRule) (models.WorkflowRule, error) {
	exec := uc.executorFactory.NewExecutor()

	rule, err := uc.workflowRepository.GetWorkflowRule(ctx, exec, ruleUpdate.Id)
	if err != nil {
		return models.WorkflowRule{}, err
	}

	scenario, err := uc.repository.GetScenarioById(ctx, exec, rule.ScenarioId)
	if err != nil {
		return models.WorkflowRule{}, err
	}

	if err := uc.enforceSecurity.PublishScenario(scenario); err != nil {
		return models.WorkflowRule{}, err
	}

	return uc.workflowRepository.UpdateWorkflowRule(ctx, exec, ruleUpdate)
}

func (uc *ScenarioUsecase) DeleteWorkflowRule(ctx context.Context, ruleId string) error {
	exec := uc.executorFactory.NewExecutor()

	rule, err := uc.workflowRepository.GetWorkflowRule(ctx, exec, ruleId)
	if err != nil {
		return err
	}

	scenario, err := uc.repository.GetScenarioById(ctx, exec, rule.ScenarioId)
	if err != nil {
		return err
	}

	if err := uc.enforceSecurity.PublishScenario(scenario); err != nil {
		return err
	}

	return uc.workflowRepository.DeleteWorkflowRule(ctx, exec, ruleId)
}

func (uc *ScenarioUsecase) CreateWorkflowCondition(ctx context.Context, cond models.WorkflowCondition) (models.WorkflowCondition, error) {
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

func (uc *ScenarioUsecase) UpdateWorkflowCondition(ctx context.Context, cond models.WorkflowCondition) (models.WorkflowCondition, error) {
	exec := uc.executorFactory.NewExecutor()

	rule, err := uc.workflowRepository.GetWorkflowRule(ctx, exec, cond.RuleId)
	if err != nil {
		return models.WorkflowCondition{}, err
	}
	if cond.RuleId != rule.Id {
		return models.WorkflowCondition{}, errors.Wrap(models.NotFoundError, "could not find condition linked to rule")
	}

	scenario, err := uc.repository.GetScenarioById(ctx, exec, rule.ScenarioId)
	if err != nil {
		return models.WorkflowCondition{}, err
	}

	if err := uc.enforceSecurity.PublishScenario(scenario); err != nil {
		return models.WorkflowCondition{}, err
	}

	return uc.workflowRepository.UpdateWorkflowCondition(ctx, exec, cond)
}

func (uc *ScenarioUsecase) DeleteWorkflowCondition(ctx context.Context, ruleId, conditionId string) error {
	exec := uc.executorFactory.NewExecutor()

	condition, err := uc.workflowRepository.GetWorkflowCondition(ctx, exec, conditionId)
	if err != nil {
		return err
	}

	rule, err := uc.workflowRepository.GetWorkflowRule(ctx, exec, condition.RuleId)
	if err != nil {
		return err
	}
	if ruleId != rule.Id {
		return errors.Wrap(models.NotFoundError, "could not find condition linked to rule")
	}

	scenario, err := uc.repository.GetScenarioById(ctx, exec, rule.ScenarioId)
	if err != nil {
		return err
	}

	if err := uc.enforceSecurity.PublishScenario(scenario); err != nil {
		return err
	}

	return uc.workflowRepository.DeleteWorkflowCondition(ctx, exec, ruleId, conditionId)
}

func (uc *ScenarioUsecase) CreateWorkflowAction(ctx context.Context, action models.WorkflowAction) (models.WorkflowAction, error) {
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

func (uc *ScenarioUsecase) UpdateWorkflowAction(ctx context.Context, action models.WorkflowAction) (models.WorkflowAction, error) {
	exec := uc.executorFactory.NewExecutor()

	rule, err := uc.workflowRepository.GetWorkflowRule(ctx, exec, action.RuleId)
	if err != nil {
		return models.WorkflowAction{}, err
	}
	if action.RuleId != rule.Id {
		return models.WorkflowAction{}, errors.Wrap(models.NotFoundError, "could not find condition linked to rule")
	}

	scenario, err := uc.repository.GetScenarioById(ctx, exec, rule.ScenarioId)
	if err != nil {
		return models.WorkflowAction{}, err
	}

	if err := uc.enforceSecurity.PublishScenario(scenario); err != nil {
		return models.WorkflowAction{}, err
	}

	return uc.workflowRepository.UpdateWorkflowAction(ctx, exec, action)
}

func (uc *ScenarioUsecase) DeleteWorkflowAction(ctx context.Context, ruleId, actionId string) error {
	exec := uc.executorFactory.NewExecutor()

	condition, err := uc.workflowRepository.GetWorkflowAction(ctx, exec, actionId)
	if err != nil {
		return err
	}

	rule, err := uc.workflowRepository.GetWorkflowRule(ctx, exec, condition.RuleId)
	if err != nil {
		return err
	}
	if ruleId != rule.Id {
		return errors.Wrap(models.NotFoundError, "could not find condition linked to rule")
	}

	scenario, err := uc.repository.GetScenarioById(ctx, exec, rule.ScenarioId)
	if err != nil {
		return err
	}

	if err := uc.enforceSecurity.PublishScenario(scenario); err != nil {
		return err
	}

	return uc.workflowRepository.DeleteWorkflowAction(ctx, exec, ruleId, actionId)
}

func (uc *ScenarioUsecase) ReorderWorkflowRules(ctx context.Context, scenarioId string, ids []uuid.UUID) error {
	exec := uc.executorFactory.NewExecutor()

	scenario, err := uc.repository.GetScenarioById(ctx, exec, scenarioId)
	if err != nil {
		return err
	}

	if err := uc.enforceSecurity.PublishScenario(scenario); err != nil {
		return err
	}

	return uc.workflowRepository.ReorderWorkflowRules(ctx, exec, scenarioId, ids)
}
