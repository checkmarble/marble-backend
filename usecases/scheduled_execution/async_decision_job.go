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
	var testRunCallback func()
	err := w.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		var err error
		webhookEventIds, testRunCallback, err = w.handleDecision(ctx, args, tx)
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
	if testRunCallback != nil {
		testRunCallback()
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
) (webhookIds []string, testRunCallback func(), err error) {
	decisionToCreate, err := w.repository.GetDecisionToCreate(ctx, tx, args.DecisionToCreateId, true)
	if err != nil {
		return nil, nil, err
	}
	if slices.Contains([]models.DecisionToCreateStatus{
		models.DecisionToCreateStatusCreated,
		models.DecisionToCreateStatusTriggerConditionMismatch,
	}, decisionToCreate.Status) {
		return nil, nil, nil
	}

	decisionCreated, webhookEventIds, testRunCallback, err :=
		w.createSingleDecisionForObjectId(ctx, args, tx)
	if err != nil {
		statusErr := w.repository.UpdateDecisionToCreateStatus(
			ctx,
			tx,
			args.DecisionToCreateId,
			models.DecisionToCreateStatusFailed,
		)
		return nil, nil, errors.Join(err, statusErr)
	}

	if decisionCreated {
		err = w.repository.UpdateDecisionToCreateStatus(ctx, tx, args.DecisionToCreateId, models.DecisionToCreateStatusCreated)
		if err != nil {
			return nil, nil, err
		}
	} else {
		err = w.repository.UpdateDecisionToCreateStatus(
			ctx, tx, args.DecisionToCreateId, models.DecisionToCreateStatusTriggerConditionMismatch)
		if err != nil {
			return nil, nil, err
		}
	}

	err = w.possiblyUpdateScheduledExecNumbers(ctx, tx, args)
	if err != nil {
		return nil, nil, err
	}

	return webhookEventIds, testRunCallback, nil
}

func (w *AsyncDecisionWorker) createSingleDecisionForObjectId(
	ctx context.Context,
	args models.AsyncDecisionArgs,
	tx repositories.Transaction,
) (decisionCreated bool, webhookIds []string, testRunCallback func(), err error) {
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
		return false, nil, nil, err
	}
	if scheduledExecution.Status != models.ScheduledExecutionProcessing {
		return false, nil, nil, nil
	}

	scenarioAndIteration, err := w.scenarioFetcher.FetchScenarioAndIteration(ctx, tx, args.ScenarioIterationId)
	if err != nil {
		return false, nil, nil, err
	}
	scenario := scenarioAndIteration.Scenario

	dataModel, err := w.dataModelRepository.GetDataModel(ctx, tx, scenario.OrganizationId, false)
	if err != nil {
		return false, nil, nil, err
	}
	tables := dataModel.Tables
	table, ok := tables[scenario.TriggerObjectType]
	if !ok {
		return false, nil, nil, fmt.Errorf(
			"trigger object type %s not found in data model: %w",
			scenario.TriggerObjectType, models.NotFoundError)
	}

	pivotsMeta, err := w.dataModelRepository.ListPivots(ctx, tx, scenario.OrganizationId, nil)
	if err != nil {
		return false, nil, nil, err
	}
	pivot := models.FindPivot(pivotsMeta, scenario.TriggerObjectType, dataModel)

	// list objects to score
	db, err := w.executorFactory.NewClientDbExecutor(ctx, scenario.OrganizationId)
	if err != nil {
		return false, nil, nil, err
	}
	objectMap, err := w.ingestedDataReadRepository.QueryIngestedObject(ctx, db, table, args.ObjectId)
	if err != nil {
		return false, nil, nil, errors.Wrap(err, "error while querying ingested objects in AsyncDecisionWorker.createSingleDecisionForObjectId")
	} else if len(objectMap) == 0 {
		utils.LogAndReportSentryError(ctx, errors.Newf("object %s not found in table %s", args.ObjectId, table.Name))
		return false, nil, nil, nil
	}

	object := models.ClientObject{TableName: table.Name, Data: objectMap[0].Data}

	evaluationParameters := evaluate_scenario.ScenarioEvaluationParameters{
		Scenario:          scenario,
		TargetIterationId: &args.ScenarioIterationId,
		ClientObject:      object,
		DataModel:         dataModel,
		Pivot:             pivot,
	}

	// Note: Statistics on test runs when using batch scenarios may still be slightly off, because the batch execution does a partial
	// precomputation of the filters from the trigger condition, based on the live version of the scenario.
	// The cleaner solution would be to do a parallel execution in batch of the test run, but even then it would be extremely tiresome to
	// protect against all edge cases if part of one or the other job fails to run for unexpected reasons.
	// We probably want to live with this for the time being.
	executeTestRun := func(se *models.ScenarioExecution) {
		evaluationParameters.TargetIterationId = nil
		if se != nil {
			evaluationParameters.CachedSanctionCheck = se.SanctionCheckExecution
		}
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
	}

	triggerPassed, scenarioExecution, err :=
		w.scenarioEvaluator.EvalScenario(ctx, evaluationParameters)
	if err != nil {
		return false, nil, nil, errors.Wrapf(err, "error evaluating scenario in AsyncDecisionWorker %s", scenario.Id)
	}
	if !triggerPassed {
		if err := w.repository.UpdateDecisionToCreateStatus(
			ctx,
			tx,
			args.DecisionToCreateId,
			models.DecisionToCreateStatusTriggerConditionMismatch,
		); err != nil {
			return false, nil, nil, err
		}

		return false, nil, func() { executeTestRun(nil) }, nil
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
		return false, nil, nil, errors.Wrapf(err, "error storing decision in AsyncDecisionWorker %s", scenario.Id)
	}

	if decision.SanctionCheckExecution != nil {
		_, err := w.sanctionCheckRepository.InsertSanctionCheck(ctx, tx,
			decision.DecisionId, *decision.SanctionCheckExecution, true)
		if err != nil {
			return false, nil, nil, errors.Wrap(err,
				"could not store sanction check execution")
		}
	}

	webhookEventId := uuid.NewString()
	err = w.webhookEventsSender.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
		Id:             webhookEventId,
		OrganizationId: decision.OrganizationId,
		EventContent:   models.NewWebhookEventDecisionCreated(decision.DecisionId),
	})
	if err != nil {
		return false, nil, nil, err
	}
	sendWebhookEventId = append(sendWebhookEventId, webhookEventId)

	caseWebhookEventId := uuid.NewString()
	webhookEventCreated, err := w.decisionWorkflows.AutomaticDecisionToCase(ctx, tx, scenario,
		decision, evaluationParameters, caseWebhookEventId)
	if err != nil {
		return false, nil, nil, err
	}

	if webhookEventCreated {
		sendWebhookEventId = append(sendWebhookEventId, caseWebhookEventId)
	}

	err = w.repository.UpdateDecisionToCreateStatus(ctx, tx, args.DecisionToCreateId, models.DecisionToCreateStatusCreated)
	if err != nil {
		return false, nil, nil, err
	}

	return true, sendWebhookEventId, func() { executeTestRun(&scenarioExecution) }, nil
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
