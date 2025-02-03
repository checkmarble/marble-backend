package models

import (
	"time"

	"github.com/checkmarble/marble-backend/models/ast"
)

type ScenarioIteration struct {
	Id                            string
	OrganizationId                string
	ScenarioId                    string
	Version                       *int
	CreatedAt                     time.Time
	UpdatedAt                     time.Time
	TriggerConditionAstExpression *ast.Node
	Rules                         []Rule
	SanctionCheckConfig           *SanctionCheckConfig
	ScoreReviewThreshold          *int
	ScoreBlockAndReviewThreshold  *int
	ScoreDeclineThreshold         *int
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
	ScoreBlockAndReviewThreshold  *int
	ScoreDeclineThreshold         *int
	Schedule                      string
}

type UpdateScenarioIterationInput struct {
	Id   string
	Body UpdateScenarioIterationBody
}

type UpdateScenarioIterationBody struct {
	TriggerConditionAstExpression *ast.Node
	SanctionCheckConfig           *UpdateSanctionCheckConfigInput
	ScoreReviewThreshold          *int
	ScoreBlockAndReviewThreshold  *int
	ScoreDeclineThreshold         *int
	Schedule                      *string
}

type SanctionCheckConfig struct {
	Datasets    []string
	TriggerRule ast.Node
	Query       SanctionCheckConfigQuery
	Outcome     SanctionCheckOutcome
}

type SanctionCheckOutcome struct {
	ForceOutcome  Outcome
	ScoreModifier int
}

type UpdateSanctionCheckConfigInput struct {
	Datasets    []string
	TriggerRule *ast.Node
	Query       *SanctionCheckConfigQuery
	Outcome     UpdateSanctionCheckOutcomeInput
}

type SanctionCheckConfigQuery struct {
	Name ast.Node
}

type UpdateSanctionCheckOutcomeInput struct {
	ForceOutcome  *Outcome
	ScoreModifier *int
}
