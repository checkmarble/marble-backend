package evaluate

import (
	"marble/marble-backend/models/ast"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNot_Evaluate_true(t *testing.T) {
	result, errs := Not{}.Evaluate(ast.Arguments{Args: []any{true}})
	assert.Empty(t, errs)
	assert.Equal(t, false, result)
}

func TestNot_Evaluate_false(t *testing.T) {
	result, errs := Not{}.Evaluate(ast.Arguments{Args: []any{false}})
	assert.Empty(t, errs)
	assert.Equal(t, true, result)
}
