package models

import (
	"fmt"
	"marble/marble-backend/models/operators"
	"time"
)

type PublishedScenarioIteration struct {
	ID         string
	ScenarioID string
	Version    int
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Body       PublishedScenarioIterationBody
}

type PublishedScenarioIterationBody struct {
	TriggerCondition     operators.OperatorBool
	Rules                []Rule
	ScoreReviewThreshold int
	ScoreRejectThreshold int
}

type ScenarioIteration struct {
	ID         string
	ScenarioID string
	Version    *int
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Body       ScenarioIterationBody
}

type GetScenarioIterationFilters struct {
	ScenarioID *string
}

type ScenarioIterationBody struct {
	TriggerCondition     operators.OperatorBool
	Rules                []Rule
	ScoreReviewThreshold *int
	ScoreRejectThreshold *int
}

type CreateScenarioIterationInput struct {
	ScenarioID string
	Body       *CreateScenarioIterationBody
}

type CreateScenarioIterationBody struct {
	TriggerCondition     operators.OperatorBool
	Rules                []CreateRuleInput
	ScoreReviewThreshold *int
	ScoreRejectThreshold *int
}

type UpdateScenarioIterationInput struct {
	ID   string
	Body *UpdateScenarioIterationBody
}

type UpdateScenarioIterationBody struct {
	TriggerCondition     operators.OperatorBool
	ScoreReviewThreshold *int
	ScoreRejectThreshold *int
}

func NewPublishedScenarioIteration(si ScenarioIteration) (PublishedScenarioIteration, error) {
	result := PublishedScenarioIteration{
		ID:         si.ID,
		ScenarioID: si.ScenarioID,
		CreatedAt:  si.CreatedAt,
		UpdatedAt:  si.UpdatedAt,
	}

	err := si.IsValidForPublication()
	if err != nil {
		return PublishedScenarioIteration{}, err
	}

	result.Version = *si.Version
	result.Body.ScoreReviewThreshold = *si.Body.ScoreReviewThreshold
	result.Body.ScoreRejectThreshold = *si.Body.ScoreRejectThreshold
	result.Body.Rules = si.Body.Rules
	result.Body.TriggerCondition = si.Body.TriggerCondition

	return result, nil
}

func (si ScenarioIteration) IsValidForPublication() error {
	if si.Body.ScoreReviewThreshold == nil {
		return fmt.Errorf("Scenario iteration has no ScoreReviewThreshold: \n%w", ErrScenarioIterationNotValid)
	}

	if si.Body.ScoreRejectThreshold == nil {
		return fmt.Errorf("Scenario iteration has no ScoreRejectThreshold: \n%w", ErrScenarioIterationNotValid)
	}

	if len(si.Body.Rules) < 1 {
		return fmt.Errorf("Scenario iteration has no rules: \n%w", ErrScenarioIterationNotValid)
	}
	for _, rule := range si.Body.Rules {
		if !rule.Formula.IsValid() {
			return fmt.Errorf("Scenario iteration rule has invalid rules: \n%w", ErrScenarioIterationNotValid)
		}
	}

	if si.Body.TriggerCondition == nil {
		return fmt.Errorf("Scenario iteration has no trigger condition: \n%w", ErrScenarioIterationNotValid)
	} else if !si.Body.TriggerCondition.IsValid() {
		return fmt.Errorf("Scenario iteration trigger condition is invalid: \n%w", ErrScenarioIterationNotValid)
	}

	return nil
}
