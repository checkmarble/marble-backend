package app

import (
	"context"
	"errors"
	"fmt"
	"log"
)

var ErrScenarioNotFound = errors.New("scenario not found")
var ErrDataModelNotFound = errors.New("data model not found")

func (app *App) CreateDecision(ctx context.Context, organizationID string, scenarioID string, payload Payload) (Decision, error) {
	///////////////////////////////
	// Get scenario
	///////////////////////////////
	s, err := app.repository.GetScenario(ctx, organizationID, scenarioID)

	if errors.Is(err, ErrNotFoundInRepository) {
		return Decision{}, ErrScenarioNotFound
	} else if err != nil {
		return Decision{}, fmt.Errorf("error getting scenario: %w", err)
	}

	///////////////////////////////
	// Get Data Model
	///////////////////////////////
	dm, err := app.repository.GetDataModel(context.TODO(), organizationID)
	if errors.Is(err, ErrNotFoundInRepository) {
		return Decision{}, ErrDataModelNotFound
	} else if err != nil {
		return Decision{}, fmt.Errorf("error getting data model: %w", err)
	}

	///////////////////////////////
	// Execute scenario
	///////////////////////////////
	scenarioExecution, err := s.Eval(app.repository, payload, dm)
	if err != nil {
		return Decision{}, fmt.Errorf("error evaluating scenario: %w", err)
	}

	///////////////////////////////
	// Build and persist decision
	///////////////////////////////
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
