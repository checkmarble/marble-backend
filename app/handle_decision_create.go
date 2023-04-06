package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"
)

var ErrScenarioNotFound = errors.New("scenario not found")
var ErrDataModelNotFound = errors.New("data model not found")

func (app *App) CreateDecision(organizationID string, scenarioID string, payload Payload) (Decision, error) {

	t := time.Now().UTC()

	///////////////////////////////
	// Get scenario
	///////////////////////////////
	s, err := app.repository.GetScenario(context.TODO(), organizationID, scenarioID)

	if errors.Is(err, ErrNotFoundInRepository) {
		return Decision{}, ErrScenarioNotFound
	} else if err != nil {
		return Decision{}, fmt.Errorf("error getting scenario: %w", err)
	}

	///////////////////////////////
	// Get Data Model
	///////////////////////////////
	dm, err := app.repository.GetDataModel(organizationID)
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
		// ID is empty as of now
		Created_at:          t,
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

	id, err := a.repository.StoreDecision(context.TODO(), organizationID, d)
	if err != nil {
		log.Printf("error storing decision: %v", err)
	}

	// succesfully created decision
	d.ID = id

	return d, nil
}
