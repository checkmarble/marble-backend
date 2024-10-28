package models

import (
	"testing"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/stretchr/testify/assert"
)

func TestFilterToSql(t *testing.T) {
	type testCase struct {
		name     string
		filter   Filter
		expected string
		args     []any
	}

	testCases := []testCase{
		{
			name: "simple equal filter",
			filter: Filter{
				LeftSql:    "left",
				Operator:   ast.FUNC_EQUAL,
				RightValue: 1,
			},
			expected: "left = ?",
			args:     []any{1},
		},
		{
			name: "string in list filter",
			filter: Filter{
				LeftValue:  "blabla",
				Operator:   ast.FUNC_IS_IN_LIST,
				RightValue: []string{"bla", "and bla"},
			},
			expected: "? = ANY(?)",
			args:     []any{"blabla", []string{"bla", "and bla"}},
		},
		{
			name: "string contains filter",
			filter: Filter{
				LeftValue:  "COMMIT",
				Operator:   ast.FUNC_STRING_CONTAINS,
				RightValue: "mit",
			},
			expected: "? ILIKE CONCAT('%',?::text,'%')",
			args:     []any{"COMMIT", "mit"},
		},
		{
			name: "string in list",
			filter: Filter{
				LeftValue:  "blabla",
				Operator:   ast.FUNC_IS_IN_LIST,
				RightValue: []string{"bla", "and bla"},
			},
			expected: "? = ANY(?)",
			args:     []any{"blabla", []string{"bla", "and bla"}},
		},
		{
			name: "nested math with division",
			filter: Filter{
				LeftValue: float64(1),
				Operator:  ast.FUNC_LESS,
				RightNestedFilter: &Filter{
					LeftValue: float64(3),
					Operator:  ast.FUNC_DIVIDE,
					RightSql:  "\"schema\".\"table\".\"num\"",
				},
			},
			expected: "? < (? / NULLIF(\"schema\".\"table\".\"num\", 0))",
			args:     []any{float64(1), float64(3)},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sql, args := tc.filter.ToSql()
			assert.Equal(t, tc.expected, sql, tc.name)
			assert.Equal(t, tc.args, args, tc.name)
		})
	}
}
