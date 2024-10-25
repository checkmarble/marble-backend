package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/guregu/null/v5"
)

func handleGetTransferAlertSender(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		alertId := c.Param("alert_id")

		usecase := usecasesWithCreds(ctx, uc).NewTransferAlertsUsecase()
		alert, err := usecase.GetTransferAlert(ctx, alertId, "sender")
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"alert": dto.AdaptSenderTransferAlert(alert)})
	}
}

func handleGetTransferAlertBeneficiary(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		alertId := c.Param("alert_id")

		usecase := usecasesWithCreds(ctx, uc).NewTransferAlertsUsecase()
		alert, err := usecase.GetTransferAlert(ctx, alertId, "beneficiary")
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"alert": dto.AdaptBeneficiaryTransferAlert(alert)})
	}
}

func handleListTransferAlertsSender(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, _ := utils.CredentialsFromCtx(ctx)
		var partnerId string
		if creds.PartnerId != nil {
			partnerId = *creds.PartnerId
		}

		var filters struct {
			TransferId string `form:"transfer_id"`
		}
		if err := c.ShouldBind(&filters); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewTransferAlertsUsecase()
		alerts, err := usecase.ListTransferAlerts(
			ctx,
			creds.OrganizationId,
			partnerId,
			"sender",
			null.NewString(filters.TransferId, filters.TransferId != ""),
		)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"alerts": pure_utils.Map(alerts, dto.AdaptSenderTransferAlert)})
	}
}

func handleListTransferAlertsBeneficiary(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, _ := utils.CredentialsFromCtx(ctx)
		var partnerId string
		if creds.PartnerId != nil {
			partnerId = *creds.PartnerId
		}

		var filters struct {
			TransferId string `form:"transfer_id"`
		}
		if err := c.ShouldBind(&filters); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewTransferAlertsUsecase()
		alerts, err := usecase.ListTransferAlerts(
			ctx,
			creds.OrganizationId,
			partnerId,
			"beneficiary",
			null.NewString(filters.TransferId, filters.TransferId != ""),
		)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"alerts": pure_utils.Map(alerts, dto.AdaptBeneficiaryTransferAlert)})
	}
}

func handleCreateTransferAlert(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var data dto.TransferAlertCreateBody
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		creds, _ := utils.CredentialsFromCtx(ctx)
		var partnerId string
		if creds.PartnerId != nil {
			partnerId = *creds.PartnerId
		}

		usecase := usecasesWithCreds(ctx, uc).NewTransferAlertsUsecase()

		alert, err := usecase.CreateTransferAlert(ctx, models.TransferAlertCreateBody{
			TransferId:         data.TransferId,
			OrganizationId:     creds.OrganizationId,
			SenderPartnerId:    partnerId,
			Message:            data.Message,
			TransferEndToEndId: data.TransferEndToEndId,
			BeneficiaryIban:    data.BeneficiaryIban,
			SenderIban:         data.SenderIban,
		})
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"alert": dto.AdaptSenderTransferAlert(alert)})
	}
}

func handleUpdateTransferAlertSender(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		alertId := c.Param("alert_id")
		creds, _ := utils.CredentialsFromCtx(ctx)

		var data dto.TransferAlertUpdateAsSenderBody
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewTransferAlertsUsecase()
		alert, err := usecase.UpdateTransferAlertAsSender(ctx, alertId, models.TransferAlertUpdateBodySender{
			Message:            data.Message,
			TransferEndToEndId: data.TransferEndToEndId,
			BeneficiaryIban:    data.BeneficiaryIban,
			SenderIban:         data.SenderIban,
		}, creds.OrganizationId)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"alert": dto.AdaptSenderTransferAlert(alert)})
	}
}

func handleUpdateTransferAlertBeneficiary(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		alertId := c.Param("alert_id")
		creds, _ := utils.CredentialsFromCtx(ctx)

		var data dto.TransferAlertUpdateAsBeneficiaryBody
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewTransferAlertsUsecase()
		alert, err := usecase.UpdateTransferAlertAsBeneficiary(ctx, alertId, models.TransferAlertUpdateBodyBeneficiary{
			Status: data.Status,
		}, creds.OrganizationId)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"alert": dto.AdaptBeneficiaryTransferAlert(alert)})
	}
}
