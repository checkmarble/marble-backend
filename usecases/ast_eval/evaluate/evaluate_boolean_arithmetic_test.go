package evaluate

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models/ast"

	"github.com/stretchr/testify/assert"
)

func helperTestBooleanArithmetic(t *testing.T, function ast.Function, args []any, expected bool) {
	r, errs := BooleanArithmetic{Function: function}.Evaluate(context.TODO(), ast.Arguments{Args: args})
	assert.Empty(t, errs)
	assert.Equal(t, expected, r)
}

func TestBooleanArithmetic_one_operand(t *testing.T) {
	helperTestBooleanArithmetic(t, ast.FUNC_AND, []any{true}, true)
	helperTestBooleanArithmetic(t, ast.FUNC_AND, []any{false}, false)

	helperTestBooleanArithmetic(t, ast.FUNC_OR, []any{true}, true)
	helperTestBooleanArithmetic(t, ast.FUNC_OR, []any{false}, false)
}

func TestBooleanArithmetic_two_operands(t *testing.T) {
	helperTestBooleanArithmetic(t, ast.FUNC_AND, []any{true, true}, true)
	helperTestBooleanArithmetic(t, ast.FUNC_AND, []any{false, false}, false)
	helperTestBooleanArithmetic(t, ast.FUNC_AND, []any{true, false}, false)

	helperTestBooleanArithmetic(t, ast.FUNC_OR, []any{true, true}, true)
	helperTestBooleanArithmetic(t, ast.FUNC_OR, []any{false, false}, false)
	helperTestBooleanArithmetic(t, ast.FUNC_OR, []any{true, false}, true)
}

func TestBooleanArithmetic_three_operands(t *testing.T) {
	helperTestBooleanArithmetic(t, ast.FUNC_AND, []any{true, true, true}, true)
	helperTestBooleanArithmetic(t, ast.FUNC_AND, []any{true, true, false}, false)

	helperTestBooleanArithmetic(t, ast.FUNC_OR, []any{false, false, false}, false)
	helperTestBooleanArithmetic(t, ast.FUNC_OR, []any{false, false, true}, true)
}

func TestBooleanArithmetic_zero_operator(t *testing.T) {
	_, errs := BooleanArithmetic{Function: ast.FUNC_AND}.Evaluate(context.TODO(), ast.Arguments{Args: []any{}})
	if assert.Len(t, errs, 1) {
		assert.ErrorIs(t, errs[0], ast.ErrWrongNumberOfArgument)
	}
}

func TestBooleanArithmeticEvalOr(t *testing.T) {
	tests := []struct {
		name     string
		args     []any
		expected any
		err      error
	}{
		{
			name:     "all false",
			args:     []any{false, false, false},
			expected: false,
			err:      nil,
		},
		{
			name:     "one true",
			args:     []any{false, true, false},
			expected: true,
			err:      nil,
		},
		{
			name:     "all true",
			args:     []any{true, true, true},
			expected: true,
			err:      nil,
		},
		{
			name:     "contains nil",
			args:     []any{false, nil, false},
			expected: nil,
			err:      nil,
		},
		{
			name:     "non-boolean argument",
			args:     []any{false, "string", false},
			expected: nil,
			err:      ast.ErrArgumentMustBeBool,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := booleanArithmeticEvalOr(tt.args)
			if tt.err != nil {
				assert.ErrorIs(t, err, tt.err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBooleanArithmeticEvalAnd(t *testing.T) {
	tests := []struct {
		name     string
		args     []any
		expected any
		err      error
	}{
		{
			name:     "all true",
			args:     []any{true, true, true},
			expected: true,
			err:      nil,
		},
		{
			name:     "one false",
			args:     []any{true, false, true},
			expected: false,
			err:      nil,
		},
		{
			name:     "all false",
			args:     []any{false, false, false},
			expected: false,
			err:      nil,
		},
		{
			name:     "contains nil",
			args:     []any{true, nil, true},
			expected: nil,
			err:      nil,
		},
		{
			name:     "non-boolean argument",
			args:     []any{true, "string", true},
			expected: nil,
			err:      ast.ErrArgumentMustBeBool,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := booleanArithmeticEvalAnd(tt.args)
			if tt.err != nil {
				assert.ErrorIs(t, err, tt.err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}
