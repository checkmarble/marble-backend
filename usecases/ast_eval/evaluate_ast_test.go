package ast_eval

import (
	"testing"

	"marble/marble-backend/models/ast"
	"marble/marble-backend/usecases/ast_eval/evaluate"

	"github.com/stretchr/testify/assert"
)

func TestEval(t *testing.T) {

	payload := map[string]any{
		"balance": 96,
	}
	environment := NewAstEvaluationEnvironment()
	environment.AddEvaluator(ast.FUNC_VARIABLE, evaluate.Variable{Variables: payload})

	root := ast.NewAstCompareBalance()
	result, err := EvaluateAst(environment, root)
	assert.NoError(t, err)

	assert.Equal(t, true, result)
}
