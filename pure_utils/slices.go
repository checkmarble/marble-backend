package pure_utils

import (
	"github.com/hashicorp/go-set/v2"
)

func ContainsSameElements[T comparable](a, b []T) bool {
	return set.From(a).Equal(set.From(b))
}
