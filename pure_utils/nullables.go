package pure_utils

import (
	"encoding/json"
	"fmt"
)

type Null[T any] struct {
	value T
	Valid bool // Valid is true if value is not NULL
	Set   bool // Set is true if the value was present in JSON, even if it was null
}

func (u *Null[T]) UnmarshalJSON(data []byte) error {
	u.Set = true // Set to true if the value was present in JSON

	if string(data) == "null" {
		u.Valid = false
		return nil
	}

	if err := json.Unmarshal(data, &u.value); err != nil {
		return fmt.Errorf("invalid UUID: %w", err)
	}

	u.Valid = true
	return nil
}

func (u Null[T]) Ptr() *T {
	if !u.Valid {
		return nil
	}
	return &u.value
}

func (u Null[T]) Value() T {
	return u.value
}

func NullFrom[T any](u T) Null[T] {
	return Null[T]{
		value: u,
		Valid: true,
		Set:   true,
	}
}
