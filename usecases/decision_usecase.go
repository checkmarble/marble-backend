package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/analytics"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/decision_phantom"
	"github.com/checkmarble/marble-backend/usecases/evaluate_scenario"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/payload_parser"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
)

const PHANTOM_DECISION_TIMEOUT = 5 * time.Second

type DecisionUsecaseRepository interface {
	DecisionWithRuleExecutionsById(
		ctx context.Context,
		exec repositories.Executor,
		decisionId string,
	) (models.DecisionWithRuleExecutions, error)
	DecisionsWithRuleExecutionsByIds(
		ctx context.Context,
		exec repositories.Executor,
		decisionIds []string,
	) ([]models.DecisionWithRuleExecutions, error)
	StoreDecision(
		ctx context.Context,
		exec repositories.Executor,
		decision models.DecisionWithRuleExecutions,
		organizationId string,
		newDecisionId string,
		dbFields map[string]any) error
	DecisionsOfOrganization(
		ctx context.Context,
		exec repositories.Executor,
		organizationId string,
		paginationAndSorting models.PaginationAndSorting,
		filters models.DecisionFilters,
	) ([]models.Decision, error)

	GetScenarioById(ctx context.Context, exec repositories.Executor, scenarioId string) (models.Scenario, error)

	GetSummarizedDecisionStatForTestRun(ctx context.Context, exec repositories.Executor,
		testRunId string) ([]models.DecisionsByVersionByOutcome, error)
	ListScenariosOfOrganization(ctx context.Context, exec repositories.Executor, organizationId string) ([]models.Scenario, error)
	ListWorkflowsForScenario(ctx context.Context, exec repositories.Executor, scenarioId uuid.UUID) ([]models.Workflow, error)

	GetAnalyticsSettings(ctx context.Context, exec repositories.Executor, orgId string) (map[string]analytics.Settings, error)
}

type ScenarioEvaluator interface {
	EvalScenario(ctx context.Context, params evaluate_scenario.ScenarioEvaluationParameters) (
		triggerPassed bool, se models.ScenarioExecution, err error)
	GetDataAccessor(params evaluate_scenario.ScenarioEvaluationParameters) evaluate_scenario.DataAccessor
}

type decisionUsecaseScreeningWriter interface {
	ListScreeningsForDecision(ctx context.Context, exec repositories.Executor, decisionId string,
		initialOnly bool) ([]models.ScreeningWithMatches, error)
	InsertScreening(
		ctx context.Context,
		exec repositories.Executor,
		sc models.ScreeningWithMatches,
	) error
}

type DecisionUsecase struct {
	enforceSecurity           security.EnforceSecurityDecision
	enforceSecurityScenario   security.EnforceSecurityScenario
	transactionFactory        executor_factory.TransactionFactory
	executorFactory           executor_factory.ExecutorFactory
	dataModelRepository       repositories.DataModelRepository
	repository                DecisionUsecaseRepository
	screeningRepository       decisionUsecaseScreeningWriter
	scenarioTestRunRepository repositories.ScenarioTestRunRepository
	offloadedReader           OffloadedReader
	webhookEventsSender       webhookEventsUsecase
	phantomUseCase            decision_phantom.PhantomDecisionUsecase
	scenarioEvaluator         ScenarioEvaluator
	openSanctionsRepository   repositories.OpenSanctionsRepository
	taskQueueRepository       repositories.TaskQueueRepository
}

func (usecase *DecisionUsecase) GetDecision(ctx context.Context, decisionId string) (models.DecisionWithRuleExecutions, error) {
	decision, err := usecase.repository.DecisionWithRuleExecutionsById(ctx,
		usecase.executorFactory.NewExecutor(), decisionId)
	if err != nil {
		return models.DecisionWithRuleExecutions{}, err
	}

	if err := usecase.enforceSecurity.ReadDecision(decision.Decision); err != nil {
		return models.DecisionWithRuleExecutions{}, err
	}

	if err := usecase.offloadedReader.MutateWithOffloadedDecisionRules(ctx,
		decision.OrganizationId.String(), decision); err != nil {
		return models.DecisionWithRuleExecutions{}, err
	}

	scs, err := usecase.screeningRepository.ListScreeningsForDecision(ctx,
		usecase.executorFactory.NewExecutor(), decision.DecisionId.String(), false)
	if err != nil {
		return models.DecisionWithRuleExecutions{}, err
	}

	decision.ScreeningExecutions = make([]models.ScreeningWithMatches, len(scs))

	for idx, sc := range scs {
		if sc.NumberOfMatches == 0 && len(sc.Matches) > 0 {
			sc.NumberOfMatches = len(sc.Matches)
		}
		decision.ScreeningExecutions[idx] = sc
	}

	return decision, nil
}

