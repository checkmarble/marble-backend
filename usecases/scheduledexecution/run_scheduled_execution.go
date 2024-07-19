package scheduledexecution

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/adhocore/gronx"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
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

	ListScheduledExecutions(ctx context.Context, exec repositories.Executor,
		filters models.ListScheduledExecutionsFilters) ([]models.ScheduledExecution, error)
	CreateScheduledExecution(ctx context.Context, exec repositories.Executor,
		input models.CreateScheduledExecutionInput, newScheduledExecutionId string) error
	UpdateScheduledExecution(ctx context.Context, exec repositories.Executor,
		updateScheduledEx models.UpdateScheduledExecutionInput) error
	GetScheduledExecution(ctx context.Context, exec repositories.Executor, id string) (models.ScheduledExecution, error)
}

type RunScheduledExecution struct {
	repository                     RunScheduledExecutionRepository
	executorFactory                executor_factory.ExecutorFactory
	exportScheduleExecution        ExportScheduleExecution
	scenarioPublicationsRepository repositories.ScenarioPublicationRepository
	dataModelRepository            repositories.DataModelRepository
	ingestedDataReadRepository     repositories.IngestedDataReadRepository
	evaluateAstExpression          ast_eval.EvaluateAstExpression
	decisionRepository             repositories.DecisionRepository
	transactionFactory             executor_factory.TransactionFactory
	decisionWorkflows              decisionWorkflowsUsecase
	webhookEventsSender            webhookEventsUsecase
}

func NewRunScheduledExecution(
	repository RunScheduledExecutionRepository,
	executorFactory executor_factory.ExecutorFactory,
	exportScheduleExecution ExportScheduleExecution,
	scenarioPublicationsRepository repositories.ScenarioPublicationRepository,
	dataModelRepository repositories.DataModelRepository,
	ingestedDataReadRepository repositories.IngestedDataReadRepository,
	evaluateAstExpression ast_eval.EvaluateAstExpression,
	decisionRepository repositories.DecisionRepository,
	transactionFactory executor_factory.TransactionFactory,
	decisionWorkflows decisionWorkflowsUsecase,
	webhookEventsSender webhookEventsUsecase,
) *RunScheduledExecution {
	return &RunScheduledExecution{
		repository:                     repository,
		executorFactory:                executorFactory,
		exportScheduleExecution:        exportScheduleExecution,
		scenarioPublicationsRepository: scenarioPublicationsRepository,
		dataModelRepository:            dataModelRepository,
		ingestedDataReadRepository:     ingestedDataReadRepository,
		evaluateAstExpression:          evaluateAstExpression,
		decisionRepository:             decisionRepository,
		transactionFactory:             transactionFactory,
		decisionWorkflows:              decisionWorkflows,
		webhookEventsSender:            webhookEventsSender,
	}
}

func (usecase *RunScheduledExecution) ScheduleScenarioIfDue(ctx context.Context, organizationId string, scenarioId string) error {
	exec := usecase.executorFactory.NewExecutor()
	logger := utils.LoggerFromContext(ctx)
	scenario, err := usecase.repository.GetScenarioById(ctx, exec, scenarioId)
	if err != nil {
		return err
	}

	publishedVersion, err := usecase.getPublishedScenarioIteration(ctx, exec, scenario)
	if err != nil {
		return err
	}
	if publishedVersion == nil {
		logger.DebugContext(ctx, fmt.Sprintf("scenario %s has no published version", scenarioId))
		return nil
	}

	previousExecutions, err := usecase.repository.ListScheduledExecutions(ctx, exec, models.ListScheduledExecutionsFilters{
		ScenarioId: scenarioId, Status: []models.ScheduledExecutionStatus{
			models.ScheduledExecutionPending, models.ScheduledExecutionProcessing,
		},
	})
	if err != nil {
		return err
	}
	if len(previousExecutions) > 0 {
		logger.DebugContext(ctx, fmt.Sprintf("scenario %s has already a pending or processing scheduled execution", scenarioId))
		return nil
	}

	isDue, err := usecase.scenarioIsDue(ctx, *publishedVersion, scenario)
	if err != nil {
		return err
	}
	if !isDue {
		return nil
	}

	logger.DebugContext(ctx, fmt.Sprintf("Scenario iteration %s is due", publishedVersion.Id))
	scheduledExecutionId := pure_utils.NewPrimaryKey(organizationId)
	return usecase.repository.CreateScheduledExecution(ctx, exec, models.CreateScheduledExecutionInput{
		OrganizationId:      organizationId,
		ScenarioId:          scenarioId,
		ScenarioIterationId: publishedVersion.Id,
		Manual:              false,
	}, scheduledExecutionId)
}

