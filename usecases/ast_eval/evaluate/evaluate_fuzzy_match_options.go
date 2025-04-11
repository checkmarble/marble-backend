package evaluate

import (
	"context"

	"github.com/checkmarble/marble-backend/models/ast"
)

type FuzzyMatchOptions struct {
	Algorithm     string
	Threshold     float64
	Value         string
	NamedChildren map[string]any
}

func (f FuzzyMatchOptions) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	algorithm, err := AdaptNamedArgument(arguments.NamedArgs, "algorithm", adaptArgumentToString)
	if err != nil {
		return nil, []error{err}
	}
	f.Algorithm = algorithm

	threshold, err := AdaptNamedArgument(arguments.NamedArgs, "threshold", promoteArgumentToFloat64)
	if err != nil {
		return nil, []error{err}
	}
	f.Threshold = threshold

	value, err := AdaptNamedArgument(arguments.NamedArgs, "value", adaptArgumentToString)
	if err != nil {
		return nil, []error{err}
	}
	f.Value = value

	return f, nil
}
