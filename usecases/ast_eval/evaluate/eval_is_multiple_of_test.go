package evaluate

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/stretchr/testify/assert"
)

func TestIsMultipleOf(t *testing.T) {
	tests := []struct {
		name   string
		args   map[string]any
		want   any
		errors []error
	}{
		{
			name:   "with a value which is not a float or an int",
			args:   map[string]any{"value": "hello world", "divider": 10},
			errors: []error{ast.ErrArgumentMustBeIntOrFloat},
		},
		{
			name: "with a value which is not an int",
			args: map[string]any{"value": 100.1, "divider": 10},
			want: false,
		},
		{
			name: "with value: 100, divider: 10",
			args: map[string]any{"value": 100, "divider": 10},
			want: true,
		},
		{
			name: "with value: 1000, divider: 100",
			args: map[string]any{"value": 1000, "divider": 100},
			want: true,
		},
		{
			name: "with value: 10, divider: 10",
			args: map[string]any{"value": 10, "divider": 10},
			want: true,
		},
		{
			name: "with value: 1000000, divider: 10000",
			args: map[string]any{"value": 1000000, "divider": 10000},
			want: true,
		},
		{
			name: "with value: 101, divider: 10",
			args: map[string]any{"value": 101, "divider": 10},
			want: false,
		},
		{
			name: "with value: 1005, divider: 100",
			args: map[string]any{"value": 1005, "divider": 100},
			want: false,
		},
		{
			name: "with value: 999, divider: 1000",
			args: map[string]any{"value": 999, "divider": 1000},
			want: false,
		},
		{
			name: "with value: 12345, divider: 100",
			args: map[string]any{"value": 12345, "divider": 100},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, errs := IsMultipleOf{}.Evaluate(context.TODO(), ast.Arguments{
				Args:      []any{},
				NamedArgs: tt.args,
			})
			assert.Equal(t, len(tt.errors), len(errs))
			if len(errs) > 0 {
				assert.ErrorContains(t, errs[0], tt.errors[0].Error())
			}
			assert.Equal(t, tt.want, r)
		})
	}
}

func TestDowncastToInt64(t *testing.T) {
	// Cas de test
	tests := []struct {
		input    float64
		expected int64
		success  bool
	}{
		// Edge cases
		{math.MinInt64, math.MinInt64, true}, // Min int64
		{math.MaxInt64, math.MaxInt64, true}, // Max int64
		{math.MinInt64 - 1, 0, false},        // Out of bounds (too low)
		{math.MaxInt64 + 1, 0, false},        // Out of bounds (too big)

		// Entiers simples
		{0, 0, true},         // Zero
		{123.0, 123, true},   // Positive int
		{-123.0, -123, true}, // Negative

		// Décimaux
		{123.45, 0, false},         // Decimal
		{-123.45, 0, false},        // Negative decimal
		{123.00000000001, 0, true}, // Decimal as close as possible of int

		// Cas spécifiques
		{1e20, 0, false},   // Huge number, out of bounds
		{-1e20, 0, false},  // Tiny number, out of bounds
		{1e18, 1e18, true}, // exact number in int64 range
	}

	// Exécution des tests
	for _, test := range tests {
		result, success := downcastToInt64(test.input)
		fmt.Printf("Input: %f, Expected: %d, Success: %v, Got: %d, Success: %v\n",
			test.input, test.expected, test.success, result, success)
	}
}
