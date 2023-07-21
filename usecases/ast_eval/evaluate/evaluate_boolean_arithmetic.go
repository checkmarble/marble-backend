package evaluate

import (
	"fmt"
	"marble/marble-backend/models/ast"
)

type BooleanArithmetic struct {
	Function ast.Function
}

func NewBooleanArithmetic(f ast.Function) BooleanArithmetic {
	return BooleanArithmetic{
		Function: f,
	}
}

func (f BooleanArithmetic) Evaluate(arguments ast.Arguments) (any, error) {

	leftAny, rightAny, err := leftAndRight(f.Function, arguments.Args)
	if err != nil {
		return nil, err
	}
	// promote to bnool
	if left, right, err := adaptLeftAndRight(f.Function, leftAny, rightAny, adaptArgumentToBool); err == nil {
		return f.booleanArithmeticEval(left, right)
	}

	return nil, fmt.Errorf(
		"all argments of function %s must be booleans %w",
		f.Function.DebugString(), ErrRuntimeExpression,
	)
}

func (f BooleanArithmetic) booleanArithmeticEval(l, r bool) (bool, error) {

	switch f.Function {
	case ast.FUNC_AND:
		return l && r, nil
	case ast.FUNC_OR:
		return l || r, nil
	default:
		return false, fmt.Errorf("Arithmetic does not support %s function", f.Function.DebugString())
	}

}
