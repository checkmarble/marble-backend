package models

import (
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

	result.Version = *si.Version
	result.Body.ScoreReviewThreshold = *si.ScoreReviewThreshold
	result.Body.ScoreRejectThreshold = *si.ScoreRejectThreshold
	result.Body.Rules = si.Rules
	result.Body.TriggerConditionAstExpression = *si.TriggerConditionAstExpression
	result.Body.BatchTriggerSQL = si.BatchTriggerSQL
	result.Body.Schedule = si.Schedule

	return result, nil
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
