package evaluate

import (
	"context"

	"github.com/checkmarble/marble-backend/models/ast"
)

type ScoreComputation struct{}

func (p ScoreComputation) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	modifier, modifierErr := AdaptNamedArgument(arguments.NamedArgs, "modifier", promoteArgumentToFloat64)
	floor, floorErr := AdaptNamedArgument(arguments.NamedArgs, "floor", promoteArgumentToFloat64)
	result, resultErr := adaptArgumentToBool(arguments.Args[0])

	errs := filterNilErrors(modifierErr, floorErr, resultErr)
	if len(errs) > 0 {
		return nil, errs
	}

	if !result {
		modifier, floor = 0.0, 0.0
	}

	return ast.ScoreComputationResult{
		Triggered: result,
		Modifier:  int(modifier),
		Floor:     int(floor),
	}, nil
}
