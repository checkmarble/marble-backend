package evaluate

import (
	"fmt"
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
	// try to promote to int64
	if operands, err := promoteOperandsToInt64(arguments.Args, f.Function); err == nil {
		return arithmeticEval(f.Function, operands)
	}

	// promote to float64
	if operands, err := promoteOperandsToFloat64(arguments.Args, f.Function); err == nil {
		return arithmeticEval(f.Function, operands)
	}

	return nil, fmt.Errorf("arithmeticFunction %s support int64 and float64", f.Function.DebugString())
}

func arithmeticEval[T int64 | float64](function ast.Function, operands []T) (T, error) {
	l, r, err := leftAndRight(operands)
	if err != nil {
		return 0, err
	}

	if function == ast.FUNC_ADD {
		return l + r, nil
	}
	if function == ast.FUNC_SUBTRACT {
		return l - r, nil
	}
	if function == ast.FUNC_MULTIPLY {
		return l * r, nil
	}
	if function == ast.FUNC_DIVIDE {
		return l / r, nil
	}

	return 0, fmt.Errorf("Arithmetic does not support %s function", function.DebugString())
}
