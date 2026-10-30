package worker_jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
)

const (
	// postgres will only accept 65535 parameters in a query, so we need to batch the decisions_to_create creation
	// this is taking into account the fact that we have 2 parameters per decision_to_create
	batchSize = 5000

	// Max time (allowing for several retries) for a scheduled (batch) execution to start, before it is definitely marked as failed.
	// Smaller than the typical min interval before a next scheduled iteration (daily as allowed by the UI).
	scheduledExecMaxInitiationTime = time.Hour * 12

	// Timeout of the job that reads candidates and sets up the execution (see ScheduledExecutionWorker).
	scheduledExecSetupJobTimeout = 1 * time.Hour

	// Safety margin added to job timeouts when computing how long a cancelled execution's worker
	// may still be running.
	scheduledExecCancelMargin = 1 * time.Minute
)

// UsesBatchExecutionV2 tells whether an organization's scheduled executions are processed through
// a manifest and the batch execution coordinator, rather than with one job per object.
func UsesBatchExecutionV2(organizationId uuid.UUID, manifestBucketUrl string) bool {
	return infra.HasFeatureFlag(infra.BATCH_EXECUTION_V2_FEATURE_FLAG, organizationId) && manifestBucketUrl != ""
}

// ScheduledExecutionCancelGracePeriod returns the maximum time the job owning a (v2) scheduled
// execution in the given status may keep running after a cancellation is requested. Past it, the
// execution can safely be considered stopped even if the worker never acknowledged the cancellation.
func ScheduledExecutionCancelGracePeriod(status models.ScheduledExecutionStatus) time.Duration {
	if status == models.ScheduledExecutionPending {
		return scheduledExecSetupJobTimeout + scheduledExecCancelMargin
	}

	return batchExecJobTimeout + scheduledExecCancelMargin
}

type scheduledExecutionFinisher interface {
	UpdateScheduledExecutionStatus(ctx context.Context, exec repositories.Executor, input models.UpdateScheduledExecutionStatusInput) (bool, error)
	AckScheduledExecutionCancel(ctx context.Context, exec repositories.Executor, id string) error
}

// finishScheduledExecution sets a final status on an execution the caller has stopped working on.
// The update only applies to pending or processing executions: if it does not, a cancellation may
// have been requested since the caller last read the status, so acknowledge it instead, rather than
// leaving the scenario blocked until the cancel deadline. The acknowledgement only applies to
// cancelling executions, so it is a no-op otherwise.
func finishScheduledExecution(ctx context.Context, repository scheduledExecutionFinisher, exec repositories.Executor, id string, status models.ScheduledExecutionStatus) error {
	updated, err := repository.UpdateScheduledExecutionStatus(ctx, exec,
		models.UpdateScheduledExecutionStatusInput{Id: id, Status: status})
	if err != nil || updated {
		return err
	}

	return repository.AckScheduledExecutionCancel(ctx, exec, id)
}

type RunScheduledExecutionRepository interface {
	GetScenarioById(ctx context.Context, exec repositories.Executor, scenarioId string, screeningProvider models.ScreeningProvider) (models.Scenario, error)
	GetScenarioIteration(ctx context.Context, exec repositories.Executor, scenarioIterationId string,
		useCache bool) (models.ScenarioIteration, error)
	StoreDecisionsToCreate(
		ctx context.Context,
		exec repositories.Executor,
		decisionsToCreate models.DecisionToCreateBatchCreateInput,
	) ([]models.DecisionToCreate, error)
	ListAllScenarios(ctx context.Context, exec repositories.Executor,
		filters models.ListAllScenariosFilters) ([]models.Scenario, error)

	ListScheduledExecutions(ctx context.Context, exec repositories.Executor,
		filters models.ListScheduledExecutionsFilters, paging *models.PaginationAndSorting) ([]models.ScheduledExecution, error)
	CreateScheduledExecution(ctx context.Context, exec repositories.Executor,
		input models.CreateScheduledExecutionInput, newScheduledExecutionId string) error
	UpdateScheduledExecutionStatus(
		ctx context.Context,
		exec repositories.Executor,
		updateScheduledEx models.UpdateScheduledExecutionStatusInput,
	) (bool, error)
	AckScheduledExecutionCancel(ctx context.Context, exec repositories.Executor, id string) error
	FinalizeExpiredCancellations(ctx context.Context, exec repositories.Executor,
		filters models.FinalizeExpiredCancellationsFilters) error
	UpdateScheduledExecution(
		ctx context.Context,
		exec repositories.Executor,
		input models.UpdateScheduledExecutionInput,
	) error
	GetScheduledExecution(ctx context.Context, exec repositories.Executor, id string) (models.ScheduledExecution, error)
}