func (usecase *DecisionUsecase) GetDecisionsByOutcomeAndScore(ctx context.Context,
	testrunId string,
) ([]models.DecisionsByVersionByOutcome, error) {
	exec := usecase.executorFactory.NewExecutor()
	testrun, errTestRun := usecase.scenarioTestRunRepository.GetTestRunByID(ctx, exec, testrunId)
	if errTestRun != nil {
		return nil, errTestRun
	}

	decisions, err := usecase.repository.GetSummarizedDecisionStatForTestRun(ctx, exec, testrun.Id)
	if err != nil {
		return []models.DecisionsByVersionByOutcome{}, err
	}

	return decisions, nil
}

func (usecase *DecisionUsecase) ListDecisions(
	ctx context.Context,
	organizationId string,
	paginationAndSorting models.PaginationAndSorting,
	filters dto.DecisionFilters,
) (models.DecisionListPage, error) {
	if !filters.AllowInvalidScenarioId {
		if err := usecase.validateScenarioIds(ctx, filters.ScenarioIds, organizationId); err != nil {
			return models.DecisionListPage{}, err
		}
	}

	outcomes, err := usecase.validateOutcomes(filters.Outcomes)
	if err != nil {
		return models.DecisionListPage{}, err
	}

	if !filters.StartDate.IsZero() && !filters.EndDate.IsZero() &&
		filters.StartDate.After(filters.EndDate) {
		return models.DecisionListPage{}, fmt.Errorf(
			"start date must be before end date: %w", models.BadParameterError)
	}

	triggerObjectTypes, err := usecase.validateTriggerObjects(ctx, filters.TriggerObjects, organizationId)
	if err != nil {
		return models.DecisionListPage{}, err
	}

	if err := models.ValidatePagination(paginationAndSorting); err != nil {
		return models.DecisionListPage{}, err
	}

	paginationAndSortingWithOneMore := paginationAndSorting
	paginationAndSortingWithOneMore.Limit++

	scenarioIds, err := utils.ParseSliceUUID(filters.ScenarioIds)
	if err != nil {
		return models.DecisionListPage{}, err
	}
	decisions, err := usecase.repository.DecisionsOfOrganization(
		ctx,
		usecase.executorFactory.NewExecutor(),
		organizationId,
		paginationAndSortingWithOneMore,
		models.DecisionFilters{
			CaseIds:               filters.CaseIds,
			CaseInboxIds:          filters.CaseInboxIds,
			EndDate:               filters.EndDate,
			HasCase:               filters.HasCase,
			Outcomes:              outcomes,
			PivotValue:            filters.PivotValue,
			ReviewStatuses:        filters.ReviewStatuses,
			ScenarioIds:           scenarioIds,
			ScheduledExecutionIds: filters.ScheduledExecutionIds,
			StartDate:             filters.StartDate,
			TriggerObjects:        triggerObjectTypes,
			TriggerObjectId:       filters.TriggerObjectId,
		})
	if err != nil {
		return models.DecisionListPage{}, err
	}
	for _, decision := range decisions {
		if err := usecase.enforceSecurity.ReadDecision(decision); err != nil {
			return models.DecisionListPage{}, err
		}
	}

	hasNextPage := len(decisions) > paginationAndSorting.Limit
	if hasNextPage {
		decisions = decisions[:len(decisions)-1]
	}

	return models.DecisionListPage{
		Decisions:   decisions,
		HasNextPage: hasNextPage,
	}, nil
}

