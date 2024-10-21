package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/pure_utils"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
)

func handleListApiKeys(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewApiKeyUseCase()
		apiKeys, err := usecase.ListApiKeys(c.Request.Context(), organizationId)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"api_keys": pure_utils.Map(apiKeys, dto.AdaptApiKeyDto),
		})
	}
}

func handlePostApiKey(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		organizationId, err := utils.OrganizationIdFromRequest(c.Request)
		if presentError(ctx, c, err) {
			return
		}

		var input dto.CreateApiKeyBody
		if presentError(ctx, c, c.ShouldBindJSON(&input)) {
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewApiKeyUseCase()
		apiKey, err := usecase.CreateApiKey(c.Request.Context(), models.CreateApiKeyInput{
			OrganizationId: organizationId,
			Description:    input.Description,
			Role:           models.RoleFromString(input.Role),
		})
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusCreated, gin.H{"api_key": dto.AdaptCreatedApiKeyDto(apiKey)})
	}
}

type ApiKeyUriInput struct {
	ApiKeyId string `uri:"api_key_id" binding:"required,uuid"`
}

func handleRevokeApiKey(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var apiKeyUriInput ApiKeyUriInput
		if err := c.ShouldBindUri(&apiKeyUriInput); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewApiKeyUseCase()
		err := usecase.DeleteApiKey(c.Request.Context(), apiKeyUriInput.ApiKeyId)
		if presentError(ctx, c, err) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}
