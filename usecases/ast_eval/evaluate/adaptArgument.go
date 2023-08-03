package evaluate

import (
	"fmt"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/utils"
	"time"
)

func promoteArgumentToInt64(function ast.Function, argument any) (int64, error) {
	result, err := ToInt64(argument)
	if err != nil {
		return 0, fmt.Errorf("function %s can't promote argument %v to int64 %w %w",
			function.DebugString(), argument, err, ErrRuntimeExpression,
		)
	}
	return result, nil
}

func promoteArgumentToFloat64(function ast.Function, argument any) (float64, error) {
	result, err := ToFloat64(argument)
	if err != nil {
		return 0, fmt.Errorf(
			"function %s can't promote argument %v to float64 %w %w",
			function.DebugString(),
			argument,
			err,
			ErrRuntimeExpression,
		)

	}
	return result, nil
}

func adaptArgumentToString(function ast.Function, argument any) (string, error) {
	if result, ok := argument.(string); ok {
		return result, nil
	}
	return "", fmt.Errorf(
		"function %s can't promote argument %v to string %w",
		function.DebugString(),
		argument,
		ErrRuntimeExpression,
	)
}

func adaptArgumentToTime(function ast.Function, argument any) (time.Time, error) {
	if result, ok := argument.(time.Time); ok {
		return result, nil
	}
	return time.Time{}, fmt.Errorf(
		"function %s can't promote argument %v to time %w",
		function.DebugString(),
		argument,
		ErrRuntimeExpression,
	)
}

func adaptArgumentToDuration(function ast.Function, argument any) (time.Duration, error) {
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
		"function %s can't promote argument %v to duration %w",
		function.DebugString(),
		argument,
		ErrRuntimeExpression,
	)
}

func adaptArgumentToListOfStrings(function ast.Function, argument any) ([]string, error) {

	if strings, ok := argument.([]string); ok {
		return strings, nil
	}

	if list, ok := argument.([]any); ok {
		return utils.MapErr(list, func(item any) (string, error) {
			return adaptArgumentToString(function, item)
		})
	}

	return nil, fmt.Errorf(
		"function %s can't promote argument %v to []string %w",
		function.DebugString(),
		argument,
		ErrRuntimeExpression,
	)
}

func adaptArgumentToBool(function ast.Function, argument any) (bool, error) {

	if value, ok := argument.(bool); ok {
		return value, nil
	}

	return false, fmt.Errorf(
		"function %s can't promote argument %v to bool %w",
		function.DebugString(),
		argument,
		ErrRuntimeExpression,
	)
}
