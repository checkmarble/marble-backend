package evaluate

import (
	"fmt"
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
)

type ArithmeticDivide struct {
}

func (f ArithmeticDivide) Evaluate(arguments ast.Arguments) (any, error) {

	leftAny, rightAny, err := leftAndRight(ast.FUNC_DIVIDE, arguments.Args)
	if err != nil {
		return nil, err
	}

	// try to promote to float64
	if left, right, err := adaptLeftAndRight(ast.FUNC_DIVIDE, leftAny, rightAny, promoteArgumentToFloat64); err == nil {

		if right == 0.0 {
			return 0.0, models.DivisionByZeroError
		}

		return left / right, nil

	}

	return nil, fmt.Errorf(
		"all argments of function %s must be float64 %w",
		ast.FUNC_DIVIDE.DebugString(), models.ErrRuntimeExpression,
	)
}
