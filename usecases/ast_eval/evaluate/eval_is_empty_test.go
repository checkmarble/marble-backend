package evaluate

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/stretchr/testify/assert"
)

func TestIsEmpty_Evaluate(t *testing.T) {
	tests := []struct {
		name      string
		arguments ast.Arguments
		expected  any
	}{
		{
			name: "Argument is nil",
			arguments: ast.Arguments{
				Args: []any{nil},
			},
			expected: true,
		},
		{
			name: "Argument is empty string",
			arguments: ast.Arguments{
				Args: []any{""},
			},
			expected: true,
		},
		{
			name: "Argument is non-empty string",
			arguments: ast.Arguments{
				Args: []any{"non-empty"},
			},
			expected: false,
		},
		{
			name: "Argument is non-empty integer",
			arguments: ast.Arguments{
				Args: []any{123},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isEmpty := IsEmpty{}
			result, err := isEmpty.Evaluate(context.Background(), tt.arguments)
			assert.Nil(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsNotEmpty_Evaluate(t *testing.T) {
	tests := []struct {
		name      string
		arguments ast.Arguments
		expected  any
	}{
		{
			name: "Argument is nil",
			arguments: ast.Arguments{
				Args: []any{nil},
			},
			expected: false,
		},
		{
			name: "Argument is empty string",
			arguments: ast.Arguments{
				Args: []any{""},
			},
			expected: false,
		},
		{
			name: "Argument is non-empty string",
			arguments: ast.Arguments{
				Args: []any{"non-empty"},
			},
			expected: true,
		},
		{
			name: "Argument is non-empty integer",
			arguments: ast.Arguments{
				Args: []any{123},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isNotEmpty := IsNotEmpty{}
			result, err := isNotEmpty.Evaluate(context.Background(), tt.arguments)
			assert.Nil(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
