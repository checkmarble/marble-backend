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
