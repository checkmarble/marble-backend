package evaluate

import (
	"context"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"
)

type ArithmeticDivide struct{}

func (f ArithmeticDivide) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	leftAny, rightAny, err := leftAndRight(arguments.Args)
	if err != nil {
		return MakeEvaluateError(errors.Wrap(err, "Error in Evaluate function Divide"))
	}

	// promote to float64
	left, right, errs := adaptLeftAndRight(leftAny, rightAny, promoteArgumentToFloat64)
	if len(errs) > 0 {
		return nil, errs
	}

	if right == 0.0 {
		return MakeEvaluateError(errors.Wrap(models.DivisionByZeroError, "Zero division error in Evaluate function Divide"))
	}

	return MakeEvaluateResult(left / right)
}
