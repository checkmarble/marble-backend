package ast_eval

import (
	"marble/marble-backend/models/ast"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEval(t *testing.T) {
	environment := NewAstEvaluationEnvironment()
	root := ast.NewAstCompareBalance()
	evaluation := EvaluateAst(environment, root)
	assert.NoError(t, evaluation.EvaluationError)
	assert.Equal(t, true, evaluation.ReturnValue)
}

func TestEvalAndOrFunction(t *testing.T) {
	environment := NewAstEvaluationEnvironment()

	evaluation := EvaluateAst(environment, ast.NewAstAndTrue())
	assert.NoError(t, evaluation.EvaluationError)
	assert.Equal(t, true, evaluation.ReturnValue)

	evaluation = EvaluateAst(environment, ast.NewAstAndFalse())
	assert.NoError(t, evaluation.EvaluationError)
	assert.Equal(t, false, evaluation.ReturnValue)

	evaluation = EvaluateAst(environment, ast.NewAstOrTrue())
	assert.NoError(t, evaluation.EvaluationError)
	assert.Equal(t, true, evaluation.ReturnValue)

	evaluation = EvaluateAst(environment, ast.NewAstOrFalse())
	assert.NoError(t, evaluation.EvaluationError)
	assert.Equal(t, false, evaluation.ReturnValue)

}
