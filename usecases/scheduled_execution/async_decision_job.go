package scheduled_execution

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/decision_phantom"
	"github.com/checkmarble/marble-backend/usecases/evaluate_scenario"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/scenarios"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type decisionWorkflowsUsecase interface {
	AutomaticDecisionToCase(
		ctx context.Context,
		tx repositories.Transaction,
		scenario models.Scenario,
		decision models.DecisionWithRuleExecutions,
		params evaluate_scenario.ScenarioEvaluationParameters,
		webhookEventId string,
	) (bool, error)
}

type webhookEventsUsecase interface {
	CreateWebhookEvent(
		ctx context.Context,
		tx repositories.Transaction,
		input models.WebhookEventCreate,
	) error
	SendWebhookEventAsync(ctx context.Context, webhookEventId string)
}

type asyncDecisionWorkerRepository interface {
	GetScenarioIteration(ctx context.Context, exec repositories.Executor, scenarioIterationId string) (models.ScenarioIteration, error)
	UpdateDecisionToCreateStatus(
		ctx context.Context,
		exec repositories.Executor,
		id string,
		status models.DecisionToCreateStatus,
	) error
	GetDecisionToCreate(
		ctx context.Context,
		tx repositories.Executor,
		decisionToCreateId string,
		forUpdate ...bool,
	) (models.DecisionToCreate, error)
	ListDecisionsToCreate(
		ctx context.Context,
		exec repositories.Executor,
		filters models.ListDecisionsToCreateFilters,
		limit *int,
	) ([]models.DecisionToCreate, error)
	CountCompletedDecisionsByStatus(
		ctx context.Context,
		exec repositories.Executor,
		ScheduledExecutionId string,
	) (models.DecisionToCreateCountMetadata, error)
	GetScheduledExecution(ctx context.Context, exec repositories.Executor, id string) (models.ScheduledExecution, error)
	UpdateScheduledExecutionStatus(
		ctx context.Context,
		exec repositories.Executor,
		updateScheduledEx models.UpdateScheduledExecutionStatusInput,
	) (executed bool, err error)
}

type ScenarioEvaluator interface {
	EvalScenario(ctx context.Context, params evaluate_scenario.ScenarioEvaluationParameters) (
		triggerPassed bool, se models.ScenarioExecution, err error)
}

type decisionWorkerSanctionCheckWriter interface {
	InsertSanctionCheck(
		ctx context.Context,
		exec repositories.Executor,
		decisionid string,
		sc models.SanctionCheckWithMatches,
		storeMatches bool,
	) (models.SanctionCheckWithMatches, error)
}

type AsyncDecisionWorker struct {
	river.WorkerDefaults[models.AsyncDecisionArgs]

	repository                 asyncDecisionWorkerRepository
	executorFactory            executor_factory.ExecutorFactory
	dataModelRepository        repositories.DataModelRepository
	ingestedDataReadRepository repositories.IngestedDataReadRepository
	decisionRepository         repositories.DecisionRepository
	transactionFactory         executor_factory.TransactionFactory
	decisionWorkflows          decisionWorkflowsUsecase
	webhookEventsSender        webhookEventsUsecase
	scenarioFetcher            scenarios.ScenarioFetcher
	phantomDecision            decision_phantom.PhantomDecisionUsecase
	scenarioEvaluator          ScenarioEvaluator
	sanctionCheckRepository    decisionWorkerSanctionCheckWriter
}

