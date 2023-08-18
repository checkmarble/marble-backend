package utils

import (
	"reflect"
	"testing"
)

func TestColumnList(t *testing.T) {
	type Expected = []string
	type TestCase struct {
		name     string
		data     any
		expected Expected
	}
	cases := []TestCase{
		{
			name: "Should only keep key",
			data: ColumnList[struct {
				Key        any `db:"key"`
				ignoredKey any `db:"ignored_key"`
				IgnoredKey any
			}](),
			expected: Expected{"key"},
		},
		{
			name: "Should get all columns",
			data: ColumnList[struct {
				C1 any `db:"c1"`
				C2 any `db:"c2"`
				C3 any `db:"c3"`
				C6 any `db:"c6"`
				C9 any `db:"c9"`
			}](),
			expected: Expected{"c1", "c2", "c3", "c6", "c9"},
		},
		{
			name: "Should add provided prefixes",
			data: ColumnList[struct {
				C1 any `db:"c1"`
				C2 any `db:"c2"`
				C3 any `db:"c3"`
				C6 any `db:"c6"`
				C9 any `db:"c9"`
			}]("p1", "p2"),
			expected: Expected{"p1.p2.c1", "p1.p2.c2", "p1.p2.c3", "p1.p2.c6", "p1.p2.c9"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !reflect.DeepEqual(c.expected, c.data) {
				t.Errorf("Expected %v, got %v", c.expected, c.data)
			}
		})
	}
}

func TestColumnValueMap(t *testing.T) {
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
			got := ColumnValueMap(&c.data)

			if !reflect.DeepEqual(c.expected, got) {
				t.Errorf("Expected %v, got %v", c.expected, got)
			}
		})
	}
}
