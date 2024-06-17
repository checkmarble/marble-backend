package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	"github.com/cockroachdb/errors"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/evaluate_scenario"
	"github.com/checkmarble/marble-backend/usecases/executor_factory"
	"github.com/checkmarble/marble-backend/usecases/payload_parser"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
)

type DecisionUsecaseRepository interface {
	GetScenarioById(ctx context.Context, exec repositories.Executor, scenarioId string) (models.Scenario, error)
	ListScenariosOfOrganization(ctx context.Context, exec repositories.Executor, organizationId string) ([]models.Scenario, error)

	GetScenarioIteration(ctx context.Context, exec repositories.Executor, scenarioIterationId string) (
		models.ScenarioIteration, error,
	)

	GetCaseById(ctx context.Context, exec repositories.Executor, caseId string) (models.Case, error)
}

type decisionWorkflowsUsecase interface {
	AutomaticDecisionToCase(
		ctx context.Context,
		tx repositories.Executor,
		scenario models.Scenario,
		decision models.DecisionWithRuleExecutions,
	) error
}

type DecisionUsecase struct {
	enforceSecurity            security.EnforceSecurityDecision
	enforceSecurityScenario    security.EnforceSecurityScenario
	transactionFactory         executor_factory.TransactionFactory
	executorFactory            executor_factory.ExecutorFactory
	ingestedDataReadRepository repositories.IngestedDataReadRepository
	decisionRepository         repositories.DecisionRepository
	dataModelRepository        repositories.DataModelRepository
	repository                 DecisionUsecaseRepository
	evaluateAstExpression      ast_eval.EvaluateAstExpression
	decisionWorkflows          decisionWorkflowsUsecase
	organizationIdOfContext    func() (string, error)
}

func (usecase *DecisionUsecase) GetDecision(ctx context.Context, decisionId string) (models.DecisionWithRuleExecutions, error) {
	decision, err := usecase.decisionRepository.DecisionWithRuleExecutionsById(ctx,
		usecase.executorFactory.NewExecutor(), decisionId)
	if err != nil {
		return models.DecisionWithRuleExecutions{}, err
	}
	if err := usecase.enforceSecurity.ReadDecision(decision.Decision); err != nil {
		return models.DecisionWithRuleExecutions{}, err
	}

	return decision, nil
}

