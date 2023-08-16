package evaluate

import (
	"marble/marble-backend/models/ast"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEqual_Evaluate_int(t *testing.T) {

	r, err := Equal{}.Evaluate(ast.Arguments{Args: []any{1, 1}})
	assert.NoError(t, err)
	assert.Equal(t, r, true)
}

func TestEqual_Evaluate_float(t *testing.T) {

	r, err := Equal{}.Evaluate(ast.Arguments{Args: []any{22.3, 22.3}})
	assert.NoError(t, err)
	assert.Equal(t, r, true)
}

func TestEqual_Evaluate_string(t *testing.T) {

	r, err := Equal{}.Evaluate(ast.Arguments{Args: []any{"a", "a"}})
	assert.NoError(t, err)
	assert.Equal(t, r, true)
}

func TestEqual_Evaluate_bool(t *testing.T) {

	r, err := Equal{}.Evaluate(ast.Arguments{Args: []any{false, false}})
	assert.NoError(t, err)
	assert.Equal(t, r, true)
}
