package evaluate

import (
	"fmt"

	"github.com/checkmarble/marble-backend/models/ast"
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

	if left, right, errs := adaptLeftAndRight(leftAny, rightAny, adaptArgumentToTime); len(errs) == 0 {
		return MakeEvaluateResult(left.Equal(right))
	}

	return MakeEvaluateError(fmt.Errorf("all arguments must be string, boolean, time, int or float %w", ast.ErrArgumentInvalidType))
}
