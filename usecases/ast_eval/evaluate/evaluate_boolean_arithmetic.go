package evaluate

import (
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/utils"
)

type BooleanArithmetic struct {
	Function ast.Function
}

func (f BooleanArithmetic) Evaluate(arguments ast.Arguments) (any, error) {

	numberOfOperands := len(arguments.Args)
	if numberOfOperands < 1 {
		return false, fmt.Errorf(
			"function %s expects at least %d operands, got %d %w",
			f.Function.DebugString(), 2, numberOfOperands, ast.ErrWrongNumberOfArgument,
		)
	}

	args, err := utils.MapErr(arguments.Args, func(arg any) (bool, error) {
		return adaptArgumentToBool(f.Function, arg)
	})

	if err != nil {
		return nil, fmt.Errorf(
			"all argments of function %s must be booleans %w",
			f.Function.DebugString(), models.ErrRuntimeExpression,
		)
	}
	return f.booleanArithmeticEval(args)
}

func (f BooleanArithmetic) booleanArithmeticEval(args []bool) (bool, error) {

	r := args[0]
	numberOfOperands := len(args)
	for i := 1; i < numberOfOperands; i++ {
		switch f.Function {
		case ast.FUNC_AND:
			r = r && args[i]
		case ast.FUNC_OR:
			r = r || args[i]
		default:
			return false, fmt.Errorf("Arithmetic does not support %s function", f.Function.DebugString())
		}
	}
	return r, nil
}
