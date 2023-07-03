package evaluate

import (
	"fmt"
	"marble/marble-backend/models/ast"
)

type StringInList struct {
	Function ast.Function
}

func NewStringInList(f ast.Function) StringInList {
	return StringInList{
		Function: f,
	}
}

func (f StringInList) Evaluate(arguments ast.Arguments) (any, error) {
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

	if f.Function == ast.FUNC_IS_IN_LIST {
		return stringInList(str, list), nil
	} else if f.Function == ast.FUNC_IS_NOT_IN_LIST {
		return !stringInList(str, list), nil
	} else {
		return false, fmt.Errorf("StringInList does not support %s function", f.Function.DebugString())
	}
}

func stringInList(str string, list []string) bool {

	for _, v := range list {
		if v == str {
			return true
		}
	}
	return false
}
