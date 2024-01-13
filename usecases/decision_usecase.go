package usecases

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/evaluate_scenario"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/transaction"
	"github.com/checkmarble/marble-backend/utils"
)

type DecisionUsecaseRepository interface {
	GetScenarioById(ctx context.Context, tx repositories.Transaction, scenarioId string) (models.Scenario, error)
	ListScenariosOfOrganization(ctx context.Context, tx repositories.Transaction, organizationId string) ([]models.Scenario, error)

	GetScenarioIteration(ctx context.Context, tx repositories.Transaction, scenarioIterationId string) (
		models.ScenarioIteration, error,
	)

	GetCaseById(ctx context.Context, tx repositories.Transaction, caseId string) (models.Case, error)
}

type DecisionUsecase struct {
	enforceSecurity            security.EnforceSecurityDecision
	transactionFactory         transaction.TransactionFactory
	orgTransactionFactory      transaction.Factory
	ingestedDataReadRepository repositories.IngestedDataReadRepository
	decisionRepository         repositories.DecisionRepository
	datamodelRepository        repositories.DataModelRepository
	repository                 DecisionUsecaseRepository
	evaluateRuleAstExpression  ast_eval.EvaluateRuleAstExpression
	organizationIdOfContext    func() (string, error)
}

func (usecase *DecisionUsecase) GetDecision(ctx context.Context, decisionId string) (models.Decision, error) {
	decision, err := usecase.decisionRepository.DecisionById(ctx, nil, decisionId)
	if err != nil {
		return models.Decision{}, err
	}
	if err := usecase.enforceSecurity.ReadDecision(decision); err != nil {
		return models.Decision{}, err
	}

	return decision, nil
}

