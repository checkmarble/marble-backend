package dto

import (
	"marble/marble-backend/models/ast"
)

type IdentifierDto struct {
	Node NodeDto `json:"node"`
}

func AdaptIdentifierDto(identifier ast.Identifier) (IdentifierDto, error) {
	nodeDto, err := AdaptNodeDto(identifier.Node)
	if err != nil {
		return IdentifierDto{}, err
	}
	return IdentifierDto{
		Node: nodeDto,
	}, nil
}
