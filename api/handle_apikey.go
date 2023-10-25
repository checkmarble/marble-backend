package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/utils"
)

type apiKeysUseCase interface {
	GetApiKeysOfOrganization(ctx context.Context, organizationID string) ([]models.ApiKey, error)
}

type ApiKeysHandler struct {
	useCase apiKeysUseCase
}

func (h *ApiKeysHandler) GetApiKeys(c *gin.Context) {
	organizationID, err := utils.OrganizationIdFromRequest(c.Request)
	if presentError(c.Writer, c.Request, err) {
		return
	}

	keys, err := h.useCase.GetApiKeysOfOrganization(c.Request.Context(), organizationID)
	if err != nil {
		_ = c.Error(fmt.Errorf("useCase.GetApiKeysOfOrganization error: %w", err))
		c.Status(http.StatusInternalServerError)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"api_keys": utils.Map(keys, dto.AdaptApiKeyDto),
	})
}

func NewApiKeysHandler(u apiKeysUseCase) *ApiKeysHandler {
	return &ApiKeysHandler{
		useCase: u,
	}
}
