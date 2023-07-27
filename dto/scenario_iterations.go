package dto

import (
	"encoding/json"
	"fmt"
	"marble/marble-backend/models"
	"time"
)

type ScenarioIterationBodyDto struct {
	TriggerCondition              json.RawMessage            `json:"triggerCondition"`
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
	ID         string    `json:"id"`
	ScenarioID string    `json:"scenarioId"`
	Version    *int      `json:"version"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type ScenarioIterationRuleDto struct {
	ID                   string          `json:"id"`
	ScenarioIterationID  string          `json:"scenarioIterationId"`
	DisplayOrder         int             `json:"displayOrder"`
	Name                 string          `json:"name"`
	Description          string          `json:"description"`
	Formula              json.RawMessage `json:"formula"`
	FormulaAstExpression *NodeDto        `json:"formula_ast_expression"`
	ScoreModifier        int             `json:"scoreModifier"`
	CreatedAt            time.Time       `json:"createdAt"`
}

func AdaptScenarioIterationRuleDto(rule models.Rule) (ScenarioIterationRuleDto, error) {
	formula, err := (*rule.Formula).MarshalJSON()
	if err != nil {
		return ScenarioIterationRuleDto{}, fmt.Errorf("unable to marshal formula: %w", err)
	}

	var formulaAstExpression *NodeDto
	if rule.FormulaAstExpression != nil {
		nodeDto, err := AdaptNodeDto(*rule.FormulaAstExpression)
		if err != nil {
			return ScenarioIterationRuleDto{}, nil
		}
		formulaAstExpression = &nodeDto
	}

	return ScenarioIterationRuleDto{
		ID:                   rule.ID,
		ScenarioIterationID:  rule.ScenarioIterationID,
		DisplayOrder:         rule.DisplayOrder,
		Name:                 rule.Name,
		Description:          rule.Description,
		Formula:              formula,
		FormulaAstExpression: formulaAstExpression,
		ScoreModifier:        rule.ScoreModifier,
		CreatedAt:            rule.CreatedAt,
	}, nil
}

func AdaptScenarioIterationDto(si models.ScenarioIteration) ScenarioIterationDto {
	return ScenarioIterationDto{
		ID:         si.ID,
		ScenarioID: si.ScenarioID,
		Version:    si.Version,
		CreatedAt:  si.CreatedAt,
		UpdatedAt:  si.UpdatedAt,
	}
}

func AdaptScenarioIterationWithBodyDto(si models.ScenarioIteration) (ScenarioIterationWithBodyDto, error) {
	body := ScenarioIterationBodyDto{
		ScoreReviewThreshold: si.Body.ScoreReviewThreshold,
		ScoreRejectThreshold: si.Body.ScoreRejectThreshold,
		BatchTriggerSQL:      si.Body.BatchTriggerSQL,
		Schedule:             si.Body.Schedule,
		Rules:                make([]ScenarioIterationRuleDto, len(si.Body.Rules)),
	}
	for i, rule := range si.Body.Rules {
		apiRule, err := AdaptScenarioIterationRuleDto(rule)
		if err != nil {
			return ScenarioIterationWithBodyDto{}, fmt.Errorf("could not create new api scenario iteration rule: %w", err)
		}
		body.Rules[i] = apiRule
	}

	if si.Body.TriggerCondition != nil {
		triggerConditionBytes, err := si.Body.TriggerCondition.MarshalJSON()
		if err != nil {
			return ScenarioIterationWithBodyDto{}, fmt.Errorf("unable to marshal trigger condition: %w", err)
		}
		body.TriggerCondition = triggerConditionBytes
	}

	if si.Body.TriggerConditionAstExpression != nil {
		triggerDto, err := AdaptNodeDto(*si.Body.TriggerConditionAstExpression)
		if err != nil {
			return ScenarioIterationWithBodyDto{}, fmt.Errorf("unable to marshal trigger condition ast expression: %w", err)
		}
		body.TriggerConditionAstExpression = &triggerDto
	}

	return ScenarioIterationWithBodyDto{
		ScenarioIterationDto: AdaptScenarioIterationDto(si),
		Body:                 body,
	}, nil
}
