package scheduled_execution

import (
	"context"
	"fmt"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/evaluate_scenario"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/utils"
)

type decisionWorkflowsUsecase interface {
	AutomaticDecisionToCase(
		ctx context.Context,
		tx repositories.Executor,
		scenario models.Scenario,
		decision models.DecisionWithRuleExecutions,
		webhookEventId string,
	) (bool, error)
}

type webhookEventsUsecase interface {
	CreateWebhookEvent(
		ctx context.Context,
		tx repositories.Executor,
		input models.WebhookEventCreate,
	) error
	SendWebhookEventAsync(ctx context.Context, webhookEventId string)
}

type RunScheduledExecutionRepository interface {
	GetScenarioById(ctx context.Context, exec repositories.Executor, scenarioId string) (models.Scenario, error)
	GetScenarioIteration(ctx context.Context, exec repositories.Executor, scenarioIterationId string) (models.ScenarioIteration, error)

	StoreDecisionsToCreate(
		ctx context.Context,
		exec repositories.Executor,
		decisionsToCreate models.DecisionToCreateBatchCreateInput,
	) ([]models.DecisionToCreate, error)
	UpdateDecisionToCreateStatus(
		ctx context.Context,
		exec repositories.Executor,
		id string,
		status models.DecisionToCreateStatus,
	) error

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

type snoozesForDecisionReader interface {
	ListActiveRuleSnoozesForDecision(
		ctx context.Context,
		exec repositories.Executor,
		snoozeGroupIds []string,
		pivotValue string,
	) ([]models.RuleSnooze, error)
}

type RunScheduledExecution struct {
	repository                     RunScheduledExecutionRepository
	executorFactory                executor_factory.ExecutorFactory
	scenarioPublicationsRepository repositories.ScenarioPublicationRepository
	dataModelRepository            repositories.DataModelRepository
	ingestedDataReadRepository     repositories.IngestedDataReadRepository
	evaluateAstExpression          ast_eval.EvaluateAstExpression
	decisionRepository             repositories.DecisionRepository
	transactionFactory             executor_factory.TransactionFactory
	decisionWorkflows              decisionWorkflowsUsecase
	webhookEventsSender            webhookEventsUsecase
	snoozesReader                  snoozesForDecisionReader
}

func NewRunScheduledExecution(
	repository RunScheduledExecutionRepository,
	executorFactory executor_factory.ExecutorFactory,
	scenarioPublicationsRepository repositories.ScenarioPublicationRepository,
	dataModelRepository repositories.DataModelRepository,
	ingestedDataReadRepository repositories.IngestedDataReadRepository,
	evaluateAstExpression ast_eval.EvaluateAstExpression,
	decisionRepository repositories.DecisionRepository,
	transactionFactory executor_factory.TransactionFactory,
	decisionWorkflows decisionWorkflowsUsecase,
	webhookEventsSender webhookEventsUsecase,
	snoozesReader snoozesForDecisionReader,
) *RunScheduledExecution {
	return &RunScheduledExecution{
		repository:                     repository,
		executorFactory:                executorFactory,
		scenarioPublicationsRepository: scenarioPublicationsRepository,
		dataModelRepository:            dataModelRepository,
		ingestedDataReadRepository:     ingestedDataReadRepository,
		evaluateAstExpression:          evaluateAstExpression,
		decisionRepository:             decisionRepository,
		transactionFactory:             transactionFactory,
		decisionWorkflows:              decisionWorkflows,
		webhookEventsSender:            webhookEventsSender,
		snoozesReader:                  snoozesReader,
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
				With("scheduledExecutionId", scheduledExecution.Id).
				With("organizationId", scheduledExecution.OrganizationId),
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

	completed, numberOfCreatedDecisions, err := usecase.createScheduledScenarioDecisions(
		ctx,
		scheduledExecution.Id,
		scheduledExecution.Scenario,
	)
	if err != nil {
		// if an error occurs, try to update the scheduled execution status to failure then return the error (no decisions have been created)
		_, nestedErr := usecase.repository.UpdateScheduledExecutionStatus(
			ctx,
			exec,
			models.UpdateScheduledExecutionStatusInput{
				Id:                     scheduledExecution.Id,
				Status:                 models.ScheduledExecutionFailure,
				CurrentStatusCondition: models.ScheduledExecutionProcessing,
			})
		if nestedErr != nil {
			return errors.Join(err, nestedErr)
		}

		return err
	}

	var finalStatus models.ScheduledExecutionStatus
	if completed {
		finalStatus = models.ScheduledExecutionSuccess
	} else {
		finalStatus = models.ScheduledExecutionPartialFailure
	}
	_, err = usecase.repository.UpdateScheduledExecutionStatus(
		ctx,
		exec,
		models.UpdateScheduledExecutionStatusInput{
			Id:                         scheduledExecution.Id,
			Status:                     finalStatus,
			NumberOfCreatedDecisions:   &numberOfCreatedDecisions.created,
			NumberOfEvaluatedDecisions: &numberOfCreatedDecisions.evaluated,
			CurrentStatusCondition:     models.ScheduledExecutionProcessing,
		},
	)
	if err != nil {
		return err
	}

	logger.InfoContext(ctx, fmt.Sprintf("Execution completed for %s", scheduledExecution.Id))
	return nil
}

type numbersOfDecisions struct {
	evaluated int64
	created   int64
}

func (usecase *RunScheduledExecution) createScheduledScenarioDecisions(
	ctx context.Context,
	scheduledExecutionId string,
	scenario models.Scenario,
) (completed bool, nbDecisions numbersOfDecisions, err error) {
	exec := usecase.executorFactory.NewExecutor()
	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, exec, scenario.OrganizationId, false)
	if err != nil {
		return false, numbersOfDecisions{}, err
	}
	tables := dataModel.Tables
	table, ok := tables[scenario.TriggerObjectType]
	if !ok {
		return false, numbersOfDecisions{}, fmt.Errorf(
			"trigger object type %s not found in data model: %w",
			scenario.TriggerObjectType, models.NotFoundError)
	}

	pivotsMeta, err := usecase.dataModelRepository.ListPivots(ctx, exec, scenario.OrganizationId, nil)
	if err != nil {
		return false, numbersOfDecisions{}, err
	}
	pivot := models.FindPivot(pivotsMeta, scenario.TriggerObjectType, dataModel)

	// list objects to score
	db, err := usecase.executorFactory.NewClientDbExecutor(ctx, scenario.OrganizationId)
	if err != nil {
		return false, numbersOfDecisions{}, err
	}

	liveVersion, err := usecase.repository.GetScenarioIteration(ctx, exec, *scenario.LiveVersionID)
	if err != nil {
		return false, numbersOfDecisions{}, err
	}
	filters := selectFiltersFromTriggerAstRootAnd(
		*liveVersion.TriggerConditionAstExpression,
		models.TableIdentifier{Table: table.Name, Schema: db.DatabaseSchema().Schema},
	)

	objectIds, err := usecase.ingestedDataReadRepository.ListAllObjectIdsFromTable(ctx, db, table, filters...)
	if err != nil {
		return false, numbersOfDecisions{}, err
	}

	nbPlannedDecisions := int64(len(objectIds))
	err = usecase.repository.UpdateScheduledExecution(ctx, exec, models.UpdateScheduledExecutionInput{
		Id:                       scheduledExecutionId,
		NumberOfPlannedDecisions: &nbPlannedDecisions,
	})
	if err != nil {
		return false, numbersOfDecisions{}, err
	}

	decisionsToCreate, err := usecase.repository.StoreDecisionsToCreate(ctx, exec, models.DecisionToCreateBatchCreateInput{
		ScheduledExecutionId: scheduledExecutionId,
		ObjectId:             objectIds,
	})
	if err != nil {
		return false, numbersOfDecisions{}, err
	}

	var nbEvaluatedDec int64
	var nbCreatedDec int64
	for i, decisionToCreate := range decisionsToCreate {
		created, err := usecase.createSingleDecisionForObjectId(
			ctx,
			db,
			decisionToCreate,
			scenario,
			dataModel,
			scheduledExecutionId,
			pivot,
			i,
		)
		nbEvaluatedDec += 1
		if err != nil {
			fmt.Printf("%+v\n", err)
			updateErr := usecase.repository.UpdateDecisionToCreateStatus(
				ctx,
				exec,
				decisionToCreate.Id,
				models.DecisionToCreateStatusFailed,
			)
			if updateErr != nil {
				utils.LogAndReportSentryError(ctx, updateErr)
			}
			// Stop at the first encountered error, store the number of created decisions on the scheduled execution
			return false, numbersOfDecisions{
				evaluated: nbEvaluatedDec,
				created:   nbCreatedDec,
			}, nil
		}
		if created {
			nbCreatedDec += 1
		}
	}

	return true, numbersOfDecisions{evaluated: nbEvaluatedDec, created: nbCreatedDec}, nil
}

func (usecase *RunScheduledExecution) createSingleDecisionForObjectId(
	ctx context.Context,
	db repositories.Executor,
	decisionToCreate models.DecisionToCreate,
	scenario models.Scenario,
	dataModel models.DataModel,
	scheduledExecutionId string,
	pivot *models.Pivot,
	objectIdx int,
) (decisionCreated bool, err error) {
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(
		ctx,
		"Batch RunScheduledExecution.executeScheduledScenario",
		trace.WithAttributes(
			attribute.String("scenario_id", scenario.Id),
			attribute.Int64("object_index", int64(objectIdx)),
			attribute.String("trigger_object_type", scenario.TriggerObjectType),
		))
	defer span.End()

	table := dataModel.Tables[scenario.TriggerObjectType]

	objectMap, err := usecase.ingestedDataReadRepository.QueryIngestedObject(ctx, db, table, decisionToCreate.ObjectId)
	if err != nil {
		return false, errors.Wrap(err, "error while querying ingested objects in RunScheduledExecution.createSingleDecisionForObjectId")
	} else if len(objectMap) == 0 {
		return false, fmt.Errorf("object %s not found in table %s", decisionToCreate.ObjectId, table.Name)
	}
	object := models.ClientObject{TableName: table.Name, Data: objectMap[0]}
	scenarioExecution, err := evaluate_scenario.EvalScenario(
		ctx,
		evaluate_scenario.ScenarioEvaluationParameters{
			Scenario:     scenario,
			ClientObject: object,
			DataModel:    dataModel,
			Pivot:        pivot,
		},
		evaluate_scenario.ScenarioEvaluationRepositories{
			EvalScenarioRepository:     usecase.repository,
			ExecutorFactory:            usecase.executorFactory,
			IngestedDataReadRepository: usecase.ingestedDataReadRepository,
			EvaluateAstExpression:      usecase.evaluateAstExpression,
			SnoozeReader:               usecase.snoozesReader,
		},
	)

	if errors.Is(err, models.ErrScenarioTriggerConditionAndTriggerObjectMismatch) {
		logger := utils.LoggerFromContext(ctx)
		logger.InfoContext(ctx, "Trigger condition and trigger object mismatch",
			"scenario_id", scenario.Id,
			"object_index", objectIdx,
			"trigger_object_type", scenario.TriggerObjectType,
			"object", object)

		if err := usecase.repository.UpdateDecisionToCreateStatus(
			ctx,
			usecase.executorFactory.NewExecutor(),
			decisionToCreate.Id,
			models.DecisionToCreateStatusTriggerConditionMismatch,
		); err != nil {
			return false, err
		}
		return false, nil
	} else if err != nil {
		return false, errors.Wrapf(err, "error evaluating scenario in executeScheduledScenario %s", scenario.Id)
	}

	decision := models.AdaptScenarExecToDecision(scenarioExecution, object, &scheduledExecutionId)
	sendWebhookEventId := make([]string, 0, 2)
	err = usecase.transactionFactory.Transaction(ctx, func(tx repositories.Executor) error {
		err = usecase.decisionRepository.StoreDecision(
			ctx,
			tx,
			decision,
			scenario.OrganizationId,
			decision.DecisionId,
		)
		if err != nil {
			return errors.Wrapf(err, "error storing decision in executeScheduledScenario %s", scenario.Id)
		}

		webhookEventId := uuid.NewString()
		err = usecase.webhookEventsSender.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
			Id:             webhookEventId,
			OrganizationId: decision.OrganizationId,
			EventContent:   models.NewWebhookEventDecisionCreated(decision.DecisionId),
		})
		if err != nil {
			return err
		}
		sendWebhookEventId = append(sendWebhookEventId, webhookEventId)

		caseWebhookEventId := uuid.NewString()
		webhookEventCreated, err := usecase.decisionWorkflows.AutomaticDecisionToCase(
			ctx, tx, scenario, decision, caseWebhookEventId,
		)
		if err != nil {
			return err
		}

		if webhookEventCreated {
			sendWebhookEventId = append(sendWebhookEventId, caseWebhookEventId)
		}

		return usecase.repository.UpdateDecisionToCreateStatus(ctx, tx, decisionToCreate.Id, models.DecisionToCreateStatusCreated)
	})
	if err != nil {
		return false, err
	}

	for _, webhookEventId := range sendWebhookEventId {
		usecase.webhookEventsSender.SendWebhookEventAsync(ctx, webhookEventId)
	}

	return true, nil
}
