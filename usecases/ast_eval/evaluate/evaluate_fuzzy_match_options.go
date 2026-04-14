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
	"bag_of_words_similarity_db",
	"direct_string_similarity_db",
}

func (f FuzzyMatchOptionsEvaluator) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	// "value" input comes from payload or DB, can be null and should not fail.
	// Bubble up nil so the parent Filter evaluator can handle it (returns Filter{Value: nil},
	// which the Aggregator then treats as a no-op filter and returns the default value).
	if arguments.NamedArgs["value"] == nil {
		return nil, nil
	}

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

	// Threshold is received as an integer (0–100) from the client and converted to a float (0.0–1.0) for internal processing.
	threshold, err := AdaptNamedArgument(arguments.NamedArgs, "threshold",
		promoteArgumentToFloat64)
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
		Threshold: threshold / 100,
		Value:     value,
	}, nil
}
