package evaluate

import (
	"fmt"
	"marble/marble-backend/models/ast"
)

type Equal struct{}

func (f Equal) Evaluate(arguments ast.Arguments) (any, error) {

	function := ast.FUNC_EQUAL

	leftAny, rightAny, err := leftAndRight(function, arguments.Args)
	if err != nil {
		return nil, err
	}

	if left, right, err := adaptLeftAndRight(function, leftAny, rightAny, adaptArgumentToString); err == nil {
		return left == right, nil
	}

	if left, right, err := adaptLeftAndRight(function, leftAny, rightAny, adaptArgumentToBool); err == nil {
		return left == right, nil
	}

	if left, right, err := adaptLeftAndRight(function, leftAny, rightAny, promoteArgumentToInt64); err == nil {
		return left == right, nil
	}

	if left, right, err := adaptLeftAndRight(function, leftAny, rightAny, promoteArgumentToFloat64); err == nil {
		return left == right, nil
	}

	return nil, fmt.Errorf(
		"all argments of function %s must be string, boolean, int64 or float64 %w",
		function.DebugString(), ErrRuntimeExpression,
	)

}
