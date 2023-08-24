package evaluate

import (
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/utils"
	"time"
)

func promoteArgumentToInt64(argument any) (int64, error) {
	result, err := ToInt64(argument)
	if err != nil {
		return 0, fmt.Errorf("can't promote argument %v to int64 %w %w",
			argument, err, ast.ErrArgumentMustBeInt,
		)
	}
	return result, nil
}

func promoteArgumentToFloat64(argument any) (float64, error) {
	result, err := ToFloat64(argument)
	if err != nil {
		return 0, fmt.Errorf(
			"can't promote argument %v to float64 %w %w",
			argument,
			err,
			ast.ErrArgumentMustBeIntOrFloat,
		)

	}
	return result, nil
}

func adaptArgumentToString(argument any) (string, error) {
	if result, ok := argument.(string); ok {
		return result, nil
	}
	return "", fmt.Errorf(
		"can't promote argument %v to string %w",
		argument,
		ast.ErrArgumentMustBeString,
	)
}

func adaptArgumentToTime(argument any) (time.Time, error) {
	if result, ok := argument.(time.Time); ok {
		return result, nil
	}
	return time.Time{}, fmt.Errorf(
		"can't promote argument %v to time %w",
		argument,
		ast.ErrArgumentCantBeTime,
	)
}

func adaptArgumentToDuration(argument any) (time.Duration, error) {
	if result, ok := argument.(time.Duration); ok {
		return result, nil
	}

	if str, ok := argument.(string); ok {
		if result, err := time.ParseDuration(str); err == nil {
			return result, nil
		}
	}

	if result, err := ToInt64(argument); err == nil {
		return time.Duration(result), nil
	}

	return 0, fmt.Errorf(
		"can't promote argument %v to duration %w",
		argument,
		ast.ErrArgumentCantBeConvertedToDuration,
	)
}

func adaptArgumentToListOfThings[T any](argument any) ([]T, error) {
	var zero T

	if things, ok := argument.([]T); ok {
		return things, nil
	}

	if list, ok := argument.([]any); ok {
		return utils.MapErr(list, func(item any) (T, error) {
			i, ok := item.(T)
			if !ok {
				return zero, fmt.Errorf("Couldn't cast argument to %T", zero)
			}
			return i, nil
		})
	}

	return nil, fmt.Errorf(
		"can't promote argument %v to []%T %w",
		argument,
		zero,
		ast.ErrArgumentMustBeBool,
	)
}

func adaptArgumentToListOfStrings(argument any) ([]string, error) {
	return adaptArgumentToListOfThings[string](argument)
}


func adaptArgumentToBool(argument any) (bool, error) {

	if value, ok := argument.(bool); ok {
		return value, nil
	}

	return false, fmt.Errorf(
		"can't promote argument %v to bool %w",
		argument,
		ast.ErrArgumentMustBeBool,
	)
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
		return nil, fmt.Errorf("datatype %s not supported", datatype)
	}
}
