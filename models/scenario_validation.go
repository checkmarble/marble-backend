package models

import "marble/marble-backend/models/ast"

type ScenarioValidation struct {
	Errs              []error
	TriggerEvaluation ast.NodeEvaluation
	RulesEvaluations  []ast.NodeEvaluation
}
