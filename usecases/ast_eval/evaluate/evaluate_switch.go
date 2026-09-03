package evaluate

import (
	"context"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/cockroachdb/errors"
)

type Switch struct{}

func (Switch) Evaluate(_ context.Context, arguments ast.Arguments) (any, []error) {
	if len(arguments.Args) == 0 {
		return MakeEvaluateError(errors.Wrap(ast.ErrWrongNumberOfArgument, "Switch should have at least one branch"))
	}

	switch arguments.Args[0].(type) {
	case ast.ScoreComputationResult:
		return evaluateScoringSwitch(arguments)
	case ast.SwitchCaseResult:
		return evaluateNumericSwitch(arguments)
	default:
		return MakeEvaluateError(errors.Wrap(
			ast.ErrArgumentInvalidType,
			"Switch branches must return either ScoreComputationResult or SwitchCaseResult",
		))
	}
}

func evaluateScoringSwitch(arguments ast.Arguments) (any, []error) {
	nodes, errs := AdaptArguments(arguments.Args, adaptArgumentToThing[ast.ScoreComputationResult])
	if len(errs) > 0 {
		return nil, errs
	}

	for idx, node := range nodes {
		if node.Triggered {
			node.Fallback = false
			node.Default = false
			node.Branch = new(idx)

			return node, nil
		}
	}

	fallback, err := AdaptNamedArgument(arguments.NamedArgs, "fallback", adaptArgumentToThing[ast.ScoreComputationResult])
	if err == nil {
		fallback.Fallback = true
		fallback.Default = false
		fallback.Branch = nil

		return fallback, nil
	}

	return ast.ScoreComputationResult{Triggered: true, Default: true}, nil
}

func evaluateNumericSwitch(arguments ast.Arguments) (any, []error) {
	cases, errs := AdaptArguments(arguments.Args, adaptArgumentToThing[ast.SwitchCaseResult])
	fallback, fallbackErr := AdaptNamedArgument(arguments.NamedArgs, "fallback", promoteArgumentToFloat64)
	errs = append(errs, fallbackErr)
	errs = filterNilErrors(errs...)
	if len(errs) > 0 {
		return nil, errs
	}

	for _, switchCase := range cases {
		if switchCase.Matched {
			return switchCase.Value, nil
		}
	}

	return fallback, nil
}