type taskQueueRepository interface {
	EnqueueDecisionTaskMany(
		ctx context.Context,
		tx repositories.Transaction,
		organizationId uuid.UUID,
		decision []models.DecisionToCreate,
		scenarioIterationId string,
	) error
	EnqueueScheduledExecStatusTask(
		ctx context.Context,
		tx repositories.Transaction,
		organizationId uuid.UUID,
		scheduledExecutionId string,
	) error
	EnqueueScheduledExecutionTask(
		ctx context.Context,
		tx repositories.Transaction,
		organizationId uuid.UUID,
		scheduledExecutionId string,
	) error
	EnqueueBatchExecutionCoordinator(
		ctx context.Context,
		tx repositories.Transaction,
		organizationId uuid.UUID,
		scheduledExecutionId string,
	) error
}

type RunScheduledExecution struct {
	repository                     RunScheduledExecutionRepository
	executorFactory                executor_factory.ExecutorFactory
	scenarioPublicationsRepository repositories.ScenarioPublicationRepository
	ingestedDataReadRepository     repositories.IngestedDataReadRepository
	transactionFactory             executor_factory.TransactionFactory
	taskQueueRepository            taskQueueRepository
	blobRepository                 repositories.BlobRepository
	manifestBucketUrl              string
}

func NewRunScheduledExecution(
	repository RunScheduledExecutionRepository,
	executorFactory executor_factory.ExecutorFactory,
	ingestedDataReadRepository repositories.IngestedDataReadRepository,
	transactionFactory executor_factory.TransactionFactory,
	taskQueueRepository taskQueueRepository,
	scenarioPublicationsRepository repositories.ScenarioPublicationRepository,
	blobRepository repositories.BlobRepository,
	manifestBucketUrl string,
) *RunScheduledExecution {
	return &RunScheduledExecution{
		repository:                     repository,
		executorFactory:                executorFactory,
		ingestedDataReadRepository:     ingestedDataReadRepository,
		transactionFactory:             transactionFactory,
		taskQueueRepository:            taskQueueRepository,
		scenarioPublicationsRepository: scenarioPublicationsRepository,
		blobRepository:                 blobRepository,
		manifestBucketUrl:              manifestBucketUrl,
	}
}

