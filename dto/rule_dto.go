package dto

import (
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models"
)

type RuleDto struct {
	Id                   string    `json:"id"`
	ScenarioIterationId  string    `json:"scenario_iteration_id"`
	DisplayOrder         int       `json:"display_order"`
	Name                 string    `json:"name"`
	Description          string    `json:"description"`
	FormulaAstExpression *NodeDto  `json:"formula_ast_expression"`
	ScoreModifier        int       `json:"score_modifier"`
	CreatedAt            time.Time `json:"created_at"`
	RuleGroup            string    `json:"rule_group"`
}

type RuleExecutionData struct {
	Version string `json:"version"`
	Status  string `json:"status"`
	Total   int    `json:"total"`
}

type CreateRuleInputBody struct {
	ScenarioIterationId  string   `json:"scenario_iteration_id"`
	DisplayOrder         int      `json:"display_order"`
	Name                 string   `json:"name"`
	Description          string   `json:"description"`
	FormulaAstExpression *NodeDto `json:"formula_ast_expression"`
	ScoreModifier        int      `json:"score_modifier"`
	RuleGroup            string   `json:"rule_group"`
}

type UpdateRuleBody struct {
	DisplayOrder         *int     `json:"display_order,omitempty"`
	Name                 *string  `json:"name,omitempty"`
	Description          *string  `json:"description,omitempty"`
	FormulaAstExpression *NodeDto `json:"formula_ast_expression"`
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

func ProcessRuleExecutionDataDtoFromModels(inputs []models.RuleExecutionStat) []RuleExecutionData {
	result := make([]RuleExecutionData, len(inputs))
	for i, input := range inputs {
		item := RuleExecutionData{
			Version: input.Version,
			Status:  input.Outcome,
			Total:   input.Total,
		}
		result[i] = item
	}
	return result
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
