package utils

// Amazingly, the Go standard library to not provide the function 'map'
// The rational of why the Go team rejects it is explained in this wonderfull stack overflow answer.
// https://stackoverflow.com/questions/71624828/is-there-a-way-to-map-an-array-of-objects-in-golang

// MapErr returns a new slice with the same length as src, but with values transformed by f
// if src is nil, returns nil
func Map[T, U any](src []T, f func(T) U) []U {
	if src == nil {
		return nil
	}
	us := make([]U, len(src))
	for i := range src {
		us[i] = f(src[i])
	}
	return us
}

// MapErr returns a new slice with the same length as src, but with values transformed by f
// If f returns an error, the function stops and returns the error.
// if src is nil, returns nil
func MapErr[T, U any](src []T, f func(T) (U, error)) ([]U, error) {
	if src == nil {
		return nil, nil
	}
	us := make([]U, len(src))
	for i := range src {
		var err error
		us[i], err = f(src[i])
		if err != nil {
			return us, err
		}
	}
	return us, nil
}

func MapErrWithParam[T, K, U any](src []T, param K, f func(K, T) (U, error)) ([]U, error) {
	if src == nil {
		return nil, nil
	}
	us := make([]U, len(src))
	for i := range src {
		var err error
		us[i], err = f(param, src[i])
		if err != nil {
			return us, err
		}
	}
	return us, nil
}

// MapMap return a new map with the same keys as src, but with values transformed by f
func MapMap[Key comparable, T any, U any](src map[Key]T, f func(T) U) map[Key]U {
	if src == nil {
		return nil
	}
	result := make(map[Key]U, len(src))
	for key, value := range src {
		result[key] = f(value)
	}
	return result
}

// MapMapErr return a new map with the same keys as src, but with values transformed by f
// If f returns an error, the function stops and returns the error.
func MapMapErr[Key comparable, T any, U any](src map[Key]T, f func(T) (U, error)) (map[Key]U, error) {
	if src == nil {
		return nil, nil
	}
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
