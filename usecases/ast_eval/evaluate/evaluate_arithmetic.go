package evaluate

import (
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
)

type Arithmetic struct {
	Function ast.Function
}

func NewArithmetic(f ast.Function) Arithmetic {
	return Arithmetic{
		Function: f,
	}
}

func (f Arithmetic) Evaluate(arguments ast.Arguments) (any, error) {

	leftAny, rightAny, err := leftAndRight(f.Function, arguments.Args)
	if err != nil {
		return nil, err
	}

	// try to promote to int64
	if left, right, err := adaptLeftAndRight(f.Function, leftAny, rightAny, promoteArgumentToInt64); err == nil {
		return arithmeticEval(f.Function, left, right)
	}

	// try to promote to float64
	if left, right, err := adaptLeftAndRight(f.Function, leftAny, rightAny, promoteArgumentToFloat64); err == nil {
		return arithmeticEval(f.Function, left, right)
	}

	return nil, fmt.Errorf(
		"all argments of function %s must be int64 or float64 %w",
		f.Function.DebugString(), models.ErrRuntimeExpression,
	)
}

func arithmeticEval[T int64 | float64](function ast.Function, l, r T) (T, error) {

	var zero T
	if function == ast.FUNC_DIVIDE && r == zero {
		return zero, models.DivisionByZeroError
	}

	switch function {
	case ast.FUNC_ADD:
		return l + r, nil
	case ast.FUNC_SUBTRACT:
		return l - r, nil
	case ast.FUNC_MULTIPLY:
		return l * r, nil
	case ast.FUNC_DIVIDE:
		return l / r, nil
	default:
		return 0, fmt.Errorf("Arithmetic does not support %s function", function.DebugString())
	}
}
