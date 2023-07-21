package ast_eval

import (
	"errors"
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/usecases/ast_eval/evaluate"
)

type EvaluateRuleAstExpression struct {
	AstEvaluationEnvironmentFactory func(organizationId string, payload models.PayloadReader) AstEvaluationEnvironment
}

func (evaluator *EvaluateRuleAstExpression) EvaluateRuleAstExpression(ruleAstExpression ast.Node, organizationId string, payload models.PayloadReader) (bool, error) {
	environment := evaluator.AstEvaluationEnvironmentFactory(organizationId, payload)

	evaluation := EvaluateAst(environment, ruleAstExpression)

	result := evaluation.ReturnValue
	if result == nil {
		return false, errors.Join(evaluation.AllErrors()...)
	}

	if value, ok := result.(bool); ok {
		return value, nil
	}

	return false, fmt.Errorf("rule ast expression does not return a boolean, '%v' instead %w", result, evaluate.ErrRuntimeExpression)
}
