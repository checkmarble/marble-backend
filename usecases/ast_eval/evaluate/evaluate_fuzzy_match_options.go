package evaluate

import (
	"context"
	"fmt"
	"slices"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/cockroachdb/errors"
)

type FuzzyMatchOptionsEvaluator struct{}

var allowedFuzzyMatchAlgorithms = []string{
	"bag_of_words_similarity",
	"direct_string_similarity",
	"token_set_ratio", // TODO: remove this option
}

func (f FuzzyMatchOptionsEvaluator) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	algorithm, err := AdaptNamedArgument(arguments.NamedArgs, "algorithm", adaptArgumentToString)
	if err != nil {
		return nil, []error{err}
	}
	if !slices.Contains(allowedFuzzyMatchAlgorithms, algorithm) {
		return MakeEvaluateError(errors.Join(
			ast.NewNamedArgumentError("algorithm"),
			errors.Wrap(ast.ErrRuntimeExpression,
				fmt.Sprintf("algorithm %s is not valid in Evaluate fuzzy match options", algorithm)),
		))
	}

	threshold, err := AdaptNamedArgument(arguments.NamedArgs, "threshold", promoteArgumentToFloat64)
	if err != nil {
		return nil, []error{err}
	}
	if threshold < 0 || threshold > 100 {
		return MakeEvaluateError(errors.Join(
			ast.NewNamedArgumentError("threshold"),
			errors.Wrap(ast.ErrRuntimeExpression,
				fmt.Sprintf("threshold %f is not valid in Evaluate fuzzy match options", threshold)),
		))
	}

	value, err := AdaptNamedArgument(arguments.NamedArgs, "value", adaptArgumentToString)
	if err != nil {
		return nil, []error{err}
	}

	return ast.FuzzyMatchOptions{
		Algorithm: algorithm,
		Threshold: threshold,
		Value:     value,
	}, nil
}
