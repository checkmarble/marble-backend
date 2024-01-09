package evaluate

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models/ast"

	"github.com/stretchr/testify/assert"
)

func helperTestArithmetic[T int64 | float64](t *testing.T, f ast.Function, args []any, expected T) {
	r, errs := NewArithmetic(f).Evaluate(context.TODO(), ast.Arguments{Args: args})
	assert.Empty(t, errs)
	assert.Equal(t, r, expected)
}

func TestNewArithmetic_basic(t *testing.T) {
	helperTestArithmetic(t, ast.FUNC_ADD, []any{2, 1}, int64(3))
	helperTestArithmetic(t, ast.FUNC_ADD, []any{2.0, 1}, float64(3.0))

	helperTestArithmetic(t, ast.FUNC_SUBTRACT, []any{11, 2}, int64(9))
	helperTestArithmetic(t, ast.FUNC_MULTIPLY, []any{4, 3}, int64(12))
}

func TestNewArithmetic_fail(t *testing.T) {
	_, errs := NewArithmetic(ast.FUNC_ADD).Evaluate(context.TODO(), ast.Arguments{Args: []any{2, "totally not an int or a float"}})
	if assert.Len(t, errs, 1) {
		assert.ErrorIs(t, errs[0], ast.ErrArgumentMustBeIntOrFloat)
	}

}

func TestNewArithmetic_ErrWrongNumberOfArgument(t *testing.T) {
	_, errs := NewArithmetic(ast.FUNC_ADD).Evaluate(context.TODO(), ast.Arguments{Args: []any{}})
	if assert.Len(t, errs, 1) {
		assert.ErrorIs(t, errs[0], ast.ErrWrongNumberOfArgument)
	}

}
