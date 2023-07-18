package models

import (
	"marble/marble-backend/models/ast"
	"marble/marble-backend/models/operators"
	"time"
)

///////////////////////////////
// Rule
///////////////////////////////

type Rule struct {
	ID                   string
	ScenarioIterationID  string
	DisplayOrder         int
	Name                 string
	Description          string
	Formula              operators.OperatorBool
	FormulaAstExpression *ast.Node
	ScoreModifier        int
	CreatedAt            time.Time
}

type GetScenarioIterationRulesFilters struct {
	ScenarioIterationID *string
}

type CreateRuleInput struct {
	ScenarioIterationID string
	DisplayOrder        int
	Name                string
	Description         string
	Formula             operators.OperatorBool
	ScoreModifier       int
}

type UpdateRuleInput struct {
	ID            string
	DisplayOrder  *int
	Name          *string
	Description   *string
	Formula       *operators.OperatorBool
	ScoreModifier *int
}
