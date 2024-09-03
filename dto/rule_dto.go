package dto

import (
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type RuleDto struct {
	Id                         string    `json:"id"`
	ScenarioIterationId_deprec string    `json:"scenarioIterationId"`
	ScenarioIterationId        string    `json:"scenario_iteration_id"`
	DisplayOrder_deprec        int       `json:"displayOrder"`
	DisplayOrder               int       `json:"display_order"`
	Name                       string    `json:"name"`
	Description                string    `json:"description"`
	FormulaAstExpression       *NodeDto  `json:"formula_ast_expression"`
	ScoreModifier_deprec       int       `json:"scoreModifier"`
	ScoreModifier              int       `json:"score_modifier"`
	CreatedAt_deprec           time.Time `json:"createdAt"`
	CreatedAt                  time.Time `json:"created_at"`
	RuleGroup                  string    `json:"rule_group"`
}

type CreateRuleInputBody struct {
	ScenarioIterationId_deprec string   `json:"scenarioIterationId"`
	ScenarioIterationId        string   `json:"scenario_iteration_id"`
	DisplayOrder_deprec        int      `json:"displayOrder"`
	DisplayOrder               int      `json:"display_order"`
	Name                       string   `json:"name"`
	Description                string   `json:"description"`
	FormulaAstExpression       *NodeDto `json:"formula_ast_expression"`
	ScoreModifier_deprec       int      `json:"scoreModifier"`
	ScoreModifier              int      `json:"score_modifier"`
	RuleGroup                  string   `json:"rule_group"`
}

type UpdateRuleBody struct {
	DisplayOrder_deprec  *int     `json:"displayOrder,omitempty"`
	DisplayOrder         *int     `json:"display_order,omitempty"`
	Name                 *string  `json:"name,omitempty"`
	Description          *string  `json:"description,omitempty"`
	FormulaAstExpression *NodeDto `json:"formula_ast_expression"`
	ScoreModifier_deprec *int     `json:"scoreModifier,omitempty"`
	ScoreModifier        *int     `json:"score_modifier,omitempty"`
	RuleGroup            *string  `json:"rule_group"`
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
		Id:                         rule.Id,
		ScenarioIterationId_deprec: rule.ScenarioIterationId,
		ScenarioIterationId:        rule.ScenarioIterationId,
		DisplayOrder_deprec:        rule.DisplayOrder,
		DisplayOrder:               rule.DisplayOrder,
		Name:                       rule.Name,
		Description:                rule.Description,
		FormulaAstExpression:       formulaAstExpression,
		ScoreModifier_deprec:       rule.ScoreModifier,
		ScoreModifier:              rule.ScoreModifier,
		CreatedAt_deprec:           rule.CreatedAt,
		CreatedAt:                  rule.CreatedAt,
		RuleGroup:                  rule.RuleGroup,
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

	// TODO remove deprec
	if createRuleInput.ScenarioIterationId == "" {
		createRuleInput.ScenarioIterationId = body.ScenarioIterationId_deprec
	}
	if createRuleInput.DisplayOrder == 0 {
		createRuleInput.DisplayOrder = body.DisplayOrder_deprec
	}
	if createRuleInput.ScoreModifier == 0 {
		createRuleInput.ScoreModifier = body.ScoreModifier_deprec
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

	if body.DisplayOrder == nil {
		updateRuleInput.DisplayOrder = body.DisplayOrder_deprec
	}
	if body.ScoreModifier == nil {
		updateRuleInput.ScoreModifier = body.ScoreModifier_deprec
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
