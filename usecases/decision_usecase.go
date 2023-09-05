package usecases

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/repositories"
	"github.com/checkmarble/marble-backend/usecases/ast_eval"
	"github.com/checkmarble/marble-backend/usecases/org_transaction"
	"github.com/checkmarble/marble-backend/usecases/security"
	"github.com/checkmarble/marble-backend/utils"
)

type DecisionUsecase struct {
	enforceSecurity                 security.EnforceSecurityDecision
	transactionFactory              repositories.TransactionFactory
	orgTransactionFactory           org_transaction.Factory
	ingestedDataReadRepository      repositories.IngestedDataReadRepository
	customListRepository            repositories.CustomListRepository
	decisionRepository              repositories.DecisionRepository
	datamodelRepository             repositories.DataModelRepository
	scenarioReadRepository          repositories.ScenarioReadRepository
	scenarioIterationReadRepository repositories.ScenarioIterationReadRepository
	evaluateRuleAstExpression       ast_eval.EvaluateRuleAstExpression
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

func (usecase *DecisionUsecase) ListDecisionsOfOrganization(organizationId string) ([]models.Decision, error) {
	return repositories.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) ([]models.Decision, error) {
			decisions, err := usecase.decisionRepository.DecisionsOfOrganization(tx, organizationId, 1000)
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
	return repositories.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.Decision, error) {
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

		scenarioExecution, err := evalScenario(ctx, scenarioEvaluationParameters{
			scenario:  scenario,
			payload:   input.PayloadStructWithReader,
			dataModel: dm,
		}, scenarioEvaluationRepositories{
			scenarioIterationReadRepository: usecase.scenarioIterationReadRepository,
			orgTransactionFactory:           usecase.orgTransactionFactory,
			ingestedDataReadRepository:      usecase.ingestedDataReadRepository,
			customListRepository:            usecase.customListRepository,
			evaluateRuleAstExpression:       usecase.evaluateRuleAstExpression,
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
