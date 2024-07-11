package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/gin-gonic/gin"
)

func (api *API) handleCreateWebhook(c *gin.Context) {
	var data dto.WebhookCreateBody
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	creds, _ := utils.CredentialsFromCtx(c.Request.Context())

	usecase := api.UsecasesWithCreds(c.Request).NewWebhooksUsecase()

	err := usecase.CreateWebhook(c.Request.Context(), dto.AdaptWebhookCreate(
		creds.OrganizationId,
		creds.PartnerId,
		data,
	))
	if presentError(c, err) {
		return
	}

	c.Status(http.StatusOK)
}
