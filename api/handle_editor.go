package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
)

func handleGetEditorIdentifiers(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		scenarioID := c.Param("scenario_id")

		usecase := usecasesWithCreds(ctx, uc).AstExpressionUsecase()
		result, err := usecase.EditorIdentifiers(ctx, scenarioID)

		if presentError(ctx, c, err) {
			return
		}

		databaseNodes, err := pure_utils.MapErr(result.DatabaseAccessors, dto.AdaptNodeDto)
		if presentError(ctx, c, err) {
			return
		}
		payloadbaseNodes, err := pure_utils.MapErr(result.PayloadAccessors, dto.AdaptNodeDto)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"database_accessors": databaseNodes,
			"payload_accessors":  payloadbaseNodes,
		})
	}
}

func handleGetEditorOperators(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		usecase := usecasesWithCreds(ctx, uc).AstExpressionUsecase()
		result := usecase.EditorOperators()

		var functions []dto.FuncAttributesDto

		for _, attributes := range result.OperatorAccessors {
			functions = append(functions, dto.AdaptFuncAttributesDto(attributes))
		}
		c.JSON(http.StatusOK, gin.H{
			"operators_accessors": functions,
		})
	}
}
