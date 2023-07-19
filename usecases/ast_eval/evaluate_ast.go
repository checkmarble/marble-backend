package ast_eval

import (
	"marble/marble-backend/models/ast"
	"marble/marble-backend/utils"
)

func EvaluateAst(environment AstEvaluationEnvironment, node ast.Node) (any, error) {

	// Early exit for constant, because it should have no children.
	if node.Function == ast.FUNC_CONSTANT {
		return node.Constant, nil
	}

	arguments := ast.Arguments{}

	var evalNode = func(node ast.Node) (any, error) { return EvaluateAst(environment, node) }

	// Eval children
	var err error
	arguments.Args, err = utils.MapErr(node.Children, evalNode)
	if err != nil {
		return nil, err
	}

	// Eval named children
	arguments.NamedArgs, err = utils.MapMapErr(node.NamedChildren, evalNode)
	if err != nil {
		return nil, err
	}

	// get evaluator
	evaluator, err := environment.GetEvaluator(node.Function)
	if err != nil {
		return nil, err
	}

	return evaluator.Evaluate(arguments)
}
