package evaluate

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models/ast"
)

type BooleanArithmetic struct {
	Function ast.Function
}

func NewBooleanArithmetic(f ast.Function) BooleanArithmetic {
	return BooleanArithmetic{
		Function: f,
	}
}

func (f BooleanArithmetic) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {

	numberOfOperands := len(arguments.Args)
	if numberOfOperands < 1 {
		return MakeEvaluateError(fmt.Errorf(
			"expects at least 1 operand, got %d %w",
			numberOfOperands, ast.ErrWrongNumberOfArgument,
		))
	}

	values, errs := AdaptArguments(arguments.Args, adaptArgumentToBool)
	if len(errs) > 0 {
		return nil, errs
	}
	return MakeEvaluateResult(f.booleanArithmeticEval(values))
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
