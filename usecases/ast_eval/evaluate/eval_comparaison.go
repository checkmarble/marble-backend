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

func (f Comparison) Evaluate(arguments ast.Arguments) (any, error) {

	leftAny, rightAny, err := leftAndRight(f.Function, arguments.Args)
	if err != nil {
		return nil, err
	}

	left, right, err := adaptLeftAndRight(f.Function, leftAny, rightAny, promoteArgumentToFloat64)
	if err != nil {
		return nil, err
	}

	return f.comparisonFunction(left, right)
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
