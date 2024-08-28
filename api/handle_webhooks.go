package api

import (
	"fmt"
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/guregu/null/v5"

	"github.com/gin-gonic/gin"
)

func handleListWebhooks(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		creds, found := utils.CredentialsFromCtx(c.Request.Context())
		if !found {
			presentError(c, fmt.Errorf("no credentials in context"))
			return
		}

		usecase := usecasesWithCreds(c.Request, uc).NewWebhooksUsecase()

		webhooks, err := usecase.ListWebhooks(c.Request.Context(), creds.OrganizationId, null.StringFromPtr(creds.PartnerId))
		if presentError(c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"webhooks": pure_utils.Map(webhooks, dto.AdaptWebhook),
		})
	}
}

func handleRegisterWebhook(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
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

		usecase := usecasesWithCreds(c.Request, uc).NewWebhooksUsecase()

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
}

func handleGetWebhook(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		webhookId := c.Param("webhook_id")

		creds, found := utils.CredentialsFromCtx(c.Request.Context())
		if !found {
			presentError(c, fmt.Errorf("no credentials in context"))
			return
		}

		usecase := usecasesWithCreds(c.Request, uc).NewWebhooksUsecase()

		webhook, err := usecase.GetWebhook(c.Request.Context(),
			creds.OrganizationId,
			null.StringFromPtr(creds.PartnerId),
			webhookId)
		if presentError(c, err) {
			return
		}

		c.JSON(http.StatusCreated, gin.H{"webhook": dto.AdaptWebhookWithSecret(webhook)})
	}
}

func handleDeleteWebhook(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		webhookId := c.Param("webhook_id")

		creds, found := utils.CredentialsFromCtx(c.Request.Context())
		if !found {
			presentError(c, fmt.Errorf("no credentials in context"))
			return
		}

		usecase := usecasesWithCreds(c.Request, uc).NewWebhooksUsecase()

		err := usecase.DeleteWebhook(c.Request.Context(),
			creds.OrganizationId,
			null.StringFromPtr(creds.PartnerId),
			webhookId)
		if presentError(c, err) {
			return
		}

		c.Status(http.StatusNoContent)
	}
}

func handleUpdateWebhook(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
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

		usecase := usecasesWithCreds(c.Request, uc).NewWebhooksUsecase()

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
}
