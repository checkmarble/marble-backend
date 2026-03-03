package worker_jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/payload_parser"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

type asyncDecisionIngester interface {
	IngestObject(
		ctx context.Context,
		organizationId uuid.UUID,
		objectType string,
		objectBody json.RawMessage,
		ingestionOptions models.IngestionOptions,
		parserOpts ...payload_parser.ParserOpt,
	) (int, error)
}

type asyncDecisionCreator interface {
	CreateAllDecisions(
		ctx context.Context,
		input models.CreateAllDecisionsInput,
		params models.CreateDecisionParams,
	) ([]models.DecisionWithRuleExecutions, int, error)
}

type asyncDecisionExecutionRepo interface {
	GetAsyncDecisionExecution(ctx context.Context, exec repositories.Executor, id uuid.UUID) (models.AsyncDecisionExecution, error)
	UpdateAsyncDecisionExecution(ctx context.Context, tx repositories.Transaction, input models.AsyncDecisionExecutionUpdate) error
}

type AsyncDecisionExecutionWorker struct {
	river.WorkerDefaults[models.AsyncDecisionExecutionArgs]

	executionRepo       asyncDecisionExecutionRepo
	executorFactory     executor_factory.ExecutorFactory
	transactionFactory  executor_factory.TransactionFactory
	ingester            asyncDecisionIngester
	decisionCreator     asyncDecisionCreator
	webhookEventsSender webhookEventsUsecase
	taskQueueRepository repositories.TaskQueueRepository
}

func NewAsyncDecisionExecutionWorker(
	executionRepo asyncDecisionExecutionRepo,
	executorFactory executor_factory.ExecutorFactory,
	transactionFactory executor_factory.TransactionFactory,
	ingester asyncDecisionIngester,
	decisionCreator asyncDecisionCreator,
	webhookEventsSender webhookEventsUsecase,
	taskQueueRepository repositories.TaskQueueRepository,
) *AsyncDecisionExecutionWorker {
	return &AsyncDecisionExecutionWorker{
		executionRepo:       executionRepo,
		executorFactory:     executorFactory,
		transactionFactory:  transactionFactory,
		ingester:            ingester,
		decisionCreator:     decisionCreator,
		webhookEventsSender: webhookEventsSender,
		taskQueueRepository: taskQueueRepository,
	}
}

func (w *AsyncDecisionExecutionWorker) Timeout(job *river.Job[models.AsyncDecisionExecutionArgs]) time.Duration {
	return 30 * time.Second
}

func (w *AsyncDecisionExecutionWorker) Work(ctx context.Context, job *river.Job[models.AsyncDecisionExecutionArgs]) error {
	logger := utils.LoggerFromContext(ctx)

	executionId, err := uuid.Parse(job.Args.AsyncDecisionExecutionId)
	if err != nil {
		return errors.Wrap(err, "invalid async_decision_execution_id")
	}

	exec := w.executorFactory.NewExecutor()
	execution, err := w.executionRepo.GetAsyncDecisionExecution(ctx, exec, executionId)
	if err != nil {
		return errors.Wrap(err, "failed to load async decision execution")
	}

	// Idempotency: if already terminal, nothing to do
	switch execution.Status {
	case models.AsyncDecisionExecution_Completed, models.AsyncDecisionExecution_Failed:
		logger.InfoContext(ctx, "async decision execution already in terminal state, skipping",
			"execution_id", executionId,
			"status", execution.Status,
		)
		return nil
	}

	// Ingestion step (with checkpoint)
	if execution.Status == models.AsyncDecisionExecution_Pending && execution.ShouldIngest {
		if err := w.ingestAndCheckpoint(ctx, execution); err != nil {
			return w.handleError(ctx, job, execution, models.AsyncDecisionExecution_StageIngestion, err)
		}
		// Update local status to reflect the checkpoint
		execution.Status = models.AsyncDecisionExecution_Ingested
	}

	// Decision creation step
	decisions, _, err := w.decisionCreator.CreateAllDecisions(ctx, models.CreateAllDecisionsInput{
		OrganizationId:     execution.OrgId,
		TriggerObjectTable: execution.ObjectType,
		PayloadRaw:         execution.TriggerObject,
	}, models.CreateDecisionParams{
		WithDecisionWebhooks:        true,
		WithRuleExecutionDetails:    true,
		WithScenarioPermissionCheck: false,
		WithDisallowUnknownFields:   false,
	})
	if err != nil {
		return w.handleError(ctx, job, execution, models.AsyncDecisionExecution_StageDecision, err)
	}

	// Extract decision IDs
	decisionIds := make([]uuid.UUID, len(decisions))
	for i, d := range decisions {
		decisionIds[i] = d.DecisionId
	}

	// Mark as completed
	err = w.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		return w.executionRepo.UpdateAsyncDecisionExecution(ctx, tx, models.AsyncDecisionExecutionUpdate{
			Id:          executionId,
			Status:      models.AsyncDecisionExecution_Completed,
			DecisionIds: decisionIds,
		})
	})
	if err != nil {
		return errors.Wrap(err, "failed to update async decision execution to completed")
	}

	logger.InfoContext(ctx, "async decision execution completed",
		"execution_id", executionId,
		"num_decisions", len(decisionIds),
	)

	return nil
}

