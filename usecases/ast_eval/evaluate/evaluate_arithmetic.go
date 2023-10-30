package evaluate

import (
	"fmt"

	"github.com/checkmarble/marble-backend/models/ast"
)

type Arithmetic struct {
	Function ast.Function
}

func NewArithmetic(f ast.Function) Arithmetic {
	return Arithmetic{
		Function: f,
	}
}

func (f Arithmetic) Evaluate(arguments ast.Arguments) (any, []error) {

	leftAny, rightAny, err := leftAndRight(arguments.Args)
	if err != nil {
		return MakeEvaluateError(err)
	}

	// try to promote to int64
	if left, right, errs := adaptLeftAndRight(leftAny, rightAny, promoteArgumentToInt64); len(errs) == 0 {
		return MakeEvaluateResult(arithmeticEval(f.Function, left, right))
	}

	// try to promote to float64
	if left, right, errs := adaptLeftAndRight(leftAny, rightAny, promoteArgumentToFloat64); len(errs) == 0 {
		return MakeEvaluateResult(arithmeticEval(f.Function, left, right))
	}

	return MakeEvaluateError(ast.ErrArgumentMustBeIntOrFloat)
}

func arithmeticEval[T int64 | float64](function ast.Function, l, r T) (T, error) {

	switch function {
	case ast.FUNC_ADD:
		return l + r, nil
	case ast.FUNC_SUBTRACT:
		return l - r, nil
	case ast.FUNC_MULTIPLY:
		return l * r, nil
	default:
		return 0, fmt.Errorf("Arithmetic does not support %s function", function.DebugString())
	}
}
