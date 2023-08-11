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
		if f.Function == ast.FUNC_DIVIDE && right == 0 {
			return nil, fmt.Errorf("Division by zero, %w", models.DivisionByZeroError)
		}
		return arithmeticEval(f.Function, left, right)
	}

	// try to promote to float64
	if left, right, err := adaptLeftAndRight(f.Function, leftAny, rightAny, promoteArgumentToFloat64); err == nil {
		if f.Function == ast.FUNC_DIVIDE && right == 0.0 {
			return nil, fmt.Errorf("Division by zero, %w", models.DivisionByZeroError)
		}
		return arithmeticEval(f.Function, left, right)
	}

	return nil, fmt.Errorf(
		"all argments of function %s must be int64 or float64 %w",
		f.Function.DebugString(), ErrRuntimeExpression,
	)
}

func arithmeticEval[T int64 | float64](function ast.Function, l, r T) (T, error) {

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
