package dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/utils"
)

// Dto for case AI recap

type CaseRecapRuleDto struct {
	Name          string                               `json:"name"`
	Description   string                               `json:"description"`
	ScoreModifier int                                  `json:"score_modifier"`
	Outcome       string                               `json:"outcome"`
	Evaluation    *ast.NodeEvaluationWithDefinitionDto `json:"evaluation,omitempty"`
}

func AcaptCaseRecapRuleDto(rule DecisionRule, ruleDef models.Rule) CaseRecapRuleDto {
	return CaseRecapRuleDto{
		Name:          rule.Name,
		Description:   rule.Description,
		ScoreModifier: rule.ScoreModifier,
		Outcome:       rule.Outcome,
		Evaluation:    utils.Ptr(ast.MergeAstTrees(*ruleDef.FormulaAstExpression, *rule.RuleEvaluation)),
	}
}

type CaseRecapDecisionDto struct {
	CreatedAt         time.Time          `json:"created_at"`
	TriggerObject     map[string]any     `json:"trigger_object"`
	TriggerObjectType string             `json:"trigger_object_type"`
	Outcome           string             `json:"outcome"`
	Scenario          DecisionScenario   `json:"scenario"`
	Score             int                `json:"score"`
	Rules             []CaseRecapRuleDto `json:"rules"`
}

func AdaptCaseRecapDecisionDto(decision models.Decision, rules []CaseRecapRuleDto) CaseRecapDecisionDto {
	return CaseRecapDecisionDto{
		CreatedAt:         decision.CreatedAt,
		TriggerObject:     decision.ClientObject.Data,
		TriggerObjectType: decision.ClientObject.TableName,
		Outcome:           decision.Outcome.String(),
		Scenario: DecisionScenario{
			Name:        decision.ScenarioName,
			Description: decision.ScenarioDescription,
			Version:     decision.ScenarioVersion,
		},
		Score: decision.Score,
		Rules: rules,
	}
}
