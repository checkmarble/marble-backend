package evaluate

import (
	"fmt"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
)

type TimeArithmetic struct {
	Function ast.Function
}

const (
	PlusSign = "+"
	MinusSign = "-"
)

func NewTimeArithmetic(f ast.Function) TimeArithmetic {
	return TimeArithmetic{
		Function: f,
	}
}

func (f TimeArithmetic) Evaluate(arguments ast.Arguments) (any, []error) {
	switch f.Function {
	case ast.FUNC_TIME_ADD:
		time, timeErr := AdaptNamedArgument(arguments.NamedArgs, "timestampField", adaptArgumentToTime)
		duration, durationErr := AdaptNamedArgument(arguments.NamedArgs, "duration", adaptArgumentToDuration)
		sign, signErr := AdaptNamedArgument(arguments.NamedArgs, "sign", adaptArgumentToString)

		errs := filterNilErrors(timeErr, durationErr, signErr)
		if len(errs) > 0 {
			return nil, errs
		}

		if sign != PlusSign && sign != MinusSign {
			return MakeEvaluateError(fmt.Errorf("sign is not a valid sign %w %w", models.ErrRuntimeExpression, ast.NewNamedArgumentError("sign")))
		}

		if sign == MinusSign {
			return time.Add(-duration), nil
		}
		return time.Add(duration), nil
	default:
		return MakeEvaluateError(fmt.Errorf(
			"function %s not implemented", f.Function.DebugString(),
		))
	}
}
