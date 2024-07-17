package evaluate

import (
	"context"

	"github.com/cockroachdb/errors"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/pure_utils"
)

// List of string cleaning steps applied:
// - normalize
// - remove diacritics
// - set to lower case
// - only letters and numbers
type FuzzyMatch struct{}

func (fuzzyMatcher FuzzyMatch) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	leftAny, rightAny, err := leftAndRight(arguments.Args)
	if err != nil {
		return MakeEvaluateError(errors.Wrap(err, "Error in Evaluate function FuzzyMatch"))
	}

	left, errLeft := adaptArgumentToString(leftAny)
	right, errRight := adaptArgumentToString(rightAny)
	algorithm, algorithmErr := AdaptNamedArgument(arguments.NamedArgs, "algorithm", adaptArgumentToString)

	errs := MakeAdaptedArgsErrors([]error{errLeft, errRight, algorithmErr})
	if len(errs) > 0 {
		return nil, errs
	}

	f, err := getSimilarityAlgo(algorithm)
	if err != nil {
		return MakeEvaluateError(err)
	}

	return f(left, right), nil
}

type FuzzyMatchAnyOf struct{}

func (fuzzyMatcher FuzzyMatchAnyOf) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	leftAny, rightAny, err := leftAndRight(arguments.Args)
	if err != nil {
		return MakeEvaluateError(errors.Wrap(err, "Error in Evaluate function FuzzyMatchAnyOf"))
	}

	left, errLeft := adaptArgumentToString(leftAny)
	right, errRight := adaptArgumentToListOfStrings(rightAny)
	algorithm, algorithmErr := AdaptNamedArgument(arguments.NamedArgs, "algorithm", adaptArgumentToString)

	errs := MakeAdaptedArgsErrors([]error{errLeft, errRight, algorithmErr})
	if len(errs) > 0 {
		return nil, errs
	}

	f, err := getSimilarityAlgo(algorithm)
	if err != nil {
		return MakeEvaluateError(err)
	}

	maxScore := 0
	for _, rVal := range right {
		maxScore = max(maxScore, f(left, rVal))
		if maxScore == 100 {
			break
		}
	}
	return maxScore, nil
}

func getSimilarityAlgo(s string) (func(s1 string, s2 string) int, error) {
	var f func(s1 string, s2 string) int

	switch s {
	case "ratio":
		return pure_utils.DirectSimilarity, nil
	case "token_set_ratio":
		// backward compatibility with an old name used in thefirst implementation. Renamed to "bag_of_words_similarity" to be
		// library agnostic and more descriptive.
		return pure_utils.BagOfWordsSimilarity, nil
	case "bag_of_words_similarity":
		return pure_utils.BagOfWordsSimilarity, nil
	}

	return f, errors.New("Unknown algorithm: " + s)
}
