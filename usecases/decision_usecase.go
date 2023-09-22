package usecases

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/evaluate_scenario"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/usecases/transaction"
	"github.com/checkmarble/marble-backend/utils"
)

type DecisionUsecase struct {
	enforceSecurity                 security.EnforceSecurityDecision
	transactionFactory              transaction.TransactionFactory
	orgTransactionFactory           transaction.Factory
	ingestedDataReadRepository      repositories.IngestedDataReadRepository
	customListRepository            repositories.CustomListRepository
	decisionRepository              repositories.DecisionRepository
	datamodelRepository             repositories.DataModelRepository
	scenarioReadRepository          repositories.ScenarioReadRepository
	scenarioIterationReadRepository repositories.ScenarioIterationReadRepository
	evaluateRuleAstExpression       ast_eval.EvaluateRuleAstExpression
	organizationIdOfContext         func() (string, error)
}

func (usecase *DecisionUsecase) GetDecision(decisionId string) (models.Decision, error) {
	decision, err := usecase.decisionRepository.DecisionById(nil, decisionId)
	if err != nil {
		return models.Decision{}, err
	}
	if err := usecase.enforceSecurity.ReadDecision(decision); err != nil {
		return models.Decision{}, err
	}
	return decision, nil
}

func (usecase *DecisionUsecase) ListDecisions(limit int) ([]models.Decision, error) {
	organizationId, err := usecase.organizationIdOfContext()
	if err != nil {
		return nil, err
	}

	return transaction.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) ([]models.Decision, error) {
			decisions, err := usecase.decisionRepository.DecisionsOfOrganization(tx, organizationId, limit)
			if err != nil {
				return []models.Decision{}, err
			}
			for _, decision := range decisions {
				if err := usecase.enforceSecurity.ReadDecision(decision); err != nil {
					return []models.Decision{}, err
				}
			}
			return decisions, nil
		},
	)
}

func (usecase *DecisionUsecase) CreateDecision(ctx context.Context, input models.CreateDecisionInput, logger *slog.Logger) (models.Decision, error) {

	if err := usecase.enforceSecurity.CreateDecision(input.OrganizationId); err != nil {
		return models.Decision{}, err
	}
	return transaction.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.Decision, error) {
		scenario, err := usecase.scenarioReadRepository.GetScenarioById(tx, input.ScenarioId)
		if errors.Is(err, models.NotFoundInRepositoryError) {
			return models.Decision{}, fmt.Errorf("scenario not found: %w", models.NotFoundError)
		} else if err != nil {
			return models.Decision{}, fmt.Errorf("error getting scenario: %w", err)
		}

		dm, err := usecase.datamodelRepository.GetDataModel(tx, input.OrganizationId)
		if errors.Is(err, models.NotFoundInRepositoryError) {
			return models.Decision{}, fmt.Errorf("data model not found: %w", models.NotFoundError)
		} else if err != nil {
			return models.Decision{}, fmt.Errorf("error getting data model: %w", err)
		}

		scenarioExecution, err := evaluate_scenario.EvalScenario(ctx, evaluate_scenario.ScenarioEvaluationParameters{
			Scenario:  scenario,
			Payload:   input.PayloadStructWithReader,
			DataModel: dm,
		}, evaluate_scenario.ScenarioEvaluationRepositories{
			ScenarioIterationReadRepository: usecase.scenarioIterationReadRepository,
			OrgTransactionFactory:           usecase.orgTransactionFactory,
			IngestedDataReadRepository:      usecase.ingestedDataReadRepository,
			EvaluateRuleAstExpression:       usecase.evaluateRuleAstExpression,
		}, logger)
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

		err = usecase.decisionRepository.StoreDecision(tx, decision, input.OrganizationId, newDecisionId)
		if err != nil {
			return models.Decision{}, fmt.Errorf("error storing decision: %w", err)
		}
		return usecase.decisionRepository.DecisionById(tx, newDecisionId)
	})
}
