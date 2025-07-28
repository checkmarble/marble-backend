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
