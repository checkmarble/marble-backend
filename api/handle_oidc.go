package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func handleOidcTokenExchange(uc usecases.Usecases, cfg Configuration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		logger := utils.LoggerFromContext(ctx)

		oidcConfig := cfg.OidcConfig
		if tenantId := c.GetHeader("x-marble-tenant-id"); tenantId != "" {
			parsedTenantId, err := uuid.Parse(tenantId)
			if err != nil {
				presentError(ctx, c, models.BadParameterError)
				return
			}
			if cfg.TenantOIDCRegistry == nil {
				presentError(ctx, c, models.NotFoundError)
				return
			}
			oidcConfig, err = cfg.TenantOIDCRegistry.Get(ctx, parsedTenantId)
			if err != nil {
				presentError(ctx, c, err)
				return
			}
		}
		if oidcConfig.Provider == nil {
			presentError(ctx, c, models.BadParameterError)
			return
		}

		tokens, err := uc.NewOidcUsecase().ExchangeToken(ctx, oidcConfig, c.Request)

		if presentError(ctx, c, err) {
			logger.ErrorContext(ctx, "could not exchange code for OIDC tokens", "error", err.Error())
			c.Status(http.StatusUnauthorized)
			return
		}

		c.JSON(http.StatusOK, dto.AdaptOidcTokens(tokens))
	}
}
