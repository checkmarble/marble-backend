package dto

import (
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
)

type ScenarioValidationErrorDto struct {
	Message string `json:"message"`
	Code    string `json:"error"`
}

func AdaptScenarioValidationErrorDto(err models.ScenarioValidationError) ScenarioValidationErrorDto {
	return ScenarioValidationErrorDto{
		Message: err.Error.Error(),
		Code:    err.Code.String(),
	}
}

type triggerValidationDto struct {
	Errors            []ScenarioValidationErrorDto `json:"errors"`
	TriggerEvaluation NodeEvaluationDto            `json:"trigger_evaluation"`
}

type ruleValidationDto struct {
	Errors         []ScenarioValidationErrorDto `json:"errors"`
	RuleEvaluation NodeEvaluationDto            `json:"rule_evaluation"`
}

type rulesValidationDto struct {
	Errors []ScenarioValidationErrorDto `json:"errors"`
	Rules  map[string]ruleValidationDto `json:"rules"`
}

type decisionValidationDto struct {
	Errors []ScenarioValidationErrorDto `json:"errors"`
}

type ScenarioValidationDto struct {
	Trigger  triggerValidationDto  `json:"trigger"`
	Rules    rulesValidationDto    `json:"rules"`
	Decision decisionValidationDto `json:"decision"`
}

func AdaptScenarioValidationDto(s models.ScenarioValidation) ScenarioValidationDto {
	return ScenarioValidationDto{
		Trigger: triggerValidationDto{
			Errors:            pure_utils.Map(s.Trigger.Errors, AdaptScenarioValidationErrorDto),
			TriggerEvaluation: AdaptNodeEvaluationDto(s.Trigger.TriggerEvaluation),
		},
		Rules: rulesValidationDto{
			Errors: pure_utils.Map(s.Rules.Errors, AdaptScenarioValidationErrorDto),
			Rules: pure_utils.MapValues(s.Rules.Rules, func(ruleValidation models.RuleValidation) ruleValidationDto {
				return ruleValidationDto{
					Errors:         pure_utils.Map(ruleValidation.Errors, AdaptScenarioValidationErrorDto),
					RuleEvaluation: AdaptNodeEvaluationDto(ruleValidation.RuleEvaluation),
				}
			}),
		},
		Decision: decisionValidationDto{
			Errors: pure_utils.Map(s.Decision.Errors, AdaptScenarioValidationErrorDto),
		},
	}
}