func NewAsyncDecisionWorker(
	repository asyncDecisionWorkerRepository,
	executorFactory executor_factory.ExecutorFactory,
	dataModelRepository repositories.DataModelRepository,
	ingestedDataReadRepository repositories.IngestedDataReadRepository,
	decisionRepository repositories.DecisionRepository,
	transactionFactory executor_factory.TransactionFactory,
	decisionWorkflows decisionWorkflowsUsecase,
	webhookEventsSender webhookEventsUsecase,
	scenarioFetcher scenarios.ScenarioFetcher,
	phantom decision_phantom.PhantomDecisionUsecase,
	scenarioEvaluator ScenarioEvaluator,
	sanctionCheckRepository decisionWorkerSanctionCheckWriter,
) AsyncDecisionWorker {
	return AsyncDecisionWorker{
		repository:                 repository,
		executorFactory:            executorFactory,
		dataModelRepository:        dataModelRepository,
		ingestedDataReadRepository: ingestedDataReadRepository,
		decisionRepository:         decisionRepository,
		transactionFactory:         transactionFactory,
		decisionWorkflows:          decisionWorkflows,
		webhookEventsSender:        webhookEventsSender,
		scenarioFetcher:            scenarioFetcher,
		phantomDecision:            phantom,
		scenarioEvaluator:          scenarioEvaluator,
		sanctionCheckRepository:    sanctionCheckRepository,
	}
}

func (w *AsyncDecisionWorker) Work(ctx context.Context, job *river.Job[models.AsyncDecisionArgs]) error {
	args := job.Args

	var webhookEventIds []string
	err := w.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		var err error
		webhookEventIds, err = w.handleDecision(ctx, args, tx)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}
	// TODO: it's really a pain to do this after we committed the transaction - we can stop doing this once webhook sending itself is also moved into a task queue
	for _, webhookEventId := range webhookEventIds {
		w.webhookEventsSender.SendWebhookEventAsync(ctx, webhookEventId)
	}

	return nil
}

func (w *AsyncDecisionWorker) Timeout(job *river.Job[models.AsyncDecisionArgs]) time.Duration {
	return 10 * time.Second
}

func (w *AsyncDecisionWorker) handleDecision(
	ctx context.Context,
	args models.AsyncDecisionArgs,
	tx repositories.Transaction,
) (webhookIds []string, err error) {
	decisionToCreate, err := w.repository.GetDecisionToCreate(ctx, tx, args.DecisionToCreateId, true)
	if err != nil {
		return nil, err
	}
	if slices.Contains([]models.DecisionToCreateStatus{
		models.DecisionToCreateStatusCreated,
		models.DecisionToCreateStatusTriggerConditionMismatch,
	}, decisionToCreate.Status) {
		return nil, nil
	}

	decisionCreated, webhookEventIds, err := w.createSingleDecisionForObjectId(ctx, args, tx)
	if err != nil {
		statusErr := w.repository.UpdateDecisionToCreateStatus(
			ctx,
			tx,
			args.DecisionToCreateId,
			models.DecisionToCreateStatusFailed,
		)
		return nil, errors.Join(err, statusErr)
	}

	if decisionCreated {
		err = w.repository.UpdateDecisionToCreateStatus(ctx, tx, args.DecisionToCreateId, models.DecisionToCreateStatusCreated)
		if err != nil {
			return nil, err
		}
	} else {
		err = w.repository.UpdateDecisionToCreateStatus(
			ctx, tx, args.DecisionToCreateId, models.DecisionToCreateStatusTriggerConditionMismatch)
		if err != nil {
			return nil, err
		}
	}

	err = w.possiblyUpdateScheduledExecNumbers(ctx, tx, args)
	if err != nil {
		return nil, err
	}

	return webhookEventIds, nil
}

