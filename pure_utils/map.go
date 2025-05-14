package pure_utils

import "slices"

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

// FlatMap returns a new flattened slice composed of the result of passing each element of the input slice
// to a function returning a slice of element. This is the equivalent of doing a Map, then a Flatten.
func FlatMap[T, U any](src []T, f func(T) []U) []U {
	us := make([]U, 0, len(src))
	for _, item := range src {
		us = append(us, f(item)...)
	}
	return slices.Clip(us)
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

// MapWhile maps over items in a slice and produces a slice of items of another type.
// Contrary to regular Map(), the callbacks returns a second boolean value to indicate if the operation
// should continue. It stops whenever the callback returns false.
func MapWhile[T, U any](src []T, f func(T) (U, bool)) []U {
	us := make([]U, 0, len(src))
	for i := range src {
		item, next := f(src[i])

		us = append(us, item)

		if !next {
			break
		}
	}
	return us
}

// MapValuesWhile maps over a map's values in a slice and produces a slice of items of another type.
// Contrary to regular MapValues(), the callbacks returns a second boolean value to indicate if the operation
// should continue. It stops whenever the callback returns false.
func MapValuesWhile[Key comparable, T any, U any](src map[Key]T, f func(T) (U, bool)) map[Key]U {
	result := make(map[Key]U, len(src))
	for key, value := range src {
		item, next := f(value)

		result[key] = item

		if !next {
			break
		}
	}
	return result
}

func MapKeyValue[KL, KR comparable, VL, VR any](in map[KL]VL, f func(k KL, v VL) (KR, VR)) map[KR]VR {
	out := make(map[KR]VR, len(in))

	for k, v := range in {
		kr, vr := f(k, v)

		out[kr] = vr
	}

	return out
}

func MapSliceToMap[T, V any, K comparable](input []T, f func(v T) (K, V)) map[K]V {
	output := make(map[K]V, len(input))

	for _, item := range input {
		k, v := f(item)
		output[k] = v
	}

	return output
}
