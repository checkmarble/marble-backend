package evaluate

import (
	"marble/marble-backend/models/ast"
	"testing"

	"github.com/stretchr/testify/assert"
)

func helperTestBooleanArithmetic(t *testing.T, function ast.Function, args []any, expected bool) {
	r, errs := BooleanArithmetic{Function: function}.Evaluate(ast.Arguments{Args: args})
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
	_, errs := BooleanArithmetic{Function: ast.FUNC_AND}.Evaluate(ast.Arguments{Args: []any{}})
	if assert.Len(t, errs, 1) {
		assert.ErrorIs(t, errs[0], ast.ErrWrongNumberOfArgument)
	}
}
