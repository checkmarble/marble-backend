package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/utils"

	"github.com/gin-gonic/gin"
)

func (api *API) handleRegisterWebhook(c *gin.Context) {
	var data dto.WebhookRegisterBody
	if err := c.ShouldBindJSON(&data); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	creds, _ := utils.CredentialsFromCtx(c.Request.Context())

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
