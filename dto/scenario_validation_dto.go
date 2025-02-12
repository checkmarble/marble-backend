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
	NameFilter               ruleValidationDto    `json:"name_filter"`
	CounterpartyIdExpression ruleValidationDto    `json:"counterparty_id_expression"`
}

type ScenarioValidationDto struct {
	Trigger             triggerValidationDto             `json:"trigger"`
	Rules               rulesValidationDto               `json:"rules"`
	SanctionCheckConfig sanctionCheckConfigValidationDto `json:"sanction_check_config"`
	Decision            decisionValidationDto            `json:"decision"`
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
		SanctionCheckConfig: sanctionCheckConfigValidationDto{
			Trigger: triggerValidationDto{
				Errors:            pure_utils.Map(s.SanctionCheck.TriggerRule.Errors, AdaptScenarioValidationErrorDto),
				TriggerEvaluation: ast.AdaptNodeEvaluationDto(s.SanctionCheck.TriggerRule.TriggerEvaluation),
			},
			NameFilter: ruleValidationDto{
				Errors:         pure_utils.Map(s.SanctionCheck.NameFilter.Errors, AdaptScenarioValidationErrorDto),
				RuleEvaluation: ast.AdaptNodeEvaluationDto(s.SanctionCheck.NameFilter.RuleEvaluation),
			},
			CounterpartyIdExpression: ruleValidationDto{
				Errors:         pure_utils.Map(s.SanctionCheck.CounterpartyIdExpression.Errors, AdaptScenarioValidationErrorDto),
				RuleEvaluation: ast.AdaptNodeEvaluationDto(s.SanctionCheck.CounterpartyIdExpression.RuleEvaluation),
			},
		},
		Decision: decisionValidationDto{
			Errors: pure_utils.Map(s.Decision.Errors, AdaptScenarioValidationErrorDto),
		},
	}
}
