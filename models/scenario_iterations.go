package models

import (
	"marble/marble-backend/models/ast"
	"marble/marble-backend/models/operators"
	"time"
)

type ScenarioIteration struct {
	ID         string
	ScenarioID string
	Version    *int
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Body       ScenarioIterationBody
}

type ScenarioIterationBody struct {
	TriggerCondition              operators.OperatorBool
	TriggerConditionAstExpression *ast.Node
	Rules                         []Rule
	ScoreReviewThreshold          *int
	ScoreRejectThreshold          *int
	BatchTriggerSQL               string
	Schedule                      string
}

type GetScenarioIterationFilters struct {
	ScenarioID *string
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
	BatchTriggerSQL      string
	Schedule             string
}

type UpdateScenarioIterationInput struct {
	ID   string
	Body *UpdateScenarioIterationBody
}

type UpdateScenarioIterationBody struct {
	TriggerCondition     operators.OperatorBool
	ScoreReviewThreshold *int
	ScoreRejectThreshold *int
	BatchTriggerSQL      *string
	Schedule             *string
}
