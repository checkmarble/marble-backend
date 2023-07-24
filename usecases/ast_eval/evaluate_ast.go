package ast_eval

import (
	"marble/marble-backend/models/ast"
	"marble/marble-backend/utils"
)

func EvaluateAst(environment AstEvaluationEnvironment, node ast.Node) ast.NodeEvaluation {

	// Early exit for constant, because it should have no children.
	if node.Function == ast.FUNC_CONSTANT {
		return ast.NodeEvaluation{
			ReturnValue: node.Constant,
		}
	}

	childEvaluationFail := false

	evalChild := func(child ast.Node) ast.NodeEvaluation {
		childEval := EvaluateAst(environment, child)
		if childEval.ReturnValue == nil {
			childEvaluationFail = true
		}
		return childEval
	}

	// eval each child
	evaluation := ast.NodeEvaluation{
		Children:      utils.Map(node.Children, evalChild),
		NamedChildren: utils.MapMap(node.NamedChildren, evalChild),
	}

	if childEvaluationFail {
		// an error occured in at least one of the children. Stop the evaluation.
		return evaluation
	}

	getReturnValue := func(e ast.NodeEvaluation) any { return e.ReturnValue }
	arguments := ast.Arguments{
		Args:      utils.Map(evaluation.Children, getReturnValue),
		NamedArgs: utils.MapMap(evaluation.NamedChildren, getReturnValue),
	}

	evaluator, err := environment.GetEvaluator(node.Function)
	if err != nil {
		evaluation.EvaluationError = err
		return evaluation
	}

	evaluation.ReturnValue, evaluation.EvaluationError = evaluator.Evaluate(arguments)
	return evaluation
}
