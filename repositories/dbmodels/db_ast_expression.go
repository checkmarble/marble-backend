package dbmodels

import (
	"encoding/json"
	"fmt"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models/ast"
)

func SerializeFormulaAstExpression(formulaAstExpression *ast.Node) (*[]byte, error) {
	if formulaAstExpression == nil {
		return nil, nil
	}

	nodeDto, err := dto.AdaptNodeDto(*formulaAstExpression)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal rule formula ast expression: %w", err)
	}

	serialized, err := json.Marshal(nodeDto)
	return &serialized, err
}

func AdaptSerializedAstExpression(serializedAstExpression []byte) (*ast.Node, error) {
	if len(serializedAstExpression) == 0 {
		return nil, nil
	}

	var nodeDto dto.NodeDto
	if err := json.Unmarshal(serializedAstExpression, &nodeDto); err != nil {
		return nil, err
	}

	node, err := dto.AdaptASTNode(nodeDto)
	if err != nil {
		return nil, err
	}
	return &node, nil
}
