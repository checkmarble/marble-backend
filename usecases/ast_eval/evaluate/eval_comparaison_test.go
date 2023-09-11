package evaluate

import (
	"testing"

	"github.com/checkmarble/marble-backend/models/ast"

	"github.com/stretchr/testify/assert"
)

func helperComparison(t *testing.T, f ast.Function, left, right int, expected bool) {
	r, errs := NewComparison(f).Evaluate(ast.Arguments{Args: []any{left, right}})
	assert.Empty(t, errs)
	assert.Equal(t, expected, r)
}

func TestComparison_comparisonFunction_greater_int(t *testing.T) {
	helperComparison(t, ast.FUNC_GREATER, 2, 1, true)
	helperComparison(t, ast.FUNC_GREATER, 1, 2, false)
	helperComparison(t, ast.FUNC_GREATER, 1, 1, false)
}

func TestComparison_comparisonFunction_greater_or_equal_int(t *testing.T) {
	helperComparison(t, ast.FUNC_GREATER_OR_EQUAL, 2, 1, true)
	helperComparison(t, ast.FUNC_GREATER_OR_EQUAL, 1, 2, false)
	helperComparison(t, ast.FUNC_GREATER_OR_EQUAL, 1, 1, true)
}

func TestComparison_comparisonFunction_less(t *testing.T) {
	helperComparison(t, ast.FUNC_LESS, 2, 1, false)
	helperComparison(t, ast.FUNC_LESS, 1, 2, true)
	helperComparison(t, ast.FUNC_LESS, 1, 1, false)
}

func TestComparison_comparisonFunction_less_or_equal(t *testing.T) {
	helperComparison(t, ast.FUNC_LESS_OR_EQUAL, 2, 1, false)
	helperComparison(t, ast.FUNC_LESS_OR_EQUAL, 1, 2, true)
	helperComparison(t, ast.FUNC_LESS_OR_EQUAL, 1, 1, true)
}

func TestComparison_comparisonFunction_mixed_int_float_false(t *testing.T) {
	r, errs := NewComparison(ast.FUNC_GREATER).Evaluate(ast.Arguments{Args: []any{(1), float64(2)}})
	assert.Empty(t, errs)
	assert.Equal(t, r, false)
}

func TestComparison_fail(t *testing.T) {
	_, errs := NewComparison(ast.FUNC_ADD).Evaluate(ast.Arguments{Args: []any{"toto", false}})
	if assert.Len(t, errs, 2) {
		assert.ErrorIs(t, errs[0], ast.ErrArgumentMustBeIntOrFloat)
		assert.ErrorIs(t, errs[1], ast.ErrArgumentMustBeIntOrFloat)
	}
}

func TestComparison_wrongnumber_of_argument(t *testing.T) {
	_, errs := NewComparison(ast.FUNC_ADD).Evaluate(ast.Arguments{Args: []any{nil}})
	if assert.Len(t, errs, 1) {
		assert.ErrorIs(t, errs[0], ast.ErrWrongNumberOfArgument)
	}
}

func TestComparison_required(t *testing.T) {
	_, errs := NewComparison(ast.FUNC_ADD).Evaluate(ast.Arguments{Args: []any{4, nil}})
	if assert.Len(t, errs, 1) {
		assert.ErrorIs(t, errs[0], ast.ErrArgumentRequired)
	}
}
