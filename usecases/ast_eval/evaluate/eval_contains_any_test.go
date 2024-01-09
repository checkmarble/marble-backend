package evaluate_test

import (
	"context"
	"testing"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/usecases/ast_eval/evaluate"
	"github.com/stretchr/testify/assert"
)

func TestContains_Any_true(t *testing.T) {
	result, errs := evaluate.NewContainsAny(ast.FUNC_CONTAINS_ANY).Evaluate(context.TODO(), ast.Arguments{Args: []any{"abc", []any{"ab", "cd"}}})
	assert.Empty(t, errs)
	assert.Equal(t, true, result)
}

func TestContains_Any_false(t *testing.T) {
	result, errs := evaluate.NewContainsAny(ast.FUNC_CONTAINS_ANY).Evaluate(context.TODO(), ast.Arguments{Args: []any{"abc", []any{"cd"}}})
	assert.Empty(t, errs)
	assert.Equal(t, false, result)
}

func TestContains_Any_wrong_number_of_arguments(t *testing.T) {
	_, errs := evaluate.NewContainsAny(ast.FUNC_CONTAINS_ANY).Evaluate(context.TODO(), ast.Arguments{Args: []any{"abc"}})
	assert.NotEmpty(t, errs)
	assert.ErrorIs(t, errs[0], ast.ErrWrongNumberOfArgument)
}

func TestContains_Any_wrong_type_of_arguments(t *testing.T) {
	_, errs := evaluate.NewContainsAny(ast.FUNC_CONTAINS_ANY).Evaluate(context.TODO(), ast.Arguments{Args: []any{"abc", 1}})
	assert.NotEmpty(t, errs)
	assert.ErrorIs(t, errs[0], ast.ErrArgumentMustBeList)
}

func TestContains_Any_case_insensitive(t *testing.T) {
	result, errs := evaluate.NewContainsAny(ast.FUNC_CONTAINS_ANY).Evaluate(context.TODO(), ast.Arguments{Args: []any{"abc", []any{"AB"}}})
	assert.Empty(t, errs)
	assert.Equal(t, true, result)
}

func TestContains_None_true(t *testing.T) {
	result, errs := evaluate.NewContainsAny(ast.FUNC_CONTAINS_NONE).Evaluate(context.TODO(), ast.Arguments{Args: []any{"abc", []any{"cd"}}})
	assert.Empty(t, errs)
	assert.Equal(t, true, result)
}

func TestContains_None_false(t *testing.T) {
	result, errs := evaluate.NewContainsAny(ast.FUNC_CONTAINS_NONE).Evaluate(context.TODO(), ast.Arguments{Args: []any{"abc", []any{"ab", "cd"}}})
	assert.Empty(t, errs)
	assert.Equal(t, false, result)
}
