package evaluate

import (
	"context"

	"github.com/checkmarble/marble-backend/models/ast"
)

type Case struct{}

func (Case) Evaluate(_ context.Context, arguments ast.Arguments) (any, []error) {
	if err := verifyNumberOfArguments(arguments.Args, 2); err != nil {
		return MakeEvaluateError(err)
	}

	var conditionErr error
	matched := false
	if arguments.Args[0] != nil {
		matched, conditionErr = adaptArgumentToBool(arguments.Args[0])
	}

	value, valueErr := promoteArgumentToFloat64(arguments.Args[1])
	errs := MakeAdaptedArgsErrors([]error{conditionErr, valueErr})
	if len(errs) > 0 {
		return nil, errs
	}

	return ast.SwitchCaseResult{
		Matched: matched,
		Value:   value,
	}, nil
}
