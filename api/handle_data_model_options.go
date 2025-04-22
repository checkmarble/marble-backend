package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
)

func handleGetDataModelOptions(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tableId := c.Param("tableID")
		orgId, err := utils.OrganizationIdFromRequest(c.Request)

		if presentError(ctx, c, err) {
			return
		}

		uc := usecasesWithCreds(ctx, uc)
		dataModelUsecase := uc.NewDataModelUseCase()

		opts, err := dataModelUsecase.GetDataModelOptions(ctx, orgId, tableId)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptDataModelOptions(opts))
	}
}

func handleSetDataModelOptions(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		tableId := c.Param("tableID")
		orgId, err := utils.OrganizationIdFromRequest(c.Request)

		if presentError(ctx, c, err) {
			return
		}

		var payload dto.UpdateDataModelOptionsInput

		if err := c.ShouldBindBodyWithJSON(&payload); err != nil {
			presentError(ctx, c, err)
			return
		}

		req := models.UpdateDataModelOptionsRequest{
			TableId:         tableId,
			DisplayedFields: payload.DisplayedFields,
			FieldOrder:      payload.FieldOrder,
		}

		uc := usecasesWithCreds(ctx, uc)
		dataModelUsecase := uc.NewDataModelUseCase()

		opts, err := dataModelUsecase.UpdateDataModelOptions(ctx, orgId, req)

		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.AdaptDataModelOptions(opts))
	}
}
