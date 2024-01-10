package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/utils"
)

func (api *API) handleGetApiKey(c *gin.Context) {
	organizationId, err := utils.OrganizationIdFromRequest(c.Request)
	if presentError(c, err) {
		return
	}

	usecase := api.UsecasesWithCreds(c.Request).NewOrganizationUseCase()
	apiKeys, err := usecase.GetApiKeysOfOrganization(c.Request.Context(), organizationId)
	if presentError(c, err) {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"api_keys": utils.Map(apiKeys, dto.AdaptApiKeyDto),
	})
}
