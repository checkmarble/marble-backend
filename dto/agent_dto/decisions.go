package agent_dto

import (
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/utils"
)

type DecisionRule struct {
	Name          string                               `json:"name"`
	Description   string                               `json:"description"`
	ScoreModifier int                                  `json:"score_modifier"`
	Outcome       string                               `json:"outcome"`
	Evaluation    *ast.NodeEvaluationWithDefinitionDto `json:"evaluation,omitempty"`
}

func AcaptDecisionRule(rule models.RuleExecution, ruleDef models.Rule) DecisionRule {
	var eval *ast.NodeEvaluationWithDefinitionDto
	if ruleDef.FormulaAstExpression != nil && rule.Evaluation != nil {
		eval = utils.Ptr(ast.MergeAstTrees(*ruleDef.FormulaAstExpression, *rule.Evaluation))
	}
	return DecisionRule{
		Name:          rule.Rule.Name,
		Description:   rule.Rule.Description,
		Outcome:       rule.Outcome,
		ScoreModifier: rule.ResultScoreModifier,
		Evaluation:    eval,
	}
}

type DecisionScenario struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     int    `json:"version"`
}

type Decision struct {
	CreatedAt         time.Time        `json:"created_at"`
	TriggerObject     map[string]any   `json:"trigger_object"`
	TriggerObjectType string           `json:"trigger_object_type"`
	Outcome           string           `json:"outcome"`
	Scenario          DecisionScenario `json:"scenario"`
	Score             int              `json:"score"`
	Rules             []DecisionRule   `json:"rules"`
}

func AdaptDecision(decision models.Decision, rules []DecisionRule) Decision {
	return Decision{
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
