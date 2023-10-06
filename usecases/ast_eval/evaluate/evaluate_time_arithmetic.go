package evaluate

import (
	"fmt"

	"github.com/checkmarble/marble-backend/models/ast"
)

type TimeArithmetic struct {
	Function ast.Function
}

func NewTimeArithmetic(f ast.Function) TimeArithmetic {
	return TimeArithmetic{
		Function: f,
	}
}

func (f TimeArithmetic) Evaluate(arguments ast.Arguments) (any, []error) {
	switch f.Function {
	case ast.FUNC_TIME_ADD:
		if err := verifyNumberOfArguments(arguments.Args, 2); err != nil {
			return MakeEvaluateError(err)
		}

		t, leftErr := adaptArgumentToTime(arguments.Args[0])
		d, rightErr := adaptArgumentToDuration(arguments.Args[1])
		errs := MakeAdaptedArgsErrors([]error{leftErr, rightErr})
		if len(errs) > 0 {
			return nil, errs
		}

		return t.Add(d), nil
	default:
		return MakeEvaluateError(fmt.Errorf(
			"function %s not implemented", f.Function.DebugString(),
		))
	}
}
