package api

import (
	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
)

func (api *API) handleGetTransferAlertSender(c *gin.Context) {
	alertId := c.Param("alert_id")

	usecase := api.UsecasesWithCreds(c.Request).NewTransferAlertsUsecase()
	alert, err := usecase.GetTransferAlert(c.Request.Context(), alertId, "sender")
	if presentError(c, err) {
		return
	}

	c.JSON(200, gin.H{"alert": dto.AdaptSenderTransferAlert(alert)})
}

func (api *API) handleGetTransferAlertReceiver(c *gin.Context) {
	alertId := c.Param("alert_id")

	usecase := api.UsecasesWithCreds(c.Request).NewTransferAlertsUsecase()
	alert, err := usecase.GetTransferAlert(c.Request.Context(), alertId, "receiver")
	if presentError(c, err) {
		return
	}

	c.JSON(200, gin.H{"alert": dto.AdaptBeneficiaryTransferAlert(alert)})
}

func (api *API) handleListTransferAlertsSender(c *gin.Context) {
	creds, _ := utils.CredentialsFromCtx(c.Request.Context())
	var partnerId string
	if creds.PartnerId != nil {
		partnerId = *creds.PartnerId
	}

	usecase := api.UsecasesWithCreds(c.Request).NewTransferAlertsUsecase()
	alerts, err := usecase.ListTransferAlerts(c.Request.Context(), creds.OrganizationId, partnerId, "sender")
	if presentError(c, err) {
		return
	}

	c.JSON(200, gin.H{"alerts": pure_utils.Map(alerts, dto.AdaptSenderTransferAlert)})
}

func (api *API) handleListTransferAlertsReceiver(c *gin.Context) {
	creds, _ := utils.CredentialsFromCtx(c.Request.Context())
	var partnerId string
	if creds.PartnerId != nil {
		partnerId = *creds.PartnerId
	}

	usecase := api.UsecasesWithCreds(c.Request).NewTransferAlertsUsecase()
	alerts, err := usecase.ListTransferAlerts(c.Request.Context(), creds.OrganizationId, partnerId, "receiver")
	if presentError(c, err) {
		return
	}

	c.JSON(200, gin.H{"alerts": pure_utils.Map(alerts, dto.AdaptBeneficiaryTransferAlert)})
}

func (api *API) handleCreateTransferAlert(c *gin.Context) {
	var data dto.TransferAlertCreateBody
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(400)
		return
	}

	creds, _ := utils.CredentialsFromCtx(c.Request.Context())
	var partnerId string
	if creds.PartnerId != nil {
		partnerId = *creds.PartnerId
	}

	usecase := api.UsecasesWithCreds(c.Request).NewTransferAlertsUsecase()

	alert, err := usecase.CreateTransferAlert(c.Request.Context(), models.TransferAlertCreateBody{
		TransferId:         data.TransferId,
		OrganizationId:     creds.OrganizationId,
		SenderPartnerId:    partnerId,
		Message:            data.Message,
		TransferEndToEndId: data.TransferEndToEndId,
		BeneficiaryIban:    data.BeneficiaryIban,
		SenderIban:         data.SenderIban,
	})
	if presentError(c, err) {
		return
	}

	c.JSON(200, gin.H{"alert": dto.AdaptSenderTransferAlert(alert)})
}

func (api *API) handleUpdateTransferAlertSender(c *gin.Context) {
	alertId := c.Param("alert_id")
	creds, _ := utils.CredentialsFromCtx(c.Request.Context())

	var data dto.TransferAlertUpdateBody
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(400)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewTransferAlertsUsecase()
	alert, err := usecase.UpdateTransferAlertAsSender(c.Request.Context(), alertId, models.TransferAlertUpdateBodySender{
		Message:            data.Message,
		TransferEndToEndId: data.TransferEndToEndId,
		BeneficiaryIban:    data.BeneficiaryIban,
		SenderIban:         data.SenderIban,
	}, creds.OrganizationId)
	if presentError(c, err) {
		return
	}

	c.JSON(200, gin.H{"alert": dto.AdaptSenderTransferAlert(alert)})
}

func (api *API) handleUpdateTransferAlertReceiver(c *gin.Context) {
	alertId := c.Param("alert_id")
	creds, _ := utils.CredentialsFromCtx(c.Request.Context())

	var data dto.TransferAlertUpdateBody
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(400)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewTransferAlertsUsecase()
	alert, err := usecase.UpdateTransferAlertAsReceiver(c.Request.Context(), alertId, models.TransferAlertUpdateBodyReceiver{
		Status: data.Status,
	}, creds.OrganizationId)
	if presentError(c, err) {
		return
	}

	c.JSON(200, gin.H{"alert": dto.AdaptBeneficiaryTransferAlert(alert)})
}
