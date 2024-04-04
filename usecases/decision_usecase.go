package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"

	"github.com/cockroachdb/errors"

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

type caseCreatorAsWorkflow interface {
	CreateCaseAsWorkflow(
		ctx context.Context,
		exec repositories.Executor,
		createCaseAttributes models.CreateCaseAttributes,
	) (models.Case, error)
}

type DecisionUsecase struct {
	enforceSecurity            security.EnforceSecurityDecision
	enforceSecurityScenario    security.EnforceSecurityScenario
	transactionFactory         executor_factory.TransactionFactory
	executorFactory            executor_factory.ExecutorFactory
	ingestedDataReadRepository repositories.IngestedDataReadRepository
	decisionRepository         repositories.DecisionRepository
	datamodelRepository        repositories.DataModelRepository
	repository                 DecisionUsecaseRepository
	evaluateAstExpression      ast_eval.EvaluateAstExpression
	caseCreator                caseCreatorAsWorkflow
	organizationIdOfContext    func() (string, error)
}

func (usecase *DecisionUsecase) GetDecision(ctx context.Context, decisionId string) (models.DecisionWithRuleExecutions, error) {
	decision, err := usecase.decisionRepository.DecisionById(ctx,
		usecase.executorFactory.NewExecutor(), decisionId)
	if err != nil {
		return models.DecisionWithRuleExecutions{}, err
	}
	if err := usecase.enforceSecurity.ReadDecision(decision.Decision); err != nil {
		return models.DecisionWithRuleExecutions{}, err
	}

	return decision, nil
}

func (usecase *DecisionUsecase) ListDecisions(ctx context.Context, organizationId string,
	paginationAndSorting models.PaginationAndSorting, filters dto.DecisionFilters,
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

	return executor_factory.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		func(tx repositories.Executor) ([]models.DecisionWithRank, error) {
			decisions, err := usecase.decisionRepository.DecisionsOfOrganization(ctx, tx,
				organizationId, paginationAndSorting, models.DecisionFilters{
					ScenarioIds:    filters.ScenarioIds,
					StartDate:      filters.StartDate,
					EndDate:        filters.EndDate,
					Outcomes:       outcomes,
					TriggerObjects: triggerObjectTypes,
					HasCase:        filters.HasCase,
					CaseIds:        filters.CaseIds,
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
		},
	)
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
) ([]models.TableName, error) {
	dataModel, err := usecase.datamodelRepository.GetDataModel(ctx,
		usecase.executorFactory.NewExecutor(), organizationId, true)
	if err != nil {
		return []models.TableName{}, err
	}
	triggerObjectTypes := make([]models.TableName, len(filtersTriggerObjects))
	for i, triggerObject := range filtersTriggerObjects {
		triggerObjectTypes[i] = models.TableName(triggerObject)
		if _, ok := dataModel.Tables[triggerObjectTypes[i]]; !ok {
			return []models.TableName{}, fmt.Errorf(
				"table %s not found on data model: %w", triggerObject, models.BadParameterError)
		}
	}
	return triggerObjectTypes, nil
}

func (usecase *DecisionUsecase) CreateDecision(
	ctx context.Context,
	input models.CreateDecisionInput,
	logger *slog.Logger,
	skipScenarioPermissionForTransferCheck bool,
) (models.DecisionWithRuleExecutions, error) {
	exec := usecase.executorFactory.NewExecutor()
	tracer := utils.OpenTelemetryTracerFromContext(ctx)
	ctx, span := tracer.Start(ctx, "DecisionUsecase.CreateDecision")
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

	dm, err := usecase.datamodelRepository.GetDataModel(ctx, exec, input.OrganizationId, false)
	if errors.Is(err, models.NotFoundError) {
		return models.DecisionWithRuleExecutions{},
			errors.Wrap(models.NotFoundError, "data model not found")
	} else if err != nil {
		return models.DecisionWithRuleExecutions{},
			errors.Wrap(err, "error getting data model")
	}

	evaluationParameters := evaluate_scenario.ScenarioEvaluationParameters{
		Scenario:     scenario,
		ClientObject: input.ClientObject,
		DataModel:    dm,
	}

	evaluationRepositories := evaluate_scenario.ScenarioEvaluationRepositories{
		EvalScenarioRepository:     usecase.repository,
		ExecutorFactory:            usecase.executorFactory,
		IngestedDataReadRepository: usecase.ingestedDataReadRepository,
		EvaluateAstExpression:      usecase.evaluateAstExpression,
	}

	scenarioExecution, err := evaluate_scenario.EvalScenario(ctx, evaluationParameters, evaluationRepositories, logger)
	if err != nil {
		return models.DecisionWithRuleExecutions{},
			fmt.Errorf("error evaluating scenario: %w", err)
	}

	newDecisionId := utils.NewPrimaryKey(input.OrganizationId)
	decision := models.DecisionWithRuleExecutions{
		Decision: models.Decision{
			ClientObject:        input.ClientObject,
			Outcome:             scenarioExecution.Outcome,
			ScenarioDescription: scenarioExecution.ScenarioDescription,
			ScenarioId:          scenarioExecution.ScenarioId,
			ScenarioIterationId: scenarioExecution.ScenarioIterationId,
			ScenarioName:        scenarioExecution.ScenarioName,
			ScenarioVersion:     scenarioExecution.ScenarioVersion,
			Score:               scenarioExecution.Score,
		},
		RuleExecutions: scenarioExecution.RuleExecutions,
	}

	return executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Executor,
	) (models.DecisionWithRuleExecutions, error) {
		err = usecase.decisionRepository.StoreDecision(ctx, tx, decision, input.OrganizationId, newDecisionId)
		if err != nil {
			return models.DecisionWithRuleExecutions{},
				fmt.Errorf("error storing decision: %w", err)
		}

		if scenario.DecisionToCaseOutcomes != nil &&
			slices.Contains(scenario.DecisionToCaseOutcomes, decision.Outcome) &&
			scenario.DecisionToCaseInboxId != nil {
			_, err = usecase.caseCreator.CreateCaseAsWorkflow(ctx, tx, models.CreateCaseAttributes{
				DecisionIds: []string{newDecisionId},
				InboxId:     *scenario.DecisionToCaseInboxId,
				Name: fmt.Sprintf(
					"Case for %s: %s",
					scenario.TriggerObjectType,
					input.ClientObject.Data["object_id"],
				),
				OrganizationId: input.OrganizationId,
			})
			if err != nil {
				return models.DecisionWithRuleExecutions{},
					fmt.Errorf("error linking decision to case: %w", err)
			}
		}

		return usecase.decisionRepository.DecisionById(ctx, tx, newDecisionId)
	})
}

