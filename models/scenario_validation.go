package models

import "github.com/checkmarble/marble-backend/models/ast"

type ScenarioValidationErrorCode int

const (
	// General
	DataModelNotFound ScenarioValidationErrorCode = iota
	TrigerObjectNotFound
	// Trigger
	TriggerConditionRequired
	// Rule
	RuleFormulaRequired
	// Decision
	ScoreReviewThresholdRequired
	ScoreRejectThresholdRequired
	ScoreRejectReviewThresholdsMissmatch
)

// Provide a string value for each outcome
func (e ScenarioValidationErrorCode) String() string {
	switch e {
	case DataModelNotFound:
		return "DATA_MODEL_NOT_FOUND"
	case TrigerObjectNotFound:
		return "TRIGGER_OBJECT_NOT_FOUND"
	case TriggerConditionRequired:
		return "TRIGGER_CONDITION_REQUIRED"
	case RuleFormulaRequired:
		return "RULE_FORMULA_REQUIRED"
	case ScoreReviewThresholdRequired:
		return "SCORE_REVIEW_THRESHOLD_REQUIRED"
	case ScoreRejectThresholdRequired:
		return "SCORE_REJECT_THRESHOLD_REQUIRED"
	case ScoreRejectReviewThresholdsMissmatch:
		return "SCORE_REJECT_REVIEW_THRESHOLDS_MISSMATCH"
	}
	return "unknown ScenarioValidationErrorCode"
}

type ScenarioValidationError struct {
	Error error
	Code  ScenarioValidationErrorCode
}

type triggerValidation struct {
	Errors            []ScenarioValidationError
	TriggerEvaluation ast.NodeEvaluation
}

type RuleValidation struct {
	Errors         []ScenarioValidationError
	RuleEvaluation ast.NodeEvaluation
}

func NewRuleValidation() RuleValidation {
	return RuleValidation{
		Errors:         make([]ScenarioValidationError, 0),
		RuleEvaluation: ast.NodeEvaluation{},
	}
}

type rulesValidation struct {
	Errors []ScenarioValidationError
	Rules  map[string]RuleValidation
}

type decisionValidation struct {
	Errors []ScenarioValidationError
}

type ScenarioValidation struct {
	Errors   []ScenarioValidationError
	Trigger  triggerValidation
	Rules    rulesValidation
	Decision decisionValidation
}

func NewScenarioValidation() ScenarioValidation {
	return ScenarioValidation{
		Errors: make([]ScenarioValidationError, 0),
		Trigger: triggerValidation{
			Errors: make([]ScenarioValidationError, 0),
		},
		Rules: rulesValidation{
			Errors: make([]ScenarioValidationError, 0),
			Rules:  make(map[string]RuleValidation),
		},
		Decision: decisionValidation{
			Errors: make([]ScenarioValidationError, 0),
		},
	}
}
