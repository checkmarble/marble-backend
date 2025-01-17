package dto

import (
	"github.com/checkmarble/marble-backend/models/ast"
)

type FuncAttributesDto struct {
	Name string `json:"name"`
}

func AdaptFuncAttributesDto(attributes ast.FuncAttributes) FuncAttributesDto {
	return FuncAttributesDto{
		Name: attributes.AstName,
	}
}