func (usecase *DecisionUsecase) validateScenarioIds(ctx context.Context, scenarioIds []string, organizationId uuid.UUID) error {
	scenarios, err := usecase.repository.ListScenariosOfOrganization(ctx,
		usecase.executorFactory.NewExecutor(), organizationId.String())
	if err != nil {
		return err
	}
	organizationScenarioIds := make([]string, len(scenarios))
	for i, scenario := range scenarios {
		organizationScenarioIds[i] = scenario.Id
	}

	for _, scenarioId := range scenarioIds {
		if !slices.Contains(organizationScenarioIds, scenarioId) {
			return fmt.Errorf("scenario id %s not found in organization %s: %w",
				scenarioId, organizationId.String(), models.BadParameterError)
		}
	}
	return nil
}

func (usecase *DecisionUsecase) validateOutcomes(filtersOutcomes []string) ([]models.Outcome, error) {
	outcomes := make([]models.Outcome, len(filtersOutcomes))
	for i, outcome := range filtersOutcomes {
		outcomes[i] = models.OutcomeFrom(outcome)
		if outcomes[i] == models.UnknownOutcome {
			return []models.Outcome{}, fmt.Errorf("invalid outcome: %s, %w", outcome, models.BadParameterError)
		}
	}
	return outcomes, nil
}

func (usecase *DecisionUsecase) validateTriggerObjects(ctx context.Context,
	filtersTriggerObjects []string, organizationId string,
) ([]string, error) {
	dataModel, err := usecase.dataModelRepository.GetDataModel(ctx,
		usecase.executorFactory.NewExecutor(), organizationId, true, true)
	if err != nil {
		return nil, err
	}
	triggerObjectTypes := make([]string, len(filtersTriggerObjects))
	for i, triggerObject := range filtersTriggerObjects {
		triggerObjectTypes[i] = triggerObject
		if _, ok := dataModel.Tables[triggerObjectTypes[i]]; !ok {
			return nil, fmt.Errorf(
				"table %s not found on data model: %w", triggerObject, models.BadParameterError)
		}
	}
	return triggerObjectTypes, nil
}

