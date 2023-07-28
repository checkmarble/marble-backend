package models

import (
	"marble/marble-backend/models/ast"
	"time"
)

///////////////////////////////
// Rule
///////////////////////////////

type Rule struct {
	ID                   string
	ScenarioIterationID  string
	OrganizationId       string
	DisplayOrder         int
	Name                 string
	Description          string
	FormulaAstExpression *ast.Node
	ScoreModifier        int
	CreatedAt            time.Time
}

type GetScenarioIterationRulesFilters struct {
	ScenarioIterationID *string
}

type CreateRuleInput struct {
	ScenarioIterationID  string
	DisplayOrder         int
	Name                 string
	Description          string
	FormulaAstExpression *ast.Node
	ScoreModifier        int
}

type UpdateRuleInput struct {
	ID                   string
	DisplayOrder         *int
	Name                 *string
	Description          *string
	FormulaAstExpression *ast.Node
	ScoreModifier        *int
}
