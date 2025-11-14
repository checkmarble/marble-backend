package pure_utils

func PtrValueOrDefault[T any](ptr *T, defaultValue T) T {
	if ptr != nil {
		return *ptr
	}
	return defaultValue
}

func PtrSliceValueOrDefault[T any](ptr *[]T, defaultValue []T) []T {
	if ptr != nil {
		return *ptr
	}
	return defaultValue
}
