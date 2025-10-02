package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/checkmarble/marble-backend/utils"
	"github.com/gin-gonic/gin"
)

func handleOidcTokenExchange(uc usecases.Usecases, cfg infra.OidcConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		logger := utils.LoggerFromContext(ctx)

		tokens, err := uc.NewOidcUsecase().ExchangeToken(ctx, cfg, c.Request)

		if presentError(ctx, c, err) {
			logger.ErrorContext(ctx, "could not exchange code for OIDC tokens", "error", err.Error())
			c.Status(http.StatusUnauthorized)
			return
		}

		c.JSON(http.StatusOK, dto.AdaptOidcTokens(tokens))
	}
}
