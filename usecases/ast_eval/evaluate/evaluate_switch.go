package evaluate

import (
	"context"
	"fmt"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/cockroachdb/errors"
)

type Switch struct{}

func (p Switch) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	var errs []error

	if len(arguments.Args) == 0 {
		errs = append(errs, errors.Wrap(ast.ErrWrongNumberOfArgument, "Switch should have at least one branch"))
	}
	if _, ok := arguments.NamedArgs["field"]; !ok {
		errs = append(errs, errors.Wrap(ast.ErrMissingNamedArgument, "Switch should have a `field` named argument"))
	}

	nodes, err := adaptArgumentToListOfThings[ast.ScoreComputationResult](arguments.Args)
	if err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return nil, errs
	}

	for _, node := range nodes {
		if node.Triggered {
			return node, nil
		}
	}

	return ast.ScoreComputationResult{}, []error{fmt.Errorf("no case triggered on switch")}
}
