package evaluate

import (
	"fmt"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/utils"
)

func promoteOperandsToInt64(operands []any, function ast.Function) ([]int64, error) {
	result, err := utils.MapErr(operands, ToInt64)
	if err != nil {
		return nil, fmt.Errorf("function %s can't promote arguments to int64 %w", function.DebugString(), err)
	}
	return result, nil
}

func promoteOperandsToFloat64(operands []any, function ast.Function) ([]float64, error) {
	result, err := utils.MapErr(operands, ToFloat64)
	if err != nil {
		return nil, fmt.Errorf("function %s can't promote arguments to float64 %w", function.DebugString(), err)
	}
	return result, nil
}

func leftAndRight[T any](args []T) (T, T, error) {

	numberOfOperands := len(args)
	if numberOfOperands != 2 {
		var zero T
		return zero, zero, fmt.Errorf("expect 2 operands, got %d", numberOfOperands)
	}
	return args[0], args[1], nil
}
