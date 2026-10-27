package api

import (
	"fmt"
	"net/http"

	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type tenantOIDCDiscoveryBody struct {
	Domain string `json:"domain" binding:"required"`
}

type tenantOIDCDiscoveryResponse struct {
	TenantId    uuid.UUID         `json:"tenant_id"`
	Issuer      string            `json:"issuer"`
	ClientId    string            `json:"client_id"`
	RedirectUri string            `json:"redirect_uri"`
	Scopes      []string          `json:"scopes"`
	ExtraParams map[string]string `json:"extra_params"`
}

func adaptTenantOIDCDiscovery(config models.TenantOIDCConfiguration, marbleAppURL string) tenantOIDCDiscoveryResponse {
	return tenantOIDCDiscoveryResponse{
		TenantId: config.TenantId, Issuer: config.Issuer, ClientId: config.ClientId,
		RedirectUri: fmt.Sprintf("%s/oidc/callback", marbleAppURL), Scopes: config.Scopes, ExtraParams: config.ExtraParams,
	}
}

func handlePostTenantOIDCDiscovery(uc usecases.Usecases, marbleAppURL string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var body tenantOIDCDiscoveryBody
		if err := c.ShouldBindJSON(&body); err != nil {
			presentError(ctx, c, errors.Wrap(models.BadParameterError, err.Error()))
			return
		}
		configuration, err := uc.NewTenantOIDCDiscoveryUsecase().ByDomain(ctx, body.Domain)
		if presentError(ctx, c, err) {
			return
		}
		c.JSON(http.StatusOK, adaptTenantOIDCDiscovery(configuration, marbleAppURL))
	}
}
