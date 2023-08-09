package evaluate

import (
	"fmt"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/utils"
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
	arr, err := utils.MapErrWithParam(arguments.Args, f.Function, adaptArgumentToBool)
	if err != nil {
		return nil, fmt.Errorf(
			"all argments of function %s must be booleans %w",
			f.Function.DebugString(), ErrRuntimeExpression,
		)
	}
	return f.booleanArithmeticEval(arr)
}

func (f BooleanArithmetic) booleanArithmeticEval(arr []bool) (bool, error) {
	switch f.Function {
	case ast.FUNC_AND:
		result := true
		for _, val := range arr {
			result = result && val
		}
		return result, nil
	case ast.FUNC_OR:
		for _, val := range arr {
			if val {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, fmt.Errorf("Arithmetic does not support %s function", f.Function.DebugString())
	}

}
