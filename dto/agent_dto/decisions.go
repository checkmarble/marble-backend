package agent_dto

import (
	"encoding/json"
	"time"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/google/uuid"
)

type DecisionRule struct {
	Name          string                               `json:"name"`
	Description   string                               `json:"description"`
	ScoreModifier int                                  `json:"score_modifier"`
	Outcome       string                               `json:"outcome"`
	Evaluation    *ast.NodeEvaluationWithDefinitionDto `json:"evaluation,omitempty"`
}

func AcaptDecisionRule(rule models.RuleExecution, ruleDefs []models.Rule) DecisionRule {
	var eval *ast.NodeEvaluationWithDefinitionDto
	var thisRuleDef *models.Rule
	for _, ruleDef := range ruleDefs {
		if ruleDef.Id == rule.Rule.Id {
			thisRuleDef = &ruleDef
			break
		}
	}

	if thisRuleDef != nil && thisRuleDef.FormulaAstExpression != nil && rule.Evaluation != nil {
		eval = utils.Ptr(ast.MergeAstTrees(*thisRuleDef.FormulaAstExpression, *rule.Evaluation))
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
	Name                    string `json:"name"`
	Description             string `json:"description"`
	Version                 int    `json:"version"`
	ReviewThreshold         *int   `json:"review_threshold"`
	BlockAndReviewThreshold *int   `json:"block_and_review_threshold"`
	DeclineThreshold        *int   `json:"decline_threshold"`
}

type Screening struct {
	Status         string          `json:"status"`
	Datasets       []string        `json:"datasets"`
	SearchInput    json.RawMessage `json:"search_input"`
	IsManual       bool            `json:"is_manual_refined_search"` //nolint:tagliatelle
	PartialResults bool            `json:"partial_results"`
	ErrorCodes     []string        `json:"error_codes"`
}

type ScreeningMatch struct {
	IsMatch bool            `json:"is_match"`
	Status  string          `json:"status"`
	Payload json.RawMessage `json:"payload"`
}

type ScreeningWithMatches struct {
	Screening
	Matches []ScreeningMatch `json:"matches"`
	Count   int              `json:"count"`
}

func AdaptScreeningMatch(match models.ScreeningMatch) ScreeningMatch {
	return ScreeningMatch{
		IsMatch: match.IsMatch,
		Status:  match.Status.String(),
		Payload: match.Payload,
	}
}

func AdaptScreening(screening models.Screening) Screening {
	return Screening{
		Status:         screening.Status.String(),
		Datasets:       screening.Datasets,
		SearchInput:    screening.SearchInput,
		IsManual:       screening.IsManual,
		PartialResults: screening.Partial,
		ErrorCodes:     screening.ErrorCodes,
	}
}

func AdaptScreeningWithMatches(screening models.ScreeningWithMatches) ScreeningWithMatches {
	return ScreeningWithMatches{
		Screening: AdaptScreening(screening.Screening),
		Matches:   pure_utils.Map(screening.Matches, AdaptScreeningMatch),
		Count:     screening.Count,
	}
}

type Decision struct {
	Id                uuid.UUID              `json:"id"`
	CreatedAt         time.Time              `json:"created_at"`
	TriggerObject     map[string]any         `json:"trigger_object"`
	TriggerObjectType string                 `json:"trigger_object_type"`
	Outcome           string                 `json:"outcome"`
	Scenario          DecisionScenario       `json:"scenario"`
	Score             int                    `json:"score"`
	Rules             []DecisionRule         `json:"rules"`
	Screenings        []ScreeningWithMatches `json:"screenings"`
}

func AdaptDecision(
	decision models.Decision,
	scenario models.ScenarioIteration,
	ruleExecutions []models.RuleExecution,
	rules []models.Rule,
	screenings []models.ScreeningWithMatches,
) Decision {
	return Decision{
		Id:                decision.DecisionId,
		CreatedAt:         decision.CreatedAt,
		TriggerObject:     decision.ClientObject.Data,
		TriggerObjectType: decision.ClientObject.TableName,
		Outcome:           decision.Outcome.String(),
		Scenario: DecisionScenario{
			Name:                    decision.ScenarioName,
			Description:             decision.ScenarioDescription,
			Version:                 decision.ScenarioVersion,
			ReviewThreshold:         scenario.ScoreReviewThreshold,
			BlockAndReviewThreshold: scenario.ScoreBlockAndReviewThreshold,
			DeclineThreshold:        scenario.ScoreDeclineThreshold,
		},
		Score: decision.Score,
		Rules: pure_utils.Map(ruleExecutions, func(ruleExec models.RuleExecution) DecisionRule {
			return AcaptDecisionRule(ruleExec, rules)
		}),
		Screenings: pure_utils.Map(screenings, AdaptScreeningWithMatches),
	}
}