// ExecuteScheduledExecutionById executes a single scheduled execution by its ID.
// This is the entry point for the ScheduledExecutionWorker.
func (usecase *RunScheduledExecution) ExecuteScheduledExecutionById(
	ctx context.Context,
	scheduledExecutionId string,
) error {
	exec := usecase.executorFactory.NewExecutor()
	logger := utils.LoggerFromContext(ctx).With("scheduled_execution_id", scheduledExecutionId)
	ctx = utils.StoreLoggerInContext(ctx, logger)
	logger.InfoContext(ctx, fmt.Sprintf("Start execution %s", scheduledExecutionId))
	scheduledExecution, err := usecase.repository.GetScheduledExecution(ctx, exec, scheduledExecutionId)
	if err != nil {
		return errors.Errorf("error getting scheduled execution %s: %w", scheduledExecutionId, err)
	}

	switch scheduledExecution.Status {
	case models.ScheduledExecutionCancelling:
		logger.InfoContext(ctx, "Scheduled execution was cancelled before it started")
		return usecase.repository.AckScheduledExecutionCancel(ctx, exec, scheduledExecutionId)
	case models.ScheduledExecutionCancelled:
		return nil
	}

	if time.Now().After(scheduledExecution.StartedAt.Add(scheduledExecMaxInitiationTime)) &&
		scheduledExecution.Status == models.ScheduledExecutionPending {
		logger.WarnContext(ctx, fmt.Sprintf("Scheduled execution %s failed to start for too long, and will now be marked as failed", scheduledExecutionId))
		return finishScheduledExecution(ctx, usecase.repository, exec, scheduledExecution.Id, models.ScheduledExecutionFailure)
	}

	scenario := scheduledExecution.Scenario

	// list objects to score
	db, err := usecase.executorFactory.NewClientDbExecutor(ctx, scenario.OrganizationId)
	if err != nil {
		return err
	}

	liveVersion, err := usecase.repository.GetScenarioIteration(ctx, exec,
		scheduledExecution.ScenarioIterationId, true)
	if err != nil {
		return err
	}
	var filters []models.Filter
	if liveVersion.TriggerConditionAstExpression != nil {
		filters = selectFiltersFromTriggerAstRootAnd(
			*liveVersion.TriggerConditionAstExpression,
			models.TableIdentifier{Table: scenario.TriggerObjectType, Schema: db.DatabaseSchema().Schema},
		)
	}

	// When enabled for the org, process the execution by streaming a manifest of object ids to
	// blob storage and handing off to a single looping coordinator, rather than inserting one
	// row and one job per object below.
	if UsesBatchExecutionV2(scenario.OrganizationId, usecase.manifestBucketUrl) {
		return usecase.executeViaManifest(ctx, db, scheduledExecution, scenario, filters)
	}
	if infra.HasFeatureFlag(infra.BATCH_EXECUTION_V2_FEATURE_FLAG, scenario.OrganizationId) {
		logger.WarnContext(ctx, "manifest-based batch execution is enabled for this org but no manifest bucket is configured; using per-object execution instead")
	}

	// The dedup setting is only honoured by the v2 coordinator, so warn loudly rather than
	// let the toggle sit there doing nothing. Read from the snapshotted per-execution value,
	// not the scenario's live setting, so the warning reflects what this run will actually do.
	if scheduledExecution.DeduplicateObjects {
		logger.WarnContext(ctx, "scenario has batch object deduplication enabled but the org is not on BATCH_EXECUTION_V2; the setting is ignored for this execution")
	}

	objectIds, err := usecase.ingestedDataReadRepository.ListAllObjectIdsFromTable(ctx, db, scenario.TriggerObjectType, filters...)
	if err != nil {
		return err
	}

	nbPlannedDecisions := len(objectIds)
	err = usecase.repository.UpdateScheduledExecution(ctx, exec, models.UpdateScheduledExecutionInput{
		Id:                       scheduledExecutionId,
		NumberOfPlannedDecisions: &nbPlannedDecisions,
	})
	if err != nil {
		return err
	}
	_, err = usecase.repository.UpdateScheduledExecutionStatus(ctx, exec, models.UpdateScheduledExecutionStatusInput{
		Id:     scheduledExecutionId,
		Status: models.ScheduledExecutionProcessing,
	})
	if err != nil {
		return err
	}

	err = usecase.transactionFactory.Transaction(
		ctx,
		func(tx repositories.Transaction) error {
			// first, enqueue all the tasks that need to be executed
			for i := 0; i < len(objectIds); i += batchSize {
				end := min(len(objectIds), i+batchSize)

				batch, err := usecase.repository.StoreDecisionsToCreate(ctx, tx, models.DecisionToCreateBatchCreateInput{
					ScheduledExecutionId: scheduledExecutionId,
					ObjectId:             objectIds[i:end],
				})
				if err != nil {
					return err
				}

				err = usecase.taskQueueRepository.EnqueueDecisionTaskMany(
					ctx,
					tx,
					scenario.OrganizationId,
					batch,
					scheduledExecution.ScenarioIterationId,
				)
				if err != nil {
					return err
				}
			}

			// Then enqueue the task that will perform the scheduled execution status monitoring
			err = usecase.taskQueueRepository.EnqueueScheduledExecStatusTask(
				ctx,
				tx,
				scenario.OrganizationId,
				scheduledExecutionId,
			)
			if err != nil {
				return err
			}

			return nil
		},
	)
	if err != nil {
		return err
	}

	logger.InfoContext(ctx, fmt.Sprintf("Inserted %d decisions to be executed", nbPlannedDecisions))

	return nil
}

func batchExecutionManifestFileKey(id string) string {
	return fmt.Sprintf("scheduled_executions/%s/manifest.txt", id)
}

