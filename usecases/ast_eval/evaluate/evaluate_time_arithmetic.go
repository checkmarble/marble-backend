package evaluate

import (
	"context"
	"fmt"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models/ast"
)

type TimeArithmetic struct {
	Function ast.Function
}

const (
	PlusSign  = "+"
	MinusSign = "-"
)

func NewTimeArithmetic(f ast.Function) TimeArithmetic {
	return TimeArithmetic{
		Function: f,
	}
}

func (f TimeArithmetic) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
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
			return MakeEvaluateError(errors.Join(
				errors.Wrap(ast.NewNamedArgumentError("sign"), "sign is not a valid sign"),
				ast.ErrRuntimeExpression,
			))
		}

		if sign == MinusSign {
			return time.Add(-duration), nil
		}
		return time.Add(duration), nil
	default:
		return MakeEvaluateError(errors.New(fmt.Sprintf("function %s not implemented", f.Function.DebugString())))
	}
}
