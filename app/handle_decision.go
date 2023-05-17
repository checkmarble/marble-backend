package app

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/exp/slog"
)

type RepositoryDecisions interface {
	StoreDecision(ctx context.Context, orgID string, decision Decision) (Decision, error)
	GetDecision(ctx context.Context, orgID string, decisionID string) (Decision, error)
}

var ErrScenarioNotFound = errors.New("scenario not found")
var ErrDataModelNotFound = errors.New("data model not found")

func (app *App) GetDecision(ctx context.Context, orgID string, decisionID string) (Decision, error) {
	return app.repository.GetDecision(ctx, orgID, decisionID)
}

type CreateDecisionInput struct {
	OrganizationID          string
	ScenarioID              string
	Payload                 Payload
	PayloadStructWithReader DynamicStructWithReader
}

func (app *App) CreateDecision(ctx context.Context, input CreateDecisionInput, logger *slog.Logger) (Decision, error) {
	s, err := app.repository.GetScenario(ctx, input.OrganizationID, input.ScenarioID)
	if errors.Is(err, ErrNotFoundInRepository) {
		return Decision{}, ErrScenarioNotFound
	} else if err != nil {
		return Decision{}, fmt.Errorf("error getting scenario: %w", err)
	}

	dm, err := app.repository.GetDataModel(ctx, input.OrganizationID)
	if errors.Is(err, ErrNotFoundInRepository) {
		return Decision{}, ErrDataModelNotFound
	} else if err != nil {
		return Decision{}, fmt.Errorf("error getting data model: %w", err)
	}

	scenarioExecution, err := s.Eval(ctx, app.repository, input.PayloadStructWithReader, dm, logger)
	if err != nil {
		return Decision{}, fmt.Errorf("error evaluating scenario: %w", err)
	}

	d := Decision{
		Payload:             input.Payload,
		Outcome:             scenarioExecution.Outcome,
		ScenarioID:          scenarioExecution.ScenarioID,
		ScenarioName:        scenarioExecution.ScenarioName,
		ScenarioDescription: scenarioExecution.ScenarioDescription,
		ScenarioVersion:     scenarioExecution.ScenarioVersion,
		RuleExecutions:      scenarioExecution.RuleExecutions,
		Score:               scenarioExecution.Score,
		// TODO DecisionError DecisionError
	}

	createdDecision, err := app.repository.StoreDecision(ctx, input.OrganizationID, d)
	if err != nil {
		return Decision{}, fmt.Errorf("error storing decision: %w", err)
	}

	return createdDecision, nil
}