func (usecase *RunScheduledExecution) ExecuteAllScheduledScenarios(ctx context.Context) error {
	logger := utils.LoggerFromContext(ctx)

	pendingScheduledExecutions, err := usecase.repository.ListScheduledExecutions(ctx,
		usecase.executorFactory.NewExecutor(), models.ListScheduledExecutionsFilters{
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
		if err := usecase.ExecuteScheduledScenario(ctx, logger, scheduledExecution); err != nil {
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

func (usecase *RunScheduledExecution) ExecuteScheduledScenario(
	ctx context.Context,
	logger *slog.Logger,
	scheduledExecution models.ScheduledExecution,
) error {
	exec := usecase.executorFactory.NewExecutor()
	logger.InfoContext(ctx, fmt.Sprintf("Start execution %s", scheduledExecution.Id))

	if err := usecase.repository.UpdateScheduledExecution(ctx, exec, models.UpdateScheduledExecutionInput{
		Id:     scheduledExecution.Id,
		Status: utils.PtrTo(models.ScheduledExecutionProcessing, nil),
	}); err != nil {
		return err
	}

	scheduledExecution, err := executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Executor) (models.ScheduledExecution, error) {
			numberOfCreatedDecisions, err := usecase.executeScheduledScenario(
				ctx,
				scheduledExecution.Id,
				scheduledExecution.Scenario,
			)
			if err != nil {
				return scheduledExecution, err
			}
			err = usecase.repository.UpdateScheduledExecution(
				ctx,
				tx,
				models.UpdateScheduledExecutionInput{
					Id:                       scheduledExecution.Id,
					Status:                   utils.PtrTo(models.ScheduledExecutionSuccess, nil),
					NumberOfCreatedDecisions: &numberOfCreatedDecisions,
				},
			)
			if err != nil {
				return scheduledExecution, err
			}
			return usecase.repository.GetScheduledExecution(ctx, tx, scheduledExecution.Id)
		})
	if err != nil {
		err2 := usecase.repository.UpdateScheduledExecution(ctx, exec, models.UpdateScheduledExecutionInput{
			Id:     scheduledExecution.Id,
			Status: utils.PtrTo(models.ScheduledExecutionFailure, nil),
		})
		if err2 != nil {
			return errors.Join(err, err2)
		}

		return err
	}

	logger.InfoContext(ctx, fmt.Sprintf("Execution completed for %s", scheduledExecution.Id))
	return usecase.exportScheduleExecution.ExportScheduledExecutionToS3(ctx,
		scheduledExecution.Scenario, scheduledExecution)
}

func (usecase *RunScheduledExecution) scenarioIsDue(
	ctx context.Context,
	publishedVersion models.PublishedScenarioIteration,
	scenario models.Scenario,
) (bool, error) {
	exec := usecase.executorFactory.NewExecutor()
	logger := utils.LoggerFromContext(ctx)
	if publishedVersion.Body.Schedule == "" {
		logger.DebugContext(ctx, fmt.Sprintf("Scenario iteration %s has no schedule", publishedVersion.Id))
		return false, nil
	}
	gron := gronx.New()
	ok := gron.IsValid(publishedVersion.Body.Schedule)
	if !ok {
		return false, fmt.Errorf("invalid schedule: %w", models.BadParameterError)
	}

	previousExecutions, err := usecase.repository.ListScheduledExecutions(ctx, exec, models.ListScheduledExecutionsFilters{
		ScenarioId: scenario.Id, ExcludeManual: true,
	})
	if err != nil {
		return false, fmt.Errorf("error listing scheduled executions: %w", err)
	}

	publications, err := usecase.scenarioPublicationsRepository.ListScenarioPublicationsOfOrganization(
		ctx, exec, scenario.OrganizationId, models.ListScenarioPublicationsFilters{ScenarioId: &scenario.Id})
	if err != nil {
		return false, err
	}

	tz, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		return false, errors.Wrap(err, "error loading timezone")
	}
	return executionIsDueNow(publishedVersion.Body.Schedule, previousExecutions, publications, tz)
}

func executionIsDueNow(
	schedule string,
	previousExecutions []models.ScheduledExecution,
	publications []models.ScenarioPublication,
	tz *time.Location,
) (bool, error) {
	if tz == nil {
		return false, errors.New("Nil timezone passed in executionIsDueNow")
	}
	var referenceTime time.Time
	if len(previousExecutions) > 0 {
		referenceTime = previousExecutions[0].StartedAt.In(tz)
	} else {
		// if there is no previous execution, consider the last iteration publication time to be the last execution time
		referenceTime = publications[0].CreatedAt.In(tz)
	}

	nextTick, err := gronx.NextTickAfter(schedule, referenceTime, false)
	if err != nil {
		return true, err
	}
	if nextTick.After(time.Now()) {
		return false, nil
	}
	return true, nil
}

func (usecase *RunScheduledExecution) executeScheduledScenario(ctx context.Context,
	scheduledExecutionId string, scenario models.Scenario,
) (int, error) {
	exec := usecase.executorFactory.NewExecutor()
	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx, exec, scenario.OrganizationId, false)
	if err != nil {
		return 0, err
	}
	tables := dataModel.Tables
	table, ok := tables[scenario.TriggerObjectType]
	if !ok {
		return 0, fmt.Errorf("trigger object type %s not found in data model: %w",
			scenario.TriggerObjectType, models.NotFoundError)
	}

	pivotsMeta, err := usecase.dataModelRepository.ListPivots(ctx, exec, scenario.OrganizationId, nil)
	if err != nil {
		return 0, err
	}
	pivot := models.FindPivot(pivotsMeta, scenario.TriggerObjectType, dataModel)

	// list objects to score
	numberOfCreatedDecisions := 0
	var objects []models.ClientObject
	db, err := usecase.executorFactory.NewClientDbExecutor(ctx, scenario.OrganizationId)
	if err != nil {
		return 0, err
	}
	objects, err = usecase.ingestedDataReadRepository.ListAllObjectsFromTable(ctx, db, table)
	if err != nil {
		return 0, err
	}

	tracer := utils.OpenTelemetryTracerFromContext(ctx)

	sendWebhookEventId := make([]string, 0)
	err = usecase.transactionFactory.Transaction(ctx, func(tx repositories.Executor) error {
		executionScenario := func(ctx context.Context, object models.ClientObject, i int) error {
			ctx, span := tracer.Start(
				ctx,
				"Batch RunScheduledExecution.executeScheduledScenario",
				trace.WithAttributes(
					attribute.String("scenario_id", scenario.Id),
					attribute.Int64("object_index", int64(i)),
					attribute.String("object_type", scenario.TriggerObjectType),
				))
			defer span.End()
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
				},
			)

			if errors.Is(err, models.ErrScenarioTriggerConditionAndTriggerObjectMismatch) {
				logger := utils.LoggerFromContext(ctx)
				logger.InfoContext(ctx,
					fmt.Sprintf("Trigger condition and trigger object mismatch: %s", err.Error()),
					"scenarioId", scenario.Id,
					"object_index", i,
					"triggerObjectType", scenario.TriggerObjectType,
					"object", object)
				return nil
			} else if err != nil {
				return errors.Wrapf(err, "error evaluating scenario in executeScheduledScenario %s", scenario.Id)
			}

			decision := models.AdaptScenarExecToDecision(scenarioExecution, object, &scheduledExecutionId)
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
				ctx, tx, scenario, decision, caseWebhookEventId)
			if err != nil {
				return err
			}
			if webhookEventCreated {
				sendWebhookEventId = append(sendWebhookEventId, caseWebhookEventId)
			}

			numberOfCreatedDecisions += 1
			return nil
		}

		// execute scenario for each object
		for i, object := range objects {
			if err := executionScenario(ctx, object, i); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return numberOfCreatedDecisions, err
	}

	for _, webhookEventId := range sendWebhookEventId {
		usecase.webhookEventsSender.SendWebhookEventAsync(ctx, webhookEventId)
	}

	return numberOfCreatedDecisions, nil
}

func (usecase *RunScheduledExecution) getPublishedScenarioIteration(
	ctx context.Context,
	exec repositories.Executor,
	scenario models.Scenario,
) (*models.PublishedScenarioIteration, error) {
	if scenario.LiveVersionID == nil {
		return nil, nil
	}

	liveVersion, err := usecase.repository.GetScenarioIteration(ctx, exec, *scenario.LiveVersionID)
	if err != nil {
		return nil, err
	}
	publishedVersion, err := models.NewPublishedScenarioIteration(liveVersion)
	if err != nil {
		return nil, err
	}
	return &publishedVersion, nil
}
