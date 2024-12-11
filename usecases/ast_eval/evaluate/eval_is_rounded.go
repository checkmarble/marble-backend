package evaluate

import (
	"context"
	"fmt"
	"math"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models/ast"
)

type IsRounded struct{}

func (f IsRounded) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	value, valueErr := AdaptNamedArgument(arguments.NamedArgs, "value", promoteArgumentToFloat64)
	threshold, thresholdErr := AdaptNamedArgument(arguments.NamedArgs, "threshold", promoteArgumentToFloat64)

	errs := MakeAdaptedArgsErrors([]error{valueErr, thresholdErr})
	if len(errs) > 0 {
		return nil, errs
	}

	thresholdInt, ok := downcastToInt64(threshold)
	if !ok || !isPowerOfTen(thresholdInt) {
		return MakeEvaluateError(errors.New(fmt.Sprintf(
			"Threshold argument must be a power of 10, got %v", thresholdInt)))
	}

	if valueInt, ok := downcastToInt64(value); ok {
		return valueInt%thresholdInt == 0, nil
	}

	return false, nil
}

func downcastToInt64(n float64) (int64, bool) {
	if n < math.MinInt64 || n > math.MaxInt64 {
		return 0, false
	}
	if n != float64(int64(n)) {
		return 0, false
	}
	return int64(n), true
}

func isPowerOfTen(n int64) bool {
	if n <= 0 {
		return false
	}

	for n > 1 {
		if n%10 != 0 {
			return false
		}
		n /= 10
	}
	return true
}
