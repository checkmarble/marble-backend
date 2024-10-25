package models

import (
	"time"

	"github.com/checkmarble/marble-backend/models/ast"
)

type ScenarioPublication struct {
	Id                  string
	Rank                int32
	OrganizationId      string
	ScenarioId          string
	ScenarioIterationId string
	TestMode            bool
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
	ScoreBlockAndReviewThreshold  int
	ScoreDeclineThreshold         int
	Schedule                      string
}

func NewPublishedScenarioIteration(si ScenarioIteration) PublishedScenarioIteration {
	result := PublishedScenarioIteration{
		Id:         si.Id,
		ScenarioId: si.ScenarioId,
		CreatedAt:  si.CreatedAt,
		UpdatedAt:  si.UpdatedAt,
	}

	result.Version = *si.Version
	result.Body.ScoreReviewThreshold = *si.ScoreReviewThreshold
	result.Body.ScoreBlockAndReviewThreshold = *si.ScoreBlockAndReviewThreshold
	result.Body.ScoreDeclineThreshold = *si.ScoreDeclineThreshold
	result.Body.Rules = si.Rules
	result.Body.Schedule = si.Schedule
	if si.TriggerConditionAstExpression != nil {
		result.Body.TriggerConditionAstExpression = *si.TriggerConditionAstExpression
	}
	return result
}

type ListScenarioPublicationsFilters struct {
	ScenarioId          *string
	ScenarioIterationId *string
}

type PublishScenarioIterationInput struct {
	ScenarioIterationId string
	PublicationAction   PublicationAction
	TestMode            bool
}

type CreateScenarioPublicationInput struct {
	OrganizationId      string
	ScenarioId          string
	ScenarioIterationId string
	PublicationAction   PublicationAction
	TestMode            bool
}

type PublicationPreparationStatus struct {
	PreparationStatus        PreparationStatus
	PreparationServiceStatus PreparationServiceStatus
}

type PreparationStatus string

var (
	PreparationStatusRequired        PreparationStatus = "required"
	PreparationStatusInProgress      PreparationStatus = "in_progress" // We are not yet able compute this one
	PreparationStatusReadyToActivate PreparationStatus = "ready_to_activate"
)

type PreparationServiceStatus string

var (
	PreparationServiceStatusAvailable PreparationServiceStatus = "available"
	PreparationServiceStatusOccupied  PreparationServiceStatus = "occupied"
)
