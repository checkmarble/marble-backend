package dto

import (
	"fmt"
	"marble/marble-backend/models"
	"time"
)

type ScenarioIterationBodyDto struct {
	TriggerConditionAstExpression *NodeDto                   `json:"trigger_condition_ast_expression"`
	Rules                         []ScenarioIterationRuleDto `json:"rules"`
	ScoreReviewThreshold          *int                       `json:"scoreReviewThreshold"`
	ScoreRejectThreshold          *int                       `json:"scoreRejectThreshold"`
	BatchTriggerSQL               string                     `json:"batchTriggerSql"`
	Schedule                      string                     `json:"schedule"`
}

type ScenarioIterationWithBodyDto struct {
	ScenarioIterationDto
	Body ScenarioIterationBodyDto `json:"body"`
}

type ScenarioIterationDto struct {
	Id         string    `json:"id"`
	ScenarioId string    `json:"scenarioId"`
	Version    *int      `json:"version"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type ScenarioIterationRuleDto struct {
	Id                   string    `json:"id"`
	ScenarioIterationId  string    `json:"scenarioIterationId"`
	DisplayOrder         int       `json:"displayOrder"`
	Name                 string    `json:"name"`
	Description          string    `json:"description"`
	FormulaAstExpression *NodeDto  `json:"formula_ast_expression"`
	ScoreModifier        int       `json:"scoreModifier"`
	CreatedAt            time.Time `json:"createdAt"`
}

func AdaptScenarioIterationRuleDto(rule models.Rule) (ScenarioIterationRuleDto, error) {

	var formulaAstExpression *NodeDto
	if rule.FormulaAstExpression != nil {
		nodeDto, err := AdaptNodeDto(*rule.FormulaAstExpression)
		if err != nil {
			return ScenarioIterationRuleDto{}, err
		}
		formulaAstExpression = &nodeDto
	}

	return ScenarioIterationRuleDto{
		Id:                   rule.Id,
		ScenarioIterationId:  rule.ScenarioIterationId,
		DisplayOrder:         rule.DisplayOrder,
		Name:                 rule.Name,
		Description:          rule.Description,
		FormulaAstExpression: formulaAstExpression,
		ScoreModifier:        rule.ScoreModifier,
		CreatedAt:            rule.CreatedAt,
	}, nil
}

func AdaptScenarioIterationWithBodyDto(si models.ScenarioIteration) (ScenarioIterationWithBodyDto, error) {
	body := ScenarioIterationBodyDto{
		ScoreReviewThreshold: si.ScoreReviewThreshold,
		ScoreRejectThreshold: si.ScoreRejectThreshold,
		BatchTriggerSQL:      si.BatchTriggerSQL,
		Schedule:             si.Schedule,
		Rules:                make([]ScenarioIterationRuleDto, len(si.Rules)),
	}
	for i, rule := range si.Rules {
		apiRule, err := AdaptScenarioIterationRuleDto(rule)
		if err != nil {
			return ScenarioIterationWithBodyDto{}, fmt.Errorf("could not create new api scenario iteration rule: %w", err)
		}
		body.Rules[i] = apiRule
	}

	if si.TriggerConditionAstExpression != nil {
		triggerDto, err := AdaptNodeDto(*si.TriggerConditionAstExpression)
		if err != nil {
			return ScenarioIterationWithBodyDto{}, fmt.Errorf("unable to marshal trigger condition ast expression: %w", err)
		}
		body.TriggerConditionAstExpression = &triggerDto
	}

	return ScenarioIterationWithBodyDto{
		ScenarioIterationDto: ScenarioIterationDto{
			Id:         si.Id,
			ScenarioId: si.ScenarioId,
			Version:    si.Version,
			CreatedAt:  si.CreatedAt,
			UpdatedAt:  si.UpdatedAt,
		},
		Body: body,
	}, nil
}
