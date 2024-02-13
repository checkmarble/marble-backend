package pure_utils

// Amazingly, the Go standard library to not provide the function 'map'
// The rational of why the Go team rejects it is explained in this wonderfull stack overflow answer.
// https://stackoverflow.com/questions/71624828/is-there-a-way-to-map-an-array-of-objects-in-golang

// Map returns a new slice with the same length as src, but with values transformed by f
func Map[T, U any](src []T, f func(T) U) []U {
	us := make([]U, len(src))
	for i := range src {
		us[i] = f(src[i])
	}
	return us
}

// MapErr returns a new slice with the same length as src, but with values transformed by f
// If f returns an error, the function stops and returns the error.
func MapErr[T, U any](src []T, f func(T) (U, error)) ([]U, error) {
	us := make([]U, len(src))
	for i := range src {
		var err error
		us[i], err = f(src[i])
		if err != nil {
			return nil, err
		}
	}
	return us, nil
}

// MapValues return a new map with the same keys as src, but with values transformed by f
func MapValues[Key comparable, T any, U any](src map[Key]T, f func(T) U) map[Key]U {
	result := make(map[Key]U, len(src))
	for key, value := range src {
		result[key] = f(value)
	}
	return result
}

// MapValuesErr return a new map with the same keys as src, but with values transformed by f
// If f returns an error, the function stops and returns the error.
func MapValuesErr[Key comparable, T any, U any](src map[Key]T, f func(T) (U, error)) (map[Key]U, error) {
	result := make(map[Key]U, len(src))
	for key, value := range src {
		var err error
		transformed, err := f(value)
		if err != nil {
			return nil, err
		}
		result[key] = transformed
	}
	return result, nil
}
