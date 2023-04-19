package app

import (
	"context"
	"errors"
	"fmt"
	"log"
)

var ErrScenarioNotFound = errors.New("scenario not found")
var ErrDataModelNotFound = errors.New("data model not found")

func (app *App) GetDecision(ctx context.Context, orgID string, decisionID string) (Decision, error) {
	return app.repository.GetDecision(ctx, orgID, decisionID)
}

func (app *App) CreateDecision(ctx context.Context, organizationID string, scenarioID string, payloadStructWithReader DynamicStructWithReader, payload Payload) (Decision, error) {
	s, err := app.repository.GetScenario(ctx, organizationID, scenarioID)
	if errors.Is(err, ErrNotFoundInRepository) {
		return Decision{}, ErrScenarioNotFound
	} else if err != nil {
		return Decision{}, fmt.Errorf("error getting scenario: %w", err)
	}

	dm, err := app.repository.GetDataModel(ctx, organizationID)
	if errors.Is(err, ErrNotFoundInRepository) {
		return Decision{}, ErrDataModelNotFound
	} else if err != nil {
		return Decision{}, fmt.Errorf("error getting data model: %w", err)
	}

	scenarioExecution, err := s.Eval(app.repository, payloadStructWithReader, dm)
	if err != nil {
		return Decision{}, fmt.Errorf("error evaluating scenario: %w", err)
	}

	d := Decision{
		Payload:             payload,
		Outcome:             scenarioExecution.Outcome,
		ScenarioID:          scenarioExecution.ScenarioID,
		ScenarioName:        scenarioExecution.ScenarioName,
		ScenarioDescription: scenarioExecution.ScenarioDescription,
		ScenarioVersion:     scenarioExecution.ScenarioVersion,
		RuleExecutions:      scenarioExecution.RuleExecutions,
		Score:               scenarioExecution.Score,
		// TODO DecisionError DecisionError
	}

	createdDecision, err := app.repository.StoreDecision(ctx, organizationID, d)
	if err != nil {
		log.Printf("error storing decision: %v", err)
	}

	return createdDecision, nil
}