func (usecase *DecisionUsecase) CreateDecision(
	ctx context.Context,
	input models.CreateDecisionInput,
	params models.CreateDecisionParams,
) (bool, models.DecisionWithRuleExecutions, error) {
	logger := utils.LoggerFromContext(ctx).With(
		"org_id", input.OrganizationId,
		"scenario_id", input.ScenarioId,
	)
	ctx = utils.StoreLoggerInContext(ctx, logger)
	decisionStart := time.Now()

	exec := usecase.executorFactory.NewExecutor()
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(
		ctx,
		"DecisionUsecase.CreateDecision",
		trace.WithAttributes(attribute.String("scenario_id", input.ScenarioId)))
	defer span.End()

	if err := usecase.enforceSecurity.CreateDecision(input.OrganizationId); err != nil {
		return false, models.DecisionWithRuleExecutions{}, err
	}
	scenario, err := usecase.repository.GetScenarioById(ctx, exec, input.ScenarioId)
	if errors.Is(err, models.NotFoundError) {
		return false, models.DecisionWithRuleExecutions{},
			errors.WithDetail(err, "scenario not found")
	} else if err != nil {
		return false, models.DecisionWithRuleExecutions{},
			errors.Wrap(err, "error getting scenario")
	}
	if params.WithScenarioPermissionCheck {
		if err := usecase.enforceSecurityScenario.ReadScenario(scenario); err != nil {
			return false, models.DecisionWithRuleExecutions{}, err
		}
	}

	payload, dataModel, err := usecase.validatePayload(
		ctx,
		input.OrganizationId,
		input.TriggerObjectTable,
		input.ClientObject,
		input.PayloadRaw,
		params.WithDisallowUnknownFields,
	)
	if err != nil {
		return false, models.DecisionWithRuleExecutions{}, err
	}

	pivotsMeta, err := usecase.dataModelRepository.ListPivots(ctx, exec, input.OrganizationId, nil, true)
	if err != nil {
		return false, models.DecisionWithRuleExecutions{}, err
	}
	pivot := models.FindPivot(pivotsMeta, input.TriggerObjectTable, dataModel)

	evaluationParameters := evaluate_scenario.ScenarioEvaluationParameters{
		Scenario:     scenario,
		ClientObject: payload,
		DataModel:    dataModel,
		Pivot:        pivot,
	}

	triggerPassed, scenarioExecution, err :=
		usecase.scenarioEvaluator.EvalScenario(ctx, evaluationParameters)
	if err != nil {
		return false, models.DecisionWithRuleExecutions{},
			fmt.Errorf("error evaluating scenario: %w", err)
	}
	if !triggerPassed {
		usecase.executeTestRun(ctx, input.OrganizationId, input.TriggerObjectTable, evaluationParameters, scenario, nil)
		return false, models.DecisionWithRuleExecutions{}, nil
	}

	decision := models.AdaptScenarExecToDecision(scenarioExecution, payload, nil)
	if !params.WithRuleExecutionDetails {
		for i := range decision.RuleExecutions {
			decision.RuleExecutions[i].Evaluation = nil
		}
	}

	ctx, span = tracer.Start(
		ctx,
		"DecisionUsecase.CreateDecision.store_decision",
		trace.WithAttributes(attribute.String("scenario_id", input.ScenarioId)),
		trace.WithAttributes(attribute.Int("nb_rule_executions", len(decision.RuleExecutions))))
	defer span.End()

	sendWebhookEventId := make([]string, 0)
	storageStart := time.Now()
	newDecision, err := executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Transaction,
	) (models.DecisionWithRuleExecutions, error) {
		if err = usecase.repository.StoreDecision(
			ctx,
			tx,
			decision,
			input.OrganizationId,
			decision.DecisionId.String(),
			usecase.scenarioEvaluator.GetDataAccessor(evaluationParameters).GetAnalyticsFields(
				ctx, exec, usecase.repository, evaluationParameters),
		); err != nil {
			return models.DecisionWithRuleExecutions{},
				fmt.Errorf("error storing decision: %w", err)
		}

		for _, sce := range decision.ScreeningExecutions {
			err := usecase.screeningRepository.InsertScreening(ctx, tx, sce)
			if err != nil {
				return models.DecisionWithRuleExecutions{},
					errors.Wrapf(err, "error storing screening execution in CreateDecision")
			}

			if usecase.openSanctionsRepository.IsSelfHosted(ctx) {
				if err := usecase.taskQueueRepository.EnqueueMatchEnrichmentTask(
					ctx, tx, input.OrganizationId, sce.Id); err != nil {
					utils.LogAndReportSentryError(ctx, errors.Wrap(err,
						"could not enqueue screening for refinement"))
				}
			}
		}

		if params.WithDecisionWebhooks {
			webhookEventId := uuid.NewString()
			err := usecase.webhookEventsSender.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
				Id:             webhookEventId,
				OrganizationId: decision.OrganizationId.String(),
				EventContent:   models.NewWebhookEventDecisionCreated(decision.DecisionId.String()),
			})
			if err != nil {
				return models.DecisionWithRuleExecutions{}, err
			}
			sendWebhookEventId = append(sendWebhookEventId, webhookEventId)
		}

		err = usecase.taskQueueRepository.EnqueueDecisionWorkflowTask(
			ctx,
			tx,
			decision.OrganizationId.String(),
			decision.DecisionId.String(),
		)
		if err != nil {
			return models.DecisionWithRuleExecutions{},
				errors.Wrap(err, "could not execute decision workflows")
		}

		return decision, nil
	})
	if err != nil {
		return false, models.DecisionWithRuleExecutions{}, err
	}

	if scenarioExecution.ExecutionMetrics != nil {
		storageDuration := time.Since(storageStart)
		decisionDuration := time.Since(decisionStart)

		utils.MetricDecisionCount.
			With(prometheus.Labels{"org_id": decision.OrganizationId.String()}).
			Inc()

		utils.MetricDecisionLatency.
			With(prometheus.Labels{"org_id": decision.OrganizationId.String()}).
			Observe(decisionDuration.Seconds())

		scenarioExecution.ExecutionMetrics.Steps[evaluate_scenario.LogStorageDurationKey] = storageDuration.Milliseconds()

		logger.InfoContext(ctx,
			fmt.Sprintf("created decision %s in %dms", decision.DecisionId, decisionDuration.Milliseconds()),
			"decision_id", decision.DecisionId,
			"score", scenarioExecution.Score,
			"outcome", scenarioExecution.Outcome,
			"duration", decisionDuration.Milliseconds(),
			"rules", scenarioExecution.ExecutionMetrics.Rules,
			"steps", scenarioExecution.ExecutionMetrics.Steps)

	}

	for _, webhookEventId := range sendWebhookEventId {
		usecase.webhookEventsSender.SendWebhookEventAsync(ctx, webhookEventId)
	}

	usecase.executeTestRun(ctx, input.OrganizationId, input.TriggerObjectTable,
		evaluationParameters, scenario, &scenarioExecution)
	return true, newDecision, nil
}

