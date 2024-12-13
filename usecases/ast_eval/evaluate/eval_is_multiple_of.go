package evaluate

import (
	"context"
	"math"

	"github.com/checkmarble/marble-backend/models/ast"
)

type IsMultipleOf struct{}

func (f IsMultipleOf) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	value, valueErr := AdaptNamedArgument(arguments.NamedArgs, "value", promoteArgumentToFloat64)
	divider, dividerErr := AdaptNamedArgument(arguments.NamedArgs, "divider", promoteArgumentToFloat64)

	errs := MakeAdaptedArgsErrors([]error{valueErr, dividerErr})
	if len(errs) > 0 {
		return nil, errs
	}

	dividerInt, ok := downcastToInt64(divider)
	if !ok {
		return MakeEvaluateError(ast.ErrArgumentMustBeInt)
	}

	if valueInt, ok := downcastToInt64(value); ok {
		return valueInt%dividerInt == 0, nil
	}

	return false, nil
}

func downcastToInt64(n float64) (int64, bool) {
	if n < math.MinInt64 || n > math.MaxInt64 {
		return 0, false
	}
	r := math.Round(n)
	if math.Abs(r-n) > 1e-8 {
		return 0, false
	}
	return int64(n), true
}
