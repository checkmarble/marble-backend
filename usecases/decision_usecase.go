package usecases

import (
	"context"
	"errors"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/organization"
	"marble/marble-backend/utils"

	"golang.org/x/exp/slog"
)

type DecisionUsecase struct {
	transactionFactory              repositories.TransactionFactory
	orgTransactionFactory           organization.OrgTransactionFactory
	ingestedDataReadRepository      repositories.IngestedDataReadRepository
	customListRepository            repositories.CustomListRepository
	decisionRepositoryLegacy        repositories.DecisionRepositoryLegacy
	decisionRepository              repositories.DecisionRepository
	datamodelRepository             repositories.DataModelRepository
	scenarioReadRepository          repositories.ScenarioReadRepository
	scenarioIterationReadRepository repositories.ScenarioIterationReadRepository
}

func (usecase *DecisionUsecase) GetDecision(creds models.Credentials, orgID string, decisionID string) (models.Decision, error) {
	decision, err := usecase.decisionRepository.DecisionById(nil, decisionID)

	if err != nil {
		return models.Decision{}, err
	}
	return decision, utils.EnforceOrganizationAccess(creds, decision.OrganizationId)
}

func (usecase *DecisionUsecase) ListDecisionsOfOrganization(orgID string) ([]models.Decision, error) {
	return repositories.TransactionReturnValue(
		usecase.transactionFactory,
		models.DATABASE_MARBLE_SCHEMA,
		func(tx repositories.Transaction) ([]models.Decision, error) {
			return usecase.decisionRepository.DecisionsOfOrganization(tx, orgID, 1000)
		},
	)
}

func (usecase *DecisionUsecase) CreateDecision(ctx context.Context, input models.CreateDecisionInput, logger *slog.Logger) (models.Decision, error) {
	if err := utils.EnforceOrganizationAccess(utils.MustCredentialsFromCtx(ctx), input.OrganizationID); err != nil {
		return models.Decision{}, err
	}

	scenario, err := usecase.scenarioReadRepository.GetScenarioById(nil, input.ScenarioID)
	if errors.Is(err, models.NotFoundInRepositoryError) {
		return models.Decision{}, fmt.Errorf("Scenario not found: %w", models.NotFoundError)
	} else if err != nil {
		return models.Decision{}, fmt.Errorf("error getting scenario: %w", err)
	}

	dm, err := usecase.datamodelRepository.GetDataModel(nil, input.OrganizationID)
	if errors.Is(err, models.NotFoundInRepositoryError) {
		return models.Decision{}, fmt.Errorf("Data model not found: %w", models.NotFoundError)
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
	}, logger)
	if err != nil {
		return models.Decision{}, fmt.Errorf("error evaluating scenario: %w", err)
	}

	newDecisionId := utils.NewPrimaryKey(input.OrganizationID)
	decision := models.Decision{
		DecisionId:          newDecisionId,
		OrganizationId:      input.OrganizationID,
		ClientObject:        input.ClientObject,
		Outcome:             scenarioExecution.Outcome,
		ScenarioId:          scenarioExecution.ScenarioID,
		ScenarioName:        scenarioExecution.ScenarioName,
		ScenarioDescription: scenarioExecution.ScenarioDescription,
		ScenarioVersion:     scenarioExecution.ScenarioVersion,
		RuleExecutions:      scenarioExecution.RuleExecutions,
		Score:               scenarioExecution.Score,
	}

	err = usecase.decisionRepositoryLegacy.StoreDecision(ctx, decision)
	if err != nil {
		return models.Decision{}, fmt.Errorf("error storing decision: %w", err)
	}

	return repositories.TransactionReturnValue(usecase.transactionFactory, models.DATABASE_MARBLE_SCHEMA, func(tx repositories.Transaction) (models.Decision, error) {
		return usecase.decisionRepository.DecisionById(tx, newDecisionId)
	})
}
