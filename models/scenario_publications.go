package models

import (
	"fmt"
	"marble/marble-backend/models/operators"
	"time"
)

type ScenarioPublication struct {
	ID                  string
	Rank                int32
	OrgID               string
	ScenarioID          string
	ScenarioIterationID string
	PublicationAction   PublicationAction
	CreatedAt           time.Time
}

type PublicationAction int

const (
	Publish PublicationAction = iota
	Unpublish
	UnknownPublicationAction
)

// Provide a string value for each publication action
func (o PublicationAction) String() string {
	switch o {
	case Publish:
		return "publish"
	case Unpublish:
		return "unpublish"
	}
	return "unknown"
}

// Provide an PublicationAction from a string value
func PublicationActionFrom(s string) PublicationAction {
	switch s {
	case "publish":
		return Publish
	case "unpublish":
		return Unpublish
	case "unknown":
		return UnknownPublicationAction
	}
	return UnknownPublicationAction
}

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
	BatchTriggerSQL      string
	Schedule             string
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
	result.Body.BatchTriggerSQL = si.Body.BatchTriggerSQL
	result.Body.Schedule = si.Body.Schedule

	return result, nil
}

func (si ScenarioIteration) IsValidForPublication() error {
	if si.Body.ScoreReviewThreshold == nil {
		return fmt.Errorf("Scenario iteration has no ScoreReviewThreshold: \n%w", BadParameterError)
	}

	if si.Body.ScoreRejectThreshold == nil {
		return fmt.Errorf("Scenario iteration has no ScoreRejectThreshold: \n%w", BadParameterError)
	}

	if len(si.Body.Rules) < 1 {
		return fmt.Errorf("Scenario iteration has no rules: \n%w", BadParameterError)
	}
	for _, rule := range si.Body.Rules {
		if !rule.Formula.IsValid() {
			return fmt.Errorf("Scenario iteration rule has invalid rules: \n%w", BadParameterError)
		}
	}

	if si.Body.TriggerCondition == nil {
		return fmt.Errorf("Scenario iteration has no trigger condition: \n%w", BadParameterError)
	} else if !si.Body.TriggerCondition.IsValid() {
		return fmt.Errorf("Scenario iteration trigger condition is invalid: \n%w", BadParameterError)
	}

	return nil
}

type ListScenarioPublicationsFilters struct {
	ScenarioID          *string
	ScenarioIterationID *string
}

type PublishScenarioIterationInput struct {
	ScenarioIterationId string
	PublicationAction   PublicationAction
}

type CreateScenarioPublicationInput struct {
	OrganizationId      string
	ScenarioId          string
	ScenarioIterationId string
	PublicationAction   PublicationAction
}
