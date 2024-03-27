package evaluate

import (
	"fmt"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/pure_utils/duration"
)

func argumentNotNil(argument any) error {
	if argument == nil {
		return ast.ErrArgumentRequired
	}
	return nil
}

func promoteArgumentToInt64(argument any) (int64, error) {
	if err := argumentNotNil(argument); err != nil {
		return 0, err
	}

	result, err := ToInt64(argument)
	if err != nil {
		return 0, errors.Join(
			errors.Wrap(ast.ErrArgumentMustBeInt,
				fmt.Sprintf("can't promote argument %v to int64", argument)),
			err,
		)
	}
	return result, nil
}

func promoteArgumentToFloat64(argument any) (float64, error) {
	if err := argumentNotNil(argument); err != nil {
		return 0, err
	}

	result, err := ToFloat64(argument)
	if err != nil {
		return 0, errors.Join(
			errors.Wrap(ast.ErrArgumentMustBeIntOrFloat,
				fmt.Sprintf("can't promote argument %v to float64", argument)),
			err,
		)
	}
	return result, nil
}

func adaptArgumentToString(argument any) (string, error) {
	if err := argumentNotNil(argument); err != nil {
		return "", err
	}

	if result, ok := argument.(string); ok {
		return pure_utils.Normalize(result), nil
	}

	return "", errors.Wrap(
		ast.ErrArgumentMustBeString,
		fmt.Sprintf("can't promote argument %v to string", argument),
	)
}

func adaptArgumentToTime(argument any) (time.Time, error) {
	if err := argumentNotNil(argument); err != nil {
		return time.Time{}, err
	}

	if result, ok := argument.(time.Time); ok {
		return result, nil
	}
	return time.Time{}, errors.Wrap(ast.ErrArgumentMustBeTime,
		fmt.Sprintf("can't promote argument %v to time", argument))
}

func adaptArgumentToDuration(argument any) (time.Duration, error) {
	if err := argumentNotNil(argument); err != nil {
		return 0, err
	}

	if result, ok := argument.(time.Duration); ok {
		return result, nil
	}

	if str, ok := argument.(string); ok {
		if result, err := duration.Parse(str); err == nil {
			return result.ToTimeDuration(), nil
		}

		if result, err := time.ParseDuration(str); err == nil {
			return result, nil
		}
	}

	if result, err := ToInt64(argument); err == nil {
		return time.Duration(result), nil
	}

	return 0, errors.Wrap(ast.ErrArgumentCantBeConvertedToDuration,
		fmt.Sprintf("can't promote argument %v to duration", argument))
}

func adaptArgumentToListOfThings[T any](argument any) ([]T, error) {
	var zero T

	if things, ok := argument.([]T); ok {
		return things, nil
	}

	if list, ok := argument.([]any); ok {
		return pure_utils.MapErr(list, func(item any) (T, error) {
			i, ok := item.(T)
			if !ok {
				return zero, errors.New(fmt.Sprintf("couldn't cast argument to %T", zero))
			}
			return i, nil
		})
	}

	if err := argumentNotNil(argument); err != nil {
		return nil, err
	}

	return nil, errors.Wrap(ast.ErrArgumentMustBeList,
		fmt.Sprintf("can't promote argument %v to []%T", argument, zero))
}

func adaptArgumentToListOfStrings(argument any) ([]string, error) {
	arr, err := adaptArgumentToListOfThings[string](argument)
	if err != nil {
		return nil, err
	}
	return pure_utils.Map(arr, pure_utils.Normalize), nil
}

func adaptArgumentToBool(argument any) (bool, error) {
	if err := argumentNotNil(argument); err != nil {
		return false, err
	}

	if value, ok := argument.(bool); ok {
		return value, nil
	}

	return false, errors.Wrap(ast.ErrArgumentMustBeBool,
		fmt.Sprintf("can't promote argument %v to bool", argument))
}

func promoteArgumentToDataType(argument any, datatype models.DataType) (any, error) {
	switch datatype {
	case models.Bool:
		return adaptArgumentToBool(argument)
	case models.Int:
		return promoteArgumentToInt64(argument)
	case models.Float:
		return promoteArgumentToFloat64(argument)
	case models.String:
		return adaptArgumentToString(argument)
	case models.Timestamp:
		return adaptArgumentToTime(argument)
	default:
		return nil, errors.New(fmt.Sprintf("datatype %s not supported", datatype))
	}
}
