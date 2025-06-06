package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
)

type ScenarioValidationErrorDto struct {
	Message string `json:"message"`
	Error   string `json:"error"`
}

func AdaptScenarioValidationErrorDto(err models.ScenarioValidationError) ScenarioValidationErrorDto {
	return ScenarioValidationErrorDto{
		Message: err.Error.Error(),
		Error:   err.Code.String(),
	}
}

type triggerValidationDto struct {
	Errors            []ScenarioValidationErrorDto `json:"errors"`
	TriggerEvaluation ast.NodeEvaluationDto        `json:"trigger_evaluation"`
}

type ruleValidationDto struct {
	Errors         []ScenarioValidationErrorDto `json:"errors"`
	RuleEvaluation ast.NodeEvaluationDto        `json:"rule_evaluation"`
}

type rulesValidationDto struct {
	Errors []ScenarioValidationErrorDto `json:"errors"`
	Rules  map[string]ruleValidationDto `json:"rules"`
}

type decisionValidationDto struct {
	Errors []ScenarioValidationErrorDto `json:"errors"`
}

type sanctionCheckConfigValidationDto struct {
	Trigger                  triggerValidationDto `json:"trigger"`
	Query                    ruleValidationDto    `json:"query"`
	QueryName                ruleValidationDto    `json:"query_name"`
	QueryLabel               ruleValidationDto    `json:"query_label"`
	CounterpartyIdExpression ruleValidationDto    `json:"counterparty_id_expression"`
}

type ScenarioValidationDto struct {
	Trigger             triggerValidationDto               `json:"trigger"`
	Rules               rulesValidationDto                 `json:"rules"`
	SanctionCheckConfig []sanctionCheckConfigValidationDto `json:"sanction_check_config"`
	Decision            decisionValidationDto              `json:"decision"`
}

func AdaptScenarioValidationDto(s models.ScenarioValidation) ScenarioValidationDto {
	return ScenarioValidationDto{
		Trigger: triggerValidationDto{
			Errors:            pure_utils.Map(s.Trigger.Errors, AdaptScenarioValidationErrorDto),
			TriggerEvaluation: ast.AdaptNodeEvaluationDto(s.Trigger.TriggerEvaluation),
		},
		Rules: rulesValidationDto{
			Errors: pure_utils.Map(s.Rules.Errors, AdaptScenarioValidationErrorDto),
			Rules: pure_utils.MapValues(s.Rules.Rules, func(ruleValidation models.RuleValidation) ruleValidationDto {
				return ruleValidationDto{
					Errors:         pure_utils.Map(ruleValidation.Errors, AdaptScenarioValidationErrorDto),
					RuleEvaluation: ast.AdaptNodeEvaluationDto(ruleValidation.RuleEvaluation),
				}
			}),
		},
		SanctionCheckConfig: pure_utils.Map(s.SanctionCheck, func(sc models.SanctionCheckConfigValidation) sanctionCheckConfigValidationDto {
			return sanctionCheckConfigValidationDto{
				Trigger: triggerValidationDto{
					Errors:            pure_utils.Map(sc.TriggerRule.Errors, AdaptScenarioValidationErrorDto),
					TriggerEvaluation: ast.AdaptNodeEvaluationDto(sc.TriggerRule.TriggerEvaluation),
				},
				Query: ruleValidationDto{
					Errors:         pure_utils.Map(sc.Query.Errors, AdaptScenarioValidationErrorDto),
					RuleEvaluation: ast.AdaptNodeEvaluationDto(sc.Query.RuleEvaluation),
				},
				QueryName: ruleValidationDto{
					Errors:         pure_utils.Map(sc.QueryName.Errors, AdaptScenarioValidationErrorDto),
					RuleEvaluation: ast.AdaptNodeEvaluationDto(sc.QueryName.RuleEvaluation),
				},
				QueryLabel: ruleValidationDto{
					Errors:         pure_utils.Map(sc.QueryLabel.Errors, AdaptScenarioValidationErrorDto),
					RuleEvaluation: ast.AdaptNodeEvaluationDto(sc.QueryLabel.RuleEvaluation),
				},
				CounterpartyIdExpression: ruleValidationDto{
					Errors:         pure_utils.Map(sc.CounterpartyIdExpression.Errors, AdaptScenarioValidationErrorDto),
					RuleEvaluation: ast.AdaptNodeEvaluationDto(sc.CounterpartyIdExpression.RuleEvaluation),
				},
			}
		}),
		Decision: decisionValidationDto{
			Errors: pure_utils.Map(s.Decision.Errors, AdaptScenarioValidationErrorDto),
		},
	}
}
