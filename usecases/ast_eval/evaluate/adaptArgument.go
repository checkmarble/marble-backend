package evaluate

import (
	"fmt"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/utils"
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
