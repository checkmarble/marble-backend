package ast_eval

import (
	"marble/marble-backend/models/ast"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEval(t *testing.T) {
	environment := NewAstEvaluationEnvironment()
	root := ast.NewAstCompareBalance()
	evaluation, ok := EvaluateAst(environment, root)
	assert.True(t, ok)
	assert.Len(t, evaluation.Errors, 0)
	assert.Equal(t, true, evaluation.ReturnValue)
}

func TestEvalUndefinedFunction(t *testing.T) {
	environment := NewAstEvaluationEnvironment()
	root := ast.Node{Function: ast.FUNC_UNDEFINED}
	evaluation, ok := EvaluateAst(environment, root)
	assert.False(t, ok)
	if assert.Len(t, evaluation.Errors, 1) {
		assert.ErrorIs(t, evaluation.Errors[0], ast.ErrUndefinedFunction)
	}
}

func TestEvalAndOrFunction(t *testing.T) {
	environment := NewAstEvaluationEnvironment()

	evaluation, ok := EvaluateAst(environment, NewAstAndTrue())
	assert.True(t, ok)
	assert.Len(t, evaluation.Errors, 0)
	assert.Equal(t, true, evaluation.ReturnValue)

	evaluation, ok = EvaluateAst(environment, NewAstAndFalse())
	assert.True(t, ok)
	assert.Len(t, evaluation.Errors, 0)
	assert.Equal(t, false, evaluation.ReturnValue)

	evaluation, ok = EvaluateAst(environment, NewAstOrTrue())
	assert.True(t, ok)
	assert.Len(t, evaluation.Errors, 0)
	assert.Equal(t, true, evaluation.ReturnValue)

	evaluation, ok = EvaluateAst(environment, NewAstOrFalse())
	assert.True(t, ok)
	assert.Len(t, evaluation.Errors, 0)
	assert.Equal(t, false, evaluation.ReturnValue)

}

func NewAstAndTrue() ast.Node {
	return ast.Node{Function: ast.FUNC_AND}.
		AddChild(ast.Node{Constant: true}).
		AddChild(ast.Node{Constant: true}).
		AddChild(ast.Node{Constant: true}).
		AddChild(ast.Node{Constant: true}).
		AddChild(ast.Node{Constant: true}).
		AddChild(ast.Node{Constant: true})
}

func NewAstAndFalse() ast.Node {
	return ast.Node{Function: ast.FUNC_AND}.
		AddChild(ast.Node{Constant: true}).
		AddChild(ast.Node{Constant: true}).
		AddChild(ast.Node{Constant: false}).
		AddChild(ast.Node{Constant: true}).
		AddChild(ast.Node{Constant: true}).
		AddChild(ast.Node{Constant: true})
}

func NewAstOrTrue() ast.Node {
	return ast.Node{Function: ast.FUNC_OR}.
		AddChild(ast.Node{Constant: false}).
		AddChild(ast.Node{Constant: false}).
		AddChild(ast.Node{Constant: true}).
		AddChild(ast.Node{Constant: false}).
		AddChild(ast.Node{Constant: false}).
		AddChild(ast.Node{Constant: false})
}

func NewAstOrFalse() ast.Node {
	return ast.Node{Function: ast.FUNC_OR}.
		AddChild(ast.Node{Constant: false}).
		AddChild(ast.Node{Constant: false}).
		AddChild(ast.Node{Constant: false}).
		AddChild(ast.Node{Constant: false}).
		AddChild(ast.Node{Constant: false}).
		AddChild(ast.Node{Constant: false})
}
