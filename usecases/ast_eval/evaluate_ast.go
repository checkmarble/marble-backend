package ast_eval

import (
	"marble/marble-backend/models/ast"
	"marble/marble-backend/utils"
)

func EvaluateAst(environment AstEvaluationEnvironment, node ast.Node) (ast.NodeEvaluation, bool) {

	// Early exit for constant, because it should have no children.
	if node.Function == ast.FUNC_CONSTANT {
		return ast.NodeEvaluation{
			ReturnValue: node.Constant,
		}, true
	}

	childEvaluationFail := false

	evalChild := func(child ast.Node) ast.NodeEvaluation {
		childEval, ok := EvaluateAst(environment, child)
		if !ok {
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
		return evaluation, false
	}

	getReturnValue := func(e ast.NodeEvaluation) any { return e.ReturnValue }
	arguments := ast.Arguments{
		Args:      utils.Map(evaluation.Children, getReturnValue),
		NamedArgs: utils.MapMap(evaluation.NamedChildren, getReturnValue),
	}

	evaluator, err := environment.GetEvaluator(node.Function)
	if err != nil {
		evaluation.Errors = append(evaluation.Errors, err)
		return evaluation, false
	}

	evaluation.ReturnValue, evaluation.Errors = evaluator.Evaluate(arguments)

	if evaluation.Errors == nil {
		// Assign an empty array to indicate that the evaluation occured.
		// Operator is not supposed to return nil array of errors, but let's be nice.
		evaluation.Errors = make([]error, 0)
	}

	ok := len(evaluation.Errors) == 0

	if !ok {
		// Operator is supposed to return nil when an error is present.
		evaluation.ReturnValue = nil
	}

	return evaluation, ok
}
