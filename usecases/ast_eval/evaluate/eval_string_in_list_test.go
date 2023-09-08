package evaluate_test

import (
	"fmt"
	"marble/marble-backend/models/ast"
	"marble/marble-backend/usecases/ast_eval/evaluate"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsInListArgs(t *testing.T) {
	_, errs := evaluate.NewStringInList(ast.FUNC_ADD).Evaluate(ast.Arguments{Args: []any{"test3", []string{"test1", "test2", "test3"}}})
	if assert.Len(t, errs, 1) {
		assert.Equal(t, fmt.Errorf("StringInList does not support %s function", ast.FUNC_ADD.DebugString()), errs[0])
	}

	_, errs = evaluate.NewStringInList(ast.FUNC_IS_IN_LIST).Evaluate(ast.Arguments{Args: []any{[]string{"test1", "test2", "test3"}, "test3"}})
	if assert.Len(t, errs, 2) {
		assert.Contains(t, errs[0].Error(), "can't promote argument")
		assert.Contains(t, errs[1].Error(), "can't promote argument")
	}

}

func TestIsInList(t *testing.T) {
	r, errs := evaluate.NewStringInList(ast.FUNC_IS_IN_LIST).Evaluate(ast.Arguments{Args: []any{"test3", []string{"test1", "test2", "test3"}}})
	assert.Empty(t, errs)
	assert.True(t, r.(bool))

	r, errs = evaluate.NewStringInList(ast.FUNC_IS_IN_LIST).Evaluate(ast.Arguments{Args: []any{"test4", []string{"test1", "test2", "test3"}}})
	assert.Empty(t, errs)
	assert.False(t, r.(bool))
}

func TestIsNotInList(t *testing.T) {
	r, errs := evaluate.NewStringInList(ast.FUNC_IS_NOT_IN_LIST).Evaluate(ast.Arguments{Args: []any{"test4", []string{"test1", "test2", "test3"}}})
	assert.Empty(t, errs)
	assert.True(t, r.(bool))

	r, errs = evaluate.NewStringInList(ast.FUNC_IS_NOT_IN_LIST).Evaluate(ast.Arguments{Args: []any{"test3", []string{"test1", "test2", "test3"}}})
	assert.Empty(t, errs)
	assert.False(t, r.(bool))

}
