package evaluate

import (
	"marble/marble-backend/models/ast"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNot_Evaluate_true(t *testing.T) {
	result, err := Not{}.Evaluate(ast.Arguments{Args: []any{true}})
	assert.NoError(t, err)
	assert.Equal(t, false, result)
}

func TestNot_Evaluate_false(t *testing.T) {
	result, err := Not{}.Evaluate(ast.Arguments{Args: []any{false}})
	assert.NoError(t, err)
	assert.Equal(t, true, result)
}
