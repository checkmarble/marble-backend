package models

import (
	"fmt"
	"marble/marble-backend/models/ast"
	"time"
)

type ScenarioPublication struct {
	Id                  string
	Rank                int32
	OrganizationId      string
	ScenarioId          string
	ScenarioIterationId string
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
	Id         string
	ScenarioId string
	Version    int
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Body       PublishedScenarioIterationBody
}

type PublishedScenarioIterationBody struct {
	TriggerConditionAstExpression ast.Node
	Rules                         []Rule
	ScoreReviewThreshold          int
	ScoreRejectThreshold          int
	BatchTriggerSQL               string
	Schedule                      string
}

func NewPublishedScenarioIteration(si ScenarioIteration) (PublishedScenarioIteration, error) {
	result := PublishedScenarioIteration{
		Id:         si.Id,
		ScenarioId: si.ScenarioId,
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
	result.Body.TriggerConditionAstExpression = *si.Body.TriggerConditionAstExpression
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
		if rule.FormulaAstExpression == nil {
			return fmt.Errorf("Scenario iteration rule has no formula ast expression %w", BadParameterError)
		}
		// TODO: DRY-run the ast expression

		// if rule.Formula == nil || !(*rule.Formula).IsValid() {
		// 	return fmt.Errorf("Scenario iteration rule has invalid rules: \n%w", BadParameterError)
		// }
	}

	if si.Body.TriggerConditionAstExpression == nil {
		return fmt.Errorf("Scenario iteration has no trigger condition ast expression%w", BadParameterError)
	}

	// TODO: validity check of si.Body.TriggerConditionAstExpression

	return nil
}

type ListScenarioPublicationsFilters struct {
	ScenarioId          *string
	ScenarioIterationId *string
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
