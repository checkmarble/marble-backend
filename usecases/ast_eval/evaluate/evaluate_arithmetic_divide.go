package evaluate

import (
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
)

type ArithmeticDivide struct {
}

func (f ArithmeticDivide) Evaluate(arguments ast.Arguments) (any, []error) {

	leftAny, rightAny, err := leftAndRight(arguments.Args)
	if err != nil {
		return MakeEvaluateError(err)
	}

	// promote to float64
	left, right, errs := adaptLeftAndRight(leftAny, rightAny, promoteArgumentToFloat64)
	if len(errs) > 0 {
		return nil, errs
	}

	if right == 0.0 {
		return MakeEvaluateError(models.DivisionByZeroError)
	}

	return MakeEvaluateResult(left / right)
}