func (usecase *DecisionUsecase) CreateAllDecisions(
	ctx context.Context,
	input models.CreateAllDecisionsInput,
	params models.CreateDecisionParams,
) (decisions []models.DecisionWithRuleExecutions, nbSkipped int, err error) {
	logger := utils.LoggerFromContext(ctx).
		With("org_id", input.OrganizationId,
			"trigger_object_table", input.TriggerObjectTable,
		)
	ctx = utils.StoreLoggerInContext(ctx, logger)
	decisionStart := time.Now()
	exec := usecase.executorFactory.NewExecutor()
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(ctx, "DecisionUsecase.CreateAllDecisions",
		trace.WithAttributes(attribute.String("trigger_object_table", input.TriggerObjectTable)),
		trace.WithAttributes(attribute.String("org_id", input.OrganizationId)),
	)
	defer span.End()

	if err = usecase.enforceSecurity.CreateDecision(input.OrganizationId); err != nil {
		return
	}

	payload, dataModel, err := usecase.validatePayload(
		ctx,
		input.OrganizationId,
		input.TriggerObjectTable,
		nil,
		input.PayloadRaw,
		params.WithDisallowUnknownFields,
	)
	if err != nil {
		return nil, 0, err
	}

	pivotsMeta, err := usecase.dataModelRepository.ListPivots(ctx, exec, input.OrganizationId, nil, true)
	if err != nil {
		return nil, 0, err
	}
	pivot := models.FindPivot(pivotsMeta, input.TriggerObjectTable, dataModel)

	scenarios, err := usecase.repository.ListScenariosOfOrganization(ctx, exec, input.OrganizationId)
	if err != nil {
		return nil, 0, errors.Wrap(err, "error getting scenarios in CreateAllDecisions")
	}
	var filteredScenarios []models.Scenario
	for _, scenario := range scenarios {
		if scenario.TriggerObjectType == input.TriggerObjectTable && scenario.LiveVersionID != nil {
			if err := usecase.enforceSecurityScenario.ReadScenario(scenario); err != nil {
				return nil, 0, err
			}
			filteredScenarios = append(filteredScenarios, scenario)
		}
	}

	type decisionAndScenario struct {
		decision        models.DecisionWithRuleExecutions
		scenario        models.Scenario
		execution       models.ScenarioExecution
		analyticsFields map[string]any
	}
	var items []decisionAndScenario
	for _, scenario := range filteredScenarios {
		evaluationParameters := evaluate_scenario.ScenarioEvaluationParameters{
			Scenario:     scenario,
			ClientObject: payload,
			DataModel:    dataModel,
			Pivot:        pivot,
		}

		ctx, cancel := context.WithTimeout(ctx, models.DECISION_TIMEOUT)
		defer cancel()
		ctx, span := tracer.Start(
			ctx,
			"DecisionUsecase.CreateAllDecisions",
			trace.WithAttributes(attribute.String("scenario_id", scenario.Id)),
		)
		defer span.End()

		triggerPassed, scenarioExecution, err :=
			usecase.scenarioEvaluator.EvalScenario(ctx, evaluationParameters)
		switch {
		case err != nil:
			return nil, 0, errors.Wrapf(err, `error evaluating scenario "%s" in CreateAllDecisions`, scenario.Name)
		case !triggerPassed:
			nbSkipped++
			sinceStart := time.Since(decisionStart)
			logger.DebugContext(ctx,
				fmt.Sprintf(`In CreateAllDecisions, skipped scenario "%s" after %dms`,
					scenario.Name, sinceStart.Milliseconds()),
				"scenario_id", scenario.Id,
				"since_start", sinceStart.Milliseconds(),
			)
			usecase.executeTestRun(ctx, input.OrganizationId, input.TriggerObjectTable, evaluationParameters, scenario, nil)
		default:
			decision := models.AdaptScenarExecToDecision(scenarioExecution, payload, nil)
			items = append(items, decisionAndScenario{
				decision: decision,
				scenario: scenario, execution: scenarioExecution,
				analyticsFields: usecase.scenarioEvaluator.GetDataAccessor(
					evaluationParameters).GetAnalyticsFields(ctx, exec, usecase.repository, evaluationParameters),
			})
			usecase.executeTestRun(ctx, input.OrganizationId, input.TriggerObjectTable,
				evaluationParameters, scenario, &scenarioExecution)
		}
	}

	ctx, span2 := tracer.Start(ctx, "DecisionUsecase.CreateAllDecisions - store decisions")
	defer span2.End()

	// gather the decisions to return in this slice
	decisions = make([]models.DecisionWithRuleExecutions, len(items))
	sendWebhookEventIds := make([]string, 0)
	lastWriteTime := time.Now()
	err = usecase.transactionFactory.Transaction(ctx, func(tx repositories.Transaction) error {
		for i, item := range items {
			thisStorageStart := time.Now()

			if err = usecase.repository.StoreDecision(
				ctx,
				tx,
				item.decision,
				input.OrganizationId,
				item.decision.DecisionId.String(),
				item.analyticsFields,
			); err != nil {
				return fmt.Errorf("error storing decision in CreateAllDecisions: %w", err)
			}

			utils.MetricDecisionCount.
				With(prometheus.Labels{"org_id": item.decision.OrganizationId.String()}).
				Inc()

			utils.MetricDecisionLatency.
				With(prometheus.Labels{"org_id": item.decision.OrganizationId.String()}).
				Observe(time.Since(decisionStart).Seconds())

			for _, sce := range item.decision.ScreeningExecutions {
				err := usecase.screeningRepository.InsertScreening(ctx, tx, sce)
				if err != nil {
					return errors.Wrapf(err, "error storing screening execution in CreateAllDecisions")
				}

				if usecase.openSanctionsRepository.IsSelfHosted(ctx) {
					if err := usecase.taskQueueRepository.EnqueueMatchEnrichmentTask(
						ctx,
						tx,
						input.OrganizationId,
						sce.Id,
					); err != nil {
						utils.LogAndReportSentryError(ctx, errors.Wrap(err,
							"could not enqueue screening for refinement"))
					}
				}
			}
			decisions[i] = item.decision

			webhookEventId := uuid.NewString()
			err := usecase.webhookEventsSender.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
				Id:             webhookEventId,
				OrganizationId: item.decision.OrganizationId.String(),
				EventContent:   models.NewWebhookEventDecisionCreated(item.decision.DecisionId.String()),
			})
			if err != nil {
				return err
			}
			sendWebhookEventIds = append(sendWebhookEventIds, webhookEventId)

			err = usecase.taskQueueRepository.EnqueueDecisionWorkflowTask(
				ctx,
				tx,
				item.decision.OrganizationId.String(),
				item.decision.DecisionId.String(),
			)
			if err != nil {
				return errors.Wrapf(err, `Failed to execute decision workflows for decision on scenario "%s"`, item.scenario.Name)
			}

			if item.execution.ExecutionMetrics != nil {
				storageDuration := time.Since(thisStorageStart)
				sinceStart := time.Since(decisionStart)
				item.execution.ExecutionMetrics.Steps[evaluate_scenario.LogStorageDurationKey] = storageDuration.Milliseconds()

				logger.InfoContext(ctx,
					fmt.Sprintf(`In CreateAllDecisions, created decision for scenario "%s" after %dms`,
						item.scenario.Name, sinceStart.Milliseconds()),
					"decision_id", item.decision.DecisionId,
					"scenario_id", item.scenario.Id,
					"score", item.execution.Score,
					"outcome", item.execution.Outcome,
					"since_start", sinceStart.Milliseconds(),
					"rules", item.execution.ExecutionMetrics.Rules,
					"steps", item.execution.ExecutionMetrics.Steps,
				)
			}
		}

		lastWriteTime = time.Now()
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	// Note: do not do anything that may take time after this, otherwise the info logs about decision execution time will be incorrect.

	sinceLastWrite := time.Since(lastWriteTime)
	logger.InfoContext(ctx, fmt.Sprintf("In CreateAllDecisions, committed all decisions in %dms", sinceLastWrite.Milliseconds()),
		"since_last_write", sinceLastWrite.Milliseconds(),
		"nb_decisions", len(decisions),
	)

	for _, caseWebhookEventId := range sendWebhookEventIds {
		usecase.webhookEventsSender.SendWebhookEventAsync(ctx, caseWebhookEventId)
	}

	return decisions, nbSkipped, nil
}

