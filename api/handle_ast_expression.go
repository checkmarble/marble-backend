package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models/ast"
)

func (api *API) handleAvailableFunctions(c *gin.Context) {
	functions := make(map[string]dto.FuncAttributesDto)

	for f, attributes := range ast.FuncAttributesMap {
		if f == ast.FUNC_CONSTANT || f == ast.FUNC_UNDEFINED {
			continue
		}
		functions[attributes.AstName] = dto.AdaptFuncAttributesDto(attributes)
	}

	c.JSON(http.StatusOK, gin.H{
		"functions": functions,
	})
}
