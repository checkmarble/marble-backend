package models

import (
	"marble/marble-backend/models/ast"
	"time"
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
}

type UpdateRuleInput struct {
	Id                   string
	DisplayOrder         *int
	Name                 *string
	Description          *string
	FormulaAstExpression *ast.Node
	ScoreModifier        *int
}
