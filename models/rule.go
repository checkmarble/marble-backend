package models

import (
	"time"

	"github.com/checkmarble/marble-backend/models/ast"
)

///////////////////////////////
// Rule
///////////////////////////////

type Rule struct {
	Id                   string
	ScenarioIterationId  string
	OrganizationId       string
	DisplayOrder         int
	Name                 string
	Description          string
	FormulaAstExpression *ast.Node
	ScoreModifier        int
	CreatedAt            time.Time
	RuleGroup            string
	SnoozeGroupId        string
}

type CreateRuleInput struct {
	Id                   string
	OrganizationId       string
	ScenarioIterationId  string
	DisplayOrder         int
	Name                 string
	Description          string
	FormulaAstExpression *ast.Node
	ScoreModifier        int
	RuleGroup            string
}

type UpdateRuleInput struct {
	Id                   string
	DisplayOrder         *int
	Name                 *string
	Description          *string
	FormulaAstExpression *ast.Node
	ScoreModifier        *int
	RuleGroup            *string
}
