package dto

import (
	"marble/marble-backend/models"
	"marble/marble-backend/utils"
)

type ScenarioValidationDto struct {
	Errs              []string            `json:"errors"`
	TriggerEvaluation NodeEvaluationDto   `json:"trigger_evaluation"`
	RulesEvaluations  []NodeEvaluationDto `json:"rules_evaluations"`
}

func AdaptScenarioValidationDto(s models.ScenarioValidation) ScenarioValidationDto {
	return ScenarioValidationDto{
		Errs:              utils.Map(s.Errs, func(err error) string { return err.Error() }),
		TriggerEvaluation: AdaptNodeEvaluationDto(s.TriggerEvaluation),
		RulesEvaluations:  utils.Map(s.RulesEvaluations, AdaptNodeEvaluationDto),
	}
}
