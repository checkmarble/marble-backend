package evaluate

import (
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
)

type TimeArithmetic struct {
	Function ast.Function
}

func NewTimeArithmetic(f ast.Function) TimeArithmetic {
	return TimeArithmetic{
		Function: f,
	}
}

func (f TimeArithmetic) Evaluate(arguments ast.Arguments) (any, error) {
	switch f.Function {
	case ast.FUNC_ADD_TIME:
		if err := verifyNumberOfArguments(f.Function, arguments.Args, 2); err != nil {
			return nil, err
		}

		t, err := adaptArgumentToTime(f.Function, arguments.Args[0])
		if err != nil {
			return nil, fmt.Errorf("TimeArithmetic (FUNC_ADD_TIME): error reading time from payload: %w", err)
		}
		d, err := adaptArgumentToDuration(f.Function, arguments.Args[1])
		if err != nil {
			return nil, fmt.Errorf("TimeArithmetic (FUNC_ADD_TIME): error reading duration from payload: %w", err)
		}
		return t.Add(d), nil
	default:
		return nil, fmt.Errorf(
			"function %s not implemented: %w", f.Function.DebugString(), models.ErrRuntimeExpression,
		)
	}
}
