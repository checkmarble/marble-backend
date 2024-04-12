package evaluate

import (
	"fmt"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
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
		return errors.Wrap(
			ast.ErrWrongNumberOfArgument,
			fmt.Sprintf("expects %d operands, got %d", requiredNumberOfArguments, numberOfOperands),
		)
	}
	return nil
}

func AdaptArguments[T any](args []any, adapter func(any) (T, error)) ([]T, []error) {
	values := make([]T, 0, len(args))
	errs := make([]error, 0, len(args))

	for argumentIndex, arg := range args {
		value, err := adapter(arg)
		if err != nil {
			errs = append(errs, errors.Join(err, ast.NewArgumentError(argumentIndex)))
		}
		values = append(values, value)
	}

	return values, errs
}

func AdaptNamedArgument[T any](namedArgs map[string]any, name string, adapter func(any) (T, error)) (T, error) {
	value, ok := namedArgs[name]
	if !ok {
		var zero T
		return zero, errors.Join(
			errors.Wrap(ast.NewNamedArgumentError(name),
				fmt.Sprintf("missing named argument %s not found", name)),
			ast.ErrMissingNamedArgument,
		)
	}

	result, err := adapter(value)
	if err != nil {
		err = errors.Join(err, ast.NewNamedArgumentError(name))
	}
	return result, err
}

func MakeAdaptedArgsErrors(errs []error) []error {
	result := make([]error, 0, len(errs))
	for argumentIndex, err := range errs {
		if err != nil {
			result = append(result, errors.Join(err, ast.NewArgumentError(argumentIndex)))
		}
	}
	return result
}

func MakeEvaluateResult(result any, errs ...error) (any, []error) {
	return result, filterNilErrors(errs...)
}

func MakeEvaluateError(err error) (any, []error) {
	return nil, []error{err}
}

func getFieldType(dataModel models.DataModel, tableName models.TableName, fieldName string) (models.DataType, error) {
	table, ok := dataModel.Tables[tableName]
	if !ok {
		return models.UnknownDataType, errors.Wrap(
			ast.ErrRuntimeExpression,
			fmt.Sprintf("couldn't find table %s in data model", tableName),
		)
	}

	field, ok := table.Fields[fieldName]
	if !ok {
		return models.UnknownDataType, errors.Wrap(
			ast.ErrRuntimeExpression,
			fmt.Sprintf("couldn't find field %s in table %s", fieldName, tableName),
		)
	}

	return field.DataType, nil
}

func filterNilErrors(errs ...error) []error {
	result := make([]error, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			result = append(result, err)
		}
	}
	return result
}
