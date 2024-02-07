package utils

// Map returns a new slice with the same length as src, but with values transformed by f
func Filter[T any](src []T, f func(T) bool) []T {
	us := make([]T, 0, len(src))
	for i := range src {
		if f(src[i]) {
			us = append(us, src[i])
		}
	}
	return us
}
