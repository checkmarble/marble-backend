package evaluate_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/checkmarble/marble-backend/models/ast"
	"github.com/checkmarble/marble-backend/usecases/ast_eval/evaluate"

	"github.com/stretchr/testify/assert"
)

func TestIsInListArgs(t *testing.T) {
	_, errs := evaluate.NewStringInList(ast.FUNC_ADD).Evaluate(context.TODO(), ast.Arguments{Args: []any{"test3", []string{"test1", "test2", "test3"}}})
	if assert.Len(t, errs, 1) {
		assert.Contains(t, errs[0].Error(), fmt.Sprintf("StringInList does not support %s function", ast.FUNC_ADD.DebugString()))

	}

	_, errs = evaluate.NewStringInList(ast.FUNC_IS_IN_LIST).Evaluate(context.TODO(), ast.Arguments{Args: []any{[]string{"test1", "test2", "test3"}, "test3"}})
	if assert.Len(t, errs, 2) {
		assert.Contains(t, errs[0].Error(), "can't promote argument")
		assert.Contains(t, errs[1].Error(), "can't promote argument")
	}

}

func TestIsInList(t *testing.T) {
	r, errs := evaluate.NewStringInList(ast.FUNC_IS_IN_LIST).Evaluate(context.TODO(), ast.Arguments{Args: []any{"test3", []string{"test1", "test2", "test3"}}})
	assert.Empty(t, errs)
	assert.True(t, r.(bool))

	r, errs = evaluate.NewStringInList(ast.FUNC_IS_IN_LIST).Evaluate(context.TODO(), ast.Arguments{Args: []any{"test4", []string{"test1", "test2", "test3"}}})
	assert.Empty(t, errs)
	assert.False(t, r.(bool))
}

func TestIsNotInList(t *testing.T) {
	r, errs := evaluate.NewStringInList(ast.FUNC_IS_NOT_IN_LIST).Evaluate(context.TODO(), ast.Arguments{Args: []any{"test4", []string{"test1", "test2", "test3"}}})
	assert.Empty(t, errs)
	assert.True(t, r.(bool))

	r, errs = evaluate.NewStringInList(ast.FUNC_IS_NOT_IN_LIST).Evaluate(context.TODO(), ast.Arguments{Args: []any{"test3", []string{"test1", "test2", "test3"}}})
	assert.Empty(t, errs)
	assert.False(t, r.(bool))

}
