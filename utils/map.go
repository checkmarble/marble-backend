package utils

// Amazingly, the Go standard library to not provide the function 'map'
// The rational of why the Go team rejects it is explained in this wonderfull stack overflow answer.
// https://stackoverflow.com/questions/71624828/is-there-a-way-to-map-an-array-of-objects-in-golang
func Map[T, U any](ts []T, f func(T) U) []U {
	us := make([]U, len(ts))
	for i := range ts {
		us[i] = f(ts[i])
	}
	return us
}
