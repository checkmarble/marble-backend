package worker_jobs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
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
)

type RunScheduledExecutionRepository interface {
	GetScenarioById(ctx context.Context, exec repositories.Executor, scenarioId string) (models.Scenario, error)
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
	) (err error)
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
}

type RunScheduledExecution struct {
	repository                     RunScheduledExecutionRepository
	executorFactory                executor_factory.ExecutorFactory
	scenarioPublicationsRepository repositories.ScenarioPublicationRepository
	ingestedDataReadRepository     repositories.IngestedDataReadRepository
	transactionFactory             executor_factory.TransactionFactory
	taskQueueRepository            taskQueueRepository
}

func NewRunScheduledExecution(
	repository RunScheduledExecutionRepository,
	executorFactory executor_factory.ExecutorFactory,
	ingestedDataReadRepository repositories.IngestedDataReadRepository,
	transactionFactory executor_factory.TransactionFactory,
	taskQueueRepository taskQueueRepository,
	scenarioPublicationsRepository repositories.ScenarioPublicationRepository,
) *RunScheduledExecution {
	return &RunScheduledExecution{
		repository:                     repository,
		executorFactory:                executorFactory,
		ingestedDataReadRepository:     ingestedDataReadRepository,
		transactionFactory:             transactionFactory,
		taskQueueRepository:            taskQueueRepository,
		scenarioPublicationsRepository: scenarioPublicationsRepository,
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
		return fmt.Errorf("error getting scheduled execution %s: %w", scheduledExecutionId, err)
	}

	if time.Now().After(scheduledExecution.StartedAt.Add(scheduledExecMaxInitiationTime)) &&
		scheduledExecution.Status == models.ScheduledExecutionPending {
		logger.WarnContext(ctx, fmt.Sprintf("Scheduled execution %s failed to start for too long, and will now be marked as failed", scheduledExecutionId))
		err := usecase.repository.UpdateScheduledExecutionStatus(ctx, exec, models.UpdateScheduledExecutionStatusInput{
			Id:     scheduledExecution.Id,
			Status: models.ScheduledExecutionFailure,
		})
		return err
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
	err = usecase.repository.UpdateScheduledExecutionStatus(ctx, exec, models.UpdateScheduledExecutionStatusInput{
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

// ScheduleDueScenariosForOrg checks all live scenarios for an organization and schedules any that are due.
// When a scenario is due, it creates a scheduled_execution row and enqueues a job to execute it.
func (usecase *RunScheduledExecution) ScheduleDueScenariosForOrg(ctx context.Context, orgId uuid.UUID) error {
	logger := utils.LoggerFromContext(ctx)
	logger = logger.With(slog.String("organization_id", orgId.String()))
	exec := usecase.executorFactory.NewExecutor()

	scenarios, err := usecase.repository.ListAllScenarios(ctx, exec,
		models.ListAllScenariosFilters{Live: true, OrganizationId: &orgId})
	if err != nil {
		return fmt.Errorf("error listing live scenarios for org %s: %w", orgId, err)
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
	return 1 * time.Hour
}

func (w *ScheduledExecutionWorker) Work(ctx context.Context, job *river.Job[models.ScheduledExecutionArgs]) error {
	return w.runScheduledExecution.ExecuteScheduledExecutionById(ctx, job.Args.ScheduledExecutionId)
}
