package scheduled_execution

import (
	"context"
	"log/slog"
	"slices"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/scenarios"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/riverqueue/river"
)

const (
	maxJobDuration                      = 4 * time.Hour
	scheduledExecStatusPollingFreqStep1 = 500 * time.Millisecond
	scheduledExecPollingFreqDelay1      = 20 * time.Second
	scheduledExecStatusPollingFreqStep2 = 1 * time.Second
	scheduledExecPollingFreqDelay2      = 5 * time.Minute
	scheduledExecStatusPollingFreqStep3 = 5 * time.Second
)

type AsyncScheduledExecWorker struct {
	river.WorkerDefaults[models.ScheduledExecStatusSyncArgs]

	repository                     asyncDecisionWorkerRepository
	executorFactory                executor_factory.ExecutorFactory
	scenarioPublicationsRepository repositories.ScenarioPublicationRepository
	dataModelRepository            repositories.DataModelRepository
	ingestedDataReadRepository     repositories.IngestedDataReadRepository
	evaluateAstExpression          ast_eval.EvaluateAstExpression
	decisionRepository             repositories.DecisionRepository
	decisionWorkflows              decisionWorkflowsUsecase
	webhookEventsSender            webhookEventsUsecase
	snoozesReader                  snoozesForDecisionReader
	scenarioFetcher                scenarios.ScenarioFetcher
}

func NewAsyncScheduledExecWorker(
	repository asyncDecisionWorkerRepository,
	executorFactory executor_factory.ExecutorFactory,
	scenarioPublicationsRepository repositories.ScenarioPublicationRepository,
	dataModelRepository repositories.DataModelRepository,
	ingestedDataReadRepository repositories.IngestedDataReadRepository,
	evaluateAstExpression ast_eval.EvaluateAstExpression,
	decisionRepository repositories.DecisionRepository,
	decisionWorkflows decisionWorkflowsUsecase,
	webhookEventsSender webhookEventsUsecase,
	snoozesReader snoozesForDecisionReader,
	scenarioFetcher scenarios.ScenarioFetcher,
) AsyncScheduledExecWorker {
	return AsyncScheduledExecWorker{
		repository:                     repository,
		executorFactory:                executorFactory,
		scenarioPublicationsRepository: scenarioPublicationsRepository,
		dataModelRepository:            dataModelRepository,
		ingestedDataReadRepository:     ingestedDataReadRepository,
		evaluateAstExpression:          evaluateAstExpression,
		decisionRepository:             decisionRepository,
		decisionWorkflows:              decisionWorkflows,
		webhookEventsSender:            webhookEventsSender,
		snoozesReader:                  snoozesReader,
		scenarioFetcher:                scenarioFetcher,
	}
}

func (w *AsyncScheduledExecWorker) Work(ctx context.Context, job *river.Job[models.ScheduledExecStatusSyncArgs]) error {
	return w.handleScheduledExecStatusRefres(ctx, job.Args, w.executorFactory.NewExecutor(), job)
}

func (w *AsyncScheduledExecWorker) Timeout(job *river.Job[models.ScheduledExecStatusSyncArgs]) time.Duration {
	return 10 * time.Second
}

func (w *AsyncScheduledExecWorker) handleScheduledExecStatusRefres(
	ctx context.Context,
	args models.ScheduledExecStatusSyncArgs,
	tx repositories.Executor,
	job *river.Job[models.ScheduledExecStatusSyncArgs],
) error {
	logger := utils.LoggerFromContext(ctx)
	scheduledExec, err := w.repository.GetScheduledExecution(ctx, tx, args.ScheduledExecutionId)
	if err != nil {
		return err
	}
	if slices.Contains([]models.ScheduledExecutionStatus{
		models.ScheduledExecutionPending,
		models.ScheduledExecutionSuccess,
		models.ScheduledExecutionPartialFailure,
		models.ScheduledExecutionFailure,
	}, scheduledExec.Status, // anything other than "Processing", in fact
	) {
		return nil
	}

	// Just check if there is at least one pending or failed decision left
	decisionsToCreate, err := w.repository.ListDecisionsToCreate(
		ctx,
		tx,
		models.ListDecisionsToCreateFilters{
			ScheduledExecutionId: args.ScheduledExecutionId,
			Status: []models.DecisionToCreateStatus{
				models.DecisionToCreateStatusPending, models.DecisionToCreateStatusFailed,
			},
		},
		utils.Ptr(1),
	)
	if err != nil {
		return err
	}

	if len(decisionsToCreate) != 0 && time.Since(job.CreatedAt) < maxJobDuration {
		// if there are still decisions to create, and the job is not too old, re-enqueue
		// Retry more frequently at the beginning for a better experience with small jobs (and in particular for when it runs in integration tests)
		var delay time.Duration
		if time.Since(job.CreatedAt) < scheduledExecPollingFreqDelay1 {
			delay = scheduledExecStatusPollingFreqStep1
		} else if time.Since(job.CreatedAt) < scheduledExecPollingFreqDelay2 {
			delay = scheduledExecStatusPollingFreqStep2
		} else {
			delay = scheduledExecStatusPollingFreqStep3
		}
		return river.JobSnooze(delay)
	}

	counts, err := w.repository.CountDecisionsToCreateByStatus(ctx, tx, args.ScheduledExecutionId)
	if err != nil {
		return err
	}

	var finalStatus models.ScheduledExecutionStatus
	if counts.SuccessfullyEvaluated == *scheduledExec.NumberOfPlannedDecisions {
		finalStatus = models.ScheduledExecutionSuccess
	} else if counts.Created > 0 {
		finalStatus = models.ScheduledExecutionPartialFailure
	} else {
		finalStatus = models.ScheduledExecutionFailure
	}

	done, err := w.repository.UpdateScheduledExecutionStatus(
		ctx,
		tx,
		models.UpdateScheduledExecutionStatusInput{
			Id:                         args.ScheduledExecutionId,
			NumberOfCreatedDecisions:   &counts.Created,
			NumberOfEvaluatedDecisions: &counts.SuccessfullyEvaluated,
			Status:                     finalStatus,
			CurrentStatusCondition:     models.ScheduledExecutionProcessing,
		},
	)
	if err != nil {
		return err
	}
	if !done {
		logger.InfoContext(ctx,
			"Scheduled execution is no longer in processing status, stop the retries",
			slog.String("scheduled_execution_id", args.ScheduledExecutionId),
		)
	}

	return nil
}
