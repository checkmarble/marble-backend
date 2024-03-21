package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
)

func (api *API) handleTransferCheck(c *gin.Context) {
	usecase := api.UsecasesWithCreds(c.Request).NewTransferCheckUsecase()

	orgId, err := utils.OrgIDFromCtx(c.Request.Context(), c.Request)
	if presentError(c, err) {
		return
	}

	var data dto.TransferCheckCreateBody
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	transferCheck, err := usecase.TransferCheck(c.Request.Context(), orgId, dto.AdaptTransferCheckCreateBody(data))
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, dto.AdaptTransferCheckResultDto(transferCheck))
}
