package ast_eval
import (
	"testing"
	"marble/marble-backend/models/ast"
	"github.com/stretchr/testify/assert"
)

func TestEval(t *testing.T) {
	environment := NewAstEvaluationEnvironment()
	root := ast.NewAstCompareBalance()
	result, err := EvaluateAst(environment, root)
	assert.NoError(t, err)
	assert.Equal(t, true, result)
}