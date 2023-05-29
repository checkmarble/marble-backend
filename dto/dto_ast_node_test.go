package dto

import (
	"encoding/json"
	"fmt"
	"marble/marble-backend/models/ast"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdaptNodeDto(t *testing.T) {

	root := ast.Node{Function: ast.FUNC_GREATER}.
		AddChild(ast.Node{Constant: 1}).
		AddNamedChild("named", ast.Node{Constant: 2})

	dto, err := AdaptNodeDto(root)

	assert.NoError(t, err)
	assert.Equal(t,
		dto,
		NodeDto{
			FuncName:      ">",
			Children:      []NodeDto{{Constant: 1}},
			NamedChildren: map[string]NodeDto{"named": {Constant: 2}},
		},
	)
}

func TestAdaptASTNode(t *testing.T) {

	dto := NodeDto{
		FuncName: "+",
		Children: []NodeDto{{Constant: 1}},
		NamedChildren: map[string]NodeDto{
			"named": {Constant: 2},
		},
	}

	node, err := AdaptASTNode(dto)

	assert.NoError(t, err)
	assert.Equal(t,
		node,
		ast.Node{Function: ast.FUNC_PLUS}.
			AddChild(ast.Node{Constant: 1}).
			AddNamedChild("named", ast.Node{Constant: 2}),
	)
}

func TestAstToJson(t *testing.T) {

	node := ast.NewAstCompareBalance()

	dto, err := AdaptNodeDto(node)
	assert.NoError(t, err)

	serialized, err := json.Marshal(dto)
	assert.NoError(t, err)
	assert.NotEmpty(t, serialized)
	fmt.Println(string(serialized))
}
