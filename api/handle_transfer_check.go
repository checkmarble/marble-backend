package api

import (
	"fmt"
	"net/http"

	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func handleCreateTransfer(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		usecase := usecasesWithCreds(ctx, uc).NewTransferCheckUsecase()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		creds, _ := utils.CredentialsFromCtx(ctx)
		partnerId := creds.PartnerId

		var data dto.TransferCreateBody
		if err := c.ShouldBindJSON(&data); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		transferCheck, err := usecase.CreateTransfer(ctx, orgId, partnerId, dto.AdaptTransferCreateBody(data))
		var fieldValidationError models.FieldValidationError
		if errors.As(err, &fieldValidationError) {
			c.JSON(http.StatusBadRequest, gin.H{"errors": fieldValidationError})
			return
		} else if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"transfer": dto.AdaptTransferCheckResultDto(transferCheck)})
	}
}

func handleUpdateTransfer(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.Param("transfer_id")

		usecase := usecasesWithCreds(ctx, uc).NewTransferCheckUsecase()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var data dto.TransferUpdateBody
		if err := c.ShouldBindJSON(&data); err != nil {
			fmt.Println(err)
			c.Status(http.StatusBadRequest)
			return
		}

		transferCheck, err := usecase.UpdateTransfer(ctx, orgId, id, models.TransferUpdateBody(data))
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"transfer": dto.AdaptTransferCheckResultDto(transferCheck)})
	}
}

type TransferFilters struct {
	TransferId string `form:"transfer_id" binding:"required"`
}

func handleQueryTransfers(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var filters TransferFilters
		if err := c.ShouldBind(&filters); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewTransferCheckUsecase()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		creds, _ := utils.CredentialsFromCtx(ctx)
		var partnerId string
		if creds.PartnerId != nil {
			partnerId = *creds.PartnerId
		}

		transferCheck, err := usecase.QueryTransfers(ctx, orgId, partnerId, filters.TransferId)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"transfers": pure_utils.Map(transferCheck, dto.AdaptTransferCheckResultDto)})
	}
}

func handleGetTransfer(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.Param("transfer_id")

		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewTransferCheckUsecase()
		transferCheck, err := usecase.GetTransfer(ctx, organizationId, id)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"transfer": dto.AdaptTransferCheckResultDto(transferCheck)})
	}
}

func handleScoreTransfer(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		id := c.Param("transfer_id")

		usecase := usecasesWithCreds(ctx, uc).NewTransferCheckUsecase()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		transferCheck, err := usecase.ScoreTransfer(ctx, orgId, id)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"transfer": dto.AdaptTransferCheckResultDto(transferCheck)})
	}
}
