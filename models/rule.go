package models

import (
	"time"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/google/uuid"
)

///////////////////////////////
// Rule
///////////////////////////////

type Rule struct {
	Id                   string
	ScenarioIterationId  string
	OrganizationId       uuid.UUID
	DisplayOrder         int
	Name                 string
	Description          string
	FormulaAstExpression *ast.Node
	ScoreModifier        int
	CreatedAt            time.Time
	RuleGroup            string
	SnoozeGroupId        *string
	StableRuleId         string
}

type CreateRuleInput struct {
	Id                   string
	OrganizationId       uuid.UUID
	ScenarioIterationId  string
	DisplayOrder         int
	Name                 string
	Description          string
	FormulaAstExpression *ast.Node
	ScoreModifier        int
	RuleGroup            string
	SnoozeGroupId        *string
	StableRuleId         string
}

type UpdateRuleInput struct {
	Id                   string
	DisplayOrder         *int
	Name                 *string
	Description          *string
	FormulaAstExpression *ast.Node
	ScoreModifier        *int
	RuleGroup            *string
	SnoozeGroupId        *string
	StableRuleId         *string
}

type AiRuleDescription struct {
	Description string
	IsRuleValid bool
}