// executeViaManifest streams the matching object ids into a manifest blob, records the planned
// count, deadline, and manifest key on the execution, then enqueues a single coordinator job to
// process it.
func (usecase *RunScheduledExecution) executeViaManifest(
	ctx context.Context,
	db repositories.Executor,
	scheduledExecution models.ScheduledExecution,
	scenario models.Scenario,
	filters []models.Filter,
) error {
	logger := utils.LoggerFromContext(ctx)

	manifestKey := batchExecutionManifestFileKey(scheduledExecution.Id)
	writer, err := usecase.blobRepository.OpenStream(ctx, usecase.manifestBucketUrl, manifestKey, "manifest.txt")
	if err != nil {
		return errors.Errorf("error opening manifest writer: %w", err)
	}
	defer writer.Close()

	count, streamErr := usecase.ingestedDataReadRepository.StreamAllObjectIdsFromTable(
		ctx, db, writer, scenario.TriggerObjectType, filters...)
	// Always close to flush/commit the blob, even on error.
	closeErr := writer.Close()
	if streamErr != nil {
		return errors.Errorf("error streaming object ids to manifest: %w", streamErr)
	}
	if closeErr != nil {
		return errors.Errorf("error finalizing manifest blob: %w", closeErr)
	}

	nbPlanned := int(count)
	deadline := time.Now().Add(scheduledExecutionFullMaxRuntime)

	err = usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		// Strict pending -> processing transition: if a cancellation was requested while the
		// manifest was being written, roll back so that no coordinator is enqueued.
		updated, err := usecase.repository.UpdateScheduledExecutionStatus(ctx, tx, models.UpdateScheduledExecutionStatusInput{
			Id:         scheduledExecution.Id,
			Status:     models.ScheduledExecutionProcessing,
			FromStatus: utils.Ptr(models.ScheduledExecutionPending),
		})
		if err != nil {
			return err
		}
		if !updated {
			return models.ErrScheduledExecutionNotProcessing
		}

		err = usecase.repository.UpdateScheduledExecution(ctx, tx, models.UpdateScheduledExecutionInput{
			Id:                       scheduledExecution.Id,
			NumberOfPlannedDecisions: &nbPlanned,
			ManifestBlobKey:          &manifestKey,
			Deadline:                 &deadline,
		})
		if err != nil {
			return err
		}

		return usecase.taskQueueRepository.EnqueueBatchExecutionCoordinator(
			ctx, tx, scenario.OrganizationId, scheduledExecution.Id)
	})
	if errors.Is(err, models.ErrScheduledExecutionNotProcessing) {
		logger.InfoContext(ctx, "Scheduled execution was cancelled during setup, not starting the batch coordinator")
		return usecase.repository.AckScheduledExecutionCancel(ctx, usecase.executorFactory.NewExecutor(), scheduledExecution.Id)
	}
	if err != nil {
		return err
	}

	logger.InfoContext(ctx, fmt.Sprintf(
		"Batch execution v2: wrote manifest with %d objects and enqueued coordinator", nbPlanned))
	return nil
}

// ScheduleDueScenariosForOrg checks all live scenarios for an organization and schedules any that are due.
// When a scenario is due, it creates a scheduled_execution row and enqueues a job to execute it.
func (usecase *RunScheduledExecution) ScheduleDueScenariosForOrg(ctx context.Context, orgId uuid.UUID) error {
	logger := utils.LoggerFromContext(ctx)
	logger = logger.With(slog.String("organization_id", orgId.String()))
	exec := usecase.executorFactory.NewExecutor()

	scenarios, err := usecase.repository.ListAllScenarios(ctx, exec,
		models.ListAllScenariosFilters{Live: true, OrganizationId: &orgId})
	if err != nil {
		return errors.Errorf("error listing live scenarios for org %s: %w", orgId, err)
	}

	count := 0
	for _, scenario := range scenarios {
		logger := logger.With(
			slog.String("scenario_id", scenario.Id),
			slog.String("scenario_name", scenario.Name))
		ctx := utils.StoreLoggerInContext(ctx, logger)
		if done, err := usecase.ScheduleScenarioIfDue(ctx, scenario); err != nil {
			return err
		} else if done {
			count++
		}
	}
	logger.InfoContext(ctx, fmt.Sprintf(`Done scheduling %d scenarios for org "%s"`, count, orgId))
	return nil
}

// ScheduledExecutionWorker is a River worker that executes a single scheduled execution.
type ScheduledExecutionWorker struct {
	river.WorkerDefaults[models.ScheduledExecutionArgs]
	runScheduledExecution *RunScheduledExecution
}

func NewScheduledExecutionWorker(runScheduledExecution *RunScheduledExecution) *ScheduledExecutionWorker {
	return &ScheduledExecutionWorker{runScheduledExecution: runScheduledExecution}
}

// One hour is the timeout to read candidates from ingested data and insert the decisions to create & the job tasks.
func (w *ScheduledExecutionWorker) Timeout(job *river.Job[models.ScheduledExecutionArgs]) time.Duration {
	return scheduledExecSetupJobTimeout
}

func (w *ScheduledExecutionWorker) Work(ctx context.Context, job *river.Job[models.ScheduledExecutionArgs]) error {
	return w.runScheduledExecution.ExecuteScheduledExecutionById(ctx, job.Args.ScheduledExecutionId)
}