func (w *AsyncDecisionWorker) createSingleDecisionForObjectId(
	ctx context.Context,
	args models.AsyncDecisionArgs,
	tx repositories.Transaction,
) (decisionCreated bool, webhookIds []string, err error) {
	logger := utils.LoggerFromContext(ctx)
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(
		ctx,
		"AsyncDecisionWorker.createSingleDecisionForObjectId",
		trace.WithAttributes(
			attribute.String("scheduled_execution_id", args.ScenarioIterationId),
			attribute.String("object_id", args.ObjectId),
			attribute.String("scenario_iteration_id", args.ScenarioIterationId),
		))
	defer span.End()

	scheduledExecution, err := w.repository.GetScheduledExecution(ctx, tx, args.ScheduledExecutionId)
	if err != nil {
		return false, nil, err
	}
	if scheduledExecution.Status != models.ScheduledExecutionProcessing {
		return false, nil, nil
	}

	scenarioAndIteration, err := w.scenarioFetcher.FetchScenarioAndIteration(ctx, tx, args.ScenarioIterationId)
	if err != nil {
		return false, nil, err
	}
	scenario := scenarioAndIteration.Scenario

	dataModel, err := w.dataModelRepository.GetDataModel(ctx, tx, scenario.OrganizationId, false)
	if err != nil {
		return false, nil, err
	}
	tables := dataModel.Tables
	table, ok := tables[scenario.TriggerObjectType]
	if !ok {
		return false, nil, fmt.Errorf(
			"trigger object type %s not found in data model: %w",
			scenario.TriggerObjectType, models.NotFoundError)
	}

	pivotsMeta, err := w.dataModelRepository.ListPivots(ctx, tx, scenario.OrganizationId, nil)
	if err != nil {
		return false, nil, err
	}
	pivot := models.FindPivot(pivotsMeta, scenario.TriggerObjectType, dataModel)

	// list objects to score
	db, err := w.executorFactory.NewClientDbExecutor(ctx, scenario.OrganizationId)
	if err != nil {
		return false, nil, err
	}
	objectMap, err := w.ingestedDataReadRepository.QueryIngestedObject(ctx, db, table, args.ObjectId)
	if err != nil {
		return false, nil, errors.Wrap(err, "error while querying ingested objects in AsyncDecisionWorker.createSingleDecisionForObjectId")
	} else if len(objectMap) == 0 {
		utils.LogAndReportSentryError(ctx, errors.Newf("object %s not found in table %s", args.ObjectId, table.Name))
		return false, nil, nil
	}

	object := models.ClientObject{TableName: table.Name, Data: objectMap[0].Data}

	evaluationParameters := evaluate_scenario.ScenarioEvaluationParameters{
		Scenario:          scenario,
		TargetIterationId: &args.ScenarioIterationId,
		ClientObject:      object,
		DataModel:         dataModel,
		Pivot:             pivot,
	}

	triggerPassed, scenarioExecution, err :=
		w.scenarioEvaluator.EvalScenario(ctx, evaluationParameters)
	if err != nil {
		return false, nil, errors.Wrapf(err, "error evaluating scenario in AsyncDecisionWorker %s", scenario.Id)
	}
	if !triggerPassed {
		logger.InfoContext(ctx, "Trigger condition and trigger object mismatch",
			"scenario_id", scenario.Id,
			"trigger_object_type", scenario.TriggerObjectType,
			"object_id", args.ObjectId)
		if err := w.repository.UpdateDecisionToCreateStatus(
			ctx,
			tx,
			args.DecisionToCreateId,
			models.DecisionToCreateStatusTriggerConditionMismatch,
		); err != nil {
			return false, nil, err
		}
		return false, nil, nil
	}

	decision := models.AdaptScenarExecToDecision(scenarioExecution, object, &args.ScheduledExecutionId)
	sendWebhookEventId := make([]string, 0, 2)

	err = w.decisionRepository.StoreDecision(
		ctx,
		tx,
		decision,
		scenario.OrganizationId,
		decision.DecisionId,
	)
	if err != nil {
		return false, nil, errors.Wrapf(err, "error storing decision in AsyncDecisionWorker %s", scenario.Id)
	}

	if decision.SanctionCheckExecution != nil {
		_, err := w.sanctionCheckRepository.InsertSanctionCheck(ctx, tx,
			decision.DecisionId, *decision.SanctionCheckExecution, true)
		if err != nil {
			return false, nil, errors.Wrap(err, "could not store sanction check execution")
		}
	}

	webhookEventId := uuid.NewString()
	err = w.webhookEventsSender.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
		Id:             webhookEventId,
		OrganizationId: decision.OrganizationId,
		EventContent:   models.NewWebhookEventDecisionCreated(decision.DecisionId),
	})
	if err != nil {
		return false, nil, err
	}
	sendWebhookEventId = append(sendWebhookEventId, webhookEventId)

	caseWebhookEventId := uuid.NewString()
	webhookEventCreated, err := w.decisionWorkflows.AutomaticDecisionToCase(ctx, tx, scenario,
		decision, evaluationParameters, caseWebhookEventId)
	if err != nil {
		return false, nil, err
	}

	if webhookEventCreated {
		sendWebhookEventId = append(sendWebhookEventId, caseWebhookEventId)
	}

	err = w.repository.UpdateDecisionToCreateStatus(ctx, tx, args.DecisionToCreateId, models.DecisionToCreateStatusCreated)
	if err != nil {
		return false, nil, err
	}
	go func() {
		evaluationParameters.TargetIterationId = nil
		evaluationParameters.CachedSanctionCheck = scenarioExecution.SanctionCheckExecution
		phantomInput := models.CreatePhantomDecisionInput{
			OrganizationId: scenario.OrganizationId,
			Scenario:       scenario,
			ClientObject:   object,
			Pivot:          pivot,
		}
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		logger := utils.LoggerFromContext(ctx).With("phantom_decisions_with_scenario_id", phantomInput.Scenario.Id)
		_, _, errPhantom := w.phantomDecision.CreatePhantomDecision(ctx, phantomInput, evaluationParameters)
		if errPhantom != nil {
			logger.ErrorContext(ctx, fmt.Sprintf("Error when creating phantom decisions with scenario id %s: %s",
				phantomInput.Scenario.Id, errPhantom.Error()))
		}
	}()

	return true, sendWebhookEventId, nil
}