func (usecase *DecisionUsecase) CreateAllDecisions(
	ctx context.Context,
	input models.CreateAllDecisionsInput,
) (decisions []models.DecisionWithRuleExecutions, nbSkipped int, err error) {
	exec := usecase.executorFactory.NewExecutor()
	logger := utils.LoggerFromContext(ctx)

	if err = usecase.enforceSecurity.CreateDecision(input.OrganizationId); err != nil {
		return
	}

	dataModel, err := usecase.datamodelRepository.GetDataModel(ctx, exec, input.OrganizationId, false)
	if err != nil {
		return nil, 0, errors.Wrap(err, "error getting data model in CreateAllDecisions")
	}

	tables := dataModel.Tables
	table, ok := tables[models.TableName(input.TriggerObjectTable)]
	if !ok {
		return nil, 0, errors.Wrap(
			models.NotFoundError,
			fmt.Sprintf("table %s not found in data model in CreateAllDecisions", input.TriggerObjectTable),
		)
	}

	parser := payload_parser.NewParser()
	payload, validationErrors, err := parser.ParsePayload(table, input.PayloadRaw)
	if err != nil {
		return nil, 0, errors.Wrap(
			models.BadParameterError,
			fmt.Sprintf("Error while validating payload in CreateAllDecisions: %v", err),
		)
	}
	if len(validationErrors) > 0 {
		encoded, _ := json.Marshal(validationErrors)
		logger.InfoContext(ctx, fmt.Sprintf("Validation errors on POST all decisions: %s", string(encoded)))
		return nil, 0, errors.Wrap(models.BadParameterError, string(encoded))
	}

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
		}
		scenarioExecution, err := evaluate_scenario.EvalScenario(ctx, evaluationParameters, evaluationRepositories, logger)
		if errors.Is(err, models.ErrScenarioTriggerConditionAndTriggerObjectMismatch) {
			nbSkipped++
			continue
		} else if err != nil {
			return nil, 0, errors.Wrap(err, "error evaluating scenario in CreateAllDecisions")
		}

		decision := models.DecisionWithRuleExecutions{
			Decision: models.Decision{
				ClientObject:        payload,
				Outcome:             scenarioExecution.Outcome,
				ScenarioDescription: scenarioExecution.ScenarioDescription,
				ScenarioId:          scenarioExecution.ScenarioId,
				ScenarioIterationId: scenarioExecution.ScenarioIterationId,
				ScenarioName:        scenarioExecution.ScenarioName,
				ScenarioVersion:     scenarioExecution.ScenarioVersion,
				Score:               scenarioExecution.Score,
			},
			RuleExecutions: scenarioExecution.RuleExecutions,
		}
		items = append(items, decisionAndScenario{decision: decision, scenario: scenario})

	}

	decisions, err = executor_factory.TransactionReturnValue(ctx, usecase.transactionFactory, func(
		tx repositories.Executor,
	) ([]models.DecisionWithRuleExecutions, error) {
		var ids []string
		for _, item := range items {
			newDecisionId := utils.NewPrimaryKey(input.OrganizationId)
			ids = append(ids, newDecisionId)
			err = usecase.decisionRepository.StoreDecision(ctx, tx, item.decision, input.OrganizationId, newDecisionId)
			if err != nil {
				return nil, fmt.Errorf("error storing decision in CreateAllDecisions: %w", err)
			}

			if item.scenario.DecisionToCaseOutcomes != nil &&
				slices.Contains(item.scenario.DecisionToCaseOutcomes, item.decision.Outcome) &&
				item.scenario.DecisionToCaseInboxId != nil {
				_, err = usecase.caseCreator.CreateCaseAsWorkflow(ctx, tx, models.CreateCaseAttributes{
					DecisionIds: []string{newDecisionId},
					InboxId:     *item.scenario.DecisionToCaseInboxId,
					Name: fmt.Sprintf(
						"Case for %s: %s",
						item.scenario.TriggerObjectType,
						payload.Data["object_id"],
					),
					OrganizationId: input.OrganizationId,
				})
				if err != nil {
					return nil, fmt.Errorf("error linking decision to case: %w", err)
				}
			}
		}

		return usecase.decisionRepository.DecisionsByIds(ctx, tx, ids)
	})
	return
}
