package dto

import (
	"github.com/checkmarble/marble-backend/models/ast"
)

type FuncAttributesDto struct {
	Name              string   `json:"name"`
	NumberOfArguments int      `json:"number_of_arguments,omitempty"`
	NamedArguments    []string `json:"named_arguments,omitempty"`
}

func AdaptFuncAttributesDto(attributes ast.FuncAttributes) FuncAttributesDto {
	return FuncAttributesDto{
		Name:              attributes.AstName,
		NumberOfArguments: attributes.NumberOfArguments,
		NamedArguments:    attributes.NamedArguments,
	}
}
