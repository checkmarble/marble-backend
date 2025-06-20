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
	// Ast output
	FormulaMustReturnBoolean
	FormulaMustReturnString
	FormulaIncorrectReturnType
	// Decision
	ScoreThresholdMissing
	ScoreThresholdsMismatch
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
	case FormulaMustReturnBoolean:
		return "FORMULA_MUST_RETURN_BOOLEAN"
	case FormulaMustReturnString:
		return "FORMULA_MUST_RETURN_STRING"
	case FormulaIncorrectReturnType:
		return "FORMULA_INCORRECT_RETURN_TYPE"
	case ScoreThresholdMissing:
		return "SCORE_THRESHOLD_MISSING"
	case ScoreThresholdsMismatch:
		return "SCORE_THRESHOLDS_MISMATCH"
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

type ScreeningConfigValidation struct {
	TriggerRule              triggerValidation
	Query                    RuleValidation
	QueryFields              map[string]RuleValidation
	CounterpartyIdExpression RuleValidation
}

type ScenarioValidation struct {
	Errors     []ScenarioValidationError
	Trigger    triggerValidation
	Rules      rulesValidation
	Screenings []ScreeningConfigValidation
	Decision   decisionValidation
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
		Screenings: []ScreeningConfigValidation{},
		Decision: decisionValidation{
			Errors: make([]ScenarioValidationError, 0),
		},
	}
}

func NewScreeningValidation() ScreeningConfigValidation {
	return ScreeningConfigValidation{
		TriggerRule: triggerValidation{
			Errors: make([]ScenarioValidationError, 0),
		},
		QueryFields: make(map[string]RuleValidation),
	}
}
