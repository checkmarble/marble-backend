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
		Array              [2]string         `db:"array"`
		Slice              []string          `db:"slice"`
		Map                map[string]string `db:"map"`
		IgnoreNestedStruct NestedData        `db:"ignore_nested_struct"`
		NestedStruct       *NestedData       `db:"nested_struct"`
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
			expected: Expected{"key": "string", "array": [2]string{}},
		},
		{
			name:     "No db tag: should ignore IgnoredKey",
			data:     Data{IgnoredKey: Ptr("string")},
			expected: Expected{"array": [2]string{}},
		},
		{
			name:     "Unexported field: should ignore ignoredKey",
			data:     Data{ignoredKey: Ptr("string")},
			expected: Expected{"array": [2]string{}},
		},
		{
			name:     "Should handle array",
			data:     Data{Array: [2]string{"a1", "a2"}},
			expected: Expected{"array": [2]string{"a1", "a2"}},
		},
		{
			name:     "Should handle slice",
			data:     Data{Slice: []string{"a1", "a2"}},
			expected: Expected{"slice": []string{"a1", "a2"}, "array": [2]string{}},
		},
		{
			name:     "Should handle map",
			data:     Data{Map: map[string]string{"k1": "v2"}},
			expected: Expected{"map": map[string]string{"k1": "v2"}, "array": [2]string{}},
		},
		{
			name:     "Ignore Nested struct: should ignore IgnoreNestedStruct",
			data:     Data{IgnoreNestedStruct: NestedData{Nested: Ptr("string")}},
			expected: Expected{"array": [2]string{}},
		},
		{
			name:     "Pointer to Nested struct: should recursively handle pointer to nested struct",
			data:     Data{NestedStruct: &NestedData{Nested: Ptr("string")}},
			expected: Expected{"nested_struct": Expected{"nested": "string"}, "array": [2]string{}},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := columnValueMap(&c.data)

			if !reflect.DeepEqual(c.expected, got) {
				t.Errorf("ExpecteMockedTestCased %v, got %v", c.expected, got)
			}
		})
	}
}
