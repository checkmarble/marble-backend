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
		ctx := c.Request.Context()
		creds, found := utils.CredentialsFromCtx(ctx)
		if !found {
			presentError(ctx, c, fmt.Errorf("no credentials in context"))
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewWebhooksUsecase()

		webhooks, err := usecase.ListWebhooks(ctx, creds.OrganizationId, null.StringFromPtr(creds.PartnerId))
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"webhooks": pure_utils.Map(webhooks, dto.AdaptWebhook),
		})
	}
}

func handleRegisterWebhook(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		creds, found := utils.CredentialsFromCtx(ctx)
		if !found {
			presentError(ctx, c, fmt.Errorf("no credentials in context"))
			return
		}

		var data dto.WebhookRegisterBody
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewWebhooksUsecase()

		webhook, err := usecase.RegisterWebhook(ctx,
			creds.OrganizationId,
			null.StringFromPtr(creds.PartnerId),
			models.WebhookRegister{
				EventTypes:        data.EventTypes,
				Url:               data.Url,
				HttpTimeout:       data.HttpTimeout,
				RateLimit:         data.RateLimit,
				RateLimitDuration: data.RateLimitDuration,
			})
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, gin.H{"webhook": dto.AdaptWebhookWithSecret(webhook)})
	}
}

func handleGetWebhook(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		webhookId := c.Param("webhook_id")

		creds, found := utils.CredentialsFromCtx(ctx)
		if !found {
			presentError(ctx, c, fmt.Errorf("no credentials in context"))
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewWebhooksUsecase()

		webhook, err := usecase.GetWebhook(ctx,
			creds.OrganizationId,
			null.StringFromPtr(creds.PartnerId),
			webhookId)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusCreated, gin.H{"webhook": dto.AdaptWebhookWithSecret(webhook)})
	}
}

func handleDeleteWebhook(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		webhookId := c.Param("webhook_id")

		creds, found := utils.CredentialsFromCtx(ctx)
		if !found {
			presentError(ctx, c, fmt.Errorf("no credentials in context"))
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewWebhooksUsecase()

		err := usecase.DeleteWebhook(ctx,
			creds.OrganizationId,
			null.StringFromPtr(creds.PartnerId),
			webhookId)
		if presentError(ctx, c, err) {
			return
		}

		c.Status(http.StatusNoContent)
	}
}

func handleUpdateWebhook(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		webhookId := c.Param("webhook_id")

		creds, found := utils.CredentialsFromCtx(ctx)
		if !found {
			presentError(ctx, c, fmt.Errorf("no credentials in context"))
			return
		}

		var data dto.WebhookUpdateBody
		if err := c.ShouldBindJSON(&data); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewWebhooksUsecase()

		webhook, err := usecase.UpdateWebhook(ctx,
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
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{"webhook": dto.AdaptWebhook(webhook)})
	}
}
