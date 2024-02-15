package evaluate

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/models/ast"

	"github.com/stretchr/testify/assert"
)

const TEN_DIVIDE_BY_THREE = float64(3.3333333333333335)

func TestNewArithmetic_divide_float64(t *testing.T) {
	r, errs := ArithmeticDivide{}.Evaluate(context.TODO(), ast.Arguments{Args: []any{10.0, 3}})
	assert.Empty(t, errs)
	assert.Equal(t, r, TEN_DIVIDE_BY_THREE)
}

func TestNewArithmetic_divide_int(t *testing.T) {
	// check that no integer division is performed
	r, errs := ArithmeticDivide{}.Evaluate(context.TODO(), ast.Arguments{Args: []any{10, 3}})
	assert.Empty(t, errs)
	assert.Equal(t, r, TEN_DIVIDE_BY_THREE)
}

func TestNewArithmeticFunction_float_divide_by_zero(t *testing.T) {
	_, errs := ArithmeticDivide{}.Evaluate(context.TODO(), ast.Arguments{Args: []any{1.0, 0.0}})
	if assert.Len(t, errs, 1) {
		assert.ErrorIs(t, errs[0], models.DivisionByZeroError)
	}
}
