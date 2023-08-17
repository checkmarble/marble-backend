package evaluate

import (
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
	"testing"

	"github.com/stretchr/testify/assert"
)

func helperTestArithmetic[T int64 | float64](t *testing.T, f ast.Function, args []any, expected T) {
	r, err := NewArithmetic(f).Evaluate(ast.Arguments{Args: args})
	assert.NoError(t, err)
	assert.Equal(t, r, expected)
}

func TestNewArithmetic_basic(t *testing.T) {
	helperTestArithmetic(t, ast.FUNC_ADD, []any{2, 1}, int64(3))
	helperTestArithmetic(t, ast.FUNC_ADD, []any{2.0, 1}, float64(3.0))

	helperTestArithmetic(t, ast.FUNC_SUBTRACT, []any{11, 2}, int64(9))
	helperTestArithmetic(t, ast.FUNC_MULTIPLY, []any{4, 3}, int64(12))
	helperTestArithmetic(t, ast.FUNC_DIVIDE, []any{10, 3}, int64(3))
	helperTestArithmetic(t, ast.FUNC_DIVIDE, []any{10.0, 3}, float64(3.3333333333333335))
}

func TestNewArithmeticFunction_int_divide_by_zero(t *testing.T) {
	_, err := NewArithmetic(ast.FUNC_DIVIDE).Evaluate(ast.Arguments{Args: []any{1, 0}})
	assert.ErrorIs(t, err, models.DivisionByZeroError)
}

func TestNewArithmeticFunction_float_divide_by_zero(t *testing.T) {
	_, err := NewArithmetic(ast.FUNC_DIVIDE).Evaluate(ast.Arguments{Args: []any{1.0, 0.0}})
	assert.ErrorIs(t, err, models.DivisionByZeroError)
}
