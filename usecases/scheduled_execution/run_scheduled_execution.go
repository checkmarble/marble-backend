package scheduled_execution

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
)

// postgres will only accept 65535 parameters in a query, so we need to batch the decisions_to_create creation
// this is taking into account the fact that we have 2 parameters per decision_to_create
const batchSize = 5000

type RunScheduledExecutionRepository interface {
	GetScenarioById(ctx context.Context, exec repositories.Executor, scenarioId string) (models.Scenario, error)
	GetScenarioIteration(ctx context.Context, exec repositories.Executor, scenarioIterationId string) (models.ScenarioIteration, error)
	StoreDecisionsToCreate(
		ctx context.Context,
		exec repositories.Executor,
		decisionsToCreate models.DecisionToCreateBatchCreateInput,
	) ([]models.DecisionToCreate, error)

	ListScheduledExecutions(ctx context.Context, exec repositories.Executor,
		filters models.ListScheduledExecutionsFilters) ([]models.ScheduledExecution, error)
	CreateScheduledExecution(ctx context.Context, exec repositories.Executor,
		input models.CreateScheduledExecutionInput, newScheduledExecutionId string) error
	UpdateScheduledExecutionStatus(
		ctx context.Context,
		exec repositories.Executor,
		updateScheduledEx models.UpdateScheduledExecutionStatusInput,
	) (executed bool, err error)
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
		organizationId string,
		decision []models.DecisionToCreate,
		scenarioIterationId string,
	) error
	EnqueueScheduledExecStatusTask(
		ctx context.Context,
		tx repositories.Transaction,
		organizationId string,
		scheduledExecutionId string,
	) error
}

type RunScheduledExecution struct {
	repository                     RunScheduledExecutionRepository
	testrunScenarioRepository      repositories.EvalTestRunScenarioRepository
	executorFactory                executor_factory.ExecutorFactory
	scenarioPublicationsRepository repositories.ScenarioPublicationRepository
	ingestedDataReadRepository     repositories.IngestedDataReadRepository
	transactionFactory             executor_factory.TransactionFactory
	taskQueueRepository            taskQueueRepository
}

func NewRunScheduledExecution(
	repository RunScheduledExecutionRepository,
	testrunScenarioRepository repositories.EvalTestRunScenarioRepository,
	executorFactory executor_factory.ExecutorFactory,
	ingestedDataReadRepository repositories.IngestedDataReadRepository,
	transactionFactory executor_factory.TransactionFactory,
	taskQueueRepository taskQueueRepository,
	scenarioPublicationsRepository repositories.ScenarioPublicationRepository,
) *RunScheduledExecution {
	return &RunScheduledExecution{
		repository:                     repository,
		testrunScenarioRepository:      testrunScenarioRepository,
		executorFactory:                executorFactory,
		ingestedDataReadRepository:     ingestedDataReadRepository,
		transactionFactory:             transactionFactory,
		taskQueueRepository:            taskQueueRepository,
		scenarioPublicationsRepository: scenarioPublicationsRepository,
	}
}

func (usecase *RunScheduledExecution) ExecuteAllScheduledScenarios(ctx context.Context) error {
	logger := utils.LoggerFromContext(ctx)

	pendingScheduledExecutions, err := usecase.repository.ListScheduledExecutions(
		ctx,
		usecase.executorFactory.NewExecutor(),
		models.ListScheduledExecutionsFilters{
			Status: []models.ScheduledExecutionStatus{models.ScheduledExecutionPending},
		})
	if err != nil {
		return fmt.Errorf("error while listing pending ScheduledExecutions: %w", err)
	}

	logger.InfoContext(ctx, fmt.Sprintf("Found %d pending scheduled executions", len(pendingScheduledExecutions)))

	var waitGroup sync.WaitGroup
	executionErrorChan := make(chan error, len(pendingScheduledExecutions))

	startScheduledExecution := func(scheduledExecution models.ScheduledExecution) {
		defer waitGroup.Done()
		ctx = utils.StoreLoggerInContext(
			ctx,
			logger.
				With("scheduled_execution_id", scheduledExecution.Id).
				With("organization_id", scheduledExecution.OrganizationId),
		)
		if err := usecase.executeScheduledScenario(ctx, scheduledExecution); err != nil {
			executionErrorChan <- err
		}
	}

	for _, pendingExecution := range pendingScheduledExecutions {
		waitGroup.Add(1)
		go startScheduledExecution(pendingExecution)
	}

	waitGroup.Wait()
	close(executionErrorChan)

	executionErr := <-executionErrorChan
	return executionErr
}

func (usecase *RunScheduledExecution) executeScheduledScenario(ctx context.Context, scheduledExecution models.ScheduledExecution) error {
	exec := usecase.executorFactory.NewExecutor()
	logger := utils.LoggerFromContext(ctx)
	logger.InfoContext(ctx, fmt.Sprintf("Start execution %s", scheduledExecution.Id))

	if done, err := usecase.repository.UpdateScheduledExecutionStatus(
		ctx,
		exec,
		models.UpdateScheduledExecutionStatusInput{
			Id:                     scheduledExecution.Id,
			Status:                 models.ScheduledExecutionProcessing,
			CurrentStatusCondition: models.ScheduledExecutionPending,
		},
	); err != nil {
		return err
	} else if !done {
		logger.InfoContext(ctx, fmt.Sprintf("Execution %s is already being processed", scheduledExecution.Id))
		return nil
	}
	return usecase.insertAsyncDecisionTasks(
		ctx,
		scheduledExecution.Id,
		scheduledExecution.Scenario,
	)
}

func (usecase *RunScheduledExecution) insertAsyncDecisionTasks(
	ctx context.Context,
	scheduledExecutionId string,
	scenario models.Scenario,
) error {
	logger := utils.LoggerFromContext(ctx)
	exec := usecase.executorFactory.NewExecutor()

	// list objects to score
	db, err := usecase.executorFactory.NewClientDbExecutor(ctx, scenario.OrganizationId)
	if err != nil {
		return err
	}

	scheduledExecution, err := usecase.repository.GetScheduledExecution(ctx, exec, scheduledExecutionId)
	if err != nil {
		return err
	}

	liveVersion, err := usecase.repository.GetScenarioIteration(ctx, exec, scheduledExecution.ScenarioIterationId)
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

	logger.InfoContext(ctx, fmt.Sprintf("Inserted %d decisions to be executed", nbPlannedDecisions),
		slog.String("scheduled_execution_id", scheduledExecution.Id),
		slog.String("organization_id", scheduledExecution.OrganizationId),
	)

	return nil
}
