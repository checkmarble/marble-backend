package api

import (
	"time"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/gin-gonic/gin"
)

func (api *API) handleGetTransferAlertSender(c *gin.Context) {
	alertId := c.Param("alert_id")

	alert := models.TransferAlert{
		Id:                 alertId,
		TransferId:         "transfer_id",
		SenderPartnerId:    "sender_partner_id",
		CreatedAt:          time.Now(),
		Status:             "status",
		Message:            "message",
		TransferEndToEndId: "transfer_end_to_end_id",
		BeneficiaryIban:    "beneficiary_iban",
		SenderIban:         "sender_iban",
	}

	c.JSON(200, gin.H{"alert": dto.AdaptSenderTransferAlert(alert)})
}

func (api *API) handleGetTransferAlertReceiver(c *gin.Context) {
	alertId := c.Param("alert_id")

	alert := models.TransferAlert{
		Id:                 alertId,
		TransferId:         "transfer_id",
		SenderPartnerId:    "sender_partner_id",
		CreatedAt:          time.Now(),
		Status:             "status",
		Message:            "message",
		TransferEndToEndId: "transfer_end_to_end_id",
		BeneficiaryIban:    "beneficiary_iban",
		SenderIban:         "sender_iban",
	}

	c.JSON(200, gin.H{"alert": dto.AdaptBeneficiaryTransferAlert(alert)})
}

func (api *API) handleListTransferAlertsSender(c *gin.Context) {
	alerts := []models.TransferAlert{
		{
			Id:                 "id",
			TransferId:         "transfer_id",
			SenderPartnerId:    "sender_partner_id",
			CreatedAt:          time.Now(),
			Status:             "status",
			Message:            "message",
			TransferEndToEndId: "transfer_end_to_end_id",
			BeneficiaryIban:    "beneficiary_iban",
			SenderIban:         "sender_iban",
		},
	}

	c.JSON(200, gin.H{"alerts": pure_utils.Map(alerts, dto.AdaptSenderTransferAlert)})
}

func (api *API) handleListTransferAlertsReceiver(c *gin.Context) {
	alerts := []models.TransferAlert{
		{
			Id:                 "id",
			TransferId:         "transfer_id",
			SenderPartnerId:    "sender_partner_id",
			CreatedAt:          time.Now(),
			Status:             "status",
			Message:            "message",
			TransferEndToEndId: "transfer_end_to_end_id",
			BeneficiaryIban:    "beneficiary_iban",
			SenderIban:         "sender_iban",
		},
	}

	c.JSON(200, gin.H{"alerts": pure_utils.Map(alerts, dto.AdaptBeneficiaryTransferAlert)})
}

func (api *API) handleCreateTransferAlert(c *gin.Context) {
	var data dto.TransferAlertCreateBody
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(400)
		return
	}

	alert := models.TransferAlert{
		Id:                 "id",
		TransferId:         data.TransferId,
		SenderPartnerId:    "qezfqzef",
		CreatedAt:          time.Now(),
		Status:             "unread",
		Message:            data.Message,
		TransferEndToEndId: data.TransferEndToEndId,
		BeneficiaryIban:    data.BeneficiaryIban,
		SenderIban:         data.SenderIban,
	}

	c.JSON(200, gin.H{"alert": dto.AdaptSenderTransferAlert(alert)})
}

func (api *API) handleUpdateTransferAlertSender(c *gin.Context) {
	alertId := c.Param("alert_id")

	var data dto.TransferAlertUpdateBody
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(400)
		return
	}

	alert := models.TransferAlert{
		Id:                 alertId,
		TransferId:         "transfer_id",
		SenderPartnerId:    "SenderPartnerId",
		CreatedAt:          time.Now(),
		Status:             "",
		Message:            data.Message.String,
		TransferEndToEndId: data.TransferEndToEndId.String,
		BeneficiaryIban:    data.BeneficiaryIban.String,
		SenderIban:         data.SenderIban.String,
	}

	c.JSON(200, gin.H{"alert": dto.AdaptSenderTransferAlert(alert)})
}

func (api *API) handleUpdateTransferAlertReceiver(c *gin.Context) {
	alertId := c.Param("alert_id")

	var data dto.TransferAlertUpdateBody
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(400)
		return
	}

	alert := models.TransferAlert{
		Id:                 alertId,
		CreatedAt:          time.Now(),
		Status:             data.Status.String,
		Message:            "",
		TransferEndToEndId: "",
		BeneficiaryIban:    "",
		SenderIban:         "",
	}

	c.JSON(200, gin.H{"alert": dto.AdaptBeneficiaryTransferAlert(alert)})
}
