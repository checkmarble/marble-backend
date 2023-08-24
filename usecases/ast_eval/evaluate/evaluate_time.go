package evaluate

import (
	"fmt"
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

func (f TimeFunctions) Evaluate(arguments ast.Arguments) (any, []error) {
	switch f.Function {
	case ast.FUNC_TIME_NOW:
		if err := verifyNumberOfArguments(arguments.Args, 0); err != nil {
			return MakeEvaluateError(err)
		}
		return time.Now(), nil

	case ast.FUNC_PARSE_TIME:
		if err := verifyNumberOfArguments(arguments.Args, 1); err != nil {
			return MakeEvaluateError(err)
		}

		timeString, err := adaptArgumentToString(arguments.Args[0])
		if err != nil {
			return MakeEvaluateError(err)
		}

		return MakeEvaluateResult(time.Parse(time.RFC3339, timeString))
	default:
		return MakeEvaluateError(fmt.Errorf(
			"function %s not implemented", f.Function.DebugString(),
		))
	}
}
