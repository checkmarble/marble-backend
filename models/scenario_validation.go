package models

import "github.com/checkmarble/marble-backend/models/ast"

type ScenarioValidation struct {
	Errs              []error
	TriggerEvaluation ast.NodeEvaluation
	RulesEvaluations  map[string]ast.NodeEvaluation
}