func (usecase *DecisionUsecase) executeTestRun(
	ctx context.Context,
	organizationId string,
	triggerObjectTable string,
	evaluationParameters evaluate_scenario.ScenarioEvaluationParameters,
	scenario models.Scenario,
	scenarioExecution *models.ScenarioExecution,
) {
	defer utils.RecoverAndReportSentryError(ctx, "executeTestRun")
	phantomInput := models.CreatePhantomDecisionInput{
		OrganizationId:     organizationId,
		Scenario:           scenario,
		ClientObject:       evaluationParameters.ClientObject,
		Pivot:              evaluationParameters.Pivot,
		TriggerObjectTable: triggerObjectTable,
	}
	if scenarioExecution != nil {
		evaluationParameters.CachedScreenings = pure_utils.MapSliceToMap(
			scenarioExecution.ScreeningExecutions,
			func(scm models.ScreeningWithMatches) (string, models.ScreeningWithMatches) {
				return scenarioExecution.ScenarioIterationId.String(), scm
			},
		)
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), PHANTOM_DECISION_TIMEOUT)
	defer cancel()
	logger := utils.LoggerFromContext(ctx).With(
		"phantom_decisions_with_scenario_id", phantomInput.Scenario.Id)
	_, _, errPhantom := usecase.phantomUseCase.CreatePhantomDecision(ctx, phantomInput, evaluationParameters)
	if errPhantom != nil {
		logger.ErrorContext(ctx, fmt.Sprintf("Error when creating phantom decisions with scenario id %s: %s",
			phantomInput.Scenario.Id, errPhantom.Error()))
	}
}

