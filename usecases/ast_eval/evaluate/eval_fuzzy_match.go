package evaluate

import (
	"context"
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/cockroachdb/errors"
	fuzzy "github.com/paul-mannino/go-fuzzywuzzy"

	"github.com/checkmarble/marble-backend/models/ast"
)

type FuzzyMatch struct{}

// Implements a fuzzy match using the go-fuzzywuzzy library.
// List of strign cleaning steps applied:
// - normalize
// - remove diacritics
// - set to lower case
// - keep only letters and numbers
// (- keep non-ASCII characters)

func (fuzzyMatcher FuzzyMatch) Evaluate(ctx context.Context, arguments ast.Arguments) (any, []error) {
	leftAny, rightAny, err := leftAndRight(arguments.Args)
	if err != nil {
		return MakeEvaluateError(errors.Wrap(err, "Error in Evaluate function StringInList"))
	}

	left, errLeft := adaptArgumentToString(leftAny)
	left = cleanseString(left)
	right, errRight := adaptArgumentToString(rightAny)
	right = cleanseString(right)
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
		return MakeEvaluateError(errors.Wrap(err, "Error in Evaluate function StringInList"))
	}

	left, errLeft := adaptArgumentToString(leftAny)
	left = cleanseString(left)
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
		maxScore = max(maxScore, f(left, cleanseString(rVal)))
		if maxScore == 100 {
			break
		}
	}
	return maxScore, nil
}

func getSimilarityAlgo(s string) (func(s1 string, s2 string, opts ...bool) int, error) {
	var f func(s1 string, s2 string, opts ...bool) int
	switch s {
	case "ratio":
		f = func(s1 string, s2 string, opts ...bool) int { return fuzzy.Ratio(s1, s2) }
	case "partial_ratio":
		f = func(s1 string, s2 string, opts ...bool) int { return fuzzy.PartialRatio(s1, s2) }
	case "token_sort_ratio":
		f = fuzzy.TokenSortRatio
	case "token_set_ratio":
		f = fuzzy.TokenSetRatio
	case "partial_token_set_ratio":
		f = fuzzy.PartialTokenSetRatio
	case "partial_token_sort_ratio":
		f = fuzzy.PartialTokenSortRatio
	default:
		return f, errors.New("Unknown algorithm: " + s)
	}
	return f, nil
}

func normalizeAndRemoveDiacritics(s string) string {
	t := transform.Chain(
		norm.NFD,
		runes.Remove(runes.In(unicode.Mn)),
		norm.NFC,
	)
	result, _, _ := transform.String(t, s)
	return result
}

func cleanseString(s string) string {
	// - normalize
	// - remove diacritics
	// - set to lower case
	// - keep only letters and numbers
	// - keep non-ASCII characters
	return strings.TrimSpace(fuzzy.Cleanse(normalizeAndRemoveDiacritics(s), false))
}
