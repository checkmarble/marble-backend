package evaluate

import (
	"errors"
	"fmt"
	"marble/marble-backend/models/ast"
)

var ErrRuntimeExpression = errors.New("expression runtime error")

func leftAndRight(function ast.Function, args []any) (any, any, error) {

	if err := verifyNumberOfArguments(function, args, 2); err != nil {
		return nil, nil, err
	}
	return args[0], args[1], nil
}

type FuncAdaptArgument[T any] func(function ast.Function, argument any) (T, error)

func adaptLeftAndRight[T any](function ast.Function, left any, right any, adapt FuncAdaptArgument[T]) (T, T, error) {

	leftT, errLeft := adapt(function, left)
	rightT, errRight := adapt(function, right)

	if err := errors.Join(errLeft, errRight); err != nil {
		var zero T
		return zero, zero, err
	}

	return leftT, rightT, nil
}

func verifyNumberOfArguments(function ast.Function, args []any, requiredNumberOfArguments int) error {
	numberOfOperands := len(args)
	if numberOfOperands != requiredNumberOfArguments {
		return fmt.Errorf(
			"function %s expects %d operands, got %d %w",
			function.DebugString(), requiredNumberOfArguments, numberOfOperands, ErrRuntimeExpression,
		)
	}
	return nil
}
