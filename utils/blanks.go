package utils

func Or[T any](value *T, or T) T {
	if value != nil {
		return *value
	}
	return or
}

func NilOrZero[T comparable](value *T) bool {
	if value == nil {
		return true
	}
	if *value == *new(T) {
		return true
	}
	return false
}