// used in different contexts, so allow different cases of input: pass client object or raw payload
func (usecase DecisionUsecase) validatePayload(
	ctx context.Context,
	organizationId string,
	triggerObjectTable string,
	clientObject *models.ClientObject,
	rawPayload json.RawMessage,
	disallowUnknownFields bool,
) (payload models.ClientObject, dataModel models.DataModel, err error) {
	exec := usecase.executorFactory.NewExecutor()

	if clientObject == nil && len(rawPayload) == 0 {
		err = errors.Wrap(
			models.BadParameterError,
			"empty payload received in validatePayload")
		return
	}

	dataModel, err = usecase.dataModelRepository.GetDataModel(ctx, exec, organizationId, false, true)
	if err != nil {
		err = errors.Wrap(err, "error getting data model in validatePayload")
		return
	}

	tables := dataModel.Tables
	table, ok := tables[triggerObjectTable]
	if !ok {
		err = errors.Wrap(
			models.NotFoundError,
			fmt.Sprintf("table %s not found in data model in validatePayload", triggerObjectTable),
		)
		return
	}

	if clientObject != nil {
		payload = *clientObject
		return
	}

	parser := payload_parser.NewParser()

	if disallowUnknownFields {
		parser = payload_parser.NewParser(payload_parser.DisallowUnknownFields())
	}

	payload, err = parser.ParsePayload(ctx, table, rawPayload)
	if err != nil {
		err = errors.Wrap(err, "error parsing payload in decision usecase validate payload")
		return
	}

	return
}
