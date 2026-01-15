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

func (r Rule) ToMetadata() RuleMetadata {
	return RuleMetadata{
		Id:                  r.Id,
		ScenarioIterationId: r.ScenarioIterationId,
		OrganizationId:      r.OrganizationId,
		DisplayOrder:        r.DisplayOrder,
		Name:                r.Name,
		Description:         r.Description,
		ScoreModifier:       r.ScoreModifier,
		CreatedAt:           r.CreatedAt,
		RuleGroup:           r.RuleGroup,
		StableRuleId:        r.StableRuleId,
	}
}

type RuleMetadata struct {
	Id                  string
	ScenarioIterationId string
	OrganizationId      uuid.UUID
	DisplayOrder        int
	Name                string
	Description         string
	ScoreModifier       int
	CreatedAt           time.Time
	RuleGroup           string
	StableRuleId        string
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
