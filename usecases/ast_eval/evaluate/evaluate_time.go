package evaluate

import (
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
	"time"
)

type TimeFunctions struct {
	Function ast.Function
}

func NewTimeFunctions(f ast.Function) TimeFunctions {
	return TimeFunctions{
		Function: f,
	}
}

func (f TimeFunctions) Evaluate(arguments ast.Arguments) (any, error) {
	switch f.Function {
	case ast.FUNC_TIME_NOW:
		if err := verifyNumberOfArguments(f.Function, arguments.Args, 0); err != nil {
			return nil, err
		}

		return time.Now(), nil
	default:
		return nil, fmt.Errorf(
			"function %s not implemented: %w", f.Function.DebugString(), models.ErrRuntimeExpression,
		)
	}
}
