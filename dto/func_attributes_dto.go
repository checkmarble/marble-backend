package dto

import (
	"marble/marble-backend/models/ast"
)

type FuncAttributesDto struct {
	AstName           string   `json:"name"`
	NumberOfArguments int      `json:"number_of_arguments,omitempty"`
	NamedArguments    []string `json:"named_arguments,omitempty"`
}

func AdaptFuncAttributesDto(attributes ast.FuncAttributes) FuncAttributesDto {
	return FuncAttributesDto{
		AstName:           attributes.AstName,
		NumberOfArguments: attributes.NumberOfArguments,
		NamedArguments:    attributes.NamedArguments,
	}
}