func (w *AsyncDecisionWorker) possiblyUpdateScheduledExecNumbers(
	ctx context.Context,
	tx repositories.Transaction,
	args models.AsyncDecisionArgs,
) error {
	logger := utils.LoggerFromContext(ctx)

	if sample, err := w.sampleUpdateNumbers(ctx, args.ScheduledExecutionId); err != nil {
		return err
	} else if !sample {
		return nil
	}

	counts, err := w.repository.CountCompletedDecisionsByStatus(ctx, tx, args.ScheduledExecutionId)
	if err != nil {
		return err
	}

	done, err := w.repository.UpdateScheduledExecutionStatus(
		ctx,
		tx,
		models.UpdateScheduledExecutionStatusInput{
			Id:                         args.ScheduledExecutionId,
			NumberOfCreatedDecisions:   &counts.Created,
			NumberOfEvaluatedDecisions: &counts.SuccessfullyEvaluated,
			Status:                     models.ScheduledExecutionProcessing,
			CurrentStatusCondition:     models.ScheduledExecutionProcessing,
		},
	)
	if err != nil {
		return err
	}
	if !done {
		logger.InfoContext(ctx,
			"Scheduled execution is no longer in processing status, the numbers of decisions evaluated must have been updated by another task",
			slog.String("scheduled_execution_id", args.ScheduledExecutionId),
		)
	}

	return nil
}

func (w *AsyncDecisionWorker) sampleUpdateNumbers(ctx context.Context, scheduledExecutionId string) (isSampled bool, err error) {
	// naive random heuristic. We want to avoid updating the numbers too often, but we also want to avoid having the numbers be too stale.
	execution, err := w.repository.GetScheduledExecution(ctx, w.executorFactory.NewExecutor(), scheduledExecutionId)
	if err != nil {
		return false, err
	}
	var everyN int
	if execution.NumberOfPlannedDecisions == nil || *execution.NumberOfPlannedDecisions < 100 {
		everyN = 1
	} else {
		// every 32 decisions for 1000 planned, every 100 for 10k planned, every 1000 for 1M planned...
		everyN = int(math.Sqrt(float64(*execution.NumberOfPlannedDecisions)))
	}

	if rand.Int64N(int64(everyN)) == 0 {
		return true, nil
	}
	return false, nil
}
