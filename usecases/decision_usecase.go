package usecases

import (
	"context"
	"errors"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/repositories"
	"marble/marble-backend/usecases/organization"

	"golang.org/x/exp/slog"
)

type DecisionUsecase struct {
	orgTransactionFactory           organization.OrgTransactionFactory
	ingestedDataReadRepository      repositories.IngestedDataReadRepository
	decisionRepository              repositories.DecisionRepository
	datamodelRepository             repositories.DataModelRepository
	scenarioReadRepository          repositories.ScenarioReadRepository
	scenarioIterationReadRepository repositories.ScenarioIterationReadRepository
}

func (usecase *DecisionUsecase) GetDecision(ctx context.Context, orgID string, decisionID string) (models.Decision, error) {
	return usecase.decisionRepository.GetDecision(ctx, orgID, decisionID)
}

func (usecase *DecisionUsecase) ListDecisions(ctx context.Context, orgID string) ([]models.Decision, error) {
	return usecase.decisionRepository.ListDecisions(ctx, orgID)
}

func (usecase *DecisionUsecase) CreateDecision(ctx context.Context, input models.CreateDecisionInput, logger *slog.Logger) (models.Decision, error) {
	scenario, err := usecase.scenarioReadRepository.GetScenario(ctx, input.OrganizationID, input.ScenarioID)
	if errors.Is(err, models.NotFoundInRepositoryError) {
		return models.Decision{}, fmt.Errorf("Scenario not found: %w", models.NotFoundError)
	} else if err != nil {
		return models.Decision{}, fmt.Errorf("error getting scenario: %w", err)
	}

	dm, err := usecase.datamodelRepository.GetDataModel(ctx, input.OrganizationID)
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
	}, logger)
	if err != nil {
		return models.Decision{}, fmt.Errorf("error evaluating scenario: %w", err)
	}

	d := models.Decision{
		ClientObject:        input.ClientObject,
		Outcome:             scenarioExecution.Outcome,
		ScenarioID:          scenarioExecution.ScenarioID,
		ScenarioName:        scenarioExecution.ScenarioName,
		ScenarioDescription: scenarioExecution.ScenarioDescription,
		ScenarioVersion:     scenarioExecution.ScenarioVersion,
		RuleExecutions:      scenarioExecution.RuleExecutions,
		Score:               scenarioExecution.Score,
	}

	createdDecision, err := usecase.decisionRepository.StoreDecision(ctx, input.OrganizationID, d)
	if err != nil {
		return models.Decision{}, fmt.Errorf("error storing decision: %w", err)
	}

	return createdDecision, nil
}
