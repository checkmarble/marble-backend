package evaluate

import (
	"context"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/cockroachdb/errors"
)

type Switch struct{}

func (p Switch) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	var errs []error

	if len(arguments.Args) == 0 {
		errs = append(errs, errors.Wrap(ast.ErrWrongNumberOfArgument, "Switch should have at least one branch"))
	}
	field, ok := arguments.NamedArgs["field"]
	if !ok {
		errs = append(errs, errors.Wrap(ast.ErrMissingNamedArgument, "Switch should have a `field` named argument"))
	}

	if len(errs) > 0 {
		return nil, errs
	}

	if field != nil {
		nodes, err := adaptArgumentToListOfThings[ast.ScoreComputationResult](arguments.Args)
		if err != nil {
			errs = append(errs, err)
		}

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
