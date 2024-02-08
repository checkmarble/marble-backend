package ast_eval

import (
	"context"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
)

func EvaluateAst(ctx context.Context, environment AstEvaluationEnvironment, node ast.Node) (ast.NodeEvaluation, bool) {
	// Early exit for constant, because it should have no children.
	if node.Function == ast.FUNC_CONSTANT {
		return ast.NodeEvaluation{
			ReturnValue: node.Constant,
			Errors:      []error{},
		}, true
	}

	childEvaluationFail := false

	evalChild := func(child ast.Node) ast.NodeEvaluation {
		childEval, ok := EvaluateAst(ctx, environment, child)
		if !ok {
			childEvaluationFail = true
		}
		return childEval
	}

	// eval each child
	evaluation := ast.NodeEvaluation{
		Children:      pure_utils.Map(node.Children, evalChild),
		NamedChildren: pure_utils.MapValues(node.NamedChildren, evalChild),
	}

	if childEvaluationFail {
		// an error occurred in at least one of the children. Stop the evaluation.

		// the frontend expects an ErrUndefinedFunction error to be present even when no evaluation happened.
		if node.Function == ast.FUNC_UNDEFINED {
			evaluation.Errors = append(evaluation.Errors, ast.ErrUndefinedFunction)
		}

		return evaluation, false
	}

	getReturnValue := func(e ast.NodeEvaluation) any { return e.ReturnValue }
	arguments := ast.Arguments{
		Args:      pure_utils.Map(evaluation.Children, getReturnValue),
		NamedArgs: pure_utils.MapValues(evaluation.NamedChildren, getReturnValue),
	}

	evaluator, err := environment.GetEvaluator(node.Function)
	if err != nil {
		evaluation.Errors = append(evaluation.Errors, err)
		return evaluation, false
	}

	evaluation.ReturnValue, evaluation.Errors = evaluator.Evaluate(ctx, arguments)

	if evaluation.Errors == nil {
		// Assign an empty array to indicate that the evaluation occured.
		// The evaluator is not supposed to return a nil array of errors, but let's be nice.
		evaluation.Errors = []error{}
	}

	ok := len(evaluation.Errors) == 0

	if !ok {
		// The evaluator is supposed to return nil ReturnValue when an error is present.
		evaluation.ReturnValue = nil
	}

	return evaluation, ok
}
