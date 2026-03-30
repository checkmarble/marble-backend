package evaluate

import (
	"context"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/cockroachdb/errors"
)

type ScoreComputation struct{}

func (p ScoreComputation) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	modifier, modifierErr := AdaptNamedArgument(arguments.NamedArgs, "modifier", promoteArgumentToFloat64)
	floor, floorErr := AdaptNamedArgument(arguments.NamedArgs, "floor", promoteArgumentToFloat64)
	if floorErr != nil {
		floor = 0
	}

	var childrenErr error

	if len(arguments.Args) != 1 {
		childrenErr = errors.Wrap(ast.ErrWrongNumberOfArgument, "ScoreComputation must have exactly one child")
	}
	errs := filterNilErrors(childrenErr, modifierErr)
	if len(errs) > 0 {
		return nil, errs
	}

	if arguments.Args[0] == nil {
		return ast.ScoreComputationResult{}, nil
	}

	result, err := adaptArgumentToBool(arguments.Args[0])
	if err != nil {
		return MakeEvaluateError(err)
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
