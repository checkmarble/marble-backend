package evaluate

import (
	"fmt"
	"marble/marble-backend/models/ast"
)

type Not struct {
	Function ast.Function
}

func (f Not) Evaluate(arguments ast.Arguments) (any, error) {

	numberOfOperands := len(arguments.Args)
	if numberOfOperands != 1 {
		return false, fmt.Errorf("function %s expects 1 operand, got %d", f.Function.DebugString(), numberOfOperands)
	}

	firstArgument := arguments.Args[0]

	v, ok := firstArgument.(bool)
	if !ok {
		return false, fmt.Errorf("function %s only accept bool: %v is not a bool", f.Function.DebugString(), firstArgument)
	}

	return !v, nil
}
