package evaluate

import (
	"fmt"
	"marble/marble-backend/models/ast"
)

type StringListComparison struct {
	Function ast.Function
}

func NewStringListComparison(f ast.Function) StringListComparison {
	return StringListComparison{
		Function: f,
	}
}

func (f StringListComparison) Evaluate(arguments ast.Arguments) (any, error) {
	// promote to float64
	firstArgument := arguments.Args[0]
	str, ok := firstArgument.(string)
	if !ok {
		return nil, fmt.Errorf("first argument is not a string %w", ErrRuntimeExpression)
	}
	secondArgument := arguments.Args[1]
	list, ok := secondArgument.([]string)
	if !ok {
		return nil, fmt.Errorf("second argument is not a []string %w", ErrRuntimeExpression)
	}
	return f.comparisonFunction(str, list)
}

func (f StringListComparison) comparisonFunction(str string, list []string) (bool, error) {
	if f.Function == ast.FUNC_IS_IN_LIST {
		for _, v := range list {
			if v == str {
				return true, nil
			}
		}
		return false, nil
	} else if f.Function == ast.FUNC_IS_NOT_IN_LIST {
		for _, v := range list {
			if v == str {
				return false, nil
			}
			return true, nil
		}
	}
	return false, fmt.Errorf("StringListComparison does not support %s function", f.Function.DebugString())
}
