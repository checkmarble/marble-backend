package evaluate

import (
	"context"

	"github.com/checkmarble/marble-backend/models/ast"
)

type IsEmpty struct{}

func (f IsEmpty) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	if err := verifyNumberOfArguments(arguments.Args, 1); err != nil {
		return MakeEvaluateError(err)
	}
	if arguments.Args[0] == nil || arguments.Args[0] == "" {
		return true, nil
	}

	return false, nil
}

type IsNotEmpty struct{}

func (f IsNotEmpty) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	if err := verifyNumberOfArguments(arguments.Args, 1); err != nil {
		return MakeEvaluateError(err)
	}
	if arguments.Args[0] == nil || arguments.Args[0] == "" {
		return false, nil
	}

	return true, nil
}
