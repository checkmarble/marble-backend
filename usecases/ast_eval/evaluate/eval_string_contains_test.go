package evaluate_test

import (
	"testing"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/usecases/ast_eval/evaluate"

	"github.com/stretchr/testify/assert"
)

func TestString_Contains_true(t *testing.T) {
	result, errs := evaluate.NewStringContains(ast.FUNC_STRING_CONTAINS).Evaluate(ast.Arguments{Args: []any{"abc", "ab"}})
	assert.Empty(t, errs)
	assert.Equal(t, true, result)
}

func TestString_Contains_false(t *testing.T) {
	result, errs := evaluate.NewStringContains(ast.FUNC_STRING_CONTAINS).Evaluate(ast.Arguments{Args: []any{"abc", "cd"}})
	assert.Empty(t, errs)
	assert.Equal(t, false, result)
}

func TestString_Contains_wrong_number_of_arguments(t *testing.T) {
	_, errs := evaluate.NewStringContains(ast.FUNC_STRING_CONTAINS).Evaluate(ast.Arguments{Args: []any{"abc"}})
	assert.NotEmpty(t, errs)
	assert.ErrorIs(t, errs[0], ast.ErrWrongNumberOfArgument)
}

func TestString_Contains_wrong_type_of_arguments(t *testing.T) {
	_, errs := evaluate.NewStringContains(ast.FUNC_STRING_CONTAINS).Evaluate(ast.Arguments{Args: []any{"abc", 1}})
	assert.NotEmpty(t, errs)
	assert.ErrorIs(t, errs[0], ast.ErrArgumentMustBeString)
}

func TestString_Contains_case_insensitive(t *testing.T) {
	result, errs := evaluate.NewStringContains(ast.FUNC_STRING_CONTAINS).Evaluate(ast.Arguments{Args: []any{"abc", "AB"}})
	assert.Empty(t, errs)
	assert.Equal(t, true, result)
}

func TestString_Not_Contains_true(t *testing.T) {
	result, errs := evaluate.NewStringContains(ast.FUNC_STRING_NOT_CONTAIN).Evaluate(ast.Arguments{Args: []any{"abc", "cd"}})
	assert.Empty(t, errs)
	assert.Equal(t, true, result)
}

func TestString_Not_Contains_false(t *testing.T) {
	result, errs := evaluate.NewStringContains(ast.FUNC_STRING_NOT_CONTAIN).Evaluate(ast.Arguments{Args: []any{"abc", "ab"}})
	assert.Empty(t, errs)
	assert.Equal(t, false, result)
}
