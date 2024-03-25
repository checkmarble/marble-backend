package api

import (
	"fmt"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
)

func (api *API) handleTransferCheck(c *gin.Context) {
	usecase := api.UsecasesWithCreds(c.Request).NewTransferCheckUsecase()

	orgId, err := utils.OrgIDFromCtx(c.Request.Context(), c.Request)
	if presentError(c, err) {
		return
	}

	creds, _ := utils.CredentialsFromCtx(c.Request.Context())
	partnerId := creds.PartnerId
	if partnerId == "" {
		presentError(c, errors.Wrap(
			models.ForbiddenError,
			"API key with a valid partner_id is required"),
		)
		return
	}

	var data dto.TransferCreateBody
	if err := c.ShouldBindJSON(&data); err != nil {
		presentError(c, errors.Wrap(models.BadParameterError, err.Error()))
		return
	}

	transferCheck, err := usecase.CreateTransfer(c.Request.Context(), orgId, partnerId, dto.AdaptTransferCreateBody(data))
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{"transfer": dto.AdaptTransferCheckResultDto(transferCheck)})
}

func (api *API) handleUpdateTransfer(c *gin.Context) {
	id := c.Param("transfer_id")

	usecase := api.UsecasesWithCreds(c.Request).NewTransferCheckUsecase()

	orgId, err := utils.OrgIDFromCtx(c.Request.Context(), c.Request)
	if presentError(c, err) {
		return
	}

	var data dto.TransferUpdateBody
	if err := c.ShouldBindJSON(&data); err != nil {
		fmt.Println(err)
		c.Status(http.StatusBadRequest)
		return
	}

	transferCheck, err := usecase.UpdateTransfer(c.Request.Context(), orgId, id, models.TransferUpdateBody(data))
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{"transfer": dto.AdaptTransferCheckResultDto(transferCheck)})
}

type TransferFilters struct {
	TransferId string `form:"transfer_id" binding:"required"`
}

func (api *API) handleQueryTransfers(c *gin.Context) {
	var filters TransferFilters
	if err := c.ShouldBind(&filters); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewTransferCheckUsecase()

	orgId, err := utils.OrgIDFromCtx(c.Request.Context(), c.Request)
	if presentError(c, err) {
		return
	}

	transferCheck, err := usecase.QueryTransfers(c.Request.Context(), orgId, filters.TransferId)
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{"transfers": pure_utils.Map(transferCheck, dto.AdaptTransferCheckResultDto)})
}

func (api *API) handleGetTransfer(c *gin.Context) {
	id := c.Param("transfer_id")

	organizationId, err := utils.OrgIDFromCtx(c.Request.Context(), c.Request)
	if presentError(c, err) {
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewTransferCheckUsecase()
	transferCheck, err := usecase.GetTransfer(c.Request.Context(), organizationId, id)
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{"transfer": dto.AdaptTransferCheckResultDto(transferCheck)})
}

func (api *API) handleScoreTransfer(c *gin.Context) {
	id := c.Param("transfer_id")

	usecase := api.UsecasesWithCreds(c.Request).NewTransferCheckUsecase()

	orgId, err := utils.OrgIDFromCtx(c.Request.Context(), c.Request)
	if presentError(c, err) {
		return
	}

	transferCheck, err := usecase.ScoreTransfer(c.Request.Context(), orgId, id)
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{"transfer": dto.AdaptTransferCheckResultDto(transferCheck)})
}
