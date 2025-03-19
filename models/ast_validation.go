package models

import "github.com/checkmarble/marble-backend/models/ast"

type AstValidation struct {
	Errors     []ScenarioValidationError
	Evaluation ast.NodeEvaluation
}

func NewAstValidation() AstValidation {
	return AstValidation{
		Errors: make([]ScenarioValidationError, 0),
	}
}
