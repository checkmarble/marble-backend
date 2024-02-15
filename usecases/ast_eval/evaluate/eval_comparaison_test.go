package evaluate

import (
	"context"
	"testing"
	"time"

	"github.com/checkmarble/marble-backend/models/ast"

	"github.com/stretchr/testify/assert"
)

func helperFloatComparison(t *testing.T, f ast.Function, left, right float64, expected bool) {
	r, errs := NewComparison(f).Evaluate(context.TODO(), ast.Arguments{
		Args: []any{left, right},
	})
	assert.Empty(t, errs)
	assert.Equal(t, expected, r)
}

func helperIntComparison(t *testing.T, f ast.Function, left, right int, expected bool) {
	r, errs := NewComparison(f).Evaluate(context.TODO(), ast.Arguments{
		Args: []any{left, right},
	})
	assert.Empty(t, errs)
	assert.Equal(t, expected, r)
}

func helperTimeComparison(t *testing.T, f ast.Function, left, right time.Time, expected bool) {
	r, errs := NewComparison(f).Evaluate(context.TODO(), ast.Arguments{
		Args: []any{left, right},
	})
	assert.Empty(t, errs)
	assert.Equal(t, expected, r)
}

func TestComparison_comparisonFunction_greater_int(t *testing.T) {
	helperIntComparison(t, ast.FUNC_GREATER, 2, 1, true)
	helperIntComparison(t, ast.FUNC_GREATER, 1, 2, false)
	helperIntComparison(t, ast.FUNC_GREATER, 1, 1, false)

	date1 := time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC)
	date2 := time.Date(2023, time.February, 1, 0, 0, 0, 0, time.UTC)

	helperTimeComparison(t, ast.FUNC_GREATER, date2, date1, true)
	helperTimeComparison(t, ast.FUNC_GREATER, date1, date2, false)
	helperTimeComparison(t, ast.FUNC_GREATER, date1, date1, false)
}

func TestComparison_comparisonFunction_greater_or_equal_int(t *testing.T) {
	helperIntComparison(t, ast.FUNC_GREATER_OR_EQUAL, 2, 1, true)
	helperIntComparison(t, ast.FUNC_GREATER_OR_EQUAL, 1, 2, false)
	helperIntComparison(t, ast.FUNC_GREATER_OR_EQUAL, 1, 1, true)

	date1 := time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC)
	date2 := time.Date(2023, time.February, 1, 0, 0, 0, 0, time.UTC)

	helperTimeComparison(t, ast.FUNC_GREATER_OR_EQUAL, date2, date1, true)
	helperTimeComparison(t, ast.FUNC_GREATER_OR_EQUAL, date1, date2, false)
	helperTimeComparison(t, ast.FUNC_GREATER_OR_EQUAL, date1, date1, true)
}

func TestComparison_comparisonFunction_less(t *testing.T) {
	helperIntComparison(t, ast.FUNC_LESS, 2, 1, false)
	helperIntComparison(t, ast.FUNC_LESS, 1, 2, true)
	helperIntComparison(t, ast.FUNC_LESS, 1, 1, false)

	date1 := time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC)
	date2 := time.Date(2023, time.February, 1, 0, 0, 0, 0, time.UTC)

	helperTimeComparison(t, ast.FUNC_LESS, date1, date2, true)
	helperTimeComparison(t, ast.FUNC_LESS, date2, date1, false)
	helperTimeComparison(t, ast.FUNC_LESS, date1, date1, false)
}

func TestComparison_comparisonFunction_less_or_equal(t *testing.T) {
	helperIntComparison(t, ast.FUNC_LESS_OR_EQUAL, 2, 1, false)
	helperIntComparison(t, ast.FUNC_LESS_OR_EQUAL, 1, 2, true)
	helperIntComparison(t, ast.FUNC_LESS_OR_EQUAL, 1, 1, true)

	date1 := time.Date(2023, time.January, 1, 0, 0, 0, 0, time.UTC)
	date2 := time.Date(2023, time.February, 1, 0, 0, 0, 0, time.UTC)

	helperTimeComparison(t, ast.FUNC_LESS_OR_EQUAL, date1, date2, true)
	helperTimeComparison(t, ast.FUNC_LESS_OR_EQUAL, date2, date1, false)
	helperTimeComparison(t, ast.FUNC_LESS_OR_EQUAL, date1, date1, true)
}

func TestComparison_comparisonFunction_mixed_int_float_false(t *testing.T) {
	r, errs := NewComparison(ast.FUNC_GREATER).Evaluate(context.TODO(), ast.Arguments{
		Args: []any{1, float64(2)},
	})
	assert.Empty(t, errs)
	assert.Equal(t, r, false)
}

func TestComparison_comparisonFunction_Float(t *testing.T) {
	helperFloatComparison(t, ast.FUNC_GREATER, 0.3, 0.2+0.1, false)
	helperFloatComparison(t, ast.FUNC_GREATER, 0.3, 0.300001, false)
	helperFloatComparison(t, ast.FUNC_GREATER_OR_EQUAL, 0.3, 0.2+0.1, true)
	helperFloatComparison(t, ast.FUNC_GREATER_OR_EQUAL, 0.3, 0.300001, false)
	helperFloatComparison(t, ast.FUNC_LESS, 0.3, 0.2+0.1, false)
	helperFloatComparison(t, ast.FUNC_LESS, 0.3, 0.300001, true)
	helperFloatComparison(t, ast.FUNC_LESS_OR_EQUAL, 0.3, 0.2+0.1, true)
	helperFloatComparison(t, ast.FUNC_LESS_OR_EQUAL, 0.3, 0.300001, true)
}

func TestComparison_fail(t *testing.T) {
	_, errs := NewComparison(ast.FUNC_ADD).Evaluate(context.TODO(), ast.Arguments{
		Args: []any{"toto", false},
	})
	assert.Equal(t, len(errs), 1)
	assert.ErrorIs(t, errs[0], ast.ErrArgumentMustBeIntFloatOrTime)
}

func TestComparison_wrongnumber_of_argument(t *testing.T) {
	_, errs := NewComparison(ast.FUNC_ADD).Evaluate(context.TODO(), ast.Arguments{Args: []any{nil}})
	if assert.Len(t, errs, 1) {
		assert.ErrorIs(t, errs[0], ast.ErrWrongNumberOfArgument)
	}
}

func TestComparison_required(t *testing.T) {
	_, errs := NewComparison(ast.FUNC_ADD).Evaluate(context.TODO(), ast.Arguments{Args: []any{4, nil}})
	assert.Equal(t, len(errs), 1)
	assert.ErrorIs(t, errs[0], ast.ErrArgumentMustBeIntFloatOrTime)
}
