package pg_repository

import (
	"reflect"
	"testing"
)

func Ptr[T any](v T) *T {
	return &v
}

func TestUpdateMapByName(t *testing.T) {
	type NestedData struct {
		Nested *string `db:"nested"`
	}
	type Data struct {
		Key                *string `db:"key"`
		ignoredKey         *string `db:"ignored_key"`
		IgnoredKey         *string
		IgnoreNestedStruct NestedData  `db:"ignore_nested_struct"`
		NestedStruct       *NestedData `db:"nested_struct"`
	}

	type Expected = map[string]any
	type TestCase struct {
		name     string
		data     Data
		expected Expected
	}
	cases := []TestCase{
		{
			name:     "Should retrieve Key",
			data:     Data{Key: Ptr("string")},
			expected: Expected{"key": "string"},
		},
		{
			name:     "No db tag: should ignore IgnoredKey",
			data:     Data{IgnoredKey: Ptr("string")},
			expected: Expected{},
		},
		{
			name:     "Unexported field: should ignore ignoredKey",
			data:     Data{ignoredKey: Ptr("string")},
			expected: Expected{},
		},
		{
			name:     "Ignore Nested struct: should ignore IgnoreNestedStruct",
			data:     Data{IgnoreNestedStruct: NestedData{Nested: Ptr("string")}},
			expected: Expected{},
		},
		{
			name:     "Pointer to Nested struct: should recursively handle pointer to nested struct",
			data:     Data{NestedStruct: &NestedData{Nested: Ptr("string")}},
			expected: Expected{"nested_struct": Expected{"nested": "string"}},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := upsertMapByName(&c.data)

			if !reflect.DeepEqual(c.expected, got) {
				t.Errorf("ExpecteMockedTestCased %v, got %v", c.expected, got)
			}
		})
	}
}
