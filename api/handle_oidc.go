package api

import (
	"context"
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/models"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/cockroachdb/errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// A tenant-configured provider resolving to a reserved IP is a configuration error, not a server fault.
func hideReservedIP(ctx context.Context, err error) error {
	if errors.Is(err, infra.ErrReservedIP) {
		utils.LoggerFromContext(ctx).WarnContext(ctx, "tenant OIDC provider resolves to a reserved IP", "error", err.Error())
		return errors.Wrap(models.BadParameterError, "OIDC provider resolves to a disallowed IP address")
	}
	return err
}

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
				presentError(ctx, c, hideReservedIP(ctx, err))
				return
			}
		}
		if oidcConfig.Provider == nil {
			presentError(ctx, c, models.BadParameterError)
			return
		}

		tokens, err := uc.NewOidcUsecase().ExchangeToken(ctx, oidcConfig, c.Request)
		err = hideReservedIP(ctx, err)

		if presentError(ctx, c, err) {
			logger.ErrorContext(ctx, "could not exchange code for OIDC tokens", "error", err.Error())
			return
		}

		c.JSON(http.StatusOK, dto.AdaptOidcTokens(tokens))
	}
}
