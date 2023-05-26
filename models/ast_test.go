package models

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func NewTestExpression() *ASTNode {
	// DatabaseAccess("account", "balance") + 4 > 100
	return NewASTSuperior().
		AddChild(
			NewASTNodePlus().
				AddChild(
					NewASTNodeDatabaseAccess("account", "balance"),
				).
				AddChild(
					NewASTNodeNumber(4),
				),
		).
		AddChild(
			NewASTNodeNumber(100),
		)
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
