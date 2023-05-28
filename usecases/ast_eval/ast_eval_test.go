package ast_eval

import (
	"encoding/json"
	"fmt"
	"testing"

	"marble/marble-backend/models/ast"

	"github.com/stretchr/testify/assert"
)

func NewTestExpression() ast.Node {
	// DatabaseAccess("account", "balance") + 4 > 100
	return ast.Node{Function: ast.FUNC_GREATER}.
		AddChild(ast.Node{Function: ast.FUNC_PLUS}.
			AddChild(ast.NewNodeDatabaseAccess("account", "balance")).
			AddChild(ast.Node{Constant: 4}),
		).
		AddChild(ast.Node{Constant: 100})
}

func TestEval(t *testing.T) {

	root := NewTestExpression()
	result, err := EvalAst(root)
	assert.NoError(t, err)

	assert.Equal(t, false, result.(bool))
}

func TestAstToJson(t *testing.T) {

	root := NewTestExpression()

	serialized, err := json.Marshal(root)
	assert.NoError(t, err)
	assert.NotEmpty(t, serialized)
	fmt.Println(string(serialized))
}

func TestRenderAstToJavascript(t *testing.T) {

	root := NewTestExpression()

	js, err := RenderAstToJavascript(root, "ruleName")
	assert.NoError(t, err)
	fmt.Println(js)
}
