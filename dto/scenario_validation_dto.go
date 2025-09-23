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

type screeningConfigValidationDto struct {
	Trigger                  triggerValidationDto         `json:"trigger"`
	Query                    ruleValidationDto            `json:"query"`
	QueryFields              map[string]ruleValidationDto `json:"query_fields"`
	CounterpartyIdExpression ruleValidationDto            `json:"counterparty_id_expression"`
}

type ScenarioValidationDto struct {
	Trigger triggerValidationDto `json:"trigger"`
	Rules   rulesValidationDto   `json:"rules"`

	// Deprecated, to remove after the frontend starts consuming the new field
	ScreeningConfigs_deprec []screeningConfigValidationDto `json:"sanction_check_config"` //nolint:tagliatelle
	ScreeningConfigs        []screeningConfigValidationDto `json:"screening_configs"`
	Decision                decisionValidationDto          `json:"decision"`
}

func AdaptScenarioValidationDto(s models.ScenarioValidation) ScenarioValidationDto {
	screeningConfigs := pure_utils.Map(s.Screenings, func(
		sc models.ScreeningConfigValidation,
	) screeningConfigValidationDto {
		return screeningConfigValidationDto{
			Trigger: triggerValidationDto{
				Errors:            pure_utils.Map(sc.TriggerRule.Errors, AdaptScenarioValidationErrorDto),
				TriggerEvaluation: ast.AdaptNodeEvaluationDto(sc.TriggerRule.TriggerEvaluation),
			},
			Query: ruleValidationDto{
				Errors:         pure_utils.Map(sc.Query.Errors, AdaptScenarioValidationErrorDto),
				RuleEvaluation: ast.AdaptNodeEvaluationDto(sc.Query.RuleEvaluation),
			},
			QueryFields: pure_utils.MapValues(sc.QueryFields, func(e models.RuleValidation) ruleValidationDto {
				return ruleValidationDto{
					Errors:         pure_utils.Map(e.Errors, AdaptScenarioValidationErrorDto),
					RuleEvaluation: ast.AdaptNodeEvaluationDto(e.RuleEvaluation),
				}
			}),
			CounterpartyIdExpression: ruleValidationDto{
				Errors: pure_utils.Map(sc.CounterpartyIdExpression.Errors, AdaptScenarioValidationErrorDto),
				RuleEvaluation: ast.AdaptNodeEvaluationDto(
					sc.CounterpartyIdExpression.RuleEvaluation),
			},
		}
	})
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
		ScreeningConfigs_deprec: screeningConfigs,
		ScreeningConfigs:        screeningConfigs,
		Decision: decisionValidationDto{
			Errors: pure_utils.Map(s.Decision.Errors, AdaptScenarioValidationErrorDto),
		},
	}
}
