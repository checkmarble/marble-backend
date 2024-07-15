package api

import (
	"fmt"
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/guregu/null/v5"

	"github.com/gin-gonic/gin"
)

func (api *API) handleListWebhooks(c *gin.Context) {
	creds, found := utils.CredentialsFromCtx(c.Request.Context())
	if !found {
		presentError(c, fmt.Errorf("no credentials in context"))
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewWebhooksUsecase()

	webhooks, err := usecase.ListWebhooks(c.Request.Context(), creds.OrganizationId, null.StringFromPtr(creds.PartnerId))
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"webhooks": pure_utils.Map(webhooks, dto.AdaptWebhook),
	})
}

func (api *API) handleRegisterWebhook(c *gin.Context) {
	var data dto.WebhookRegisterBody
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	creds, found := utils.CredentialsFromCtx(c.Request.Context())
	if !found {
		presentError(c, fmt.Errorf("no credentials in context"))
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewWebhooksUsecase()

	err := usecase.RegisterWebhook(c.Request.Context(), dto.AdaptWebhookRegister(
		creds.OrganizationId,
		creds.PartnerId,
		data,
	))
	if presentError(c, err) {
		return
	}

	c.Status(http.StatusOK)
}

func (api *API) handleDeleteWebhook(c *gin.Context) {
	webhookId := c.Param("webhook_id")

	creds, found := utils.CredentialsFromCtx(c.Request.Context())
	if !found {
		presentError(c, fmt.Errorf("no credentials in context"))
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewWebhooksUsecase()

	err := usecase.DeleteWebhook(c.Request.Context(),
		creds.OrganizationId,
		null.StringFromPtr(creds.PartnerId),
		webhookId)
	if presentError(c, err) {
		return
	}

	c.Status(http.StatusNoContent)
}
