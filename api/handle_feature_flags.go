package api

import (
	"net/http"

	"github.com/checkmarble/marble-backend/dto"
	"github.com/checkmarble/marble-backend/usecases"
	"github.com/gin-gonic/gin"
)

func handleListFeatureFlags(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		usecase := usecasesWithCreds(ctx, uc).NewFeatureFlagUsecase()

		flags, err := usecase.ListFeatureFlags(ctx)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.FeatureFlagsDto{Flags: flags})
	}
}

func handleReplaceFeatureFlags(uc usecases.Usecases) func(c *gin.Context) {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		var payload dto.FeatureFlagsDto

		if err := c.ShouldBindJSON(&payload); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		usecase := usecasesWithCreds(ctx, uc).NewFeatureFlagUsecase()

		flags, err := usecase.ReplaceFeatureFlags(ctx, payload.Flags)
		if presentError(ctx, c, err) {
			return
		}

		c.JSON(http.StatusOK, dto.FeatureFlagsDto{Flags: flags})
	}
}
