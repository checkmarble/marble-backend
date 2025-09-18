package agent_dto

import (
	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
)

type Rule struct {
	Id                   string      `json:"id"`
	ScoreModifier        int         `json:"score_modifier"`
	FormulaAstExpression dto.NodeDto `json:"formula_ast_expression"`
}

func AdaptRuleDto(rule models.Rule) (Rule, error) {
	formulaAstExpression := dto.NodeDto{}
	if rule.FormulaAstExpression != nil {
		var err error
		formulaAstExpression, err = dto.AdaptNodeDto(*rule.FormulaAstExpression)
		if err != nil {
			return Rule{}, err
		}
	}
	return Rule{
		Id:                   rule.Id,
		ScoreModifier:        rule.ScoreModifier,
		FormulaAstExpression: formulaAstExpression,
	}, nil
}
