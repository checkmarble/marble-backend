package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/gin-gonic/gin"
	"github.com/guregu/null/v5"
)

func (api *API) handleGetTransferAlertSender(c *gin.Context) {
	alertId := c.Param("alert_id")

	usecase := api.UsecasesWithCreds(c.Request).NewTransferAlertsUsecase()
	alert, err := usecase.GetTransferAlert(c.Request.Context(), alertId, "sender")
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{"alert": dto.AdaptSenderTransferAlert(alert)})
}

func (api *API) handleGetTransferAlertBeneficiary(c *gin.Context) {
	alertId := c.Param("alert_id")

	usecase := api.UsecasesWithCreds(c.Request).NewTransferAlertsUsecase()
	alert, err := usecase.GetTransferAlert(c.Request.Context(), alertId, "beneficiary")
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{"alert": dto.AdaptBeneficiaryTransferAlert(alert)})
}

func (api *API) handleListTransferAlertsSender(c *gin.Context) {
	creds, _ := utils.CredentialsFromCtx(c.Request.Context())
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

	usecase := api.UsecasesWithCreds(c.Request).NewTransferAlertsUsecase()
	alerts, err := usecase.ListTransferAlerts(
		c.Request.Context(),
		creds.OrganizationId,
		partnerId,
		"sender",
		null.NewString(filters.TransferId, filters.TransferId != ""),
	)
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{"alerts": pure_utils.Map(alerts, dto.AdaptSenderTransferAlert)})
}

func (api *API) handleListTransferAlertsBeneficiary(c *gin.Context) {
	creds, _ := utils.CredentialsFromCtx(c.Request.Context())
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

	usecase := api.UsecasesWithCreds(c.Request).NewTransferAlertsUsecase()
	alerts, err := usecase.ListTransferAlerts(
		c.Request.Context(),
		creds.OrganizationId,
		partnerId,
		"beneficiary",
		null.NewString(filters.TransferId, filters.TransferId != ""),
	)
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{"alerts": pure_utils.Map(alerts, dto.AdaptBeneficiaryTransferAlert)})
}

func (api *API) handleCreateTransferAlert(c *gin.Context) {
	var data dto.TransferAlertCreateBody
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(http.StatusBadRequest)
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

	c.JSON(http.StatusOK, gin.H{"alert": dto.AdaptSenderTransferAlert(alert)})
}

func (api *API) handleUpdateTransferAlertSender(c *gin.Context) {
	alertId := c.Param("alert_id")
	creds, _ := utils.CredentialsFromCtx(c.Request.Context())

	var data dto.TransferAlertUpdateBody
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(http.StatusBadRequest)
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

	c.JSON(http.StatusOK, gin.H{"alert": dto.AdaptSenderTransferAlert(alert)})
}

func (api *API) handleUpdateTransferAlertBeneficiary(c *gin.Context) {
	alertId := c.Param("alert_id")
	creds, _ := utils.CredentialsFromCtx(c.Request.Context())

	var data dto.TransferAlertUpdateBody
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewTransferAlertsUsecase()
	alert, err := usecase.UpdateTransferAlertAsBeneficiary(c.Request.Context(), alertId, models.TransferAlertUpdateBodyBeneficiary{
		Status: data.Status,
	}, creds.OrganizationId)
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{"alert": dto.AdaptBeneficiaryTransferAlert(alert)})
}
