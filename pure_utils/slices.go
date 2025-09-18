package pure_utils

import (
	"github.com/hashicorp/go-set/v2"
)

func ContainsSameElements[T comparable](a, b []T) bool {
	return set.From(a).Equal(set.From(b))
}

func ToAnySlice[T any](input []T) []any {
	output := make([]any, len(input))
	for idx, value := range input {
		output[idx] = value
	}
	return output
}

func AnySliceAtIndex[T any](input any, index int) (T, bool) {
	dflt := *new(T)

	if input == nil {
		return dflt, false
	}

	sliceOfAny, ok := input.([]any)
	if !ok {
		return dflt, false
	}

	item, ok := sliceOfAny[index].(T)
	if !ok {
		return dflt, false
	}

	return item, true
}
