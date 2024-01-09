package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/utils"
)

func (api *API) handleGetEditorIdentifiers(c *gin.Context) {
	scenarioID := c.Param("scenario_id")

	usecase := api.UsecasesWithCreds(c.Request).AstExpressionUsecase()
	result, err := usecase.EditorIdentifiers(c.Request.Context(), scenarioID)

	if presentError(c.Writer, c.Request, err, c) {
		return
	}

	databaseNodes, err := utils.MapErr(result.DatabaseAccessors, dto.AdaptNodeDto)
	if presentError(c.Writer, c.Request, err, c) {
		return
	}
	payloadbaseNodes, err := utils.MapErr(result.PayloadAccessors, dto.AdaptNodeDto)
	if presentError(c.Writer, c.Request, err, c) {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"database_accessors": databaseNodes,
		"payload_accessors":  payloadbaseNodes,
	})
}

func (api *API) handleGetEditorOperators(c *gin.Context) {
	usecase := api.UsecasesWithCreds(c.Request).AstExpressionUsecase()
	result := usecase.EditorOperators()

	var functions []dto.FuncAttributesDto

	for _, attributes := range result.OperatorAccessors {
		functions = append(functions, dto.AdaptFuncAttributesDto(attributes))
	}
	c.JSON(http.StatusOK, gin.H{
		"operators_accessors": functions,
	})
}
