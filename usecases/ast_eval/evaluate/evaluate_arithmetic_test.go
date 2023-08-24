package evaluate

import (
	"marble/marble-backend/models/ast"
	"testing"

	"github.com/stretchr/testify/assert"
)

func helperTestArithmetic[T int64 | float64](t *testing.T, f ast.Function, args []any, expected T) {
	r, errs := NewArithmetic(f).Evaluate(ast.Arguments{Args: args})
	assert.Empty(t, errs)
	assert.Equal(t, r, expected)
}

func TestNewArithmetic_basic(t *testing.T) {
	helperTestArithmetic(t, ast.FUNC_ADD, []any{2, 1}, int64(3))
	helperTestArithmetic(t, ast.FUNC_ADD, []any{2.0, 1}, float64(3.0))

	helperTestArithmetic(t, ast.FUNC_SUBTRACT, []any{11, 2}, int64(9))
	helperTestArithmetic(t, ast.FUNC_MULTIPLY, []any{4, 3}, int64(12))
}
