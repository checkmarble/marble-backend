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
		usecase := usecasesWithCreds(c.Request, uc).NewTransferCheckUsecase()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(c, err) {
			return
		}

		creds, _ := utils.CredentialsFromCtx(c.Request.Context())
		partnerId := creds.PartnerId

		var data dto.TransferCreateBody
		if err := c.ShouldBindJSON(&data); err != nil {
			presentError(c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}

		transferCheck, err := usecase.CreateTransfer(c.Request.Context(), orgId, partnerId, dto.AdaptTransferCreateBody(data))
		var fieldValidationError models.FieldValidationError
		if errors.As(err, &fieldValidationError) {
			c.JSON(http.StatusBadRequest, gin.H{"errors": fieldValidationError})
			return
		} else if presentError(c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"transfer": dto.AdaptTransferCheckResultDto(transferCheck)})
	}
}

func handleUpdateTransfer(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		id := c.Param("transfer_id")

		usecase := usecasesWithCreds(c.Request, uc).NewTransferCheckUsecase()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
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
}

type TransferFilters struct {
	TransferId string `form:"transfer_id" binding:"required"`
}

func handleQueryTransfers(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		var filters TransferFilters
		if err := c.ShouldBind(&filters); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(c.Request, uc).NewTransferCheckUsecase()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(c, err) {
			return
		}

		creds, _ := utils.CredentialsFromCtx(c.Request.Context())
		var partnerId string
		if creds.PartnerId != nil {
			partnerId = *creds.PartnerId
		}

		transferCheck, err := usecase.QueryTransfers(c.Request.Context(), orgId, partnerId, filters.TransferId)
		if presentError(c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"transfers": pure_utils.Map(transferCheck, dto.AdaptTransferCheckResultDto)})
	}
}

func handleGetTransfer(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		id := c.Param("transfer_id")

		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(c, err) {
			return
		}

		usecase := usecasesWithCreds(c.Request, uc).NewTransferCheckUsecase()
		transferCheck, err := usecase.GetTransfer(c.Request.Context(), organizationId, id)
		if presentError(c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"transfer": dto.AdaptTransferCheckResultDto(transferCheck)})
	}
}

func handleScoreTransfer(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		id := c.Param("transfer_id")

		usecase := usecasesWithCreds(c.Request, uc).NewTransferCheckUsecase()

		orgId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(c, err) {
			return
		}

		transferCheck, err := usecase.ScoreTransfer(c.Request.Context(), orgId, id)
		if presentError(c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"transfer": dto.AdaptTransferCheckResultDto(transferCheck)})
	}
}
