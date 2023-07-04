package evaluate

import (
	"fmt"
	"marble/marble-backend/models/ast"
)

type StringComparison struct {
	Function ast.Function
}

func NewStringComparison(f ast.Function) StringComparison {
	return StringComparison{
		Function: f,
	}
}

func (f StringComparison) Evaluate(arguments ast.Arguments) (any, error) {
	// promote to float64
	firstArgument := arguments.Args[0]
	firstString, ok := firstArgument.(string)
	if !ok {
		return nil, fmt.Errorf("first argument is not a string %w", ErrRuntimeExpression)
	}
	secondArgument := arguments.Args[1]
	secondString, ok := secondArgument.(string)
	if !ok {
		return nil, fmt.Errorf("second argument is not a string %w", ErrRuntimeExpression)
	}
	return firstString == secondString ,nil
}