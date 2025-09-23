package agent_dto

import (
	"github.com/checkmarble/marble-backend/dto"
)

type Rule struct {
	Id                   string      `json:"id"`
	ScoreModifier        int         `json:"score_modifier"`
	FormulaAstExpression dto.NodeDto `json:"formula_ast_expression"`
}
