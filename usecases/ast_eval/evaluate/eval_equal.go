package evaluate

import (
	"fmt"
	"marble/marble-backend/models/ast"
)

type Equal struct{}

func (f Equal) Evaluate(arguments ast.Arguments) (any, []error) {

	leftAny, rightAny, err := leftAndRight(arguments.Args)
	if err != nil {
		return MakeEvaluateError(err)
	}

	if left, right, errs := adaptLeftAndRight(leftAny, rightAny, adaptArgumentToString); len(errs) == 0 {
		return MakeEvaluateResult(left == right)
	}

	if left, right, errs := adaptLeftAndRight(leftAny, rightAny, adaptArgumentToBool); len(errs) == 0 {
		return MakeEvaluateResult(left == right)
	}

	if left, right, errs := adaptLeftAndRight(leftAny, rightAny, promoteArgumentToInt64); len(errs) == 0 {
		return MakeEvaluateResult(left == right)
	}

	if left, right, errs := adaptLeftAndRight(leftAny, rightAny, promoteArgumentToFloat64); len(errs) == 0 {
		return MakeEvaluateResult(left == right)
	}

	return MakeEvaluateError(fmt.Errorf("all argments must be string, boolean, int or float"))
}
