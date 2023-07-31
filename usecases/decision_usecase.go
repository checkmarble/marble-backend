package usecases

import (
	"context"
	"errors"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/ast_eval"
	"marble/marble-backend/usecases/org_transaction"
	"marble/marble-backend/utils"

	"golang.org/x/exp/slog"
)

type DecisionUsecase struct {
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

func (usecase *DecisionUsecase) GetDecision(creds models.Credentials, organizationId string, decisionId string) (models.Decision, error) {
	decision, err := usecase.decisionRepository.DecisionById(nil, decisionId)

	if err != nil {
		return models.Decision{}, err
	}
	return decision, utils.EnforceOrganizationAccess(creds, decision.OrganizationId)
}

func (usecase *DecisionUsecase) ListDecisionsOfOrganization(organizationId string) ([]models.Decision, error) {
	return repositories.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) ([]models.Decision, error) {
			return usecase.decisionRepository.DecisionsOfOrganization(tx, organizationId, 1000)
		},
	)
}

func (usecase *DecisionUsecase) CreateDecision(ctx context.Context, input models.CreateDecisionInput, logger *slog.Logger) (models.Decision, error) {
	if err := utils.EnforceOrganizationAccess(utils.CredentialsFromCtx(ctx), input.OrganizationId); err != nil {
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
			logger:                          logger,
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