// ingestAndCheckpoint ingests the trigger object and updates the execution status to "ingested"
// in its own transaction, so that retries skip ingestion.
func (w *AsyncDecisionExecutionWorker) ingestAndCheckpoint(
	ctx context.Context,
	execution models.AsyncDecisionExecution,
) error {
	_, err := w.ingester.IngestObject(
		ctx,
		execution.OrgId,
		execution.ObjectType,
		execution.TriggerObject,
		models.IngestionOptions{},
	)
	if err != nil {
		return errors.Wrap(err, "failed to ingest trigger object")
	}

	// Checkpoint: mark as ingested so retries skip ingestion
	err = w.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		return w.executionRepo.UpdateAsyncDecisionExecution(ctx, tx, models.AsyncDecisionExecutionUpdate{
			Id:     execution.Id,
			Status: models.AsyncDecisionExecution_Ingested,
		})
	})
	if err != nil {
		return errors.Wrap(err, "failed to checkpoint execution as ingested")
	}

	return nil
}

// handleError decides whether to retry or mark as failed.
// Non-retryable errors (BadParameterError, NotFoundError) or last attempt -> mark as failed and send webhook.
// Otherwise, return the error so River retries.
func (w *AsyncDecisionExecutionWorker) handleError(
	ctx context.Context,
	job *river.Job[models.AsyncDecisionExecutionArgs],
	execution models.AsyncDecisionExecution,
	stage models.AsyncDecisionExecutionFailureStage,
	originalErr error,
) error {
	logger := utils.LoggerFromContext(ctx)

	isNonRetryable := errors.Is(originalErr, models.NotFoundError) ||
		errors.Is(originalErr, models.BadParameterError)
	isLastAttempt := job.Attempt >= job.MaxAttempts

	if !isNonRetryable && !isLastAttempt {
		// Let River retry
		return originalErr
	}

	// Build a user-safe error message
	safeMessage := userSafeErrorMessage(stage, originalErr)

	logger.ErrorContext(ctx, fmt.Sprintf("async decision execution failed (stage=%s)", stage),
		"execution_id", execution.Id,
		"attempt", job.Attempt,
		"max_attempts", job.MaxAttempts,
		"error", originalErr.Error(),
	)

	// Mark as failed and create webhook event in a single transaction
	var webhookEventId string
	err := w.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		if err := w.executionRepo.UpdateAsyncDecisionExecution(ctx, tx, models.AsyncDecisionExecutionUpdate{
			Id:           execution.Id,
			Status:       models.AsyncDecisionExecution_Failed,
			ErrorMessage: &safeMessage,
		}); err != nil {
			return errors.Wrap(err, "failed to update execution to failed")
		}

		webhookEventId = uuid.NewString()
		if err := w.webhookEventsSender.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
			Id:             webhookEventId,
			OrganizationId: execution.OrgId,
			EventContent: models.NewWebhookEventAsyncDecisionFailed(models.AsyncDecisionFailedEventData{
				AsyncDecisionExecutionId: execution.Id,
				ObjectType:               execution.ObjectType,
				ScenarioId:               execution.ScenarioId,
				Stage:                    stage,
				TriggerObject:            execution.TriggerObject,
				ErrorMessage:             safeMessage,
			}),
		}); err != nil {
			return errors.Wrap(err, "failed to create async_decision.failed webhook event")
		}

		return nil
	})
	if err != nil {
		// If we can't even record the failure, log and return the original error
		logger.ErrorContext(ctx, "failed to record async decision execution failure",
			"execution_id", execution.Id,
			"record_error", err.Error(),
		)
		return originalErr
	}

	// Send webhook after transaction commits (same pattern as async_decision_job.go)
	w.webhookEventsSender.SendWebhookEventAsync(ctx, webhookEventId)

	// Return nil so River doesn't retry (we've handled the failure ourselves)
	return nil
}

// userSafeErrorMessage returns a sanitized error message for end users,
// avoiding leaking internal details.
func userSafeErrorMessage(stage models.AsyncDecisionExecutionFailureStage, err error) string {
	switch stage {
	case models.AsyncDecisionExecution_StageIngestion:
		if errors.Is(err, models.NotFoundError) {
			return "Ingestion failed: the specified object type was not found in the data model."
		}
		if errors.Is(err, models.BadParameterError) {
			return "Ingestion failed: invalid parameters in the trigger object."
		}
		return "Ingestion failed: an unexpected error occurred during object ingestion."
	case models.AsyncDecisionExecution_StageDecision:
		if errors.Is(err, models.NotFoundError) {
			return "Decision creation failed: a required resource was not found."
		}
		if errors.Is(err, models.BadParameterError) {
			return "Decision creation failed: invalid parameters in the trigger object."
		}
		return "Decision creation failed: an unexpected error occurred during decision evaluation."
	default:
		return "An unexpected error occurred."
	}
}
