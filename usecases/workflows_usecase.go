package usecases

import (
	"context"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/google/uuid"
)

type workflowRepository interface {
	ListWorkflowsForScenario(ctx context.Context, exec repositories.Executor, scenarioId string) ([]models.Workflow, error)
	GetWorkflowRule(ctx context.Context, exec repositories.Executor, id string) (models.WorkflowRule, error)
	GetWorkflowCondition(ctx context.Context, exec repositories.Executor, id string) (models.WorkflowCondition, error)
	GetWorkflowAction(ctx context.Context, exec repositories.Executor, id string) (models.WorkflowAction, error)
	InsertWorkflowRule(ctx context.Context, exec repositories.Executor, rule models.WorkflowRule) (models.WorkflowRule, error)
	UpdateWorkflowRule(ctx context.Context, exec repositories.Executor, rule models.WorkflowRule) (models.WorkflowRule, error)
	InsertWorkflowCondition(ctx context.Context, exec repositories.Executor, rule models.WorkflowCondition) (models.WorkflowCondition, error)
	InsertWorkflowAction(ctx context.Context, exec repositories.Executor, rule models.WorkflowAction) (models.WorkflowAction, error)
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

func (uc *ScenarioUsecase) UpdateWorkflowRule(ctx context.Context, rule models.WorkflowRule) (models.WorkflowRule, error) {
	exec := uc.executorFactory.NewExecutor()

	scenario, err := uc.repository.GetScenarioById(ctx, exec, rule.ScenarioId)
	if err != nil {
		return models.WorkflowRule{}, err
	}

	if err := uc.enforceSecurity.PublishScenario(scenario); err != nil {
		return models.WorkflowRule{}, err
	}

	return uc.workflowRepository.UpdateWorkflowRule(ctx, exec, rule)
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
