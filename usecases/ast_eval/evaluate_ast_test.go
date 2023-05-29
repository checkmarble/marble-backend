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
	inject := NewEvaluatorInjection()
	inject.AddEvaluator(ast.FUNC_READ_PAYLOAD, evaluate.ReadPayload{Payload: payload})

	root := ast.NewAstCompareBalance()
	result, err := EvaluateAst(&inject, root)
	assert.NoError(t, err)

	assert.Equal(t, true, result)
}
