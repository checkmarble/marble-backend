package utils

import "reflect"

type PtrToOptions struct {
	OmitZero bool
}

// Return a ptr to the provided value.
//
// Use options to change internal logic :
//   - OmitZero: zero values return a nil ptr
func PtrTo[T any](v T, options *PtrToOptions) *T {
	if options == nil {
		options = &PtrToOptions{
			OmitZero: false,
		}
	}

	if options.OmitZero {
		rv := reflect.ValueOf(v)
		if rv.IsZero() {
			return nil
		}
	}

	return &v
}

// Syntactic sugar for PtrTo with default options
func Ptr[T any](v T) *T {
	return PtrTo(v, nil)
}
