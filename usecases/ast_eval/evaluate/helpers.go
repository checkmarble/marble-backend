package evaluate

import (
	"fmt"
	"marble/marble-backend/models/ast"
)

func leftAndRight(args []any) (any, any, error) {

	if err := verifyNumberOfArguments(args, 2); err != nil {
		return nil, nil, err
	}
	return args[0], args[1], nil
}

type FuncAdaptArgument[T any] func(argument any) (T, error)

func adaptLeftAndRight[T any](left any, right any, adapt FuncAdaptArgument[T]) (T, T, []error) {

	leftT, errLeft := adapt(left)
	rightT, errRight := adapt(right)

	errs := MakeAdaptedArgsErrors([]error{errLeft, errRight})
	if len(errs) > 0 {
		var zero T
		return zero, zero, errs
	}

	return leftT, rightT, errs
}

func verifyNumberOfArguments(args []any, requiredNumberOfArguments int) error {
	numberOfOperands := len(args)
	if numberOfOperands != requiredNumberOfArguments {
		return fmt.Errorf(
			"expects %d operands, got %d %w",
			requiredNumberOfArguments, numberOfOperands, ast.ErrWrongNumberOfArgument,
		)
	}
	return nil
}

func AdaptArguments[T any](args []any, adapter func(any) (T, error)) ([]T, []error) {

	values := make([]T, 0, len(args))
	errs := make([]error, 0, len(args))

	for _, arg := range args {
		value, err := adapter(arg)
		if err != nil {
			// TODO: make an error with child index
			errs = append(errs, err)
		}
		values = append(values, value)
	}

	return values, errs
}

func AdaptNamedArgument[T any](namedArgs map[string]any, name string, adapter func(any) (T, error)) (T, error) {
	value, ok := namedArgs[name]
	if !ok {
		// TODO: make an error with child name
		var zero T
		return zero, fmt.Errorf("missing named argument %s not found %w", name, ast.ErrMissingNamedArgument)
	}

	// TODO: make an error with child name
	return adapter(value)
}

func MakeAdaptedNamedArgsErrors(errs ...error) []error {
	result := make([]error, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			result = append(result, err)
		}
	}
	return result
}

func MakeAdaptedArgsErrors(errs []error) []error {

	result := make([]error, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			// TODO: make an error with child index
			result = append(result, err)
		}
	}
	return result
}

func filterNilErrors(errs []error) []error {
	result := make([]error, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			result = append(result, err)
		}
	}
	return result
}

func MakeEvaluateResult(result any, errs ...error) (any, []error) {
	return result, filterNilErrors(errs)
}

func MakeEvaluateError(err error) (any, []error) {
	return nil, []error{err}
}
