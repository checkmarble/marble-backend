package evaluate

import (
	"marble/marble-backend/models/ast"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComparison_comparisonFunction_int_true(t *testing.T) {
	r, errs := NewComparison(ast.FUNC_GREATER).Evaluate(ast.Arguments{Args: []any{2, 1}})
	assert.Empty(t, errs)
	assert.True(t, r.(bool))
}

func TestComparison_comparisonFunction_mixed_int_float_false(t *testing.T) {
	r, errs := NewComparison(ast.FUNC_GREATER).Evaluate(ast.Arguments{Args: []any{(1), float64(2)}})
	assert.Empty(t, errs)
	assert.False(t, r.(bool))
}

func TestComparison_comparisonFunction_less(t *testing.T) {
	r, errs := NewComparison(ast.FUNC_LESS).Evaluate(ast.Arguments{Args: []any{1, 2}})
	assert.Empty(t, errs)
	assert.True(t, r.(bool))
}

func TestComparison_fail(t *testing.T) {
	_, errs := NewComparison(ast.FUNC_ADD).Evaluate(ast.Arguments{Args: []any{"toto", false}})
	if assert.Len(t, errs, 2) {
		assert.ErrorIs(t, errs[0], ast.ErrArgumentMustBeIntOrFloat)
		assert.ErrorIs(t, errs[1], ast.ErrArgumentMustBeIntOrFloat)
	}

}
