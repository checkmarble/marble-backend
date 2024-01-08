package ast_eval

import (
	"fmt"
	"time"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/utils"
)

func EvaluateAst(environment AstEvaluationEnvironment, node ast.Node) (ast.NodeEvaluation, bool) {
	a := time.Now()
	// Early exit for constant, because it should have no children.
	if node.Function == ast.FUNC_CONSTANT {
		return ast.NodeEvaluation{
			ReturnValue: node.Constant,
			Errors:      []error{},
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

	b := time.Now()
	// eval each child
	evaluation := ast.NodeEvaluation{
		Children:      utils.Map(node.Children, evalChild),
		NamedChildren: utils.MapMap(node.NamedChildren, evalChild),
	}
	fmt.Println("EvaluateAst evalChild", time.Since(b))

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
		Args:      utils.Map(evaluation.Children, getReturnValue),
		NamedArgs: utils.MapMap(evaluation.NamedChildren, getReturnValue),
	}

	evaluator, err := environment.GetEvaluator(node.Function)
	if err != nil {
		evaluation.Errors = append(evaluation.Errors, err)
		return evaluation, false
	}

	b = time.Now()
	evaluation.ReturnValue, evaluation.Errors = evaluator.Evaluate(arguments)
	fmt.Println("EvaluateAst evaluator.Evaluate", node.Function.DebugString(), time.Since(b))

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

	fmt.Println("EvaluateAst", time.Since(a))
	return evaluation, ok
}
