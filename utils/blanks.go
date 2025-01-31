package utils

func Or[T any](value *T, or T) T {
	if value != nil {
		return *value
	}
	return or
}
