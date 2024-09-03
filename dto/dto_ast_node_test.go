package dto

import (
	"encoding/json"
	"testing"

	"github.com/checkmarble/marble-backend/models/ast"

	"github.com/stretchr/testify/assert"
)

func TestAdaptNodeDto(t *testing.T) {
	root := ast.Node{Function: ast.FUNC_GREATER}.
		AddChild(ast.Node{Constant: 1}).
		AddNamedChild("named", ast.Node{Constant: 2})

	dto, err := AdaptNodeDto(root)

	assert.NoError(t, err)
	assert.Equal(t,
		NodeDto{
			Name:          ">",
			Children:      []NodeDto{{Constant: 1, Children: []NodeDto{}, NamedChildren: map[string]NodeDto{}}},
			NamedChildren: map[string]NodeDto{"named": {Constant: 2, Children: []NodeDto{}, NamedChildren: map[string]NodeDto{}}},
		},
		dto,
	)
}

func TestAdaptASTNode(t *testing.T) {
	dto := NodeDto{
		Name:     "+",
		Children: []NodeDto{{Constant: 1, Children: []NodeDto{}, NamedChildren: map[string]NodeDto{}}},
		NamedChildren: map[string]NodeDto{
			"named": {Constant: 2, Children: []NodeDto{}, NamedChildren: map[string]NodeDto{}},
		},
	}

	node, err := AdaptASTNode(dto)

	assert.NoError(t, err)
	assert.Equal(t,
		ast.Node{Function: ast.FUNC_ADD}.
			AddChild(ast.NewNodeConstant(1)).
			AddNamedChild("named", ast.NewNodeConstant(2)),
		node,
	)
}

func TestAstToJson(t *testing.T) {
	node := ast.NewAstCompareBalance()

	dto, err := AdaptNodeDto(node)
	assert.NoError(t, err)

	serialized, err := json.Marshal(dto)
	assert.NoError(t, err)
	assert.NotEmpty(t, serialized)
}
