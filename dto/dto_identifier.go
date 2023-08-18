package dto

import (
	"marble/marble-backend/models/ast"
)

type IdentifierDto struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Node        NodeDto `json:"node"`
}

func AdaptIdentifierDto(identifier ast.Identifier) (IdentifierDto, error) {
	nodeDto, err := AdaptNodeDto(identifier.Node)
	if err != nil {
		return IdentifierDto{}, err
	}
	return IdentifierDto{
		Name:        identifier.Name,
		Description: identifier.Description,
		Node:        nodeDto,
	}, nil
}
