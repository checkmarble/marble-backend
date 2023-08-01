package models

import (
	"marble/marble-backend/models/ast"
	"time"
)

type ScenarioIteration struct {
	Id             string
	OrganizationId string
	ScenarioId     string
	Version        *int
	CreatedAt      time.Time
	UpdatedAt      time.Time
	Body           ScenarioIterationBody
}

type ScenarioIterationBody struct {
	TriggerConditionAstExpression *ast.Node
	Rules                         []Rule
	ScoreReviewThreshold          *int
	ScoreRejectThreshold          *int
	BatchTriggerSQL               string
	Schedule                      string
}

type GetScenarioIterationFilters struct {
	ScenarioId *string
}

type CreateScenarioIterationInput struct {
	ScenarioId string
	Body       *CreateScenarioIterationBody
}

type CreateScenarioIterationBody struct {
	TriggerConditionAstExpression *ast.Node
	Rules                         []CreateRuleInput
	ScoreReviewThreshold          *int
	ScoreRejectThreshold          *int
	BatchTriggerSQL               string
	Schedule                      string
}

type UpdateScenarioIterationInput struct {
	Id   string
	Body *UpdateScenarioIterationBody
}

type UpdateScenarioIterationBody struct {
	TriggerConditionAstExpression *ast.Node
	ScoreReviewThreshold          *int
	ScoreRejectThreshold          *int
	BatchTriggerSQL               *string
	Schedule                      *string
}
