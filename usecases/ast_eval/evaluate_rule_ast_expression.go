package ast_eval

import (
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/usecases/ast_eval/evaluate"
)

type EvaluateRuleAstExpression struct {
	EvaluatorInjectionFactory func(organizationId string, payload models.PayloadReader) EvaluatorInjection
}

func (evaluator *EvaluateRuleAstExpression) EvaluateRuleAstExpression(ruleAstExpression ast.Node, organizationId string, payload models.PayloadReader) (bool, error) {
	environment := evaluator.EvaluatorInjectionFactory(organizationId, payload)

	result, err := EvaluateAst(environment, ruleAstExpression)
	if err != nil {
		return false, err
	}

	if value, ok := result.(bool); ok {
		return value, nil
	}

	return false, fmt.Errorf("rule ast expression does not return a boolean, '%v' instead %w %w", result, err, evaluate.ErrRuntimeExpression)
}
