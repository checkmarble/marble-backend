package app

import (
	"errors"
	"fmt"
	"log"
	"time"
)

var ErrScenarioNotFound = errors.New("scenario not found")

func (a *App) CreateDecision(organizationID string, scenarioID string, payload Payload) (Decision, error) {

	t := time.Now().UTC()

	///////////////////////////////
	// Get scenario
	///////////////////////////////
	s, err := a.repository.GetScenario(organizationID, scenarioID)

	if errors.Is(err, ErrNotFoundInRepository) {
		return Decision{}, ErrScenarioNotFound
	} else if err != nil {
		return Decision{}, fmt.Errorf("error getting scenario: %w", err)
	}

	///////////////////////////////
	// Get Data Model
	///////////////////////////////
	dm, err := a.repository.GetDataModel(organizationID)
	if errors.Is(err, ErrNotFoundInRepository) {
		return Decision{}, ErrScenarioNotFound
	} else if err != nil {
		return Decision{}, fmt.Errorf("error getting scenario: %w", err)
	}

	///////////////////////////////
	// Execute scenario
	///////////////////////////////
	scenarioExecution, err := s.Eval(payload, dm)
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

	id, err := a.repository.StoreDecision(organizationID, d)
	if err != nil {
		log.Printf("error storing decision: %v", err)
	}

	// succesfully created decision
	d.ID = id

	return d, nil
}