func (usecase *DecisionUsecase) ListDecisions(
	ctx context.Context,
	organizationId string,
	paginationAndSorting models.PaginationAndSorting,
	filters dto.DecisionFilters,
) ([]models.DecisionWithRank, error) {
	if err := usecase.validateScenarioIds(ctx, filters.ScenarioIds, organizationId); err != nil {
		return []models.DecisionWithRank{}, err
	}

	outcomes, err := usecase.validateOutcomes(ctx, filters.Outcomes)
	if err != nil {
		return []models.DecisionWithRank{}, err
	}

	if !filters.StartDate.IsZero() && !filters.EndDate.IsZero() &&
		filters.StartDate.After(filters.EndDate) {
		return []models.DecisionWithRank{}, fmt.Errorf(
			"start date must be before end date: %w", models.BadParameterError)
	}

	triggerObjectTypes, err := usecase.validateTriggerObjects(ctx, filters.TriggerObjects, organizationId)
	if err != nil {
		return []models.DecisionWithRank{}, err
	}

	if err := models.ValidatePagination(paginationAndSorting); err != nil {
		return []models.DecisionWithRank{}, err
	}

	decisions, err := usecase.decisionRepository.DecisionsOfOrganization(
		ctx,
		usecase.executorFactory.NewExecutor(),
		organizationId,
		paginationAndSorting,
		models.DecisionFilters{
			CaseIds:               filters.CaseIds,
			EndDate:               filters.EndDate,
			HasCase:               filters.HasCase,
			Outcomes:              outcomes,
			PivotValue:            filters.PivotValue,
			ScenarioIds:           filters.ScenarioIds,
			ScheduledExecutionIds: filters.ScheduledExecutionIds,
			StartDate:             filters.StartDate,
			TriggerObjects:        triggerObjectTypes,
		})
	if err != nil {
		return []models.DecisionWithRank{}, err
	}
	for _, decision := range decisions {
		if err := usecase.enforceSecurity.ReadDecision(decision.Decision); err != nil {
			return []models.DecisionWithRank{}, err
		}
	}
	return decisions, nil
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

func (usecase *DecisionUsecase) validateOutcomes(_ context.Context, filtersOutcomes []string) ([]models.Outcome, error) {
	outcomes := make([]models.Outcome, len(filtersOutcomes))
	for i, outcome := range filtersOutcomes {
		outcomes[i] = models.OutcomeFrom(outcome)
		if outcomes[i] == models.UnknownOutcome || outcomes[i] == models.None {
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
	skipScenarioPermissionForTransferCheck bool,
) (models.DecisionWithRuleExecutions, error) {
	exec := usecase.executorFactory.NewExecutor()
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(
		ctx,
		"DecisionUsecase.CreateDecision",
		trace.WithAttributes(attribute.String("scenario_id", input.ScenarioId)))
	defer span.End()

	if err := usecase.enforceSecurity.CreateDecision(input.OrganizationId); err != nil {
		return models.DecisionWithRuleExecutions{}, err
	}
	scenario, err := usecase.repository.GetScenarioById(ctx, exec, input.ScenarioId)
	if errors.Is(err, models.NotFoundError) {
		return models.DecisionWithRuleExecutions{}, errors.Wrap(err, "scenario not found")
	} else if err != nil {
		return models.DecisionWithRuleExecutions{},
			errors.Wrap(err, "error getting scenario")
	}
	if !skipScenarioPermissionForTransferCheck {
		if err := usecase.enforceSecurityScenario.ReadScenario(scenario); err != nil {
			return models.DecisionWithRuleExecutions{}, err
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
		return models.DecisionWithRuleExecutions{}, err
	}

	pivotsMeta, err := usecase.dataModelRepository.ListPivots(ctx, exec, input.OrganizationId, nil)
	if err != nil {
		return models.DecisionWithRuleExecutions{}, err
	}
	pivot := models.FindPivot(pivotsMeta, input.TriggerObjectTable, dataModel)

	evaluationParameters := evaluate_scenario.ScenarioEvaluationParameters{
		Scenario:     scenario,
		ClientObject: payload,
		DataModel:    dataModel,
		Pivot:        pivot,
	}

	evaluationRepositories := evaluate_scenario.ScenarioEvaluationRepositories{
		EvalScenarioRepository:     usecase.repository,
		ExecutorFactory:            usecase.executorFactory,
		IngestedDataReadRepository: usecase.ingestedDataReadRepository,
		EvaluateAstExpression:      usecase.evaluateAstExpression,
	}

	scenarioExecution, err := evaluate_scenario.EvalScenario(ctx, evaluationParameters, evaluationRepositories)
	if err != nil {
		return models.DecisionWithRuleExecutions{},
			fmt.Errorf("error evaluating scenario: %w", err)
	}

	decision := models.AdaptScenarExecToDecision(scenarioExecution, payload, nil)

	ctx, span = tracer.Start(
		ctx,
		"DecisionUsecase.CreateDecision.store_decision",
		trace.WithAttributes(attribute.String("scenario_id", input.ScenarioId)),
		trace.WithAttributes(attribute.Int("nb_rule_executions", len(decision.RuleExecutions))))
	defer span.End()
	return executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Executor,
	) (models.DecisionWithRuleExecutions, error) {
		if err = usecase.decisionRepository.StoreDecision(
			ctx,
			tx,
			decision,
			input.OrganizationId,
			decision.DecisionId,
		); err != nil {
			return models.DecisionWithRuleExecutions{},
				fmt.Errorf("error storing decision: %w", err)
		}

		err := usecase.decisionWorkflows.AutomaticDecisionToCase(ctx, tx, scenario, decision)
		if err != nil {
			return models.DecisionWithRuleExecutions{}, err
		}

		return usecase.decisionRepository.DecisionWithRuleExecutionsById(ctx, tx, decision.DecisionId)
	})
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
		if scenario.TriggerObjectType == input.TriggerObjectTable {
			if err := usecase.enforceSecurityScenario.ReadScenario(scenario); err != nil {
				return nil, 0, err
			}
			filteredScenarios = append(filteredScenarios, scenario)
		}
	}

	evaluationRepositories := evaluate_scenario.ScenarioEvaluationRepositories{
		EvalScenarioRepository:     usecase.repository,
		ExecutorFactory:            usecase.executorFactory,
		IngestedDataReadRepository: usecase.ingestedDataReadRepository,
		EvaluateAstExpression:      usecase.evaluateAstExpression,
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
		scenarioExecution, err := evaluate_scenario.EvalScenario(ctx, evaluationParameters, evaluationRepositories)
		if errors.Is(err, models.ErrScenarioTriggerConditionAndTriggerObjectMismatch) {
			nbSkipped++
			continue
		} else if err != nil {
			return nil, 0, errors.Wrap(err, "error evaluating scenario in CreateAllDecisions")
		}

		decision := models.AdaptScenarExecToDecision(scenarioExecution, payload, nil)
		items = append(items, decisionAndScenario{decision: decision, scenario: scenario})

	}

	ctx, span2 := tracer.Start(ctx, "DecisionUsecase.CreateAllDecisions - store decisions")
	defer span2.End()
	decisions, err = executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Executor,
	) ([]models.DecisionWithRuleExecutions, error) {
		var ids []string
		for _, item := range items {
			ids = append(ids, item.decision.DecisionId)
			if err = usecase.decisionRepository.StoreDecision(
				ctx,
				tx,
				item.decision,
				input.OrganizationId,
				item.decision.DecisionId,
			); err != nil {
				return nil, fmt.Errorf("error storing decision in CreateAllDecisions: %w", err)
			}

			err := usecase.decisionWorkflows.AutomaticDecisionToCase(ctx, tx, item.scenario, item.decision)
			if err != nil {
				return nil, err
			}
		}

		return usecase.decisionRepository.DecisionsWithRuleExecutionsByIds(ctx, tx, ids)
	})
	return
}

// used in different contexts, so allow different cases of input: pass client object or raw payload
func (usecase DecisionUsecase) validatePayload(
	ctx context.Context,
	organizationId string,
	triggerObjectTable string,
	clientObject *models.ClientObject,
	rawPayload json.RawMessage,
) (payload models.ClientObject, dataModel models.DataModel, err error) {
	logger := utils.LoggerFromContext(ctx)
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
	payload, validationErrors, err := parser.ParsePayload(table, rawPayload)
	if err != nil {
		err = errors.Wrap(
			models.BadParameterError,
			fmt.Sprintf("Error while validating payload in validatePayload: %v", err),
		)
		return
	}
	if len(validationErrors) > 0 {
		encoded, _ := json.Marshal(validationErrors)
		logger.InfoContext(ctx, fmt.Sprintf("Validation errors on POST all decisions: %s", string(encoded)))
		err = errors.Wrap(models.BadParameterError, string(encoded))
	}

	return
}
