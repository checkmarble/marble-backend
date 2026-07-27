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
	AiDescription        string
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
		AiDescription:       r.AiDescription,
		ScoreModifier:       r.ScoreModifier,
		CreatedAt:           r.CreatedAt,
		RuleGroup:           r.RuleGroup,
		StableRuleId:        r.StableRuleId,
	}
}

// HasSameFormula reports whether r and other have structurally identical
// formulas, ignoring node Index. Used to decide whether an AI-generated
// description is still valid for a rule carried over into a new iteration.
func (r Rule) HasSameFormula(other Rule) bool {
	if (r.FormulaAstExpression == nil) != (other.FormulaAstExpression == nil) {
		return false
	}
	if r.FormulaAstExpression == nil {
		return true
	}
	return r.FormulaAstExpression.Hash() == other.FormulaAstExpression.Hash()
}

type RuleMetadata struct {
	Id                  string
	ScenarioIterationId string
	OrganizationId      uuid.UUID
	DisplayOrder        int
	Name                string
	Description         string
	AiDescription       string
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
	AiDescription        string
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
	AiDescription        *string
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
