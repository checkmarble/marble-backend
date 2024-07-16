package api

import (
	"fmt"
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
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
	creds, found := utils.CredentialsFromCtx(c.Request.Context())
	if !found {
		presentError(c, fmt.Errorf("no credentials in context"))
		return
	}

	var data dto.WebhookRegisterBody
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewWebhooksUsecase()

	webhook, err := usecase.RegisterWebhook(c.Request.Context(),
		creds.OrganizationId,
		null.StringFromPtr(creds.PartnerId),
		models.WebhookRegister{
			EventTypes:        data.EventTypes,
			Url:               data.Url,
			HttpTimeout:       data.HttpTimeout,
			RateLimit:         data.RateLimit,
			RateLimitDuration: data.RateLimitDuration,
		})
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusCreated, gin.H{"webhook": dto.AdaptWebhookWithSecret(webhook)})
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

func (api *API) handleUpdateWebhook(c *gin.Context) {
	webhookId := c.Param("webhook_id")

	creds, found := utils.CredentialsFromCtx(c.Request.Context())
	if !found {
		presentError(c, fmt.Errorf("no credentials in context"))
		return
	}

	var data dto.WebhookUpdateBody
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewWebhooksUsecase()

	webhook, err := usecase.UpdateWebhook(c.Request.Context(),
		creds.OrganizationId,
		null.StringFromPtr(creds.PartnerId),
		webhookId,
		models.WebhookUpdate{
			EventTypes:        data.EventTypes,
			Url:               data.Url,
			HttpTimeout:       data.HttpTimeout,
			RateLimit:         data.RateLimit,
			RateLimitDuration: data.RateLimitDuration,
		})
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{"webhook": dto.AdaptWebhook(webhook)})
}
