package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
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
		newDecisionId string) error
	DecisionsOfOrganizationWithRank(
		ctx context.Context,
		exec repositories.Executor,
		organizationId string,
		paginationAndSorting models.PaginationAndSorting,
		filters models.DecisionFilters,
	) ([]models.DecisionWithRank, error)
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
}

type decisionUsecaseFeatureAccessReader interface {
	GetOrganizationFeatureAccess(
		ctx context.Context,
		organizationId string,
	) (models.OrganizationFeatureAccess, error)
}

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

type ScenarioEvaluator interface {
	EvalScenario(ctx context.Context, params evaluate_scenario.ScenarioEvaluationParameters) (
		triggerPassed bool, se models.ScenarioExecution, err error)
}

type decisionUsecaseSanctionCheckWriter interface {
	GetSanctionChecksForDecision(ctx context.Context, exec repositories.Executor, decisionId string,
		initialOnly bool) ([]models.SanctionCheckWithMatches, error)
	InsertSanctionCheck(
		ctx context.Context,
		exec repositories.Executor,
		decisionId string,
		sc models.SanctionCheckWithMatches,
		storeMatches bool,
	) (models.SanctionCheckWithMatches, error)
}

type DecisionUsecase struct {
	enforceSecurity           security.EnforceSecurityDecision
	enforceSecurityScenario   security.EnforceSecurityScenario
	transactionFactory        executor_factory.TransactionFactory
	executorFactory           executor_factory.ExecutorFactory
	dataModelRepository       repositories.DataModelRepository
	repository                DecisionUsecaseRepository
	sanctionCheckRepository   decisionUsecaseSanctionCheckWriter
	scenarioTestRunRepository repositories.ScenarioTestRunRepository
	decisionWorkflows         decisionWorkflowsUsecase
	webhookEventsSender       webhookEventsUsecase
	phantomUseCase            decision_phantom.PhantomDecisionUsecase
	featureAccessReader       decisionUsecaseFeatureAccessReader
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

	sc, err := usecase.sanctionCheckRepository.GetSanctionChecksForDecision(ctx,
		usecase.executorFactory.NewExecutor(), decision.DecisionId, false)
	if err != nil {
		return models.DecisionWithRuleExecutions{}, err
	}

	if len(sc) > 0 {
		decision.SanctionCheckExecution = &models.SanctionCheckWithMatches{
			SanctionCheck: sc[0].SanctionCheck,
			Count:         len(sc[0].Matches),
		}
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

func (usecase *DecisionUsecase) ListDecisionsWithIndexes(
	ctx context.Context,
	organizationId string,
	paginationAndSorting models.PaginationAndSorting,
	filters dto.DecisionFilters,
) (models.DecisionListPageWithIndexes, error) {
	if err := usecase.validateScenarioIds(ctx, filters.ScenarioIds, organizationId); err != nil {
		return models.DecisionListPageWithIndexes{}, err
	}

	outcomes, err := usecase.validateOutcomes(filters.Outcomes)
	if err != nil {
		return models.DecisionListPageWithIndexes{}, err
	}

	if !filters.StartDate.IsZero() && !filters.EndDate.IsZero() &&
		filters.StartDate.After(filters.EndDate) {
		return models.DecisionListPageWithIndexes{}, fmt.Errorf(
			"start date must be before end date: %w", models.BadParameterError)
	}

	triggerObjectTypes, err := usecase.validateTriggerObjects(ctx, filters.TriggerObjects, organizationId)
	if err != nil {
		return models.DecisionListPageWithIndexes{}, err
	}

	if err := models.ValidatePagination(paginationAndSorting); err != nil {
		return models.DecisionListPageWithIndexes{}, err
	}

	paginationAndSortingWithOneMore := paginationAndSorting
	paginationAndSortingWithOneMore.Limit++

	decisions, err := usecase.repository.DecisionsOfOrganizationWithRank(
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
			ScenarioIds:           filters.ScenarioIds,
			ScheduledExecutionIds: filters.ScheduledExecutionIds,
			StartDate:             filters.StartDate,
			TriggerObjects:        triggerObjectTypes,
		})
	if err != nil {
		return models.DecisionListPageWithIndexes{}, err
	}
	for _, decision := range decisions {
		if err := usecase.enforceSecurity.ReadDecision(decision.Decision); err != nil {
			return models.DecisionListPageWithIndexes{}, err
		}
	}
	// handled separately so we're sure accessing an invalid index when checking the EndIndex below
	if len(decisions) == 0 {
		return models.DecisionListPageWithIndexes{}, nil
	}

	hasNextPage := len(decisions) > paginationAndSorting.Limit
	if hasNextPage {
		decisions = decisions[:len(decisions)-1]
	}

	decisionsWithoutRank := make([]models.Decision, len(decisions))
	for i, decision := range decisions {
		decisionsWithoutRank[i] = decision.Decision
	}
	return models.DecisionListPageWithIndexes{
		Decisions:   decisionsWithoutRank,
		StartIndex:  decisions[0].RankNumber,
		EndIndex:    decisions[len(decisions)-1].RankNumber,
		HasNextPage: hasNextPage,
	}, nil
}

func (usecase *DecisionUsecase) ListDecisions(
	ctx context.Context,
	organizationId string,
	paginationAndSorting models.PaginationAndSorting,
	filters dto.DecisionFilters,
) (models.DecisionListPage, error) {
	if err := usecase.validateScenarioIds(ctx, filters.ScenarioIds, organizationId); err != nil {
		return models.DecisionListPage{}, err
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
			ScenarioIds:           filters.ScenarioIds,
			ScheduledExecutionIds: filters.ScheduledExecutionIds,
			StartDate:             filters.StartDate,
			TriggerObjects:        triggerObjectTypes,
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

func (usecase *DecisionUsecase) validateScenarioIds(ctx context.Context, scenarioIds []string, organizationId string) error {
	scenarios, err := usecase.repository.ListScenariosOfOrganization(ctx,
		usecase.executorFactory.NewExecutor(), organizationId)
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
				scenarioId, organizationId, models.BadParameterError)
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
		usecase.executorFactory.NewExecutor(), organizationId, true)
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
		return false, models.DecisionWithRuleExecutions{}, errors.Wrap(err, "scenario not found")
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
	)
	if err != nil {
		return false, models.DecisionWithRuleExecutions{}, err
	}

	pivotsMeta, err := usecase.dataModelRepository.ListPivots(ctx, exec, input.OrganizationId, nil)
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
	newDecision, err := executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Transaction,
	) (models.DecisionWithRuleExecutions, error) {
		if err = usecase.repository.StoreDecision(
			ctx,
			tx,
			decision,
			input.OrganizationId,
			decision.DecisionId,
		); err != nil {
			return models.DecisionWithRuleExecutions{},
				fmt.Errorf("error storing decision: %w", err)
		}

		var sc models.SanctionCheckWithMatches

		if decision.SanctionCheckExecution != nil {
			sc, err = usecase.sanctionCheckRepository.InsertSanctionCheck(ctx, tx,
				decision.DecisionId, *decision.SanctionCheckExecution, true)
			if err != nil {
				return models.DecisionWithRuleExecutions{},
					errors.Wrap(err, "could not store sanction check execution")
			}

			if usecase.openSanctionsRepository.IsSelfHosted(ctx) {
				if err := usecase.taskQueueRepository.EnqueueMatchEnrichmentTask(
					ctx, input.OrganizationId, sc.Id); err != nil {
					utils.LogAndReportSentryError(ctx, errors.Wrap(err,
						"could not enqueue sanction check for refinement"))
				}
			}

			decision.SanctionCheckExecution = &sc
		}

		if params.WithDecisionWebhooks {
			webhookEventId := uuid.NewString()
			err := usecase.webhookEventsSender.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
				Id:             webhookEventId,
				OrganizationId: decision.OrganizationId,
				EventContent:   models.NewWebhookEventDecisionCreated(decision.DecisionId),
			})
			if err != nil {
				return models.DecisionWithRuleExecutions{}, err
			}
			sendWebhookEventId = append(sendWebhookEventId, webhookEventId)
		}

		caseWebhookEventId := uuid.NewString()
		addedToCase, err := usecase.decisionWorkflows.AutomaticDecisionToCase(
			ctx,
			tx,
			scenario,
			decision,
			evaluationParameters,
			caseWebhookEventId)
		if err != nil {
			return models.DecisionWithRuleExecutions{}, err
		}
		if addedToCase {
			sendWebhookEventId = append(sendWebhookEventId, caseWebhookEventId)

			dec, err := usecase.repository.DecisionWithRuleExecutionsById(ctx, tx, decision.DecisionId)
			if err != nil {
				return models.DecisionWithRuleExecutions{}, err
			}
			return dec, nil
		}

		// only refresh the decision if it has changed, meaning if it was added to a case
		return decision, nil
	})
	if err != nil {
		return false, models.DecisionWithRuleExecutions{}, err
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
) (decisions []models.DecisionWithRuleExecutions, nbSkipped int, err error) {
	exec := usecase.executorFactory.NewExecutor()
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(ctx, "DecisionUsecase.CreateAllDecisions")
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
	)
	if err != nil {
		return nil, 0, err
	}

	pivotsMeta, err := usecase.dataModelRepository.ListPivots(ctx, exec, input.OrganizationId, nil)
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
		decision models.DecisionWithRuleExecutions
		scenario models.Scenario
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
			return nil, 0, errors.Wrap(err, "error evaluating scenario in CreateAllDecisions")
		case !triggerPassed:
			nbSkipped++
			usecase.executeTestRun(ctx, input.OrganizationId, input.TriggerObjectTable, evaluationParameters, scenario, nil)
		default:
			decision := models.AdaptScenarExecToDecision(scenarioExecution, payload, nil)
			items = append(items, decisionAndScenario{decision: decision, scenario: scenario})
			usecase.executeTestRun(ctx, input.OrganizationId, input.TriggerObjectTable,
				evaluationParameters, scenario, &scenarioExecution)
		}
	}

	ctx, span2 := tracer.Start(ctx, "DecisionUsecase.CreateAllDecisions - store decisions")
	defer span2.End()

	sendWebhookEventIds := make([]string, 0)
	decisions, err = executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Transaction,
	) ([]models.DecisionWithRuleExecutions, error) {
		var ids []string
		for _, item := range items {
			ids = append(ids, item.decision.DecisionId)
			if err = usecase.repository.StoreDecision(
				ctx,
				tx,
				item.decision,
				input.OrganizationId,
				item.decision.DecisionId,
			); err != nil {
				return nil, fmt.Errorf("error storing decision in CreateAllDecisions: %w", err)
			}

			if item.decision.SanctionCheckExecution != nil {
				sc, err := usecase.sanctionCheckRepository.InsertSanctionCheck(ctx, tx,
					item.decision.DecisionId, *item.decision.SanctionCheckExecution, true)
				if err != nil {
					return nil, errors.Wrap(err, "could not store sanction check execution")
				}

				if usecase.openSanctionsRepository.IsSelfHosted(ctx) {
					if err := usecase.taskQueueRepository.EnqueueMatchEnrichmentTask(
						ctx, input.OrganizationId, sc.Id); err != nil {
						utils.LogAndReportSentryError(ctx, errors.Wrap(err,
							"could not enqueue sanction check for refinement"))
					}
				}
			}

			webhookEventId := uuid.NewString()
			err := usecase.webhookEventsSender.CreateWebhookEvent(ctx, tx, models.WebhookEventCreate{
				Id:             webhookEventId,
				OrganizationId: item.decision.OrganizationId,
				EventContent:   models.NewWebhookEventDecisionCreated(item.decision.DecisionId),
			})
			if err != nil {
				return nil, err
			}
			sendWebhookEventIds = append(sendWebhookEventIds, webhookEventId)

			caseWebhookEventId := uuid.NewString()

			evaluationParameters := evaluate_scenario.ScenarioEvaluationParameters{
				Scenario:     item.scenario,
				ClientObject: payload,
				DataModel:    dataModel,
				Pivot:        pivot,
			}
			webhookEventCreated, err := usecase.decisionWorkflows.AutomaticDecisionToCase(
				ctx, tx, item.scenario, item.decision, evaluationParameters, caseWebhookEventId)
			if err != nil {
				return nil, err
			}
			if webhookEventCreated {
				sendWebhookEventIds = append(sendWebhookEventIds, caseWebhookEventId)
			}
		}

		return usecase.repository.DecisionsWithRuleExecutionsByIds(ctx, tx, ids)
	})
	if err != nil {
		return nil, 0, err
	}

	for _, caseWebhookEventId := range sendWebhookEventIds {
		usecase.webhookEventsSender.SendWebhookEventAsync(ctx, caseWebhookEventId)
	}

	return
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
		evaluationParameters.CachedSanctionCheck = scenarioExecution.SanctionCheckExecution
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
) (payload models.ClientObject, dataModel models.DataModel, err error) {
	exec := usecase.executorFactory.NewExecutor()

	if clientObject == nil && len(rawPayload) == 0 {
		err = errors.Wrap(
			models.BadParameterError,
			"empty payload received in validatePayload")
		return
	}

	dataModel, err = usecase.dataModelRepository.GetDataModel(ctx, exec, organizationId, false)
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
	payload, err = parser.ParsePayload(table, rawPayload)
	if err != nil {
		err = errors.Wrap(err, "error parsing payload in decision usecase validate payload")
		return
	}

	return
}