func (usecase *DecisionUsecase) ListDecisions(ctx context.Context, organizationId string, paginationAndSorting models.PaginationAndSorting, filters dto.DecisionFilters) ([]models.DecisionWithRank, error) {
	if err := usecase.validateScenarioIds(ctx, filters.ScenarioIds, organizationId); err != nil {
		return []models.DecisionWithRank{}, err
	}

	outcomes, err := usecase.validateOutcomes(ctx, filters.Outcomes)
	if err != nil {
		return []models.DecisionWithRank{}, err
	}

	if !filters.StartDate.IsZero() && !filters.EndDate.IsZero() && filters.StartDate.After(filters.EndDate) {
		return []models.DecisionWithRank{}, fmt.Errorf("start date must be before end date: %w", models.BadParameterError)
	}

	triggerObjectTypes, err := usecase.validateTriggerObjects(ctx, filters.TriggerObjects, organizationId)
	if err != nil {
		return []models.DecisionWithRank{}, err
	}

	if err := models.ValidatePagination(paginationAndSorting); err != nil {
		return []models.DecisionWithRank{}, err
	}

	return transaction.TransactionReturnValue(
		ctx,
		usecase.transactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) ([]models.DecisionWithRank, error) {
			decisions, err := usecase.decisionRepository.DecisionsOfOrganization(ctx, tx, organizationId, paginationAndSorting, models.DecisionFilters{
				ScenarioIds:    filters.ScenarioIds,
				StartDate:      filters.StartDate,
				EndDate:        filters.EndDate,
				Outcomes:       outcomes,
				TriggerObjects: triggerObjectTypes,
				WithCase:       filters.WithCase,
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
	scenarios, err := usecase.repository.ListScenariosOfOrganization(ctx, nil, organizationId)
	if err != nil {
		return err
	}
	organizationScenarioIds := make([]string, len(scenarios))
	for i, scenario := range scenarios {
		organizationScenarioIds[i] = scenario.Id
	}

	for _, scenarioId := range scenarioIds {
		if !slices.Contains(organizationScenarioIds, scenarioId) {
			return fmt.Errorf("scenario id %s not found in organization %s: %w", scenarioId, organizationId, models.BadParameterError)
		}
	}
	return nil
}

func (usecase *DecisionUsecase) validateOutcomes(ctx context.Context, filtersOutcomes []string) ([]models.Outcome, error) {
	outcomes := make([]models.Outcome, len(filtersOutcomes))
	for i, outcome := range filtersOutcomes {
		outcomes[i] = models.OutcomeFrom(outcome)
		if outcomes[i] == models.UnknownOutcome || outcomes[i] == models.None {
			return []models.Outcome{}, fmt.Errorf("invalid outcome: %s, %w", outcome, models.BadParameterError)
		}
	}
	return outcomes, nil
}

func (usecase *DecisionUsecase) validateTriggerObjects(ctx context.Context, filtersTriggerObjects []string, organizationId string) ([]models.TableName, error) {
	dataModel, err := usecase.datamodelRepository.GetDataModel(ctx, organizationId, true)
	if err != nil {
		return []models.TableName{}, err
	}
	triggerObjectTypes := make([]models.TableName, len(filtersTriggerObjects))
	for i, triggerObject := range filtersTriggerObjects {
		triggerObjectTypes[i] = models.TableName(triggerObject)
		if _, ok := dataModel.Tables[triggerObjectTypes[i]]; !ok {
			return []models.TableName{}, fmt.Errorf("table %s not found on data model: %w", triggerObject, models.BadParameterError)
		}
	}
	return triggerObjectTypes, nil
}

func (usecase *DecisionUsecase) CreateDecision(ctx context.Context, input models.CreateDecisionInput, logger *slog.Logger) (models.Decision, error) {

	if err := usecase.enforceSecurity.CreateDecision(input.OrganizationId); err != nil {
		return models.Decision{}, err
	}
	scenario, err := usecase.repository.GetScenarioById(ctx, nil, input.ScenarioId)
	if errors.Is(err, models.NotFoundInRepositoryError) {
		return models.Decision{}, fmt.Errorf("scenario not found: %w", models.NotFoundError)
	} else if err != nil {
		return models.Decision{}, fmt.Errorf("error getting scenario: %w", err)
	}

	dm, err := usecase.datamodelRepository.GetDataModel(ctx, input.OrganizationId, false)
	if errors.Is(err, models.NotFoundInRepositoryError) {
		return models.Decision{}, fmt.Errorf("data model not found: %w", models.NotFoundError)
	} else if err != nil {
		return models.Decision{}, fmt.Errorf("error getting data model: %w", err)
	}

	evaluationParameters := evaluate_scenario.ScenarioEvaluationParameters{
		Scenario:  scenario,
		Payload:   input.PayloadStructWithReader,
		DataModel: dm,
	}

	evaluationRepositories := evaluate_scenario.ScenarioEvaluationRepositories{
		EvalScenarioRepository:     usecase.repository,
		OrgTransactionFactory:      usecase.orgTransactionFactory,
		IngestedDataReadRepository: usecase.ingestedDataReadRepository,
		EvaluateRuleAstExpression:  usecase.evaluateRuleAstExpression,
	}

	scenarioExecution, err := evaluate_scenario.EvalScenario(ctx, evaluationParameters, evaluationRepositories, logger)
	if err != nil {
		return models.Decision{}, fmt.Errorf("error evaluating scenario: %w", err)
	}

	newDecisionId := utils.NewPrimaryKey(input.OrganizationId)
	decision := models.Decision{
		ClientObject:        input.ClientObject,
		Outcome:             scenarioExecution.Outcome,
		RuleExecutions:      scenarioExecution.RuleExecutions,
		ScenarioDescription: scenarioExecution.ScenarioDescription,
		ScenarioId:          scenarioExecution.ScenarioId,
		ScenarioName:        scenarioExecution.ScenarioName,
		ScenarioVersion:     scenarioExecution.ScenarioVersion,
		Score:               scenarioExecution.Score,
	}

	return transaction.TransactionReturnValue(ctx, usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.Decision, error) {
		err = usecase.decisionRepository.StoreDecision(ctx, tx, decision, input.OrganizationId, newDecisionId)
		if err != nil {
			return models.Decision{}, fmt.Errorf("error storing decision: %w", err)
		}
		return usecase.decisionRepository.DecisionById(ctx, tx, newDecisionId)
	})
}
