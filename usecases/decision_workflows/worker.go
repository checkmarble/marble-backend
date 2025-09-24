package decision_workflows

import (
	"context"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/evaluate_scenario"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/riverqueue/river"
)

type decisionWorkflowsUsecase interface {
	ProcessDecisionWorkflows(
		ctx context.Context,
		tx repositories.Transaction,
		rules []models.Workflow,
		scenario models.Scenario,
		decision models.DecisionWithRuleExecutions,
		evalParams evaluate_scenario.ScenarioEvaluationParameters,
	) (models.WorkflowExecution, error)
}

type webhookEventsUsecase interface {
	SendWebhookEventAsync(ctx context.Context, webhookEventId string)
}

type decisionWorkflowsWorkerRepository interface {
	DecisionWithRuleExecutionsById(
		ctx context.Context,
		exec repositories.Executor,
		decisionId string,
	) (models.DecisionWithRuleExecutions, error)
	GetScenarioById(ctx context.Context, exec repositories.Executor, scenarioId string) (models.Scenario, error)
	ListWorkflowsForScenario(ctx context.Context, exec repositories.Executor, scenarioId uuid.UUID) ([]models.Workflow, error)
}

type dataModelRepository interface {
	GetDataModel(ctx context.Context, exec repositories.Executor, organizationID string,
		fetchEnumValues bool, useCache bool) (models.DataModel, error)
}

type ingestedDataReadRepository interface {
	QueryIngestedObject(
		ctx context.Context,
		exec repositories.Executor,
		table models.Table,
		objectId string,
	) ([]models.DataModelObject, error)
}

type DecisionWorkflowsWorker struct {
	river.WorkerDefaults[models.DecisionWorkflowArgs]

	executorFactory                   executor_factory.ExecutorFactory
	transactionFactory                executor_factory.TransactionFactory
	decisionWorkflowsUsecase          decisionWorkflowsUsecase
	dataModelRepository               dataModelRepository
	ingestedDataReadRepository        ingestedDataReadRepository
	decisionWorkflowsWorkerRepository decisionWorkflowsWorkerRepository
	webhookEventsUsecase              webhookEventsUsecase
}

func NewDecisionWorkflowsWorker(
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	decisionWorkflowsUsecase decisionWorkflowsUsecase,
	dataModelRepository dataModelRepository,
	ingestedDataReadRepository ingestedDataReadRepository,
	decisionWorkflowsWorkerRepository decisionWorkflowsWorkerRepository,
	webhookEventsUsecase webhookEventsUsecase,
) *DecisionWorkflowsWorker {
	return &DecisionWorkflowsWorker{
		executorFactory:                   executorFactory,
		transactionFactory:                transactionFactory,
		decisionWorkflowsUsecase:          decisionWorkflowsUsecase,
		dataModelRepository:               dataModelRepository,
		ingestedDataReadRepository:        ingestedDataReadRepository,
		decisionWorkflowsWorkerRepository: decisionWorkflowsWorkerRepository,
		webhookEventsUsecase:              webhookEventsUsecase,
	}
}

func (w *DecisionWorkflowsWorker) Timeout(job *river.Job[models.DecisionWorkflowArgs]) time.Duration {
	return 10 * time.Second
}

func (w *DecisionWorkflowsWorker) Work(ctx context.Context, job *river.Job[models.DecisionWorkflowArgs]) error {
	exec := w.executorFactory.NewExecutor()

	// Fetch/Build data for decision workflows
	decision, err := w.decisionWorkflowsWorkerRepository.DecisionWithRuleExecutionsById(ctx, exec, job.Args.DecisionId)
	if err != nil {
		return errors.Wrap(err, "error getting decision with rule executions")
	}

	scenario, err := w.decisionWorkflowsWorkerRepository.GetScenarioById(ctx, exec, decision.ScenarioId.String())
	if err != nil {
		return errors.Wrap(err, "error getting scenario")
	}

	dataModel, err := w.dataModelRepository.GetDataModel(
		ctx,
		exec,
		decision.OrganizationId.String(),
		true,
		true,
	)
	if err != nil {
		return errors.Wrap(err, "error getting data model")
	}

	evalParams := evaluate_scenario.ScenarioEvaluationParameters{
		Scenario:     scenario,
		ClientObject: decision.ClientObject,
		DataModel:    dataModel,
	}

	scenarioUUID, err := uuid.Parse(scenario.Id)
	if err != nil {
		return errors.Wrap(err, "invalid scenario ID: not a valid UUID")
	}
	workflowRules, err := w.decisionWorkflowsWorkerRepository.ListWorkflowsForScenario(ctx, exec, scenarioUUID)
	if err != nil {
		return errors.Wrap(err, "error getting workflows for scenario")
	}

	// Create transaction just for ProcessDecisionWorkflows because all functions in there expect a transaction
	workflowExecutions, err := executor_factory.TransactionReturnValue(ctx, w.transactionFactory, func(
		tx repositories.Transaction,
	) (models.WorkflowExecution, error) {
		workflowExecutions, err := w.decisionWorkflowsUsecase.ProcessDecisionWorkflows(
			ctx,
			tx,
			workflowRules,
			scenario,
			decision,
			evalParams,
		)
		if err != nil {
			return models.WorkflowExecution{}, errors.Wrap(err, "error processing decision workflows")
		}
		return workflowExecutions, nil
	})
	if err != nil {
		return errors.Wrap(err, "error processing decision workflows")
	}

	for _, webhookId := range workflowExecutions.WebhookIds {
		w.webhookEventsUsecase.SendWebhookEventAsync(ctx, webhookId)
	}

	return nil
}
