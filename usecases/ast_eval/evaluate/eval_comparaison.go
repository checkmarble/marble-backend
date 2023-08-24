package evaluate

import (
	"fmt"
	"marble/marble-backend/models/ast"
)

type Comparison struct {
	Function ast.Function
}

func NewComparison(f ast.Function) Comparison {
	return Comparison{
		Function: f,
	}
}

func (f Comparison) Evaluate(arguments ast.Arguments) (any, []error) {

	leftAny, rightAny, err := leftAndRight(arguments.Args)
	if err != nil {
		return MakeEvaluateError(err)
	}

	left, right, errs := adaptLeftAndRight(leftAny, rightAny, promoteArgumentToFloat64)
	if len(errs) != 0 {
		return nil, errs
	}

	return MakeEvaluateResult(f.comparisonFunction(left, right))
}

func (f Comparison) comparisonFunction(l, r float64) (bool, error) {

	switch f.Function {
	case ast.FUNC_GREATER:
		return l > r, nil
	case ast.FUNC_LESS:
		return l < r, nil
	default:
		return false, fmt.Errorf("Comparison does not support %s function", f.Function.DebugString())
	}
}
