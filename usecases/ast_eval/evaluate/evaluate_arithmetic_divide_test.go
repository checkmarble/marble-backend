package evaluate

import (
	"marble/marble-backend/models"
	"marble/marble-backend/models/ast"
	"testing"

	"github.com/stretchr/testify/assert"
)

const TEN_DIVIDE_BY_THREE = float64(3.3333333333333335)

func TestNewArithmetic_divide_float64(t *testing.T) {

	r, err := ArithmeticDivide{}.Evaluate(ast.Arguments{Args: []any{10.0, 3}})
	assert.NoError(t, err)
	assert.Equal(t, r, TEN_DIVIDE_BY_THREE)
}

func TestNewArithmetic_divide_int(t *testing.T) {

	// check that no integer division is performed
	r, err := ArithmeticDivide{}.Evaluate(ast.Arguments{Args: []any{10, 3}})
	assert.NoError(t, err)
	assert.Equal(t, r, TEN_DIVIDE_BY_THREE)
}

func TestNewArithmeticFunction_float_divide_by_zero(t *testing.T) {
	_, err := ArithmeticDivide{}.Evaluate(ast.Arguments{Args: []any{1.0, 0.0}})
	assert.ErrorIs(t, err, models.DivisionByZeroError)
}
