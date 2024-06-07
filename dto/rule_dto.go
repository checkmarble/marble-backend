package dto

import (
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type ListRulesInput struct {
	ScenarioIterationId string `in:"query=scenarioIterationId;required"`
}

type GetRuleInput struct {
	RuleID string `in:"path=ruleID"`
}

type RuleDto struct {
	Id                   string    `json:"id"`
	ScenarioIterationId  string    `json:"scenarioIterationId"`
	DisplayOrder         int       `json:"displayOrder"`
	Name                 string    `json:"name"`
	Description          string    `json:"description"`
	FormulaAstExpression *NodeDto  `json:"formula_ast_expression"`
	ScoreModifier        int       `json:"scoreModifier"`
	CreatedAt            time.Time `json:"createdAt"`
	RuleGroup            string    `json:"rule_group"`
}

type CreateRuleInputBody struct {
	ScenarioIterationId  string   `json:"scenarioIterationId"`
	DisplayOrder         int      `json:"displayOrder"`
	Name                 string   `json:"name"`
	Description          string   `json:"description"`
	FormulaAstExpression *NodeDto `json:"formula_ast_expression"`
	ScoreModifier        int      `json:"scoreModifier"`
	RuleGroup            string   `json:"rule_group"`
}

type CreateRuleInput struct {
	Body *CreateRuleInputBody `in:"body=json"`
}

type UpdateRuleBody struct {
	DisplayOrder         *int     `json:"displayOrder,omitempty"`
	Name                 *string  `json:"name,omitempty"`
	Description          *string  `json:"description,omitempty"`
	FormulaAstExpression *NodeDto `json:"formula_ast_expression"`
	ScoreModifier        *int     `json:"scoreModifier,omitempty"`
	RuleGroup            *string  `json:"rule_group"`
}

type UpdateRuleInput struct {
	RuleID string          `in:"path=ruleID"`
	Body   *UpdateRuleBody `in:"body=json"`
}
type DeleteRuleInput struct {
	RuleID string `in:"path=ruleID"`
}

func AdaptRuleDto(rule models.Rule) (RuleDto, error) {
	var formulaAstExpression *NodeDto
	if rule.FormulaAstExpression != nil {
		nodeDto, err := AdaptNodeDto(*rule.FormulaAstExpression)
		if err != nil {
			return RuleDto{}, err
		}
		formulaAstExpression = &nodeDto
	}

	return RuleDto{
		Id:                   rule.Id,
		ScenarioIterationId:  rule.ScenarioIterationId,
		DisplayOrder:         rule.DisplayOrder,
		Name:                 rule.Name,
		Description:          rule.Description,
		FormulaAstExpression: formulaAstExpression,
		ScoreModifier:        rule.ScoreModifier,
		CreatedAt:            rule.CreatedAt,
		RuleGroup:            rule.RuleGroup,
	}, nil
}

func AdaptCreateRuleInput(body CreateRuleInputBody, organizationId string) (models.CreateRuleInput, error) {
	createRuleInput := models.CreateRuleInput{
		OrganizationId:       organizationId,
		ScenarioIterationId:  body.ScenarioIterationId,
		DisplayOrder:         body.DisplayOrder,
		Name:                 body.Name,
		Description:          body.Description,
		FormulaAstExpression: nil,
		ScoreModifier:        body.ScoreModifier,
		RuleGroup:            body.RuleGroup,
	}

	if body.FormulaAstExpression != nil {
		node, err := AdaptASTNode(*body.FormulaAstExpression)
		if err != nil {
			return models.CreateRuleInput{}, fmt.Errorf(
				"could not adapt formula ast expression: %w %w", err, models.BadParameterError)
		}
		createRuleInput.FormulaAstExpression = &node
	}

	return createRuleInput, nil
}

func AdaptUpdateRule(ruleId string, body UpdateRuleBody) (models.UpdateRuleInput, error) {
	updateRuleInput := models.UpdateRuleInput{
		Id:                   ruleId,
		DisplayOrder:         body.DisplayOrder,
		Name:                 body.Name,
		Description:          body.Description,
		FormulaAstExpression: nil,
		ScoreModifier:        body.ScoreModifier,
		RuleGroup:            body.RuleGroup,
	}

	if body.FormulaAstExpression != nil {
		node, err := AdaptASTNode(*body.FormulaAstExpression)
		if err != nil {
			return models.UpdateRuleInput{}, fmt.Errorf(
				"could not adapt formula ast expression: %w %w", err, models.BadParameterError)
		}
		updateRuleInput.FormulaAstExpression = &node
	}

	return updateRuleInput, nil
}
